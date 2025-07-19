#!/bin/bash

# デプロイメント検証スクリプト
# HPA Deployment Validatorの動作確認とテスト実行

set -euo pipefail

# 設定
CLUSTER_NAME="hpa-validator-cluster"
NAMESPACE="default"
TEST_NAMESPACE="test-hpa-validator"

# 色付きログ出力用の関数
log_info() {
    echo -e "\033[0;32m[INFO]\033[0m $1"
}

log_warn() {
    echo -e "\033[0;33m[WARN]\033[0m $1"
}

log_error() {
    echo -e "\033[0;31m[ERROR]\033[0m $1"
}

log_success() {
    echo -e "\033[0;36m[SUCCESS]\033[0m $1"
}

# 前提条件チェック
check_prerequisites() {
    log_info "前提条件をチェックしています..."
    
    # kubectlコンテキストの確認
    local current_context=$(kubectl config current-context)
    if [[ "${current_context}" != "kind-${CLUSTER_NAME}" ]]; then
        log_error "kubectlコンテキストが正しくありません。期待値: kind-${CLUSTER_NAME}, 現在値: ${current_context}"
        exit 1
    fi
    
    # Webhookの存在確認
    if ! kubectl get validatingwebhookconfigurations hpa-deployment-validator &> /dev/null; then
        log_error "ValidatingAdmissionWebhook 'hpa-deployment-validator' が見つかりません"
        log_info "先に ./scripts/deploy-webhook.sh を実行してください"
        exit 1
    fi
    
    log_info "前提条件チェック完了"
}

# テスト用ネームスペースの準備
setup_test_namespace() {
    log_info "テスト用ネームスペースを準備しています..."
    
    # 既存のテストネームスペースを削除
    kubectl delete namespace "${TEST_NAMESPACE}" 2>/dev/null || true
    
    # 新しいテストネームスペースを作成
    kubectl create namespace "${TEST_NAMESPACE}"
    
    log_info "テストネームスペース '${TEST_NAMESPACE}' を作成しました"
}

# Webhookの基本動作確認
verify_webhook_basic() {
    log_info "Webhookの基本動作を確認しています..."
    
    # Podの状態確認
    local pod_status=$(kubectl get pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" -o jsonpath='{.items[0].status.phase}')
    if [[ "${pod_status}" != "Running" ]]; then
        log_error "WebhookのPodが正常に動作していません。状態: ${pod_status}"
        kubectl describe pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}"
        exit 1
    fi
    
    # ログの確認
    log_info "Webhookのログを確認中..."
    kubectl logs -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --tail=10
    
    log_success "Webhook基本動作確認完了"
}

# テストケース1: 正常なDeployment作成
test_valid_deployment() {
    log_info "テストケース1: 正常なDeployment作成テスト"
    
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-valid-deployment
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
EOF
    
    if [[ $? -eq 0 ]]; then
        log_success "正常なDeployment作成テスト: 成功"
    else
        log_error "正常なDeployment作成テスト: 失敗"
        return 1
    fi
}

# テストケース2: 1レプリカDeployment作成（拒否されるべき）
test_invalid_deployment_single_replica() {
    log_info "テストケース2: 1レプリカDeployment作成テスト（拒否されるべき）"
    
    # まずHPAを作成
    cat <<EOF | kubectl apply -f -
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
  namespace: ${TEST_NAMESPACE}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-invalid-deployment
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
EOF
    
    # 1レプリカのDeploymentを作成（拒否されるべき）
    if cat <<EOF | kubectl apply -f - 2>&1 | grep -q "admission webhook.*denied"; then
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-invalid-deployment
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-invalid-app
  template:
    metadata:
      labels:
        app: test-invalid-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
EOF
        log_success "1レプリカDeployment作成テスト: 正常に拒否されました"
    else
        log_error "1レプリカDeployment作成テスト: 拒否されませんでした（期待される動作ではありません）"
        return 1
    fi
}

# テストケース3: 正常なHPA作成
test_valid_hpa() {
    log_info "テストケース3: 正常なHPA作成テスト"
    
    cat <<EOF | kubectl apply -f -
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-valid-hpa
  namespace: ${TEST_NAMESPACE}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-valid-deployment
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
EOF
    
    if [[ $? -eq 0 ]]; then
        log_success "正常なHPA作成テスト: 成功"
    else
        log_error "正常なHPA作成テスト: 失敗"
        return 1
    fi
}

# テストケース4: 無効なHPA作成（拒否されるべき）
test_invalid_hpa() {
    log_info "テストケース4: 無効なHPA作成テスト（拒否されるべき）"
    
    # 1レプリカのDeploymentを作成
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-single-replica-deployment
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-single-app
  template:
    metadata:
      labels:
        app: test-single-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
EOF
    
    # このDeploymentを対象とするHPAを作成（拒否されるべき）
    if cat <<EOF | kubectl apply -f - 2>&1 | grep -q "admission webhook.*denied"; then
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-invalid-hpa
  namespace: ${TEST_NAMESPACE}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-single-replica-deployment
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
EOF
        log_success "無効なHPA作成テスト: 正常に拒否されました"
    else
        log_error "無効なHPA作成テスト: 拒否されませんでした（期待される動作ではありません）"
        return 1
    fi
}

# パフォーマンステスト
test_performance() {
    log_info "パフォーマンステストを実行しています..."
    
    local start_time=$(date +%s)
    
    # 複数のリソースを同時に作成
    for i in {1..5}; do
        cat <<EOF | kubectl apply -f - &
apiVersion: apps/v1
kind: Deployment
metadata:
  name: perf-test-deployment-${i}
  namespace: ${TEST_NAMESPACE}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: perf-test-app-${i}
  template:
    metadata:
      labels:
        app: perf-test-app-${i}
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
EOF
    done
    
    # 全てのバックグラウンドジョブの完了を待機
    wait
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    log_success "パフォーマンステスト完了 (所要時間: ${duration}秒)"
}

# テスト結果の表示
show_test_results() {
    log_info "テスト結果サマリー:"
    echo "----------------------------------------"
    echo "作成されたリソース:"
    kubectl get deployments,hpa -n "${TEST_NAMESPACE}"
    echo ""
    echo "Webhookログ（最新10行）:"
    kubectl logs -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --tail=10
    echo "----------------------------------------"
}

# クリーンアップ
cleanup_test_resources() {
    log_info "テストリソースをクリーンアップしています..."
    kubectl delete namespace "${TEST_NAMESPACE}" 2>/dev/null || true
    log_info "クリーンアップ完了"
}

# メイン処理
main() {
    log_info "HPA Deployment Validatorの検証テストを開始します"
    
    local test_failed=0
    
    check_prerequisites
    setup_test_namespace
    verify_webhook_basic
    
    # テストケースの実行
    test_valid_deployment || test_failed=1
    test_invalid_deployment_single_replica || test_failed=1
    test_valid_hpa || test_failed=1
    test_invalid_hpa || test_failed=1
    test_performance || test_failed=1
    
    show_test_results
    
    if [[ $test_failed -eq 0 ]]; then
        log_success "全てのテストが成功しました！"
    else
        log_error "一部のテストが失敗しました"
    fi
    
    # クリーンアップの確認
    read -p "テストリソースをクリーンアップしますか？ (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup_test_resources
    fi
    
    log_info "検証テスト完了"
    exit $test_failed
}

# スクリプト実行
main "$@"