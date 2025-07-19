#!/bin/bash

# kind環境へのWebhookデプロイスクリプト
# HPA Deployment ValidatorをKubernetesクラスターにデプロイ

set -euo pipefail

# 設定
CLUSTER_NAME="hpa-validator-cluster"
IMAGE_NAME="hpa-validator"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
NAMESPACE="default"
MANIFESTS_DIR="manifests"
CERTS_DIR="certs"

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

# 前提条件チェック
check_prerequisites() {
    log_info "前提条件をチェックしています..."
    
    # kindクラスターの存在確認
    if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_error "kindクラスター '${CLUSTER_NAME}' が見つかりません。"
        log_info "先に ./scripts/setup-kind-cluster.sh を実行してください。"
        exit 1
    fi
    
    # kubectlコンテキストの確認
    local current_context=$(kubectl config current-context)
    if [[ "${current_context}" != "kind-${CLUSTER_NAME}" ]]; then
        log_warn "kubectlコンテキストを 'kind-${CLUSTER_NAME}' に切り替えています..."
        kubectl config use-context "kind-${CLUSTER_NAME}"
    fi
    
    # Dockerイメージの存在確認
    if ! docker images "${FULL_IMAGE_NAME}" | grep -q "${IMAGE_NAME}"; then
        log_error "Dockerイメージ '${FULL_IMAGE_NAME}' が見つかりません。"
        log_info "利用可能なイメージ:"
        docker images | grep "${IMAGE_NAME}" || echo "該当するイメージがありません"
        log_info "先に ./scripts/build-image.sh を実行してください。"
        exit 1
    fi
    
    # マニフェストディレクトリの確認
    if [[ ! -d "${MANIFESTS_DIR}" ]]; then
        log_error "マニフェストディレクトリ '${MANIFESTS_DIR}' が見つかりません。"
        exit 1
    fi
    
    log_info "前提条件チェック完了"
}

# 証明書の生成
generate_certificates() {
    log_info "TLS証明書を生成しています..."
    
    if [[ -f "./scripts/generate-certs.sh" ]]; then
        ./scripts/generate-certs.sh
        if [[ $? -eq 0 ]]; then
            log_info "証明書生成完了"
        else
            log_error "証明書生成に失敗しました"
            exit 1
        fi
    else
        log_error "証明書生成スクリプトが見つかりません"
        exit 1
    fi
}

# Dockerイメージをkindクラスターにロード
load_image_to_kind() {
    log_info "Dockerイメージをkindクラスターにロードしています..."
    
    kind load docker-image "${FULL_IMAGE_NAME}" --name "${CLUSTER_NAME}"
    
    if [[ $? -eq 0 ]]; then
        log_info "イメージロード完了"
    else
        log_error "イメージロードに失敗しました"
        exit 1
    fi
}

# 既存のデプロイメントをクリーンアップ
cleanup_existing_deployment() {
    log_info "既存のデプロイメントをクリーンアップしています..."
    
    # ValidatingWebhookConfigurationの削除
    kubectl delete validatingwebhookconfiguration hpa-deployment-validator 2>/dev/null || true
    
    # Deploymentの削除（強制削除）
    kubectl delete deployment k8s-deployment-hpa-validator -n "${NAMESPACE}" --force --grace-period=0 2>/dev/null || true
    
    # 残存するPodの強制削除
    kubectl delete pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --force --grace-period=0 2>/dev/null || true
    
    # Serviceの削除
    kubectl delete service k8s-deployment-hpa-validator -n "${NAMESPACE}" 2>/dev/null || true
    kubectl delete service k8s-deployment-hpa-validator-metrics -n "${NAMESPACE}" 2>/dev/null || true
    
    # Secretの削除
    kubectl delete secret k8s-deployment-hpa-validator-certs -n "${NAMESPACE}" 2>/dev/null || true
    
    # ServiceAccountとRBACの削除
    kubectl delete serviceaccount k8s-deployment-hpa-validator -n "${NAMESPACE}" 2>/dev/null || true
    kubectl delete clusterrole k8s-deployment-hpa-validator 2>/dev/null || true
    kubectl delete clusterrolebinding k8s-deployment-hpa-validator 2>/dev/null || true
    kubectl delete role k8s-deployment-hpa-validator -n "${NAMESPACE}" 2>/dev/null || true
    kubectl delete rolebinding k8s-deployment-hpa-validator -n "${NAMESPACE}" 2>/dev/null || true
    
    # Podが完全に削除されるまで待機
    log_info "Podの削除完了を待機中..."
    local max_wait=60
    local wait_count=0
    while kubectl get pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --no-headers 2>/dev/null | grep -q .; do
        if [[ $wait_count -ge $max_wait ]]; then
            log_warn "Podの削除に時間がかかっています。続行します..."
            break
        fi
        sleep 2
        ((wait_count+=2))
        log_info "Podの削除を待機中... (${wait_count}/${max_wait}秒)"
    done
    
    log_info "クリーンアップ完了"
}

# 証明書Secretの作成
create_certificate_secret() {
    log_info "証明書Secretを作成しています..."
    
    if [[ ! -f "${CERTS_DIR}/tls.crt" ]] || [[ ! -f "${CERTS_DIR}/tls.key" ]]; then
        log_error "証明書ファイルが見つかりません"
        exit 1
    fi
    
    kubectl create secret tls k8s-deployment-hpa-validator-certs \
        --cert="${CERTS_DIR}/tls.crt" \
        --key="${CERTS_DIR}/tls.key" \
        -n "${NAMESPACE}"
    
    if [[ $? -eq 0 ]]; then
        log_info "証明書Secret作成完了"
    else
        log_error "証明書Secret作成に失敗しました"
        exit 1
    fi
}

# マニフェストのデプロイ
deploy_manifests() {
    log_info "Kubernetesマニフェストをデプロイしています..."
    
    # マニフェストファイルの存在確認と適用
    local manifest_files=(
        "rbac.yaml"
        "deployment.yaml"
        "service.yaml"
        "webhook.yaml"
    )
    
    for manifest in "${manifest_files[@]}"; do
        local manifest_path="${MANIFESTS_DIR}/${manifest}"
        if [[ -f "${manifest_path}" ]]; then
            log_info "適用中: ${manifest}"
            kubectl apply -f "${manifest_path}"
        else
            log_warn "マニフェストファイルが見つかりません: ${manifest_path}"
        fi
    done
    
    log_info "マニフェストデプロイ完了"
    
    # CA証明書をWebhookに設定
    log_info "CA証明書をValidatingAdmissionWebhookに設定中..."
    local ca_bundle=$(base64 < "${CERTS_DIR}/ca.crt" | tr -d '\n')
    kubectl patch validatingwebhookconfiguration hpa-deployment-validator \
        --type='json' \
        -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': '${ca_bundle}'}]"
}

# デプロイメントの検証
verify_deployment() {
    log_info "デプロイメントを検証しています..."
    
    # Podの起動確認
    log_info "Podの起動を確認中..."
    
    # まず、Podが作成されるまで待機
    local max_wait=60
    local wait_count=0
    while ! kubectl get pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --no-headers 2>/dev/null | grep -q .; do
        if [[ $wait_count -ge $max_wait ]]; then
            log_error "Podが作成されませんでした"
            exit 1
        fi
        sleep 2
        ((wait_count+=2))
        log_info "Podの作成を待機中... (${wait_count}/${max_wait}秒)"
    done
    
    # Podの準備完了を待機
    kubectl wait --for=condition=Ready pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" --timeout=300s
    
    if [[ $? -eq 0 ]]; then
        log_info "Pod起動確認完了"
    else
        log_error "Podの起動に失敗しました"
        log_info "Pod状態:"
        kubectl get pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}"
        log_info "Pod詳細:"
        kubectl describe pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}"
        exit 1
    fi
    
    # Serviceの確認
    log_info "Serviceの状態を確認中..."
    kubectl get service k8s-deployment-hpa-validator -n "${NAMESPACE}"
    
    # ValidatingWebhookConfigurationの確認
    log_info "ValidatingWebhookConfigurationの状態を確認中..."
    kubectl get validatingwebhookconfiguration hpa-deployment-validator
    
    # Webhook接続テスト
    if [[ -f "./scripts/verify-webhook.sh" ]]; then
        log_info "Webhook接続テストを実行中..."
        ./scripts/verify-webhook.sh
    fi
}

# デプロイメント情報の表示
show_deployment_info() {
    log_info "デプロイメント情報:"
    echo "----------------------------------------"
    echo "クラスター: ${CLUSTER_NAME}"
    echo "ネームスペース: ${NAMESPACE}"
    echo "イメージ: ${FULL_IMAGE_NAME}"
    echo ""
    echo "Pod状態:"
    kubectl get pods -l app=k8s-deployment-hpa-validator -n "${NAMESPACE}" -o wide
    echo ""
    echo "Service状態:"
    kubectl get service k8s-deployment-hpa-validator -n "${NAMESPACE}"
    echo ""
    echo "ValidatingWebhookConfiguration状態:"
    kubectl get validatingwebhookconfiguration hpa-deployment-validator
    echo "----------------------------------------"
}

# メイン処理
main() {
    log_info "HPA Deployment ValidatorのKubernetesデプロイを開始します"
    
    check_prerequisites
    generate_certificates
    load_image_to_kind
    cleanup_existing_deployment
    create_certificate_secret
    deploy_manifests
    verify_deployment
    show_deployment_info
    
    log_info "HPA Deployment Validatorのデプロイが完了しました！"
    log_info "テスト実行: kubectl apply -f test-manifests/ でWebhookの動作を確認できます"
}

# スクリプト実行
main "$@"