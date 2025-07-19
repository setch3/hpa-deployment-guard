#!/bin/bash

# kind クラスター作成スクリプト
# HPA Deployment Validator用のKubernetes環境をセットアップ

set -euo pipefail

# 設定
CLUSTER_NAME="hpa-validator-cluster"
CONFIG_FILE="kind-config.yaml"
KUBECTL_TIMEOUT="300s"

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
    
    # kindコマンドの存在確認
    if ! command -v kind &> /dev/null; then
        log_error "kindコマンドが見つかりません。kindをインストールしてください。"
        log_info "インストール方法: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
    
    # kubectlコマンドの存在確認
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectlコマンドが見つかりません。kubectlをインストールしてください。"
        exit 1
    fi
    
    # Dockerの動作確認
    if ! docker info &> /dev/null; then
        log_error "Dockerが動作していません。Dockerを起動してください。"
        exit 1
    fi
    
    log_info "前提条件チェック完了"
}

# 既存クラスターの削除
cleanup_existing_cluster() {
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_warn "既存のクラスター '${CLUSTER_NAME}' を削除しています..."
        kind delete cluster --name "${CLUSTER_NAME}"
        log_info "既存クラスターを削除しました"
    fi
}

# kindクラスターの作成
create_cluster() {
    log_info "kindクラスター '${CLUSTER_NAME}' を作成しています..."
    
    if [[ ! -f "${CONFIG_FILE}" ]]; then
        log_error "設定ファイル '${CONFIG_FILE}' が見つかりません"
        exit 1
    fi
    
    kind create cluster --name "${CLUSTER_NAME}" --config "${CONFIG_FILE}"
    
    if [[ $? -eq 0 ]]; then
        log_info "クラスター作成完了"
    else
        log_error "クラスター作成に失敗しました"
        exit 1
    fi
}

# kubectlコンテキストの設定
setup_kubectl_context() {
    log_info "kubectlコンテキストを設定しています..."
    
    # kindクラスターのコンテキストに切り替え
    kubectl config use-context "kind-${CLUSTER_NAME}"
    
    if [[ $? -eq 0 ]]; then
        log_info "kubectlコンテキスト設定完了"
    else
        log_error "kubectlコンテキスト設定に失敗しました"
        exit 1
    fi
}

# クラスターの動作確認
verify_cluster() {
    log_info "クラスターの動作確認を実行しています..."
    
    # APIサーバーの応答確認
    log_info "APIサーバーの応答を確認中..."
    if ! kubectl cluster-info --request-timeout="${KUBECTL_TIMEOUT}" &> /dev/null; then
        log_error "APIサーバーが応答しません"
        exit 1
    fi
    
    # ノードの状態確認
    log_info "ノードの状態を確認中..."
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if kubectl get nodes --no-headers | grep -q "Ready"; then
            log_info "ノードが準備完了状態になりました"
            break
        fi
        
        if [[ $attempt -eq $max_attempts ]]; then
            log_error "ノードが準備完了状態になりませんでした"
            exit 1
        fi
        
        log_info "ノードの準備完了を待機中... (${attempt}/${max_attempts})"
        sleep 10
        ((attempt++))
    done
    
    # システムPodの状態確認
    log_info "システムPodの状態を確認中..."
    kubectl wait --for=condition=Ready pods --all -n kube-system --timeout="${KUBECTL_TIMEOUT}"
    
    if [[ $? -eq 0 ]]; then
        log_info "システムPodが全て準備完了状態になりました"
    else
        log_warn "一部のシステムPodが準備完了状態になっていません"
    fi
}

# クラスター情報の表示
show_cluster_info() {
    log_info "クラスター情報:"
    echo "----------------------------------------"
    echo "クラスター名: ${CLUSTER_NAME}"
    echo "Kubernetesバージョン: $(kubectl version --short --client=false | grep 'Server Version' | cut -d' ' -f3)"
    echo "ノード情報:"
    kubectl get nodes -o wide
    echo "----------------------------------------"
    echo "コンテキスト: $(kubectl config current-context)"
    echo "APIサーバー: $(kubectl cluster-info | grep 'Kubernetes control plane' | cut -d' ' -f6-)"
    echo "----------------------------------------"
}

# メイン処理
main() {
    log_info "HPA Deployment Validator用kindクラスターのセットアップを開始します"
    
    check_prerequisites
    cleanup_existing_cluster
    create_cluster
    setup_kubectl_context
    verify_cluster
    show_cluster_info
    
    log_info "kindクラスターのセットアップが完了しました！"
    log_info "次のステップ: ./scripts/deploy-webhook.sh を実行してWebhookをデプロイしてください"
}

# スクリプト実行
main "$@"