#!/bin/bash

# Webhook設定の検証スクリプト
# ValidatingWebhookConfigurationの設定と動作を検証する

set -euo pipefail

# 設定
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WEBHOOK_NAME="hpa-deployment-validator"
WEBHOOK_DEV_NAME="k8s-deployment-hpa-validator-dev"
NAMESPACE="default"
SERVICE_NAME="k8s-deployment-hpa-validator"

# ログ関数
log_info() {
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

log_error() {
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1" >&2
}

log_success() {
    echo "[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') - $1"
}

# Webhook設定の存在確認
check_webhook_configuration() {
    local webhook_name="$1"
    log_info "ValidatingWebhookConfiguration '$webhook_name' の存在確認..."
    
    if kubectl get validatingwebhookconfigurations "$webhook_name" >/dev/null 2>&1; then
        log_success "✓ ValidatingWebhookConfiguration '$webhook_name' が存在します"
        return 0
    else
        log_error "✗ ValidatingWebhookConfiguration '$webhook_name' が見つかりません"
        return 1
    fi
}

# Webhook設定の詳細確認
verify_webhook_details() {
    local webhook_name="$1"
    log_info "Webhook設定の詳細を確認中..."
    
    # Webhook設定の取得
    local webhook_config
    webhook_config=$(kubectl get validatingwebhookconfigurations "$webhook_name" -o json)
    
    # サービス設定の確認
    local service_name service_namespace service_path
    service_name=$(echo "$webhook_config" | jq -r '.webhooks[0].clientConfig.service.name')
    service_namespace=$(echo "$webhook_config" | jq -r '.webhooks[0].clientConfig.service.namespace')
    service_path=$(echo "$webhook_config" | jq -r '.webhooks[0].clientConfig.service.path')
    
    log_info "サービス設定:"
    log_info "  - 名前: $service_name"
    log_info "  - 名前空間: $service_namespace"
    log_info "  - パス: $service_path"
    
    # CA証明書の確認
    local ca_bundle
    ca_bundle=$(echo "$webhook_config" | jq -r '.webhooks[0].clientConfig.caBundle')
    if [[ "$ca_bundle" != "null" && -n "$ca_bundle" ]]; then
        log_success "✓ CA証明書が設定されています"
    else
        log_error "✗ CA証明書が設定されていません"
        return 1
    fi
    
    # ルール設定の確認
    local rules_count
    rules_count=$(echo "$webhook_config" | jq '.webhooks[0].rules | length')
    log_info "設定されたルール数: $rules_count"
    
    # 各ルールの詳細表示
    for ((i=0; i<rules_count; i++)); do
        local operations apiGroups resources
        operations=$(echo "$webhook_config" | jq -r ".webhooks[0].rules[$i].operations | join(\", \")")
        apiGroups=$(echo "$webhook_config" | jq -r ".webhooks[0].rules[$i].apiGroups | join(\", \")")
        resources=$(echo "$webhook_config" | jq -r ".webhooks[0].rules[$i].resources | join(\", \")")
        
        log_info "ルール $((i+1)):"
        log_info "  - 操作: $operations"
        log_info "  - APIグループ: $apiGroups"
        log_info "  - リソース: $resources"
    done
    
    return 0
}

# サービスの存在確認
check_webhook_service() {
    log_info "Webhookサービスの存在確認..."
    
    if kubectl get service "$SERVICE_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
        log_success "✓ サービス '$SERVICE_NAME' が存在します"
        
        # サービスの詳細確認
        local service_info
        service_info=$(kubectl get service "$SERVICE_NAME" -n "$NAMESPACE" -o json)
        
        local cluster_ip ports
        cluster_ip=$(echo "$service_info" | jq -r '.spec.clusterIP')
        ports=$(echo "$service_info" | jq -r '.spec.ports[] | "\(.name):\(.port)->\(.targetPort)"' | tr '\n' ' ')
        
        log_info "サービス詳細:"
        log_info "  - ClusterIP: $cluster_ip"
        log_info "  - ポート: $ports"
        
        return 0
    else
        log_error "✗ サービス '$SERVICE_NAME' が見つかりません"
        return 1
    fi
}

# Podの状態確認
check_webhook_pods() {
    log_info "WebhookのPod状態確認..."
    
    local pods
    pods=$(kubectl get pods -n "$NAMESPACE" -l "app=$SERVICE_NAME" --no-headers 2>/dev/null || true)
    
    if [[ -z "$pods" ]]; then
        log_error "✗ Webhookのポッドが見つかりません"
        return 1
    fi
    
    local ready_count=0
    local total_count=0
    
    while IFS= read -r pod_line; do
        if [[ -n "$pod_line" ]]; then
            local pod_name pod_ready pod_status
            pod_name=$(echo "$pod_line" | awk '{print $1}')
            pod_ready=$(echo "$pod_line" | awk '{print $2}')
            pod_status=$(echo "$pod_line" | awk '{print $3}')
            
            ((total_count++))
            
            log_info "Pod: $pod_name - Ready: $pod_ready - Status: $pod_status"
            
            if [[ "$pod_status" == "Running" && "$pod_ready" =~ ^[1-9]/[1-9] ]]; then
                ((ready_count++))
            fi
        fi
    done <<< "$pods"
    
    if [[ $ready_count -eq $total_count && $total_count -gt 0 ]]; then
        log_success "✓ 全てのWebhookポッド ($ready_count/$total_count) が正常に動作しています"
        return 0
    else
        log_error "✗ 一部のWebhookポッドが正常に動作していません ($ready_count/$total_count)"
        return 1
    fi
}

# Webhook接続テスト
test_webhook_connectivity() {
    log_info "Webhook接続テストを実行中..."
    
    # テスト用のDeploymentマニフェストを作成
    local test_manifest="/tmp/test-deployment-$$.yaml"
    cat > "$test_manifest" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-webhook-connectivity
  namespace: default
  labels:
    test: webhook-connectivity
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-webhook-connectivity
  template:
    metadata:
      labels:
        app: test-webhook-connectivity
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
EOF
    
    # 実際にリソースを作成してテスト
    if kubectl apply -f "$test_manifest" >/dev/null 2>&1; then
        log_success "✓ Webhook接続テスト成功 - サーバーとの通信が正常です"
        # テスト用リソースを削除
        kubectl delete -f "$test_manifest" >/dev/null 2>&1 || true
        rm -f "$test_manifest"
        return 0
    else
        log_error "✗ Webhook接続テスト失敗 - サーバーとの通信に問題があります"
        rm -f "$test_manifest"
        return 1
    fi
}

# バリデーション機能テスト
test_validation_functionality() {
    log_info "バリデーション機能テストを実行中..."
    
    # テスト用の1 replica Deploymentを作成
    local test_deployment="/tmp/test-validation-deployment-$$.yaml"
    cat > "$test_deployment" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-validation-deployment
  namespace: default
  labels:
    test: validation-functionality
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-validation-deployment
  template:
    metadata:
      labels:
        app: test-validation-deployment
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
EOF
    
    # テスト用のHPAを作成
    local test_hpa="/tmp/test-validation-hpa-$$.yaml"
    cat > "$test_hpa" <<EOF
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-validation-hpa
  namespace: default
  labels:
    test: validation-functionality
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-validation-deployment
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
EOF
    
    local validation_passed=true
    
    # 1. Deploymentを先に作成してからHPAを作成（失敗すべき）
    log_info "テスト1: 1 replica Deployment + HPA作成テスト"
    
    if kubectl apply -f "$test_deployment" >/dev/null 2>&1; then
        log_info "  - Deployment作成: 成功（予期された動作）"
        
        if kubectl apply -f "$test_hpa" >/dev/null 2>&1; then
            log_error "  - HPA作成: 成功（予期しない動作 - バリデーションが機能していません）"
            validation_passed=false
        else
            log_success "  - HPA作成: 失敗（予期された動作 - バリデーションが正常に機能）"
        fi
        
        # テスト用リソースを削除
        kubectl delete -f "$test_hpa" >/dev/null 2>&1 || true
        kubectl delete -f "$test_deployment" >/dev/null 2>&1 || true
    else
        log_error "  - Deployment作成: 失敗（予期しない動作）"
        validation_passed=false
    fi
    
    # 2. 2 replica Deploymentでテスト（成功すべき）
    log_info "テスト2: 2 replica Deployment + HPA作成テスト"
    
    # Deploymentのreplicasを2に変更
    sed -i.bak 's/replicas: 1/replicas: 2/' "$test_deployment"
    
    if kubectl apply -f "$test_deployment" >/dev/null 2>&1; then
        log_success "  - Deployment作成: 成功（予期された動作）"
        
        if kubectl apply -f "$test_hpa" >/dev/null 2>&1; then
            log_success "  - HPA作成: 成功（予期された動作）"
        else
            log_error "  - HPA作成: 失敗（予期しない動作）"
            validation_passed=false
        fi
        
        # テスト用リソースを削除
        kubectl delete -f "$test_hpa" >/dev/null 2>&1 || true
        kubectl delete -f "$test_deployment" >/dev/null 2>&1 || true
    else
        log_error "  - Deployment作成: 失敗（予期しない動作）"
        validation_passed=false
    fi
    
    # クリーンアップ
    rm -f "$test_deployment" "$test_deployment.bak" "$test_hpa"
    
    if $validation_passed; then
        log_success "✓ バリデーション機能テスト成功"
        return 0
    else
        log_error "✗ バリデーション機能テスト失敗"
        return 1
    fi
}

# CA証明書の更新
update_ca_bundle() {
    local webhook_name="$1"
    local ca_bundle_file="$PROJECT_ROOT/certs/ca-bundle.yaml"
    
    if [[ ! -f "$ca_bundle_file" ]]; then
        log_error "CA証明書バンドルファイルが見つかりません: $ca_bundle_file"
        log_info "scripts/generate-certs.sh を実行してCA証明書を生成してください"
        return 1
    fi
    
    log_info "CA証明書バンドルを更新中..."
    
    local ca_bundle
    ca_bundle=$(grep "caBundle:" "$ca_bundle_file" | cut -d' ' -f2)
    
    if [[ -z "$ca_bundle" ]]; then
        log_error "CA証明書バンドルが空です"
        return 1
    fi
    
    # Webhook設定のCA証明書を更新
    kubectl patch validatingwebhookconfiguration "$webhook_name" \
        --type='json' \
        -p="[{\"op\": \"replace\", \"path\": \"/webhooks/0/clientConfig/caBundle\", \"value\": \"$ca_bundle\"}]"
    
    log_success "✓ CA証明書バンドルが更新されました"
    return 0
}

# メイン実行関数
main() {
    log_info "=== Webhook設定検証スクリプト開始 ==="
    
    local failed=0
    local environment="${1:-production}"
    local webhook_name="$WEBHOOK_NAME"
    
    if [[ "$environment" == "dev" || "$environment" == "development" ]]; then
        webhook_name="$WEBHOOK_DEV_NAME"
        log_info "開発環境モードで実行中..."
    fi
    
    # 基本的な存在確認
    check_webhook_configuration "$webhook_name" || ((failed++))
    check_webhook_service || ((failed++))
    check_webhook_pods || ((failed++))
    
    # 詳細確認（基本確認が成功した場合のみ）
    if [[ $failed -eq 0 ]]; then
        verify_webhook_details "$webhook_name" || ((failed++))
        test_webhook_connectivity || ((failed++))
        
        # バリデーション機能テスト（本番環境のみ）
        if [[ "$environment" != "dev" && "$environment" != "development" ]]; then
            test_validation_functionality || ((failed++))
        fi
    fi
    
    # 結果サマリー
    echo ""
    log_info "=== 検証結果サマリー ==="
    if [[ $failed -eq 0 ]]; then
        log_success "✓ 全ての検証が成功しました"
        log_info "Webhook設定は正常に動作しています"
    else
        log_error "✗ $failed 個の検証が失敗しました"
        log_info "問題を修正してから再度実行してください"
    fi
    
    return $failed
}

# ヘルプ表示
show_help() {
    cat <<EOF
Webhook設定検証スクリプト

使用方法:
  $0 [ENVIRONMENT] [OPTIONS]

環境:
  production (デフォルト) - 本番環境用の検証を実行
  dev, development       - 開発環境用の検証を実行

オプション:
  --update-ca            - CA証明書バンドルを更新
  --help, -h             - このヘルプを表示

例:
  $0                     # 本番環境での検証
  $0 dev                 # 開発環境での検証
  $0 --update-ca         # CA証明書バンドルの更新
EOF
}

# コマンドライン引数の処理
if [[ $# -gt 0 ]]; then
    case "$1" in
        --help|-h)
            show_help
            exit 0
            ;;
        --update-ca)
            environment="${2:-production}"
            webhook_name="$WEBHOOK_NAME"
            if [[ "$environment" == "dev" || "$environment" == "development" ]]; then
                webhook_name="$WEBHOOK_DEV_NAME"
            fi
            update_ca_bundle "$webhook_name"
            exit $?
            ;;
        *)
            main "$@"
            exit $?
            ;;
    esac
else
    main
    exit $?
fi