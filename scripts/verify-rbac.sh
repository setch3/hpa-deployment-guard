#!/bin/bash

# RBAC設定検証スクリプト
# k8s-deployment-hpa-validator用のRBAC権限を検証

set -euo pipefail

# 設定
SERVICE_ACCOUNT="k8s-deployment-hpa-validator"
NAMESPACE="default"

# 色付きログ出力
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# RBAC権限チェック関数
check_permission() {
    local resource=$1
    local verb=$2
    local api_group=${3:-""}
    local resource_name=${4:-""}
    
    local cmd="kubectl auth can-i $verb $resource"
    if [ -n "$api_group" ]; then
        cmd="$cmd --as=system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT"
    else
        cmd="$cmd --as=system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT"
    fi
    
    if [ -n "$resource_name" ]; then
        cmd="$cmd/$resource_name"
    fi
    
    if eval "$cmd" >/dev/null 2>&1; then
        log_info "✓ $verb $resource${resource_name:+/$resource_name} - 許可"
        return 0
    else
        log_error "✗ $verb $resource${resource_name:+/$resource_name} - 拒否"
        return 1
    fi
}

# ServiceAccountの存在確認
check_service_account() {
    log_info "ServiceAccountの確認中..."
    
    if kubectl get serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" >/dev/null 2>&1; then
        log_info "✓ ServiceAccount '$SERVICE_ACCOUNT' が存在します"
        
        # ServiceAccountの詳細表示
        log_debug "ServiceAccount詳細:"
        kubectl get serviceaccount "$SERVICE_ACCOUNT" -n "$NAMESPACE" -o yaml | grep -E "(name:|namespace:|automountServiceAccountToken:)"
    else
        log_error "✗ ServiceAccount '$SERVICE_ACCOUNT' が見つかりません"
        return 1
    fi
}

# ClusterRoleの存在確認
check_cluster_role() {
    log_info "ClusterRoleの確認中..."
    
    if kubectl get clusterrole "$SERVICE_ACCOUNT" >/dev/null 2>&1; then
        log_info "✓ ClusterRole '$SERVICE_ACCOUNT' が存在します"
        
        # ClusterRoleの権限表示
        log_debug "ClusterRole権限:"
        kubectl describe clusterrole "$SERVICE_ACCOUNT" | grep -A 20 "Rules:"
    else
        log_error "✗ ClusterRole '$SERVICE_ACCOUNT' が見つかりません"
        return 1
    fi
}

# ClusterRoleBindingの存在確認
check_cluster_role_binding() {
    log_info "ClusterRoleBindingの確認中..."
    
    if kubectl get clusterrolebinding "$SERVICE_ACCOUNT" >/dev/null 2>&1; then
        log_info "✓ ClusterRoleBinding '$SERVICE_ACCOUNT' が存在します"
        
        # ClusterRoleBindingの詳細表示
        log_debug "ClusterRoleBinding詳細:"
        kubectl get clusterrolebinding "$SERVICE_ACCOUNT" -o yaml | grep -E "(name:|roleRef:|subjects:)" -A 5
    else
        log_error "✗ ClusterRoleBinding '$SERVICE_ACCOUNT' が見つかりません"
        return 1
    fi
}

# 個別権限のテスト
test_permissions() {
    log_info "権限テストを実行中..."
    
    local failed=0
    
    # Deployments権限
    log_info "Deployments権限のテスト:"
    check_permission "deployments" "get" || ((failed++))
    check_permission "deployments" "list" || ((failed++))
    check_permission "deployments" "watch" || ((failed++))
    
    # HPA権限
    log_info "HPA権限のテスト:"
    check_permission "horizontalpodautoscalers" "get" || ((failed++))
    check_permission "horizontalpodautoscalers" "list" || ((failed++))
    check_permission "horizontalpodautoscalers" "watch" || ((failed++))
    
    # ValidatingAdmissionWebhook権限
    log_info "ValidatingAdmissionWebhook権限のテスト:"
    check_permission "validatingwebhookconfigurations" "get" || ((failed++))
    check_permission "validatingwebhookconfigurations" "list" || ((failed++))
    
    # Events権限
    log_info "Events権限のテスト:"
    check_permission "events" "create" || ((failed++))
    check_permission "events" "patch" || ((failed++))
    
    # API server接続確認権限（オプション）
    log_info "API server接続権限のテスト（オプション）:"
    if check_permission "" "get"; then
        log_info "✓ API server接続権限あり"
    else
        log_warn "⚠ API server接続権限なし（必須ではありません）"
    fi
    
    return $failed
}

# 実際のリソースアクセステスト
test_resource_access() {
    log_info "実際のリソースアクセステスト中..."
    
    local failed=0
    
    # Deployments一覧取得テスト
    if kubectl get deployments --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" >/dev/null 2>&1; then
        log_info "✓ Deployments一覧取得 - 成功"
    else
        log_error "✗ Deployments一覧取得 - 失敗"
        ((failed++))
    fi
    
    # HPA一覧取得テスト
    if kubectl get hpa --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" >/dev/null 2>&1; then
        log_info "✓ HPA一覧取得 - 成功"
    else
        log_error "✗ HPA一覧取得 - 失敗"
        ((failed++))
    fi
    
    # ValidatingAdmissionWebhook一覧取得テスト
    if kubectl get validatingwebhookconfigurations --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" >/dev/null 2>&1; then
        log_info "✓ ValidatingAdmissionWebhook一覧取得 - 成功"
    else
        log_error "✗ ValidatingAdmissionWebhook一覧取得 - 失敗"
        ((failed++))
    fi
    
    return $failed
}

# 不要な権限チェック（セキュリティ確認）
check_excessive_permissions() {
    log_info "不要な権限の確認中..."
    
    local warnings=0
    
    # 危険な権限のチェック
    local dangerous_permissions=(
        "create:deployments"
        "update:deployments"
        "delete:deployments"
        "create:horizontalpodautoscalers"
        "update:horizontalpodautoscalers"
        "delete:horizontalpodautoscalers"
        "create:secrets"
        "update:secrets"
        "delete:secrets"
        "*:*"
    )
    
    for perm in "${dangerous_permissions[@]}"; do
        local verb=$(echo "$perm" | cut -d: -f1)
        local resource=$(echo "$perm" | cut -d: -f2)
        
        if kubectl auth can-i "$verb" "$resource" --as="system:serviceaccount:$NAMESPACE:$SERVICE_ACCOUNT" >/dev/null 2>&1; then
            log_warn "⚠ 危険な権限が付与されています: $verb $resource"
            ((warnings++))
        fi
    done
    
    if [ $warnings -eq 0 ]; then
        log_info "✓ 不要な権限は付与されていません"
    fi
    
    return $warnings
}

# メイン処理
main() {
    log_info "RBAC設定検証を開始します..."
    log_info "ServiceAccount: $SERVICE_ACCOUNT"
    log_info "Namespace: $NAMESPACE"
    echo
    
    local total_failed=0
    
    # 基本的な存在確認
    check_service_account || ((total_failed++))
    echo
    
    check_cluster_role || ((total_failed++))
    echo
    
    check_cluster_role_binding || ((total_failed++))
    echo
    
    # 権限テスト
    test_permissions
    local perm_failed=$?
    ((total_failed += perm_failed))
    echo
    
    # リソースアクセステスト
    test_resource_access
    local access_failed=$?
    ((total_failed += access_failed))
    echo
    
    # セキュリティチェック
    check_excessive_permissions
    local security_warnings=$?
    echo
    
    # 結果サマリー
    log_info "=== 検証結果サマリー ==="
    if [ $total_failed -eq 0 ]; then
        log_info "✓ すべてのRBAC設定が正しく構成されています"
    else
        log_error "✗ $total_failed 個の問題が見つかりました"
    fi
    
    if [ $security_warnings -gt 0 ]; then
        log_warn "⚠ $security_warnings 個のセキュリティ警告があります"
    fi
    
    return $total_failed
}

# スクリプト実行
main "$@"