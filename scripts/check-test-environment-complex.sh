#!/bin/bash

# テスト環境状態チェッカー
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
TEST_NAMESPACE="hpa-validator-test"

# オプション設定
VERBOSE=false
FIX_ISSUES=false
SHOW_HELP=false
OUTPUT_FORMAT="text"  # text, json
CHECK_CATEGORIES="all"  # all, docker, cluster, webhook, certificates, network

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

# 環境状態を格納するグローバル変数（macOS互換）
ENVIRONMENT_STATE_FILE="/tmp/env_state_$$"
ISSUES_FOUND_FILE="/tmp/issues_$$"
SOLUTIONS_FILE="/tmp/solutions_$$"

# 初期化
> "$ENVIRONMENT_STATE_FILE"
> "$ISSUES_FOUND_FILE"
> "$SOLUTIONS_FILE"

# クリーンアップ関数
cleanup_temp_files() {
    rm -f "$ENVIRONMENT_STATE_FILE" "$ISSUES_FOUND_FILE" "$SOLUTIONS_FILE"
}

# 終了時にクリーンアップ
trap cleanup_temp_files EXIT

# 状態設定関数
set_env_state() {
    local key="$1"
    local value="$2"
    echo "${key}=${value}" >> "$ENVIRONMENT_STATE_FILE"
}

# 状態取得関数
get_env_state() {
    local key="$1"
    grep "^${key}=" "$ENVIRONMENT_STATE_FILE" 2>/dev/null | cut -d'=' -f2- || echo ""
}

# 問題追加関数
add_issue() {
    local key="$1"
    local value="$2"
    echo "${key}=${value}" >> "$ISSUES_FOUND_FILE"
}

# 解決策追加関数
add_solution() {
    local key="$1"
    local value="$2"
    echo "${key}=${value}" >> "$SOLUTIONS_FILE"
}

# 問題取得関数
get_issue() {
    local key="$1"
    grep "^${key}=" "$ISSUES_FOUND_FILE" 2>/dev/null | cut -d'=' -f2- || echo ""
}

# 解決策取得関数
get_solution() {
    local key="$1"
    grep "^${key}=" "$SOLUTIONS_FILE" 2>/dev/null | cut -d'=' -f2- || echo ""
}

# 問題数取得関数
get_issues_count() {
    wc -l < "$ISSUES_FOUND_FILE" 2>/dev/null || echo "0"
}

# 問題削除関数
remove_issue() {
    local key="$1"
    local temp_file="/tmp/issues_temp_$$"
    grep -v "^${key}=" "$ISSUES_FOUND_FILE" > "$temp_file" 2>/dev/null || true
    mv "$temp_file" "$ISSUES_FOUND_FILE"
}# ヘルプメ
ッセージの表示
show_help() {
    cat << EOF
使用方法: $0 [オプション]

HPA Deployment Validatorのテスト環境の状態を確認し、問題がある場合は解決策を提案します。

オプション:
  --verbose, -v        詳細な情報を表示します
  --fix               検出した問題の自動修復を試行します
  --format=FORMAT     出力形式を指定します (text, json)
  --category=CATEGORY チェックするカテゴリを指定します
  --help, -h          このヘルプメッセージを表示します

チェックカテゴリ:
  all                 すべてのカテゴリをチェック（デフォルト）
  docker              Dockerイメージとコンテナの状態
  cluster             Kubernetesクラスターの状態
  webhook             Webhookデプロイメントの状態
  certificates        TLS証明書の状態
  network             ネットワーク接続の状態

出力形式:
  text                人間が読みやすいテキスト形式（デフォルト）
  json                JSON形式で構造化された出力

例:
  $0                           # 全体的な環境状態をチェック
  $0 --verbose                 # 詳細情報付きでチェック
  $0 --fix                     # 問題を自動修復
  $0 --category=docker         # Dockerのみをチェック
  $0 --format=json             # JSON形式で出力
  $0 --verbose --fix           # 詳細情報付きで自動修復

環境変数:
  CHECK_VERBOSE=true           詳細モードを有効化
  CHECK_FIX=true              自動修復モードを有効化
  CHECK_FORMAT=json           出力形式を設定

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
            --format=*)
                OUTPUT_FORMAT="${1#*=}"
                shift
                ;;
            --category=*)
                CHECK_CATEGORIES="${1#*=}"
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
    
    # 環境変数からの設定読み込み
    if [[ "${CHECK_VERBOSE:-false}" == "true" ]]; then
        VERBOSE=true
    fi
    
    if [[ "${CHECK_FIX:-false}" == "true" ]]; then
        FIX_ISSUES=true
    fi
    
    if [[ -n "${CHECK_FORMAT:-}" ]]; then
        OUTPUT_FORMAT="${CHECK_FORMAT}"
    fi
    
    if [[ "${SHOW_HELP}" == "true" ]]; then
        show_help
        exit 0
    fi
    
    # 出力形式の検証
    if [[ "${OUTPUT_FORMAT}" != "text" && "${OUTPUT_FORMAT}" != "json" ]]; then
        log_error "無効な出力形式: ${OUTPUT_FORMAT}"
        log_info "有効な形式: text, json"
        exit 1
    fi
    
    # チェックカテゴリの検証
    local valid_categories=("all" "docker" "cluster" "webhook" "certificates" "network")
    local category_valid=false
    for valid_cat in "${valid_categories[@]}"; do
        if [[ "${CHECK_CATEGORIES}" == "${valid_cat}" ]]; then
            category_valid=true
            break
        fi
    done
    
    if [[ "${category_valid}" == "false" ]]; then
        log_error "無効なチェックカテゴリ: ${CHECK_CATEGORIES}"
        log_info "有効なカテゴリ: $(IFS=', '; echo "${valid_categories[*]}")"
        exit 1
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
        add_issue "prerequisites" "必要なコマンドが見つかりません: $(IFS=', '; echo "${missing_commands[*]}")"
        add_solution "prerequisites" "不足しているコマンドをインストールしてください"
        return 1
    fi
    
    set_env_state "prerequisites" "OK"
    return 0
}

# Dockerイメージの状態チェック
check_docker_image() {
    if [[ "${CHECK_CATEGORIES}" != "all" && "${CHECK_CATEGORIES}" != "docker" ]]; then
        return 0
    fi
    
    log_debug "Dockerイメージの状態をチェック中..."
    
    # Dockerデーモンの動作確認
    if ! docker info &> /dev/null; then
        ISSUES_FOUND["docker_daemon"]="Dockerデーモンが動作していません"
        SOLUTIONS["docker_daemon"]="Dockerを起動してください: sudo systemctl start docker"
        ENVIRONMENT_STATE["docker_daemon"]="ERROR"
        return 1
    else
        ENVIRONMENT_STATE["docker_daemon"]="OK"
    fi
    
    # イメージの存在確認
    if docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "^${FULL_IMAGE_NAME}$"; then
        ENVIRONMENT_STATE["docker_image_exists"]="OK"
        
        # イメージの詳細情報を取得
        local image_info=$(docker images --format "{{.Size}}\t{{.CreatedAt}}" "${FULL_IMAGE_NAME}")
        local image_size=$(echo "$image_info" | cut -f1)
        local image_created=$(echo "$image_info" | cut -f2)
        
        ENVIRONMENT_STATE["docker_image_size"]="$image_size"
        ENVIRONMENT_STATE["docker_image_created"]="$image_created"
        
        log_debug "Dockerイメージが見つかりました: ${FULL_IMAGE_NAME}"
        log_debug "  サイズ: ${image_size}"
        log_debug "  作成日時: ${image_created}"
        
        # イメージの基本動作確認
        if docker run --rm "${FULL_IMAGE_NAME}" --help &> /dev/null; then
            ENVIRONMENT_STATE["docker_image_functional"]="OK"
        else
            ENVIRONMENT_STATE["docker_image_functional"]="WARNING"
            ISSUES_FOUND["docker_image_functional"]="イメージの基本動作確認でエラーが発生しました"
            SOLUTIONS["docker_image_functional"]="証明書が必要な可能性があります。./scripts/generate-certs.sh を実行してください"
        fi
    else
        ENVIRONMENT_STATE["docker_image_exists"]="ERROR"
        ISSUES_FOUND["docker_image_exists"]="Dockerイメージ '${FULL_IMAGE_NAME}' が見つかりません"
        SOLUTIONS["docker_image_exists"]="イメージをビルドしてください: ./scripts/build-image.sh"
        return 1
    fi
    
    return 0
}#
 Kubernetesクラスターの状態チェック
check_kubernetes_cluster() {
    if [[ "${CHECK_CATEGORIES}" != "all" && "${CHECK_CATEGORIES}" != "cluster" ]]; then
        return 0
    fi
    
    log_debug "Kubernetesクラスターの状態をチェック中..."
    
    # kindクラスターの存在確認
    local existing_clusters=$(kind get clusters 2>/dev/null || echo "")
    if echo "$existing_clusters" | grep -q "${CLUSTER_NAME}"; then
        ENVIRONMENT_STATE["kind_cluster_exists"]="OK"
        log_debug "kindクラスター '${CLUSTER_NAME}' が見つかりました"
    else
        ENVIRONMENT_STATE["kind_cluster_exists"]="ERROR"
        ISSUES_FOUND["kind_cluster_exists"]="kindクラスター '${CLUSTER_NAME}' が見つかりません"
        SOLUTIONS["kind_cluster_exists"]="クラスターを作成してください: ./scripts/setup-kind-cluster.sh"
        return 1
    fi
    
    # kubectlコンテキストの確認
    local current_context=$(kubectl config current-context 2>/dev/null || echo "")
    if [[ "$current_context" == "kind-${CLUSTER_NAME}" ]]; then
        ENVIRONMENT_STATE["kubectl_context"]="OK"
        log_debug "kubectlコンテキストが正しく設定されています: ${current_context}"
    else
        ENVIRONMENT_STATE["kubectl_context"]="WARNING"
        ISSUES_FOUND["kubectl_context"]="kubectlコンテキストが正しく設定されていません (現在: ${current_context})"
        SOLUTIONS["kubectl_context"]="コンテキストを切り替えてください: kubectl config use-context kind-${CLUSTER_NAME}"
    fi
    
    # APIサーバーの応答確認
    if kubectl cluster-info --request-timeout=10s &> /dev/null; then
        ENVIRONMENT_STATE["api_server"]="OK"
        log_debug "APIサーバーが正常に応答しています"
    else
        ENVIRONMENT_STATE["api_server"]="ERROR"
        ISSUES_FOUND["api_server"]="APIサーバーが応答しません"
        SOLUTIONS["api_server"]="クラスターを再起動してください: kind delete cluster --name ${CLUSTER_NAME} && ./scripts/setup-kind-cluster.sh"
        return 1
    fi
    
    # ノードの状態確認
    local node_status=$(kubectl get nodes --no-headers 2>/dev/null | awk '{print $2}' | head -1)
    if [[ "$node_status" == "Ready" ]]; then
        ENVIRONMENT_STATE["nodes_ready"]="OK"
        
        # ノード詳細情報の取得
        local node_count=$(kubectl get nodes --no-headers | wc -l)
        local ready_nodes=$(kubectl get nodes --no-headers | grep -c Ready)
        
        ENVIRONMENT_STATE["node_count"]="$node_count"
        ENVIRONMENT_STATE["ready_nodes"]="$ready_nodes"
        
        log_debug "ノードが準備完了状態です (${ready_nodes}/${node_count})"
    else
        ENVIRONMENT_STATE["nodes_ready"]="ERROR"
        ISSUES_FOUND["nodes_ready"]="ノードが準備完了状態ではありません (状態: ${node_status})"
        SOLUTIONS["nodes_ready"]="ノードの準備完了を待機してください: kubectl wait --for=condition=Ready nodes --all --timeout=60s"
        return 1
    fi
    
    # システムPodの状態確認
    local system_pods_total=$(kubectl get pods -n kube-system --no-headers 2>/dev/null | wc -l)
    local system_pods_running=$(kubectl get pods -n kube-system --no-headers 2>/dev/null | grep -c Running)
    
    ENVIRONMENT_STATE["system_pods_total"]="$system_pods_total"
    ENVIRONMENT_STATE["system_pods_running"]="$system_pods_running"
    
    if [[ "$system_pods_running" -gt 0 ]] && [[ "$system_pods_running" -eq "$system_pods_total" ]]; then
        ENVIRONMENT_STATE["system_pods"]="OK"
        log_debug "システムPodが正常に動作しています (${system_pods_running}/${system_pods_total})"
    else
        ENVIRONMENT_STATE["system_pods"]="WARNING"
        ISSUES_FOUND["system_pods"]="一部のシステムPodが正常に動作していません (${system_pods_running}/${system_pods_total})"
        SOLUTIONS["system_pods"]="システムPodの状態を確認してください: kubectl get pods -n kube-system"
    fi
    
    return 0
}

# Webhookデプロイメントの状態チェック
check_webhook_deployment() {
    if [[ "${CHECK_CATEGORIES}" != "all" && "${CHECK_CATEGORIES}" != "webhook" ]]; then
        return 0
    fi
    
    log_debug "Webhookデプロイメントの状態をチェック中..."
    
    # Webhookデプロイメントの存在確認
    if kubectl get deployment "$WEBHOOK_NAME" &>/dev/null; then
        ENVIRONMENT_STATE["webhook_deployment_exists"]="OK"
        log_debug "Webhookデプロイメント '${WEBHOOK_NAME}' が見つかりました"
        
        # デプロイメントの詳細状態確認
        local replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        local ready_replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local available_replicas=$(kubectl get deployment "$WEBHOOK_NAME" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
        
        ENVIRONMENT_STATE["webhook_replicas_desired"]="$replicas"
        ENVIRONMENT_STATE["webhook_replicas_ready"]="$ready_replicas"
        ENVIRONMENT_STATE["webhook_replicas_available"]="$available_replicas"
        
        if [[ "$ready_replicas" == "$replicas" && "$available_replicas" == "$replicas" ]]; then
            ENVIRONMENT_STATE["webhook_deployment_ready"]="OK"
            log_debug "Webhookデプロイメントが準備完了状態です (${ready_replicas}/${replicas})"
        else
            ENVIRONMENT_STATE["webhook_deployment_ready"]="ERROR"
            ISSUES_FOUND["webhook_deployment_ready"]="Webhookデプロイメントが準備完了状態ではありません (準備完了: ${ready_replicas}/${replicas}, 利用可能: ${available_replicas}/${replicas})"
            SOLUTIONS["webhook_deployment_ready"]="デプロイメントの準備完了を待機してください: kubectl wait --for=condition=Available deployment/${WEBHOOK_NAME} --timeout=120s"
        fi
    else
        ENVIRONMENT_STATE["webhook_deployment_exists"]="ERROR"
        ISSUES_FOUND["webhook_deployment_exists"]="Webhookデプロイメント '${WEBHOOK_NAME}' が見つかりません"
        SOLUTIONS["webhook_deployment_exists"]="Webhookをデプロイしてください: ./scripts/deploy-webhook.sh"
        return 1
    fi
    
    # ValidatingWebhookConfigurationの確認
    if kubectl get validatingwebhookconfigurations hpa-deployment-validator &>/dev/null; then
        ENVIRONMENT_STATE["webhook_configuration"]="OK"
        log_debug "ValidatingWebhookConfiguration 'hpa-deployment-validator' が見つかりました"
    else
        ENVIRONMENT_STATE["webhook_configuration"]="ERROR"
        ISSUES_FOUND["webhook_configuration"]="ValidatingWebhookConfiguration 'hpa-deployment-validator' が見つかりません"
        SOLUTIONS["webhook_configuration"]="Webhook設定を適用してください: kubectl apply -f manifests/"
    fi
    
    return 0
}

# TLS証明書の状態チェック
check_certificates() {
    if [[ "${CHECK_CATEGORIES}" != "all" && "${CHECK_CATEGORIES}" != "certificates" ]]; then
        return 0
    fi
    
    log_debug "TLS証明書の状態をチェック中..."
    
    # 証明書ファイルの存在確認
    local cert_files=("certs/tls.crt" "certs/tls.key" "certs/ca.crt")
    local missing_certs=()
    
    for cert_file in "${cert_files[@]}"; do
        if [[ -f "$cert_file" ]]; then
            ENVIRONMENT_STATE["cert_${cert_file//\//_}"]="OK"
            
            # ファイルサイズの確認
            local file_size=$(wc -c < "$cert_file")
            ENVIRONMENT_STATE["cert_${cert_file//\//_}_size"]="$file_size"
            
            log_debug "証明書ファイルが見つかりました: ${cert_file} (${file_size} bytes)"
        else
            ENVIRONMENT_STATE["cert_${cert_file//\//_}"]="ERROR"
            missing_certs+=("$cert_file")
        fi
    done
    
    if [[ ${#missing_certs[@]} -gt 0 ]]; then
        ISSUES_FOUND["certificates_missing"]="証明書ファイルが見つかりません: $(IFS=', '; echo "${missing_certs[*]}")"
        SOLUTIONS["certificates_missing"]="証明書を生成してください: ./scripts/generate-certs.sh"
        return 1
    fi
    
    # 証明書の有効性確認
    if [[ -f "certs/tls.crt" ]]; then
        if openssl x509 -in certs/tls.crt -noout -checkend 0 &>/dev/null; then
            ENVIRONMENT_STATE["cert_validity"]="OK"
            
            # 証明書の有効期限確認
            local cert_expiry=$(openssl x509 -in certs/tls.crt -noout -enddate 2>/dev/null | cut -d= -f2 || echo "確認できません")
            ENVIRONMENT_STATE["cert_expiry"]="$cert_expiry"
            
            log_debug "証明書は有効です (有効期限: ${cert_expiry})"
        else
            ENVIRONMENT_STATE["cert_validity"]="ERROR"
            ISSUES_FOUND["cert_validity"]="証明書が無効または期限切れです"
            SOLUTIONS["cert_validity"]="証明書を再生成してください: ./scripts/generate-certs.sh"
        fi
    fi
    
    return 0
}# 
ネットワーク接続の状態チェック
check_network_connectivity() {
    if [[ "${CHECK_CATEGORIES}" != "all" && "${CHECK_CATEGORIES}" != "network" ]]; then
        return 0
    fi
    
    log_debug "ネットワーク接続の状態をチェック中..."
    
    # インターネット接続の確認
    if ping -c 1 8.8.8.8 &>/dev/null; then
        ENVIRONMENT_STATE["internet_connectivity"]="OK"
        log_debug "インターネット接続が正常です"
    else
        ENVIRONMENT_STATE["internet_connectivity"]="WARNING"
        ISSUES_FOUND["internet_connectivity"]="インターネット接続に問題があります"
        SOLUTIONS["internet_connectivity"]="ネットワーク設定を確認してください"
    fi
    
    # Kubernetes APIサーバーへの接続確認
    if kubectl cluster-info --request-timeout=5s &>/dev/null; then
        ENVIRONMENT_STATE["k8s_api_connectivity"]="OK"
        log_debug "Kubernetes APIサーバーへの接続が正常です"
    else
        ENVIRONMENT_STATE["k8s_api_connectivity"]="ERROR"
        ISSUES_FOUND["k8s_api_connectivity"]="Kubernetes APIサーバーへの接続に問題があります"
        SOLUTIONS["k8s_api_connectivity"]="クラスターが正常に動作しているか確認してください: kind get clusters"
    fi
    
    # ローカルポートの使用状況確認
    local webhook_port="8443"
    if lsof -i ":${webhook_port}" &>/dev/null; then
        local port_process=$(lsof -i ":${webhook_port}" | tail -1 | awk '{print $1 " (PID: " $2 ")"}')
        ENVIRONMENT_STATE["port_${webhook_port}_usage"]="USED"
        ENVIRONMENT_STATE["port_${webhook_port}_process"]="$port_process"
        log_debug "ポート ${webhook_port} は使用中です: ${port_process}"
    else
        ENVIRONMENT_STATE["port_${webhook_port}_usage"]="FREE"
        log_debug "ポート ${webhook_port} は空いています"
    fi
    
    return 0
}

# 高度な問題検出機能
detect_advanced_issues() {
    log_debug "高度な問題検出を実行中..."
    
    # システムリソースの確認
    check_system_resources
    
    # ポート競合の確認
    check_port_conflicts
    
    # プロセス競合の確認
    check_process_conflicts
    
    # ログファイルの確認
    check_log_files
    
    # 設定ファイルの整合性確認
    check_configuration_consistency
}

# システムリソースの確認
check_system_resources() {
    log_debug "システムリソースを確認中..."
    
    # メモリ使用量の確認
    if command -v free &> /dev/null; then
        local memory_usage=$(free | grep Mem | awk '{printf "%.1f", $3/$2 * 100.0}')
        ENVIRONMENT_STATE["memory_usage_percent"]="$memory_usage"
        
        if (( $(echo "$memory_usage > 90" | bc -l) )); then
            ISSUES_FOUND["high_memory_usage"]="メモリ使用量が高すぎます (${memory_usage}%)"
            SOLUTIONS["high_memory_usage"]="不要なプロセスを終了するか、システムを再起動してください"
        fi
    fi
    
    # ディスク使用量の確認
    local disk_usage=$(df . | tail -1 | awk '{print $5}' | sed 's/%//')
    ENVIRONMENT_STATE["disk_usage_percent"]="$disk_usage"
    
    if [[ $disk_usage -gt 90 ]]; then
        ISSUES_FOUND["high_disk_usage"]="ディスク使用量が高すぎます (${disk_usage}%)"
        SOLUTIONS["high_disk_usage"]="不要なファイルを削除するか、docker system prune -a を実行してください"
    fi
    
    # CPU負荷の確認
    if command -v uptime &> /dev/null; then
        local load_avg=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | sed 's/,//')
        ENVIRONMENT_STATE["load_average"]="$load_avg"
        
        local cpu_cores=$(nproc 2>/dev/null || echo "1")
        if (( $(echo "$load_avg > $cpu_cores * 2" | bc -l) )); then
            ISSUES_FOUND["high_cpu_load"]="CPU負荷が高すぎます (${load_avg})"
            SOLUTIONS["high_cpu_load"]="高負荷のプロセスを確認し、必要に応じて終了してください"
        fi
    fi
}

# ポート競合の確認
check_port_conflicts() {
    log_debug "ポート競合を確認中..."
    
    local important_ports=("8443" "8080" "6443" "80" "443")
    
    for port in "${important_ports[@]}"; do
        if lsof -i ":${port}" &>/dev/null; then
            local process_info=$(lsof -i ":${port}" | tail -1 | awk '{print $1 " (PID: " $2 ")"}')
            ENVIRONMENT_STATE["port_${port}_status"]="USED"
            ENVIRONMENT_STATE["port_${port}_process"]="$process_info"
            
            # Webhookポートの競合は問題として扱う
            if [[ "$port" == "8443" ]] && ! echo "$process_info" | grep -q "webhook\|hpa-validator"; then
                ISSUES_FOUND["webhook_port_conflict"]="Webhookポート8443が他のプロセスに使用されています: $process_info"
                SOLUTIONS["webhook_port_conflict"]="プロセスを終了するか、別のポートを使用してください: lsof -ti:8443 | xargs kill -9"
            fi
        else
            ENVIRONMENT_STATE["port_${port}_status"]="FREE"
        fi
    done
}

# プロセス競合の確認
check_process_conflicts() {
    log_debug "プロセス競合を確認中..."
    
    # 複数のkindプロセスの確認
    local kind_processes=$(pgrep -f "kind" | wc -l)
    if [[ $kind_processes -gt 1 ]]; then
        ISSUES_FOUND["multiple_kind_processes"]="複数のkindプロセスが実行されています (${kind_processes}個)"
        SOLUTIONS["multiple_kind_processes"]="不要なkindプロセスを終了してください: pkill -f kind"
    fi
    
    # 複数のDockerデーモンの確認
    local docker_processes=$(pgrep -f "dockerd" | wc -l)
    if [[ $docker_processes -gt 1 ]]; then
        ISSUES_FOUND["multiple_docker_processes"]="複数のDockerデーモンが実行されています (${docker_processes}個)"
        SOLUTIONS["multiple_docker_processes"]="Dockerサービスを再起動してください: sudo systemctl restart docker"
    fi
}

# ログファイルの確認
check_log_files() {
    log_debug "ログファイルを確認中..."
    
    # 最近のエラーログの確認
    local error_patterns=("ERROR" "FATAL" "panic" "failed" "timeout")
    local recent_errors=0
    
    # システムログの確認（macOSの場合）
    if command -v log &> /dev/null; then
        for pattern in "${error_patterns[@]}"; do
            local count=$(log show --last 1h --predicate 'eventMessage CONTAINS "'$pattern'"' 2>/dev/null | wc -l)
            recent_errors=$((recent_errors + count))
        done
    fi
    
    if [[ $recent_errors -gt 10 ]]; then
        ISSUES_FOUND["high_error_rate"]="最近1時間で多数のエラーが発生しています (${recent_errors}件)"
        SOLUTIONS["high_error_rate"]="システムログを確認し、根本原因を調査してください"
    fi
    
    # テストログファイルの確認
    if [[ -d "test-reports" ]]; then
        local failed_reports=$(find test-reports -name "*.md" -exec grep -l "FAIL" {} \; | wc -l)
        if [[ $failed_reports -gt 0 ]]; then
            ENVIRONMENT_STATE["failed_test_reports"]="$failed_reports"
            log_debug "失敗したテストレポートが ${failed_reports} 件見つかりました"
        fi
    fi
}

# 設定ファイルの整合性確認
check_configuration_consistency() {
    log_debug "設定ファイルの整合性を確認中..."
    
    # go.modとgo.sumの整合性
    if [[ -f "go.mod" && -f "go.sum" ]]; then
        if ! go mod verify &>/dev/null; then
            ISSUES_FOUND["go_mod_inconsistency"]="go.modとgo.sumの整合性に問題があります"
            SOLUTIONS["go_mod_inconsistency"]="依存関係を更新してください: go mod tidy"
        fi
    fi
    
    # Dockerfileとgo.modのGoバージョン整合性
    if [[ -f "Dockerfile" && -f "go.mod" ]]; then
        local dockerfile_go_version=$(grep "FROM golang:" Dockerfile | head -1 | grep -oE "[0-9]+\.[0-9]+" || echo "")
        local gomod_go_version=$(grep "^go " go.mod | awk '{print $2}' || echo "")
        
        if [[ -n "$dockerfile_go_version" && -n "$gomod_go_version" && "$dockerfile_go_version" != "$gomod_go_version" ]]; then
            ISSUES_FOUND["go_version_mismatch"]="DockerfileとGo.modのGoバージョンが一致しません (Dockerfile: ${dockerfile_go_version}, go.mod: ${gomod_go_version})"
            SOLUTIONS["go_version_mismatch"]="DockerfileまたはGo.modのGoバージョンを統一してください"
        fi
    fi
    
    # Kubernetesマニフェストの基本的な検証
    if [[ -d "manifests" ]]; then
        local invalid_manifests=0
        for manifest in manifests/*.yaml; do
            if [[ -f "$manifest" ]]; then
                if ! kubectl apply --dry-run=client -f "$manifest" &>/dev/null; then
                    ((invalid_manifests++))
                fi
            fi
        done
        
        if [[ $invalid_manifests -gt 0 ]]; then
            ISSUES_FOUND["invalid_manifests"]="無効なKubernetesマニフェストが ${invalid_manifests} 件見つかりました"
            SOLUTIONS["invalid_manifests"]="マニフェストファイルの構文を確認してください: kubectl apply --dry-run=client -f manifests/"
        fi
    fi
}

# 問題の自動修復を試行
attempt_auto_fix() {
    if [[ "${FIX_ISSUES}" != "true" ]]; then
        return 0
    fi
    
    log_info "検出した問題の自動修復を試行中..."
    
    local fixed_issues=0
    local failed_fixes=0
    
    # 各問題に対する修復処理
    for issue in "${!ISSUES_FOUND[@]}"; do
        log_fix "修復を試行中: ${issue}"
        
        case "$issue" in
            "docker_image_exists")
                log_fix "Dockerイメージをビルド中..."
                if cd "$PROJECT_ROOT" && ./scripts/build-image.sh --skip-tests; then
                    log_success "Dockerイメージのビルドが完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "Dockerイメージのビルドに失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "kind_cluster_exists")
                log_fix "kindクラスターを作成中..."
                if cd "$PROJECT_ROOT" && ./scripts/setup-kind-cluster.sh; then
                    log_success "kindクラスターの作成が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "kindクラスターの作成に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "webhook_deployment_exists")
                log_fix "Webhookをデプロイ中..."
                if cd "$PROJECT_ROOT" && ./scripts/deploy-webhook.sh; then
                    log_success "Webhookのデプロイが完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "Webhookのデプロイに失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "certificates_missing"|"cert_validity")
                log_fix "証明書を生成中..."
                if cd "$PROJECT_ROOT" && ./scripts/generate-certs.sh; then
                    log_success "証明書の生成が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "証明書の生成に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "kubectl_context")
                log_fix "kubectlコンテキストを設定中..."
                if kubectl config use-context "kind-${CLUSTER_NAME}"; then
                    log_success "kubectlコンテキストの設定が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "kubectlコンテキストの設定に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "go_mod_inconsistency")
                log_fix "Go依存関係を更新中..."
                if cd "$PROJECT_ROOT" && go mod tidy; then
                    log_success "Go依存関係の更新が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "Go依存関係の更新に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "webhook_port_conflict")
                log_fix "ポート競合を解決中..."
                if lsof -ti:8443 | xargs kill -9 2>/dev/null; then
                    log_success "ポート競合の解決が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "ポート競合の解決に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            "high_disk_usage")
                log_fix "ディスク容量を解放中..."
                if docker system prune -a -f &>/dev/null; then
                    log_success "ディスク容量の解放が完了しました"
                    unset ISSUES_FOUND["$issue"]
                    ((fixed_issues++))
                else
                    log_error "ディスク容量の解放に失敗しました"
                    ((failed_fixes++))
                fi
                ;;
            *)
                log_warning "自動修復がサポートされていない問題です: ${issue}"
                ;;
        esac
    done
    
    log_info "自動修復結果: 修復成功 ${fixed_issues}件, 修復失敗 ${failed_fixes}件"
    
    # 修復後に再チェックを実行
    if [[ $fixed_issues -gt 0 ]]; then
        log_info "修復後の状態を再チェック中..."
        sleep 2  # 少し待機してから再チェック
        
        # 修復した項目に応じて再チェック
        check_docker_image
        check_kubernetes_cluster
        check_webhook_deployment
        check_certificates
        check_network_connectivity
        detect_advanced_issues
    fi
}

# 結果をテキスト形式で出力
output_text_format() {
    echo "========================================"
    echo "テスト環境状態チェック結果"
    echo "========================================"
    echo "チェック日時: $(date)"
    echo "チェックカテゴリ: ${CHECK_CATEGORIES}"
    echo ""
    
    # 全体的な状態サマリー
    local total_checks=0
    local ok_checks=0
    local warning_checks=0
    local error_checks=0
    
    for state in "${ENVIRONMENT_STATE[@]}"; do
        ((total_checks++))
        case "$state" in
            "OK") ((ok_checks++)) ;;
            "WARNING") ((warning_checks++)) ;;
            "ERROR") ((error_checks++)) ;;
        esac
    done
    
    echo "📊 全体サマリー:"
    echo "  総チェック項目数: ${total_checks}"
    echo "  正常: ${ok_checks}"
    echo "  警告: ${warning_checks}"
    echo "  エラー: ${error_checks}"
    echo ""
    
    # 詳細な状態情報
    echo "📋 詳細状態:"
    echo ""
    
    # Docker関連
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "docker" ]]; then
        echo "🐳 Docker:"
        echo "  デーモン: ${ENVIRONMENT_STATE["docker_daemon"]:-"未チェック"}"
        echo "  イメージ存在: ${ENVIRONMENT_STATE["docker_image_exists"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["docker_image_size"]:-}" ]]; then
            echo "  イメージサイズ: ${ENVIRONMENT_STATE["docker_image_size"]}"
        fi
        if [[ -n "${ENVIRONMENT_STATE["docker_image_created"]:-}" ]]; then
            echo "  作成日時: ${ENVIRONMENT_STATE["docker_image_created"]}"
        fi
        echo "  動作確認: ${ENVIRONMENT_STATE["docker_image_functional"]:-"未チェック"}"
        echo ""
    fi
    
    # Kubernetes関連
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "cluster" ]]; then
        echo "☸️  Kubernetes:"
        echo "  kindクラスター: ${ENVIRONMENT_STATE["kind_cluster_exists"]:-"未チェック"}"
        echo "  kubectlコンテキスト: ${ENVIRONMENT_STATE["kubectl_context"]:-"未チェック"}"
        echo "  APIサーバー: ${ENVIRONMENT_STATE["api_server"]:-"未チェック"}"
        echo "  ノード準備状況: ${ENVIRONMENT_STATE["nodes_ready"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["node_count"]:-}" ]]; then
            echo "  ノード数: ${ENVIRONMENT_STATE["ready_nodes"]}/${ENVIRONMENT_STATE["node_count"]}"
        fi
        echo "  システムPod: ${ENVIRONMENT_STATE["system_pods"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["system_pods_total"]:-}" ]]; then
            echo "  システムPod数: ${ENVIRONMENT_STATE["system_pods_running"]}/${ENVIRONMENT_STATE["system_pods_total"]}"
        fi
        echo ""
    fi
    
    # Webhook関連
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "webhook" ]]; then
        echo "🔗 Webhook:"
        echo "  デプロイメント存在: ${ENVIRONMENT_STATE["webhook_deployment_exists"]:-"未チェック"}"
        echo "  デプロイメント準備: ${ENVIRONMENT_STATE["webhook_deployment_ready"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["webhook_replicas_desired"]:-}" ]]; then
            echo "  レプリカ数: ${ENVIRONMENT_STATE["webhook_replicas_ready"]}/${ENVIRONMENT_STATE["webhook_replicas_desired"]} (利用可能: ${ENVIRONMENT_STATE["webhook_replicas_available"]})"
        fi
        echo "  Webhook設定: ${ENVIRONMENT_STATE["webhook_configuration"]:-"未チェック"}"
        echo ""
    fi
    
    # 証明書関連
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "certificates" ]]; then
        echo "🔐 証明書:"
        echo "  TLS証明書: ${ENVIRONMENT_STATE["cert_certs_tls.crt"]:-"未チェック"}"
        echo "  TLS秘密鍵: ${ENVIRONMENT_STATE["cert_certs_tls.key"]:-"未チェック"}"
        echo "  CA証明書: ${ENVIRONMENT_STATE["cert_certs_ca.crt"]:-"未チェック"}"
        echo "  証明書有効性: ${ENVIRONMENT_STATE["cert_validity"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["cert_expiry"]:-}" ]]; then
            echo "  有効期限: ${ENVIRONMENT_STATE["cert_expiry"]}"
        fi
        echo ""
    fi
    
    # ネットワーク関連
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "network" ]]; then
        echo "🌐 ネットワーク:"
        echo "  インターネット接続: ${ENVIRONMENT_STATE["internet_connectivity"]:-"未チェック"}"
        echo "  K8s API接続: ${ENVIRONMENT_STATE["k8s_api_connectivity"]:-"未チェック"}"
        echo "  ポート8443使用状況: ${ENVIRONMENT_STATE["port_8443_usage"]:-"未チェック"}"
        if [[ -n "${ENVIRONMENT_STATE["port_8443_process"]:-}" ]]; then
            echo "  ポート8443使用プロセス: ${ENVIRONMENT_STATE["port_8443_process"]}"
        fi
        echo ""
    fi
    
    # システムリソース関連（全カテゴリの場合のみ）
    if [[ "${CHECK_CATEGORIES}" == "all" ]]; then
        echo "💻 システムリソース:"
        if [[ -n "${ENVIRONMENT_STATE["memory_usage_percent"]:-}" ]]; then
            echo "  メモリ使用率: ${ENVIRONMENT_STATE["memory_usage_percent"]}%"
        fi
        if [[ -n "${ENVIRONMENT_STATE["disk_usage_percent"]:-}" ]]; then
            echo "  ディスク使用率: ${ENVIRONMENT_STATE["disk_usage_percent"]}%"
        fi
        if [[ -n "${ENVIRONMENT_STATE["load_average"]:-}" ]]; then
            echo "  CPU負荷平均: ${ENVIRONMENT_STATE["load_average"]}"
        fi
        
        # ポート使用状況
        local important_ports=("8443" "8080" "6443")
        for port in "${important_ports[@]}"; do
            local port_status="${ENVIRONMENT_STATE["port_${port}_status"]:-"未チェック"}"
            echo "  ポート${port}: ${port_status}"
            if [[ "$port_status" == "USED" && -n "${ENVIRONMENT_STATE["port_${port}_process"]:-}" ]]; then
                echo "    プロセス: ${ENVIRONMENT_STATE["port_${port}_process"]}"
            fi
        done
        
        if [[ -n "${ENVIRONMENT_STATE["failed_test_reports"]:-}" ]]; then
            echo "  失敗したテストレポート: ${ENVIRONMENT_STATE["failed_test_reports"]}件"
        fi
        echo ""
    fi
    
    # 問題と解決策
    if [[ ${#ISSUES_FOUND[@]} -gt 0 ]]; then
        echo "❌ 検出された問題:"
        echo ""
        for issue in "${!ISSUES_FOUND[@]}"; do
            echo "  問題: ${ISSUES_FOUND[$issue]}"
            if [[ -n "${SOLUTIONS[$issue]:-}" ]]; then
                echo "  解決策: ${SOLUTIONS[$issue]}"
            fi
            echo ""
        done
    else
        echo "✅ 問題は検出されませんでした"
        echo ""
    fi
    
    # 推奨アクション
    echo "💡 推奨アクション:"
    if [[ ${#ISSUES_FOUND[@]} -gt 0 ]]; then
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

# 結果をJSON形式で出力
output_json_format() {
    local json_output="{"
    
    # メタデータ
    json_output+='"metadata":{'
    json_output+='"timestamp":"'$(date -Iseconds)'",'
    json_output+='"check_categories":"'${CHECK_CATEGORIES}'",'
    json_output+='"verbose":'${VERBOSE}','
    json_output+='"fix_attempted":'${FIX_ISSUES}''
    json_output+='},'
    
    # 環境状態
    json_output+='"environment_state":{'
    local first=true
    for key in "${!ENVIRONMENT_STATE[@]}"; do
        if [[ "$first" == "false" ]]; then
            json_output+=','
        fi
        json_output+='"'$key'":"'${ENVIRONMENT_STATE[$key]}'"'
        first=false
    done
    json_output+='},'
    
    # 検出された問題
    json_output+='"issues_found":{'
    first=true
    for key in "${!ISSUES_FOUND[@]}"; do
        if [[ "$first" == "false" ]]; then
            json_output+=','
        fi
        json_output+='"'$key'":"'${ISSUES_FOUND[$key]}'"'
        first=false
    done
    json_output+='},'
    
    # 解決策
    json_output+='"solutions":{'
    first=true
    for key in "${!SOLUTIONS[@]}"; do
        if [[ "$first" == "false" ]]; then
            json_output+=','
        fi
        json_output+='"'$key'":"'${SOLUTIONS[$key]}'"'
        first=false
    done
    json_output+='},'
    
    # サマリー
    local total_checks=0
    local ok_checks=0
    local warning_checks=0
    local error_checks=0
    
    for state in "${ENVIRONMENT_STATE[@]}"; do
        ((total_checks++))
        case "$state" in
            "OK") ((ok_checks++)) ;;
            "WARNING") ((warning_checks++)) ;;
            "ERROR") ((error_checks++)) ;;
        esac
    done
    
    json_output+='"summary":{'
    json_output+='"total_checks":'$total_checks','
    json_output+='"ok_checks":'$ok_checks','
    json_output+='"warning_checks":'$warning_checks','
    json_output+='"error_checks":'$error_checks','
    json_output+='"issues_count":'${#ISSUES_FOUND[@]}''
    json_output+='}'
    
    json_output+='}'
    
    # JSONを整形して出力
    if command -v jq &> /dev/null; then
        echo "$json_output" | jq .
    else
        echo "$json_output"
    fi
}

# メイン処理
main() {
    parse_arguments "$@"
    
    log_info "HPA Deployment Validatorテスト環境状態チェックを開始します"
    log_debug "チェックカテゴリ: ${CHECK_CATEGORIES}"
    log_debug "出力形式: ${OUTPUT_FORMAT}"
    log_debug "詳細モード: ${VERBOSE}"
    log_debug "自動修復: ${FIX_ISSUES}"
    
    cd "$PROJECT_ROOT"
    
    # 前提条件チェック
    check_prerequisites
    
    # カテゴリ別チェック実行
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "docker" ]]; then
        check_docker_image
    fi
    
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "cluster" ]]; then
        check_kubernetes_cluster
    fi
    
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "webhook" ]]; then
        check_webhook_deployment
    fi
    
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "certificates" ]]; then
        check_certificates
    fi
    
    if [[ "${CHECK_CATEGORIES}" == "all" || "${CHECK_CATEGORIES}" == "network" ]]; then
        check_network_connectivity
    fi
    
    # 高度な問題検出（全カテゴリの場合のみ）
    if [[ "${CHECK_CATEGORIES}" == "all" ]]; then
        detect_advanced_issues
    fi
    
    # 自動修復の試行
    attempt_auto_fix
    
    # 結果出力
    case "${OUTPUT_FORMAT}" in
        "json")
            output_json_format
            ;;
        "text"|*)
            output_text_format
            ;;
    esac
    
    # 終了コードの決定
    if [[ ${#ISSUES_FOUND[@]} -gt 0 ]]; then
        log_debug "問題が検出されました。終了コード: 1"
        exit 1
    else
        log_debug "問題は検出されませんでした。終了コード: 0"
        exit 0
    fi
}

# スクリプト実行
main "$@"