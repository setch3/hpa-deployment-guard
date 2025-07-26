#!/bin/bash

# テスト環境状態チェッカー（簡易版）
# HPA Deployment Validatorのテスト環境の状態を確認し、問題がある場合は解決策を提案

set -euo pipefail

# 設定
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CLUSTER_NAME="hpa-validator"
WEBHOOK_NAME="k8s-deployment-hpa-validator"
IMAGE_NAME="hpa-validator"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"

# オプション設定
VERBOSE=false
FIX_ISSUES=false
SHOW_HELP=false

# 色付きログ用の定数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
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

log_debug() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${CYAN}[DEBUG]${NC} $1"
    fi
}

log_fix() {
    echo -e "${PURPLE}[FIX]${NC} $1"
}

# 結果を格納する変数
TOTAL_CHECKS=0
OK_CHECKS=0
WARNING_CHECKS=0
ERROR_CHECKS=0
ISSUES_FOUND=""
SOLUTIONS_FOUND=""

# チェック結果を記録する関数
record_check() {
    local status="$1"
    local message="$2"
    local solution="${3:-}"
    
    ((TOTAL_CHECKS++))
    
    case "$status" in
        "OK")
            ((OK_CHECKS++))
            log_debug "✅ $message"
            ;;
        "WARNING")
            ((WARNING_CHECKS++))
            log_warning "⚠️  $message"
            if [[ -n "$solution" ]]; then
                SOLUTIONS_FOUND="${SOLUTIONS_FOUND}💡 $message\n   解決策: $solution\n\n"
            fi
            ;;
        "ERROR")
            ((ERROR_CHECKS++))
            log_error "❌ $message"
            ISSUES_FOUND="${ISSUES_FOUND}❌ $message\n"
            if [[ -n "$solution" ]]; then
                SOLUTIONS_FOUND="${SOLUTIONS_FOUND}💡 $message\n   解決策: $solution\n\n"
            fi
            ;;
    esac
}

# ヘルプメッセージの表示
show_help() {
    cat << EOF
使用方法: $0 [オプション]

HPA Deployment Validatorのテスト環境の状態を確認し、問題がある場合は解決策を提案します。

オプション:
  --verbose, -v        詳細な情報を表示します
  --fix               検出した問題の自動修復を試行します
  --help, -h          このヘルプメッセージを表示します

例:
  $0                           # 全体的な環境状態をチェック
  $0 --verbose                 # 詳細情報付きでチェック
  $0 --fix                     # 問題を自動修復

EOF
}

# コマンドライン引数の解析
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --verbose|-v)
                VERBOSE=true
                shift
                ;;
            --fix)
                FIX_ISSUES=true
                shift
                ;;
            --help|-h)
                SHOW_HELP=true
                shift
                ;;
            *)
                log_error "不明なオプション: $1"
                log_info "使用方法については --help を参照してください"
                exit 1
                ;;
        esac
    done
    
    if [[ "${SHOW_HELP}" == "true" ]]; then
        show_help
        exit 0
    fi
}

# 前提条件チェック
check_prerequisites() {
    log_debug "前提条件をチェック中..."
    
    # 必要なコマンドの存在確認
    local required_commands=("docker" "kubectl" "kind" "go")
    local missing_commands=()
    
    for cmd in "${required_commands[@]}"; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_commands+=("$cmd")
        fi
    done
    
    if [[ ${#missing_commands[@]} -gt 0 ]]; then
        record_check "ERROR" "必要なコマンドが見つかりません: $(IFS=', '; echo "${missing_commands[*]}")" "不足しているコマンドをインストールしてください"
        return 1
    fi
    
    record_check "OK" "前提条件チェック完了"
    return 0
}

# Dockerイメージの状態チェック
check_docker_image() {
    log_debug "Dockerイメージの状態をチェック中..."
    
    # Dockerデーモンの動作確認
    if ! docker info &> /dev/null; then
        record_check "ERROR" "Dockerデーモンが動作していません" "Dockerを起動してください: sudo systemctl start docker"
        return 1
    fi
    
    record_check "OK" "Dockerデーモンが正常に動作しています"
    
    # イメージの存在確認
    if docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "^${FULL_IMAGE_NAME}$"; then
        local image_info=$(docker images --format "{{.Size}}\t{{.CreatedAt}}" "${FULL_IMAGE_NAME}")
        local image_size=$(echo "$image_info" | cut -f1)
        local image_created=$(echo "$image_info" | cut -f2)
        
        record_check "OK" "Dockerイメージが見つかりました: ${FULL_IMAGE_NAME} (サイズ: ${image_size}, 作成: ${image_created})"
        
        # イメージの基本動作確認
        if docker run --rm "${FULL_IMAGE_NAME}" --help &> /dev/null; then
            record_check "OK" "Dockerイメージの基本動作確認完了"
        else
            record_check "WARNING" "Dockerイメージの基本動作確認でエラーが発生しました" "証明書が必要な可能性があります。./scripts/generate-certs.sh を実行してください"
        fi
    else
        record_check "ERROR" "Dockerイメージ '${FULL_IMAGE_NAME}' が見つかりません" "イメージをビルドしてください: ./scripts/build-image.sh"
        return 1
    fi
    
    return 0
}

# Kubernetesクラスターの状態チェック
check_kubernetes_cluster() {
    log_debug "Kubernetesクラスターの状態をチェック中..."
    
    # kindクラスターの存在確認
    local existing_clusters=$(kind get clusters 2>/dev/null || echo "")
    if echo "$existing_clusters" | grep -q "${CLUSTER_NAME}"; then
        record_check "OK" "kindクラスター '${CLUSTER_NAME}' が見つかりました"
    else
        record_check "ERROR" "kindクラスター '${CLUSTER_NAME}' が見つかりません" "クラスターを作成してください: ./scripts/setup-kind-cluster.sh"
        return 1
    fi
    
    # kubectlコンテキストの確認
    local current_context=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "$current_context" == "kind-${CLUSTER_NAME}" ]]; then
        record_check "OK" "kubectlコンテキストが正しく設定されています: ${current_context}"
    else
        record_check "WARNING" "kubectlコンテキストが正しく設定されていません (現在: ${current_context})" "コンテキストを切り替えてください: kubectl config use-context kind-${CLUSTER_NAME}"
    fi
    
    # APIサーバーの応答確認
    if kubectl cluster-info --request-timeout=10s &> /dev/null; then
        record_check "OK" "APIサーバーが正常に応答しています"
    else
        record_check "ERROR" "APIサーバーが応答しません" "クラスターを再起動してください: kind delete cluster --name ${CLUSTER_NAME} && ./scripts/setup-kind-cluster.sh"
        return 1
    fi
    
    # ノードの状態確認
    local node_status=$(kubectl get nodes --no-headers 2>/dev/null | awk '{print $2}' | head -1)
    if [[ "$node_status" == "Ready" ]]; then
        local node_count=$(kubectl get nodes --no-headers | wc -l)
        local ready_nodes=$(kubectl get nodes --no-headers | grep -c Ready)
        record_check "OK" "ノードが準備完了状態です (${ready_nodes}/${node_count})"
    else
        record_check "ERROR" "ノードが準備完了状態ではありません (状態: ${node_status})" "ノードの準備完了を待機してください: kubectl wait --for=condition=Ready nodes --all --timeout=60s"
        return 1
    fi
    
    return 0
}

# Webhookデプロイメントの状態チェック
check_webhook_deployment() {
    log_debug "Webhookデプロイメントの状態をチェック中..."
    
    # Webhookデプロイメントの存在確認
    if kubectl get deployment "$WEBHOOK_NAME" &>/dev/null; then
        record_check "OK" "Webhookデプロイメント '${WEBHOOK_NAME}' が見つかりました"
        
        # デプロイメントの詳細状態確認
        local replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        local ready_replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local available_replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
        
        if [[ "$ready_replicas" == "$replicas" && "$available_replicas" == "$replicas" ]]; then
            record_check "OK" "Webhookデプロイメントが準備完了状態です (${ready_replicas}/${replicas})"
        else
            record_check "ERROR" "Webhookデプロイメントが準備完了状態ではありません (準備完了: ${ready_replicas}/${replicas}, 利用可能: ${available_replicas}/${replicas})" "デプロイメントの準備完了を待機してください: kubectl wait --for=condition=Available deployment/${WEBHOOK_NAME} --timeout=120s"
        fi
    else
        record_check "ERROR" "Webhookデプロイメント '${WEBHOOK_NAME}' が見つかりません" "Webhookをデプロイしてください: ./scripts/deploy-webhook.sh"
        return 1
    fi
    
    # ValidatingWebhookConfigurationの確認
    if kubectl get validatingwebhookconfigurations hpa-deployment-validator &>/dev/null; then
        record_check "OK" "ValidatingWebhookConfiguration 'hpa-deployment-validator' が見つかりました"
    else
        record_check "ERROR" "ValidatingWebhookConfiguration 'hpa-deployment-validator' が見つかりません" "Webhook設定を適用してください: kubectl apply -f manifests/"
    fi
    
    return 0
}

# TLS証明書の状態チェック
check_certificates() {
    log_debug "TLS証明書の状態をチェック中..."
    
    # 証明書ファイルの存在確認
    local cert_files=("certs/tls.crt" "certs/tls.key" "certs/ca.crt")
    local missing_certs=()
    
    for cert_file in "${cert_files[@]}"; do
        if [[ -f "$cert_file" ]]; then
            local file_size=$(wc -c < "$cert_file")
            record_check "OK" "証明書ファイルが見つかりました: ${cert_file} (${file_size} bytes)"
        else
            missing_certs+=("$cert_file")
        fi
    done
    
    if [[ ${#missing_certs[@]} -gt 0 ]]; then
        record_check "ERROR" "証明書ファイルが見つかりません: $(IFS=', '; echo "${missing_certs[*]}")" "証明書を生成してください: ./scripts/generate-certs.sh"
        return 1
    fi
    
    # 証明書の有効性確認
    if [[ -f "certs/tls.crt" ]]; then
        if openssl x509 -in certs/tls.crt -noout -checkend 0 &>/dev/null; then
            local cert_expiry=$(openssl x509 -in certs/tls.crt -noout -enddate 2>/dev/null | cut -d= -f2 || echo "確認できません")
            record_check "OK" "証明書は有効です (有効期限: ${cert_expiry})"
        else
            record_check "ERROR" "証明書が無効または期限切れです" "証明書を再生成してください: ./scripts/generate-certs.sh"
        fi
    fi
    
    return 0
}

# ネットワーク接続の状態チェック
check_network_connectivity() {
    log_debug "ネットワーク接続の状態をチェック中..."
    
    # インターネット接続の確認
    if ping -c 1 8.8.8.8 &>/dev/null; then
        record_check "OK" "インターネット接続が正常です"
    else
        record_check "WARNING" "インターネット接続に問題があります" "ネットワーク設定を確認してください"
    fi
    
    # Kubernetes APIサーバーへの接続確認
    if kubectl cluster-info --request-timeout=5s &>/dev/null; then
        record_check "OK" "Kubernetes APIサーバーへの接続が正常です"
    else
        record_check "ERROR" "Kubernetes APIサーバーへの接続に問題があります" "クラスターが正常に動作しているか確認してください: kind get clusters"
    fi
    
    # ローカルポートの使用状況確認
    local webhook_port="8443"
    if lsof -i ":${webhook_port}" &>/dev/null; then
        local port_process=$(lsof -i ":${webhook_port}" | tail -1 | awk '{print $1 " (PID: " $2 ")"}')
        record_check "OK" "ポート ${webhook_port} は使用中です: ${port_process}"
    else
        record_check "OK" "ポート ${webhook_port} は空いています"
    fi
    
    return 0
}

# 問題の自動修復を試行
attempt_auto_fix() {
    if [[ "${FIX_ISSUES}" != "true" ]]; then
        return 0
    fi
    
    log_info "検出した問題の自動修復を試行中..."
    
    local fixed_issues=0
    local failed_fixes=0
    
    # Dockerイメージが存在しない場合
    if ! docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "^${FULL_IMAGE_NAME}$"; then
        log_fix "Dockerイメージをビルド中..."
        if cd "$PROJECT_ROOT" && ./scripts/build-image.sh --skip-tests; then
            log_success "Dockerイメージのビルドが完了しました"
            ((fixed_issues++))
        else
            log_error "Dockerイメージのビルドに失敗しました"
            ((failed_fixes++))
        fi
    fi
    
    # kindクラスターが存在しない場合
    local existing_clusters=$(kind get clusters 2>/dev/null || echo "")
    if ! echo "$existing_clusters" | grep -q "${CLUSTER_NAME}"; then
        log_fix "kindクラスターを作成中..."
        if cd "$PROJECT_ROOT" && ./scripts/setup-kind-cluster.sh; then
            log_success "kindクラスターの作成が完了しました"
            ((fixed_issues++))
        else
            log_error "kindクラスターの作成に失敗しました"
            ((failed_fixes++))
        fi
    fi
    
    # Webhookデプロイメントが存在しない場合
    if ! kubectl get deployment "$WEBHOOK_NAME" &>/dev/null; then
        log_fix "Webhookをデプロイ中..."
        if cd "$PROJECT_ROOT" && ./scripts/deploy-webhook.sh; then
            log_success "Webhookのデプロイが完了しました"
            ((fixed_issues++))
        else
            log_error "Webhookのデプロイに失敗しました"
            ((failed_fixes++))
        fi
    fi
    
    # 証明書ファイルが存在しない場合
    if [[ ! -f "certs/tls.crt" || ! -f "certs/tls.key" || ! -f "certs/ca.crt" ]]; then
        log_fix "証明書を生成中..."
        if cd "$PROJECT_ROOT" && ./scripts/generate-certs.sh; then
            log_success "証明書の生成が完了しました"
            ((fixed_issues++))
        else
            log_error "証明書の生成に失敗しました"
            ((failed_fixes++))
        fi
    fi
    
    # kubectlコンテキストが正しくない場合
    local current_context=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "$current_context" != "kind-${CLUSTER_NAME}" ]]; then
        log_fix "kubectlコンテキストを設定中..."
        if kubectl config use-context "kind-${CLUSTER_NAME}"; then
            log_success "kubectlコンテキストの設定が完了しました"
            ((fixed_issues++))
        else
            log_error "kubectlコンテキストの設定に失敗しました"
            ((failed_fixes++))
        fi
    fi
    
    log_info "自動修復結果: 修復成功 ${fixed_issues}件, 修復失敗 ${failed_fixes}件"
    
    # 修復後に再チェックを実行
    if [[ $fixed_issues -gt 0 ]]; then
        log_info "修復後の状態を再チェック中..."
        sleep 2  # 少し待機してから再チェック
        
        # 結果をリセット
        TOTAL_CHECKS=0
        OK_CHECKS=0
        WARNING_CHECKS=0
        ERROR_CHECKS=0
        ISSUES_FOUND=""
        SOLUTIONS_FOUND=""
        
        # 再チェック実行
        run_all_checks
    fi
}

# すべてのチェックを実行
run_all_checks() {
    check_prerequisites
    check_docker_image
    check_kubernetes_cluster
    check_webhook_deployment
    check_certificates
    check_network_connectivity
}

# 結果を表示
show_results() {
    echo "========================================"
    echo "テスト環境状態チェック結果"
    echo "========================================"
    echo "チェック日時: $(date)"
    echo ""
    
    # 全体的な状態サマリー
    echo "📊 全体サマリー:"
    echo "  総チェック項目数: ${TOTAL_CHECKS}"
    echo "  正常: ${OK_CHECKS}"
    echo "  警告: ${WARNING_CHECKS}"
    echo "  エラー: ${ERROR_CHECKS}"
    echo ""
    
    # 問題と解決策
    if [[ $ERROR_CHECKS -gt 0 || $WARNING_CHECKS -gt 0 ]]; then
        echo "❌ 検出された問題と解決策:"
        echo ""
        echo -e "$SOLUTIONS_FOUND"
    else
        echo "✅ 問題は検出されませんでした"
        echo ""
    fi
    
    # 推奨アクション
    echo "💡 推奨アクション:"
    if [[ $ERROR_CHECKS -gt 0 || $WARNING_CHECKS -gt 0 ]]; then
        echo "  1. 上記の解決策に従って問題を修正してください"
        echo "  2. 自動修復を試行する場合: $0 --fix"
        echo "  3. 修正後に再度チェック: $0"
    else
        echo "  1. E2Eテストを実行: ./scripts/run-e2e-tests.sh"
        echo "  2. Webhook動作確認: ./scripts/verify-webhook.sh"
        echo "  3. 定期的な環境チェック: $0 --verbose"
    fi
    echo ""
    
    echo "========================================"
}

# メイン処理
main() {
    parse_arguments "$@"
    
    log_info "HPA Deployment Validatorテスト環境状態チェックを開始します"
    log_debug "詳細モード: ${VERBOSE}"
    log_debug "自動修復: ${FIX_ISSUES}"
    
    cd "$PROJECT_ROOT"
    
    # すべてのチェックを実行
    run_all_checks
    
    # 自動修復の試行
    attempt_auto_fix
    
    # 結果表示
    show_results
    
    # 終了コードの決定
    if [[ $ERROR_CHECKS -gt 0 ]]; then
        log_debug "エラーが検出されました。終了コード: 1"
        exit 1
    else
        log_debug "問題は検出されませんでした。終了コード: 0"
        exit 0
    fi
}

# スクリプト実行
main "$@"