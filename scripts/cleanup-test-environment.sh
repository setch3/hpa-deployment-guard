#!/bin/bash

# テスト環境クリーンアップスクリプト
# E2Eテスト後の環境クリーンアップを実行します

set -euo pipefail

# 色付きログ用の定数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ログ関数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 設定
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TEST_NAMESPACE="hpa-validator-test"
WEBHOOK_NAME="k8s-deployment-hpa-validator"
KIND_CLUSTER_NAME="hpa-validator"

# クリーンアップ対象のリソース
CLEANUP_RESOURCES=(
    "deployments"
    "horizontalpodautoscalers"
    "pods"
    "services"
    "configmaps"
    "secrets"
)

# テスト用namespaceのクリーンアップ
cleanup_test_namespace() {
    log_info "テスト用namespace '$TEST_NAMESPACE' をクリーンアップ中..."
    
    if kubectl get namespace "$TEST_NAMESPACE" &>/dev/null; then
        # namespace内のリソースを個別に削除（高速化のため）
        for resource in "${CLEANUP_RESOURCES[@]}"; do
            log_info "$resource を削除中..."
            kubectl delete "$resource" --all -n "$TEST_NAMESPACE" --ignore-not-found=true --timeout=30s || {
                log_warning "$resource の削除に失敗しましたが、続行します"
            }
        done
        
        # namespace自体を削除
        kubectl delete namespace "$TEST_NAMESPACE" --timeout=60s || {
            log_warning "namespace '$TEST_NAMESPACE' の削除に失敗しました"
            
            # 強制削除を試行
            log_info "namespace '$TEST_NAMESPACE' の強制削除を試行中..."
            kubectl patch namespace "$TEST_NAMESPACE" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
            kubectl delete namespace "$TEST_NAMESPACE" --force --grace-period=0 || true
        }
        
        log_success "テスト用namespace '$TEST_NAMESPACE' をクリーンアップしました"
    else
        log_info "テスト用namespace '$TEST_NAMESPACE' は存在しません"
    fi
}

# 一時的なテストリソースのクリーンアップ
cleanup_temporary_resources() {
    log_info "一時的なテストリソースをクリーンアップ中..."
    
    # テスト用のDeploymentを削除
    local test_deployments=(
        "valid-deployment"
        "valid-deployment-3"
        "invalid-deployment"
        "temp-deployment"
        "simultaneous-deployment"
        "valid-simultaneous-deployment"
        "update-test-deployment"
        "test-response-time"
    )
    
    for deployment in "${test_deployments[@]}"; do
        kubectl delete deployment "$deployment" --ignore-not-found=true --timeout=30s || {
            log_warning "Deployment '$deployment' の削除に失敗しました"
        }
    done
    
    # テスト用のHPAを削除
    local test_hpas=(
        "valid-hpa"
        "valid-hpa-3"
        "invalid-hpa"
        "existing-hpa"
        "simultaneous-hpa"
        "valid-simultaneous-hpa"
        "update-test-hpa"
        "orphan-hpa"
    )
    
    for hpa in "${test_hpas[@]}"; do
        kubectl delete hpa "$hpa" --ignore-not-found=true --timeout=30s || {
            log_warning "HPA '$hpa' の削除に失敗しました"
        }
    done
    
    log_success "一時的なテストリソースをクリーンアップしました"
}

# 失敗したリソースの強制削除
force_cleanup_stuck_resources() {
    log_info "スタックしたリソースの強制削除を実行中..."
    
    # Finalizerが設定されているリソースを強制削除
    local stuck_resources=$(kubectl get all -n "$TEST_NAMESPACE" --ignore-not-found=true -o name 2>/dev/null || echo "")
    
    if [ -n "$stuck_resources" ]; then
        log_warning "スタックしたリソースが見つかりました。強制削除を実行します..."
        
        echo "$stuck_resources" | while read -r resource; do
            if [ -n "$resource" ]; then
                log_info "強制削除中: $resource"
                kubectl patch "$resource" -n "$TEST_NAMESPACE" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
                kubectl delete "$resource" -n "$TEST_NAMESPACE" --force --grace-period=0 || true
            fi
        done
    fi
    
    log_success "スタックしたリソースの強制削除が完了しました"
}

# 一時ファイルのクリーンアップ
cleanup_temporary_files() {
    log_info "一時ファイルをクリーンアップ中..."
    
    cd "$PROJECT_ROOT"
    
    # テスト関連の一時ファイルを削除
    local temp_files=(
        "webhook-logs.txt"
        "system-status.txt"
        "test-results.xml"
        "test-output.txt"
        "performance-info.txt"
    )
    
    for file in "${temp_files[@]}"; do
        if [ -f "$file" ]; then
            rm -f "$file"
            log_info "削除しました: $file"
        fi
    done
    
    # 古いテストレポートの削除（7日以上古いもの）
    if [ -d "test-reports" ]; then
        log_info "古いテストレポートを削除中..."
        find test-reports -name "*.md" -mtime +7 -delete || true
        find test-reports -name "*.html" -mtime +7 -delete || true
        find test-reports -name "*.xml" -mtime +7 -delete || true
        find test-reports -name "*.txt" -mtime +7 -delete || true
    fi
    
    log_success "一時ファイルのクリーンアップが完了しました"
}

# Docker関連のクリーンアップ
cleanup_docker_resources() {
    log_info "Docker関連リソースをクリーンアップ中..."
    
    # 未使用のDockerイメージを削除
    if command -v docker &> /dev/null; then
        # テスト用のイメージを削除
        docker rmi k8s-deployment-hpa-validator:latest &>/dev/null || true
        docker rmi k8s-deployment-hpa-validator:test &>/dev/null || true
        
        # 未使用のイメージとコンテナを削除
        docker system prune -f &>/dev/null || {
            log_warning "Dockerシステムのクリーンアップに失敗しました"
        }
        
        log_success "Docker関連リソースをクリーンアップしました"
    else
        log_info "Dockerが見つかりません。スキップします。"
    fi
}

# kind環境のクリーンアップ
cleanup_kind_environment() {
    log_info "kind環境をクリーンアップ中..."
    
    if command -v kind &> /dev/null; then
        if kind get clusters | grep -q "$KIND_CLUSTER_NAME"; then
            log_info "kindクラスター '$KIND_CLUSTER_NAME' を削除中..."
            kind delete cluster --name "$KIND_CLUSTER_NAME" || {
                log_warning "kindクラスターの削除に失敗しました"
            }
            log_success "kindクラスター '$KIND_CLUSTER_NAME' を削除しました"
        else
            log_info "kindクラスター '$KIND_CLUSTER_NAME' は存在しません"
        fi
    else
        log_info "kindが見つかりません。スキップします。"
    fi
}

# 証明書ファイルのクリーンアップ
cleanup_certificates() {
    log_info "証明書ファイルをクリーンアップ中..."
    
    cd "$PROJECT_ROOT"
    
    # 証明書ディレクトリの削除
    if [ -d "certs" ]; then
        rm -rf certs
        log_info "証明書ディレクトリを削除しました"
    fi
    
    # 一時的な証明書ファイルの削除
    local cert_files=(
        "tls.crt"
        "tls.key"
        "ca.crt"
        "ca.key"
        "server.crt"
        "server.key"
    )
    
    for cert_file in "${cert_files[@]}"; do
        if [ -f "$cert_file" ]; then
            rm -f "$cert_file"
            log_info "削除しました: $cert_file"
        fi
    done
    
    log_success "証明書ファイルのクリーンアップが完了しました"
}

# クリーンアップ状況の確認
verify_cleanup() {
    log_info "クリーンアップ状況を確認中..."
    
    local cleanup_issues=0
    
    # namespace確認
    if kubectl get namespace "$TEST_NAMESPACE" &>/dev/null; then
        log_warning "テスト用namespace '$TEST_NAMESPACE' がまだ存在します"
        ((cleanup_issues++))
    fi
    
    # kindクラスター確認
    if command -v kind &> /dev/null && kind get clusters | grep -q "$KIND_CLUSTER_NAME"; then
        log_warning "kindクラスター '$KIND_CLUSTER_NAME' がまだ存在します"
        ((cleanup_issues++))
    fi
    
    # 一時ファイル確認
    cd "$PROJECT_ROOT"
    local remaining_files=$(find . -maxdepth 1 -name "*.txt" -o -name "test-results.xml" | wc -l)
    if [ "$remaining_files" -gt 0 ]; then
        log_warning "一時ファイルが残っています ($remaining_files 個)"
        ((cleanup_issues++))
    fi
    
    if [ $cleanup_issues -eq 0 ]; then
        log_success "クリーンアップが正常に完了しました"
        return 0
    else
        log_warning "クリーンアップに $cleanup_issues 個の問題があります"
        return 1
    fi
}

# 使用方法の表示
show_usage() {
    echo "使用方法: $0 [オプション]"
    echo ""
    echo "オプション:"
    echo "  --namespace-only    テスト用namespaceのみクリーンアップ"
    echo "  --full              完全なクリーンアップ（kind環境も削除）"
    echo "  --force             強制クリーンアップ（確認なし）"
    echo "  --verify-only       クリーンアップ状況の確認のみ"
    echo "  --help              このヘルプを表示"
    echo ""
    echo "例:"
    echo "  $0                  # 標準クリーンアップ"
    echo "  $0 --full           # 完全クリーンアップ"
    echo "  $0 --namespace-only # namespace のみクリーンアップ"
}

# メイン実行関数
main() {
    local namespace_only=false
    local full_cleanup=false
    local force_cleanup=false
    local verify_only=false
    
    # オプション解析
    while [[ $# -gt 0 ]]; do
        case $1 in
            --namespace-only)
                namespace_only=true
                shift
                ;;
            --full)
                full_cleanup=true
                shift
                ;;
            --force)
                force_cleanup=true
                shift
                ;;
            --verify-only)
                verify_only=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "不明なオプション: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    log_info "テスト環境のクリーンアップを開始します"
    
    # 確認プロンプト（強制モードでない場合）
    if [ "$force_cleanup" = false ] && [ "$verify_only" = false ]; then
        echo -n "クリーンアップを実行しますか？ [y/N]: "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            log_info "クリーンアップをキャンセルしました"
            exit 0
        fi
    fi
    
    # 確認のみの場合
    if [ "$verify_only" = true ]; then
        verify_cleanup
        exit $?
    fi
    
    # クリーンアップ実行
    local start_time=$(date +%s)
    
    if [ "$namespace_only" = true ]; then
        # namespace のみクリーンアップ
        cleanup_test_namespace
        force_cleanup_stuck_resources
    else
        # 標準クリーンアップ
        cleanup_test_namespace
        cleanup_temporary_resources
        force_cleanup_stuck_resources
        cleanup_temporary_files
        cleanup_certificates
        
        if [ "$full_cleanup" = true ]; then
            # 完全クリーンアップ
            cleanup_docker_resources
            cleanup_kind_environment
        fi
    fi
    
    # クリーンアップ確認
    verify_cleanup
    local verification_result=$?
    
    # 実行時間計算
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    log_info "クリーンアップ実行時間: ${duration}秒"
    
    if [ $verification_result -eq 0 ]; then
        log_success "テスト環境のクリーンアップが正常に完了しました"
        exit 0
    else
        log_warning "クリーンアップは完了しましたが、一部問題があります"
        exit 1
    fi
}

# スクリプト実行
main "$@"