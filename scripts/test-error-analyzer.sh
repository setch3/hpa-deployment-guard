#!/bin/bash

# テストエラー分析スクリプト
# テストログを分析して具体的な問題と解決策を提案

set -euo pipefail

# 設定
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

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
    echo -e "${CYAN}[DEBUG]${NC} $1"
}

log_solution() {
    echo -e "${PURPLE}[SOLUTION]${NC} $1"
}

# 使用方法の表示
show_usage() {
    cat << EOF
使用方法: $0 <テストログファイル>

テストログファイルを分析して、具体的な問題と解決策を提案します。

引数:
  <テストログファイル>    分析するテストログファイルのパス

例:
  $0 /tmp/test-output.txt
  $0 test-results.log

EOF
}

# テストログファイルの検証
validate_log_file() {
    local log_file="$1"
    
    if [[ ! -f "$log_file" ]]; then
        log_error "テストログファイルが見つかりません: $log_file"
        return 1
    fi
    
    if [[ ! -r "$log_file" ]]; then
        log_error "テストログファイルを読み取れません: $log_file"
        return 1
    fi
    
    if [[ ! -s "$log_file" ]]; then
        log_warning "テストログファイルが空です: $log_file"
        return 1
    fi
    
    return 0
}

# 証明書関連エラーの分析
analyze_certificate_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "証明書関連エラーを分析中..."
    
    # 証明書ファイル不存在エラー
    if grep -q -E "(no such file|cannot find|not found).*\.(crt|key|pem)" "$log_file"; then
        found_issues=true
        log_error "証明書ファイルが見つかりません"
        log_solution "解決策:"
        log_solution "  1. 証明書を生成: ./scripts/generate-certs.sh"
        log_solution "  2. 証明書ディレクトリを確認: ls -la certs/"
        log_solution "  3. 証明書の権限を確認: chmod 600 certs/tls.key"
        echo ""
    fi
    
    # 証明書形式エラー
    if grep -q -E "(invalid certificate|bad certificate|certificate verify failed)" "$log_file"; then
        found_issues=true
        log_error "証明書の形式または内容に問題があります"
        log_solution "解決策:"
        log_solution "  1. 証明書を再生成: ./scripts/generate-certs.sh"
        log_solution "  2. 証明書の有効期限を確認: openssl x509 -in certs/tls.crt -noout -dates"
        log_solution "  3. 証明書チェーンを確認: openssl verify -CAfile certs/ca.crt certs/tls.crt"
        echo ""
    fi
    
    # TLS接続エラー
    if grep -q -E "(tls: handshake failure|x509: certificate|SSL certificate)" "$log_file"; then
        found_issues=true
        log_error "TLS接続に問題があります"
        log_solution "解決策:"
        log_solution "  1. 証明書のホスト名を確認: openssl x509 -in certs/tls.crt -noout -text | grep DNS"
        log_solution "  2. 証明書の有効期限を確認: openssl x509 -in certs/tls.crt -noout -enddate"
        log_solution "  3. CA証明書を確認: openssl x509 -in certs/ca.crt -noout -text"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# ネットワーク関連エラーの分析
analyze_network_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "ネットワーク関連エラーを分析中..."
    
    # 接続拒否エラー
    if grep -q -E "(connection refused|dial tcp.*refused)" "$log_file"; then
        found_issues=true
        log_error "接続が拒否されています"
        log_solution "解決策:"
        log_solution "  1. サービスが起動しているか確認: kubectl get pods -l app=k8s-deployment-hpa-validator"
        log_solution "  2. ポートが正しいか確認: kubectl get service -l app=k8s-deployment-hpa-validator"
        log_solution "  3. ファイアウォール設定を確認"
        log_solution "  4. ローカルポート使用状況を確認: lsof -i :8443"
        echo ""
    fi
    
    # タイムアウトエラー
    if grep -q -E "(timeout|deadline exceeded|context deadline)" "$log_file"; then
        found_issues=true
        log_error "接続またはレスポンスがタイムアウトしています"
        log_solution "解決策:"
        log_solution "  1. テストタイムアウトを延長: TEST_TIMEOUT=60s ./scripts/build-image.sh"
        log_solution "  2. システムリソースを確認: top, free -h"
        log_solution "  3. ネットワーク遅延を確認: ping -c 5 8.8.8.8"
        log_solution "  4. Kubernetesクラスターの状態を確認: kubectl get nodes"
        echo ""
    fi
    
    # DNS解決エラー
    if grep -q -E "(no such host|name resolution failed|dns)" "$log_file"; then
        found_issues=true
        log_error "DNS名前解決に問題があります"
        log_solution "解決策:"
        log_solution "  1. DNS設定を確認: cat /etc/resolv.conf"
        log_solution "  2. ネットワーク接続を確認: ping -c 3 8.8.8.8"
        log_solution "  3. Kubernetesサービス名を確認: kubectl get services"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# Kubernetes関連エラーの分析
analyze_kubernetes_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "Kubernetes関連エラーを分析中..."
    
    # APIサーバー接続エラー
    if grep -q -E "(unable to connect to server|server could not find|api server)" "$log_file"; then
        found_issues=true
        log_error "Kubernetes APIサーバーに接続できません"
        log_solution "解決策:"
        log_solution "  1. クラスターが起動しているか確認: kind get clusters"
        log_solution "  2. kubectlコンテキストを確認: kubectl config current-context"
        log_solution "  3. クラスターを再起動: kind delete cluster --name hpa-validator && ./scripts/setup-kind-cluster.sh"
        echo ""
    fi
    
    # リソース不足エラー
    if grep -q -E "(insufficient resources|out of memory|resource quota)" "$log_file"; then
        found_issues=true
        log_error "Kubernetesリソースが不足しています"
        log_solution "解決策:"
        log_solution "  1. ノードリソースを確認: kubectl top nodes"
        log_solution "  2. Podリソース使用量を確認: kubectl top pods --all-namespaces"
        log_solution "  3. 不要なPodを削除: kubectl delete pods --field-selector=status.phase=Failed --all-namespaces"
        log_solution "  4. Dockerリソースを確認: docker system df"
        echo ""
    fi
    
    # RBAC権限エラー
    if grep -q -E "(forbidden|access denied|unauthorized|rbac)" "$log_file"; then
        found_issues=true
        log_error "RBAC権限に問題があります"
        log_solution "解決策:"
        log_solution "  1. RBAC設定を確認: kubectl get clusterrolebindings | grep hpa-validator"
        log_solution "  2. ServiceAccountを確認: kubectl get serviceaccounts"
        log_solution "  3. 権限を再適用: kubectl apply -f manifests/rbac.yaml"
        log_solution "  4. RBAC検証スクリプトを実行: ./scripts/verify-rbac.sh"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# Go関連エラーの分析
analyze_go_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "Go関連エラーを分析中..."
    
    # パッケージ不存在エラー
    if grep -q -E "(cannot find package|no such package)" "$log_file"; then
        found_issues=true
        log_error "Goパッケージが見つかりません"
        log_solution "解決策:"
        log_solution "  1. 依存関係を更新: go mod tidy"
        log_solution "  2. モジュールキャッシュをクリア: go clean -modcache"
        log_solution "  3. go.modファイルを確認: cat go.mod"
        log_solution "  4. ベンダーディレクトリを削除: rm -rf vendor/"
        echo ""
    fi
    
    # ビルドエラー
    if grep -q -E "(build failed|compilation error|syntax error)" "$log_file"; then
        found_issues=true
        log_error "Goコードのビルドに失敗しています"
        log_solution "解決策:"
        log_solution "  1. 構文エラーを修正してください"
        log_solution "  2. インポートパスを確認してください"
        log_solution "  3. ビルドキャッシュをクリア: go clean -cache"
        log_solution "  4. Goバージョンを確認: go version"
        echo ""
    fi
    
    # テスト実行エラー
    if grep -q -E "(test failed|panic|runtime error)" "$log_file"; then
        found_issues=true
        log_error "Goテストの実行に失敗しています"
        log_solution "解決策:"
        log_solution "  1. 個別テストを実行して問題を特定: go test -v -run=TestName ./..."
        log_solution "  2. テストキャッシュをクリア: go clean -testcache"
        log_solution "  3. レースコンディションを確認: go test -race ./..."
        log_solution "  4. テスト環境を確認してください"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# Docker関連エラーの分析
analyze_docker_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "Docker関連エラーを分析中..."
    
    # Dockerデーモンエラー
    if grep -q -E "(docker daemon|cannot connect to docker|docker not running)" "$log_file"; then
        found_issues=true
        log_error "Dockerデーモンに接続できません"
        log_solution "解決策:"
        log_solution "  1. Dockerを起動: sudo systemctl start docker"
        log_solution "  2. Docker状態を確認: docker info"
        log_solution "  3. ユーザーをdockerグループに追加: sudo usermod -aG docker \$USER"
        log_solution "  4. ログアウト・ログインして権限を更新"
        echo ""
    fi
    
    # イメージビルドエラー
    if grep -q -E "(build failed|dockerfile|image build)" "$log_file"; then
        found_issues=true
        log_error "Dockerイメージのビルドに失敗しています"
        log_solution "解決策:"
        log_solution "  1. Dockerfileの構文を確認: docker build --no-cache ."
        log_solution "  2. ディスク容量を確認: df -h"
        log_solution "  3. 不要なイメージを削除: docker system prune -a"
        log_solution "  4. ベースイメージを更新: docker pull golang:1.24"
        echo ""
    fi
    
    # イメージ不存在エラー
    if grep -q -E "(image not found|no such image|pull access denied)" "$log_file"; then
        found_issues=true
        log_error "Dockerイメージが見つかりません"
        log_solution "解決策:"
        log_solution "  1. イメージをビルド: ./scripts/build-image.sh"
        log_solution "  2. 利用可能なイメージを確認: docker images"
        log_solution "  3. イメージ名とタグを確認してください"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# 監視・メトリクス関連エラーの分析
analyze_monitoring_errors() {
    local log_file="$1"
    local found_issues=false
    
    log_info "監視・メトリクス関連エラーを分析中..."
    
    # メトリクスエンドポイントエラー
    if grep -q -E "(metrics.*failed|prometheus.*error|/metrics)" "$log_file"; then
        found_issues=true
        log_error "メトリクスエンドポイントに問題があります"
        log_solution "解決策:"
        log_solution "  1. メトリクスエンドポイントを確認: curl http://localhost:8080/metrics"
        log_solution "  2. Prometheusクライアントライブラリを確認"
        log_solution "  3. 監視テストをスキップ: TEST_CATEGORIES=unit,integration ./scripts/build-image.sh"
        echo ""
    fi
    
    # ヘルスチェックエラー
    if grep -q -E "(health.*check.*failed|readiness.*probe|liveness.*probe)" "$log_file"; then
        found_issues=true
        log_error "ヘルスチェックに失敗しています"
        log_solution "解決策:"
        log_solution "  1. ヘルスチェックエンドポイントを確認: curl http://localhost:8080/health"
        log_solution "  2. アプリケーションの起動時間を確認"
        log_solution "  3. リソース制限を確認: kubectl describe pod <pod-name>"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# 一般的な問題パターンの検出
detect_common_patterns() {
    local log_file="$1"
    local found_issues=false
    
    log_info "一般的な問題パターンを検出中..."
    
    # 権限エラー
    if grep -q -E "(permission denied|access denied|operation not permitted)" "$log_file"; then
        found_issues=true
        log_error "権限エラーが発生しています"
        log_solution "解決策:"
        log_solution "  1. ファイル権限を確認: ls -la"
        log_solution "  2. 実行権限を付与: chmod +x <ファイル名>"
        log_solution "  3. 所有者を確認: chown \$USER <ファイル名>"
        echo ""
    fi
    
    # 環境変数エラー
    if grep -q -E "(environment variable|env.*not set|undefined variable)" "$log_file"; then
        found_issues=true
        log_error "環境変数が設定されていません"
        log_solution "解決策:"
        log_solution "  1. 必要な環境変数を設定してください"
        log_solution "  2. 環境変数を確認: env | grep <変数名>"
        log_solution "  3. .envファイルがある場合は読み込み: source .env"
        echo ""
    fi
    
    # ポート競合エラー
    if grep -q -E "(port.*already in use|address already in use|bind.*failed)" "$log_file"; then
        found_issues=true
        log_error "ポートが既に使用されています"
        log_solution "解決策:"
        log_solution "  1. ポート使用状況を確認: lsof -i :<ポート番号>"
        log_solution "  2. プロセスを終了: kill -9 <PID>"
        log_solution "  3. 別のポートを使用してください"
        echo ""
    fi
    
    if [[ "$found_issues" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# テスト結果の統計情報を表示
show_test_statistics() {
    local log_file="$1"
    
    log_info "テスト結果の統計情報:"
    
    # テスト実行数
    local total_tests=$(grep -c "=== RUN" "$log_file" 2>/dev/null || echo "0")
    local passed_tests=$(grep -c "--- PASS:" "$log_file" 2>/dev/null || echo "0")
    local failed_tests=$(grep -c "--- FAIL:" "$log_file" 2>/dev/null || echo "0")
    local skipped_tests=$(grep -c "--- SKIP:" "$log_file" 2>/dev/null || echo "0")
    
    echo "  総テスト数: $total_tests"
    echo "  成功: $passed_tests"
    echo "  失敗: $failed_tests"
    echo "  スキップ: $skipped_tests"
    
    # 実行時間
    local execution_time=$(grep -E "PASS|FAIL.*[0-9]+\.[0-9]+s" "$log_file" | tail -1 | grep -oE "[0-9]+\.[0-9]+s" || echo "不明")
    echo "  実行時間: $execution_time"
    
    # 失敗したテストの一覧
    if [[ $failed_tests -gt 0 ]]; then
        echo ""
        log_error "失敗したテスト:"
        grep "--- FAIL:" "$log_file" | while read -r line; do
            local test_name=$(echo "$line" | awk '{print $3}')
            echo "    - $test_name"
        done
    fi
    
    echo ""
}

# 推奨アクションの提案
suggest_recommended_actions() {
    local log_file="$1"
    local has_errors="$2"
    
    log_info "推奨アクション:"
    
    if [[ "$has_errors" == "true" ]]; then
        echo "  1. 上記の解決策を順番に試してください"
        echo "  2. 問題が解決しない場合は、詳細なログを確認してください"
        echo "  3. 環境をリセットする場合: ./scripts/cleanup-test-environment.sh --full"
        echo "  4. 段階的にテストを実行: go test -v -run=TestName ./path/to/package"
    else
        echo "  1. 特定の問題は検出されませんでした"
        echo "  2. テストログの詳細を手動で確認してください"
        echo "  3. 環境状態を確認: ./scripts/check-test-environment.sh --verbose"
    fi
    
    echo ""
    echo "追加のヘルプ:"
    echo "  - 環境状態チェック: ./scripts/check-test-environment.sh"
    echo "  - 自動修復を試行: ./scripts/check-test-environment.sh --fix"
    echo "  - 詳細なデバッグ: DEBUG=true <コマンド>"
    echo ""
}

# メイン処理
main() {
    if [[ $# -ne 1 ]]; then
        log_error "引数が不正です"
        show_usage
        exit 1
    fi
    
    local log_file="$1"
    
    log_info "テストエラー分析を開始します"
    log_info "ログファイル: $log_file"
    
    # ログファイルの検証
    if ! validate_log_file "$log_file"; then
        exit 1
    fi
    
    echo "========================================"
    echo "テストエラー分析結果"
    echo "========================================"
    echo "分析日時: $(date)"
    echo "ログファイル: $log_file"
    echo "ファイルサイズ: $(wc -l < "$log_file") 行"
    echo ""
    
    # テスト統計情報の表示
    show_test_statistics "$log_file"
    
    # 各種エラーパターンの分析
    local has_errors=false
    
    if analyze_certificate_errors "$log_file"; then
        has_errors=true
    fi
    
    if analyze_network_errors "$log_file"; then
        has_errors=true
    fi
    
    if analyze_kubernetes_errors "$log_file"; then
        has_errors=true
    fi
    
    if analyze_go_errors "$log_file"; then
        has_errors=true
    fi
    
    if analyze_docker_errors "$log_file"; then
        has_errors=true
    fi
    
    if analyze_monitoring_errors "$log_file"; then
        has_errors=true
    fi
    
    if detect_common_patterns "$log_file"; then
        has_errors=true
    fi
    
    # 推奨アクションの提案
    suggest_recommended_actions "$log_file" "$has_errors"
    
    echo "========================================"
    
    if [[ "$has_errors" == "true" ]]; then
        log_info "問題が検出されました。上記の解決策を参考にしてください。"
        exit 1
    else
        log_info "特定の問題パターンは検出されませんでした。"
        exit 0
    fi
}

# スクリプト実行
main "$@"