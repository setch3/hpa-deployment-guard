#!/bin/bash

# ArgoCD統合テストスクリプト
# このスクリプトはArgoCD環境でのWebhookデプロイメントをテストします

set -e

# 設定
NAMESPACE="webhook-system"
APP_NAME="k8s-deployment-hpa-validator"
ARGOCD_NAMESPACE="argocd"
TIMEOUT=300

# カラー出力用の定数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ログ出力関数
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

# ArgoCD CLIの存在確認
check_argocd_cli() {
    if ! command -v argocd &> /dev/null; then
        log_error "ArgoCD CLIが見つかりません。インストールしてください。"
        log_info "インストール方法: https://argo-cd.readthedocs.io/en/stable/cli_installation/"
        exit 1
    fi
    log_success "ArgoCD CLIが見つかりました"
}

# ArgoCD サーバーへの接続確認
check_argocd_connection() {
    log_info "ArgoCD サーバーへの接続を確認中..."
    
    if ! argocd cluster list &> /dev/null; then
        log_error "ArgoCD サーバーに接続できません"
        log_info "以下のコマンドでログインしてください:"
        log_info "  argocd login <ARGOCD_SERVER>"
        exit 1
    fi
    
    log_success "ArgoCD サーバーに接続されています"
}

# Kubernetesクラスターへの接続確認
check_kubernetes_connection() {
    log_info "Kubernetesクラスターへの接続を確認中..."
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Kubernetesクラスターに接続できません"
        exit 1
    fi
    
    log_success "Kubernetesクラスターに接続されています"
}

# ArgoCD Applicationの作成
create_argocd_application() {
    log_info "ArgoCD Applicationを作成中..."
    
    # Application定義ファイルが存在するか確認
    if [[ ! -f "argocd/application.yaml" ]]; then
        log_error "ArgoCD Application定義ファイルが見つかりません: argocd/application.yaml"
        exit 1
    fi
    
    # Applicationを作成
    kubectl apply -f argocd/application.yaml
    
    log_success "ArgoCD Applicationが作成されました"
}

# Applicationの同期待機
wait_for_sync() {
    log_info "Applicationの同期を待機中..."
    
    local count=0
    while [[ $count -lt $TIMEOUT ]]; do
        local sync_status=$(argocd app get $APP_NAME -o json | jq -r '.status.sync.status' 2>/dev/null || echo "Unknown")
        local health_status=$(argocd app get $APP_NAME -o json | jq -r '.status.health.status' 2>/dev/null || echo "Unknown")
        
        log_info "同期状態: $sync_status, ヘルス状態: $health_status"
        
        if [[ "$sync_status" == "Synced" && "$health_status" == "Healthy" ]]; then
            log_success "Applicationが正常に同期されました"
            return 0
        fi
        
        sleep 5
        count=$((count + 5))
    done
    
    log_error "Applicationの同期がタイムアウトしました"
    return 1
}

# Webhookの動作確認
verify_webhook_functionality() {
    log_info "Webhookの動作を確認中..."
    
    # Webhookポッドの確認
    local pod_count=$(kubectl get pods -n $NAMESPACE -l app=k8s-deployment-hpa-validator --no-headers | wc -l)
    if [[ $pod_count -eq 0 ]]; then
        log_error "Webhookポッドが見つかりません"
        return 1
    fi
    
    log_success "Webhookポッドが $pod_count 個実行中です"
    
    # Webhookサービスの確認
    if ! kubectl get service -n $NAMESPACE k8s-deployment-hpa-validator &> /dev/null; then
        log_error "Webhookサービスが見つかりません"
        return 1
    fi
    
    log_success "Webhookサービスが存在します"
    
    # ValidatingWebhookConfigurationの確認
    if ! kubectl get validatingwebhookconfiguration hpa-deployment-validator &> /dev/null; then
        log_error "ValidatingWebhookConfigurationが見つかりません"
        return 1
    fi
    
    log_success "ValidatingWebhookConfigurationが存在します"
    
    # ヘルスチェックエンドポイントの確認
    local pod_name=$(kubectl get pods -n $NAMESPACE -l app=k8s-deployment-hpa-validator -o jsonpath='{.items[0].metadata.name}')
    if [[ -n "$pod_name" ]]; then
        if kubectl exec -n $NAMESPACE $pod_name -- wget -q -O- http://localhost:8080/healthz &> /dev/null; then
            log_success "Webhookのヘルスチェックが正常です"
        else
            log_warning "Webhookのヘルスチェックに失敗しました"
        fi
    fi
}

# Webhookの機能テスト
test_webhook_validation() {
    log_info "Webhookのバリデーション機能をテスト中..."
    
    # テスト用の一時的なnamespaceを作成
    local test_namespace="webhook-test-$(date +%s)"
    kubectl create namespace $test_namespace
    
    # 1 replicaのDeploymentを作成（拒否されるはず）
    cat <<EOF | kubectl apply -f - || true
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: $test_namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx:latest
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
  namespace: $test_namespace
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deployment
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 80
EOF
    
    # HPAが拒否されたかどうかを確認
    sleep 5
    if kubectl get hpa -n $test_namespace test-hpa &> /dev/null; then
        log_error "Webhookが1 replicaのDeploymentを対象とするHPAを許可しました"
        kubectl delete namespace $test_namespace
        return 1
    else
        log_success "Webhookが正しく1 replicaのDeploymentを対象とするHPAを拒否しました"
    fi
    
    # 2 replicaのDeploymentでテスト（許可されるはず）
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-valid
  namespace: $test_namespace
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-valid
  template:
    metadata:
      labels:
        app: test-valid
    spec:
      containers:
      - name: test
        image: nginx:latest
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa-valid
  namespace: $test_namespace
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deployment-valid
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 80
EOF
    
    # HPAが作成されたかどうかを確認
    sleep 5
    if kubectl get hpa -n $test_namespace test-hpa-valid &> /dev/null; then
        log_success "Webhookが正しく2 replicaのDeploymentを対象とするHPAを許可しました"
    else
        log_error "Webhookが2 replicaのDeploymentを対象とするHPAを拒否しました"
        kubectl delete namespace $test_namespace
        return 1
    fi
    
    # テスト用namespaceを削除
    kubectl delete namespace $test_namespace
    log_success "Webhookの機能テストが完了しました"
}

# ロールバックテスト
test_rollback() {
    log_info "ロールバック機能をテスト中..."
    
    # 現在のリビジョンを取得
    local current_revision=$(argocd app get $APP_NAME -o json | jq -r '.status.sync.revision' 2>/dev/null || echo "")
    
    if [[ -z "$current_revision" ]]; then
        log_warning "現在のリビジョンを取得できませんでした。ロールバックテストをスキップします。"
        return 0
    fi
    
    log_info "現在のリビジョン: $current_revision"
    
    # ロールバック操作（実際には前のコミットに戻すのではなく、同期を再実行）
    argocd app sync $APP_NAME
    
    # 同期完了を待機
    if wait_for_sync; then
        log_success "ロールバック（再同期）が正常に完了しました"
    else
        log_error "ロールバック（再同期）に失敗しました"
        return 1
    fi
}

# メトリクスの確認
check_metrics() {
    log_info "メトリクスエンドポイントを確認中..."
    
    local pod_name=$(kubectl get pods -n $NAMESPACE -l app=k8s-deployment-hpa-validator -o jsonpath='{.items[0].metadata.name}')
    if [[ -n "$pod_name" ]]; then
        if kubectl exec -n $NAMESPACE $pod_name -- wget -q -O- http://localhost:8080/metrics | grep -q "webhook_requests_total"; then
            log_success "メトリクスエンドポイントが正常に動作しています"
        else
            log_warning "メトリクスエンドポイントからメトリクスを取得できませんでした"
        fi
    else
        log_warning "Webhookポッドが見つからないため、メトリクスを確認できませんでした"
    fi
}

# クリーンアップ
cleanup() {
    log_info "クリーンアップを実行中..."
    
    # ArgoCD Applicationを削除
    if argocd app get $APP_NAME &> /dev/null; then
        argocd app delete $APP_NAME --cascade
        log_success "ArgoCD Applicationが削除されました"
    fi
    
    # 残ったリソースを削除
    kubectl delete namespace $NAMESPACE --ignore-not-found=true
    kubectl delete validatingwebhookconfiguration hpa-deployment-validator --ignore-not-found=true
    
    log_success "クリーンアップが完了しました"
}

# メイン実行関数
main() {
    log_info "ArgoCD統合テストを開始します..."
    
    # 前提条件の確認
    check_argocd_cli
    check_argocd_connection
    check_kubernetes_connection
    
    # テスト実行
    create_argocd_application
    
    if wait_for_sync; then
        verify_webhook_functionality
        test_webhook_validation
        test_rollback
        check_metrics
        log_success "全てのArgoCD統合テストが成功しました！"
    else
        log_error "ArgoCD統合テストに失敗しました"
        exit 1
    fi
}

# スクリプトの引数処理
case "${1:-}" in
    "cleanup")
        cleanup
        ;;
    "test")
        main
        ;;
    *)
        log_info "使用方法:"
        log_info "  $0 test     - ArgoCD統合テストを実行"
        log_info "  $0 cleanup  - テスト環境をクリーンアップ"
        echo
        log_info "デフォルトでテストを実行します..."
        main
        ;;
esac