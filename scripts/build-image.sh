#!/bin/bash

# Docker イメージビルドスクリプト
# HPA Deployment Validator用のコンテナイメージを作成

set -euo pipefail

# 設定
IMAGE_NAME="hpa-validator"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
DOCKERFILE="Dockerfile"

# オプション設定
# 環境変数から設定を読み込み、未設定の場合はデフォルト値を使用
SKIP_TESTS=${SKIP_TESTS:-false}
FORCE_BUILD=${FORCE_BUILD:-false}
SHOW_HELP=false
# テストカテゴリ設定（空の場合はすべてのテストを実行）
TEST_CATEGORIES=${TEST_CATEGORIES:-""}
# テストタイムアウト設定（デフォルト: 30秒）
TEST_TIMEOUT=${TEST_TIMEOUT:-"30s"}

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

log_debug() {
    if [[ "${DEBUG:-false}" == "true" ]]; then
        echo -e "\033[0;36m[DEBUG]\033[0m $1"
    fi
}

log_success() {
    echo -e "\033[0;32m[SUCCESS]\033[0m $1"
}

# エラーコードと解決策を表示する関数
show_error_solution() {
    local error_code=$1
    local error_message=$2
    
    log_error "エラーコード: ${error_code}"
    log_error "エラー内容: ${error_message}"
    
    case ${error_code} in
        "E001")
            log_info "💡 解決策: Dockerデーモンが起動しているか確認してください"
            log_info "    $ sudo systemctl start docker"
            log_info "    $ docker info"
            ;;
        "E002")
            log_info "💡 解決策: プロジェクトルートディレクトリで実行してください"
            log_info "    現在のディレクトリ: $(pwd)"
            log_info "    $ cd プロジェクトルートディレクトリ"
            ;;
        "E003")
            log_info "💡 解決策: テストエラーを修正するか、--force-build オプションを使用してください"
            log_info "    $ $0 --force-build"
            log_info "    または特定のテストエラーの解決策を確認してください"
            ;;
        "E004")
            log_info "💡 解決策: go mod tidy を実行して依存関係を更新してください"
            log_info "    $ go mod tidy"
            log_info "    $ go mod verify"
            ;;
        "E005")
            log_info "💡 解決策: ./scripts/generate-certs.sh を実行して証明書を再生成してください"
            log_info "    $ ./scripts/generate-certs.sh"
            log_info "    生成後、証明書が正しく配置されているか確認: ls -la certs/"
            ;;
        "E006")
            log_info "💡 解決策: Dockerのディスク容量を確認し、不要なイメージを削除してください"
            log_info "    $ docker system df"
            log_info "    $ docker system prune -a"
            ;;
        "E007")
            log_info "💡 解決策: テスト環境の設定を確認してください"
            log_info "    $ ./scripts/check-test-environment.sh"
            log_info "    環境変数の設定を確認: env | grep TEST"
            ;;
        "E008")
            log_info "💡 解決策: 監視テストに必要な設定を確認してください"
            log_info "    $ ls -la certs/"
            log_info "    証明書が存在しない場合: ./scripts/generate-certs.sh"
            ;;
        "E009")
            log_info "💡 解決策: ネットワーク接続を確認してください"
            log_info "    $ ping -c 3 8.8.8.8"
            log_info "    $ curl -v https://golang.org"
            ;;
        "E010")
            log_info "💡 解決策: Goのバージョンを確認してください"
            log_info "    $ go version"
            log_info "    推奨バージョン: Go 1.24.2 以上"
            ;;
        *)
            log_info "💡 一般的な解決策:"
            log_info "  1. エラーメッセージを確認して問題を特定"
            log_info "  2. --skip-tests または --force-build オプションを使用"
            log_info "  3. DEBUG=true を設定して詳細なデバッグ情報を表示"
            log_info "  4. ./scripts/check-test-environment.sh を実行して環境を確認"
            ;;
    esac
    
    # 追加のヘルプ情報を表示
    log_info "📚 詳細なトラブルシューティング情報:"
    log_info "  ドキュメント: docs/troubleshooting-guide.md"
    log_info "  問題が解決しない場合は、エラーログを添えて報告してください"
}

# ヘルプメッセージの表示
show_help() {
    cat << EOF
使用方法: $0 [オプション]

HPA Deployment Validator用のDockerイメージをビルドします。

オプション:
  --skip-tests     テストの実行をスキップしてイメージのみをビルドします
                   (要件1.3: テストとイメージビルドの分離)
  --force-build    テストが失敗してもビルドを強制的に続行します
                   (要件1.2: テスト失敗時でもビルドを続行)
  --help, -h       このヘルプメッセージを表示します

例:
  $0                    # 通常のビルド（テスト実行後にイメージビルド）
  $0 --skip-tests       # テストをスキップしてイメージのみビルド
  $0 --force-build      # テスト失敗時でもビルドを続行
  $0 --skip-tests --force-build  # テストをスキップし、エラー時も続行

環境変数:
  SKIP_TESTS=true       # テストをスキップ
  FORCE_BUILD=true      # テスト失敗時も続行
  DEBUG=true            # 詳細なデバッグ情報を表示
  TEST_TIMEOUT=60s      # テストのタイムアウト時間を設定（デフォルト: 30s）
  TEST_CATEGORIES=unit,integration  # 実行するテストカテゴリを指定

デバッグ情報:
  DEBUG=true $0         # 詳細なデバッグ情報を表示
  
エラー解決:
  テスト失敗時には具体的な解決策が表示されます。
  一般的な問題の解決方法:
  1. 証明書エラー: ./scripts/generate-certs.sh を実行
  2. 依存関係エラー: go mod tidy を実行
  3. 接続エラー: ネットワーク設定を確認
  4. タイムアウト: TEST_TIMEOUT=60s $0 で時間を延長

注意:
  --skip-tests と --force-build を同時に指定した場合、
  --skip-tests が優先されテストは実行されません。

EOF
}

# コマンドライン引数の解析
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-tests)
                SKIP_TESTS=true
                log_info "テストスキップモードが有効になりました"
                shift
                ;;
            --force-build)
                FORCE_BUILD=true
                log_info "強制ビルドモードが有効になりました"
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
    
    # 環境変数の値を確認（コマンドライン引数が優先）
    if [[ "${SKIP_TESTS}" != "true" && "${SKIP_TESTS:-}" == "true" ]]; then
        SKIP_TESTS=true
        log_info "環境変数 SKIP_TESTS=true が設定されています"
    fi
    
    if [[ "${FORCE_BUILD}" != "true" && "${FORCE_BUILD:-}" == "true" ]]; then
        FORCE_BUILD=true
        log_info "環境変数 FORCE_BUILD=true が設定されています"
    fi
    
    # 両方のオプションが有効な場合の警告
    if [[ "${SKIP_TESTS}" == "true" && "${FORCE_BUILD}" == "true" ]]; then
        log_warn "両方のオプションが有効です: --skip-tests が優先され、--force-build は無視されます"
    fi
    
    if [[ "${SHOW_HELP}" == "true" ]]; then
        show_help
        exit 0
    fi
}

# 前提条件チェック
check_prerequisites() {
    log_info "前提条件をチェックしています..."
    
    # Dockerの動作確認
    log_debug "Dockerの動作を確認中..."
    if ! docker info &> /dev/null; then
        log_error "Dockerが動作していません。"
        show_error_solution "E001" "Dockerデーモンが起動していないか、権限がありません"
        exit 1
    else
        log_debug "Docker は正常に動作しています"
        
        # Dockerディスク容量の確認
        local docker_info=$(docker info 2>/dev/null)
        local disk_usage=$(echo "${docker_info}" | grep "Data Space Available" | awk '{print $4, $5}')
        if [[ -n "${disk_usage}" ]]; then
            log_debug "Docker利用可能ディスク容量: ${disk_usage}"
        fi
    fi
    
    # Dockerfileの存在確認
    log_debug "Dockerfile の存在を確認中..."
    if [[ ! -f "${DOCKERFILE}" ]]; then
        log_error "Dockerfile が見つかりません。"
        show_error_solution "E002" "Dockerfile が見つかりません。プロジェクトルートで実行してください。"
        exit 1
    else
        log_debug "Dockerfile が見つかりました: $(ls -la ${DOCKERFILE})"
    fi
    
    # go.modの存在確認
    log_debug "go.mod の存在を確認中..."
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod が見つかりません。"
        show_error_solution "E002" "go.mod が見つかりません。プロジェクトルートで実行してください。"
        exit 1
    else
        log_debug "go.mod が見つかりました: $(ls -la go.mod)"
        log_debug "Go モジュール名: $(grep "^module" go.mod | awk '{print $2}')"
    fi
    
    # cmd/webhook/main.go の存在確認
    log_debug "メインソースファイルの存在を確認中..."
    if [[ ! -f "cmd/webhook/main.go" ]]; then
        log_warn "cmd/webhook/main.go が見つかりません。ソースコードの構造が変更されている可能性があります。"
    else
        log_debug "メインソースファイルが見つかりました: $(ls -la cmd/webhook/main.go)"
    fi
    
    # 証明書ファイルの存在確認（テスト用）
    log_debug "証明書ファイルの存在を確認中..."
    if [[ ! -f "certs/tls.crt" || ! -f "certs/tls.key" ]]; then
        log_warn "証明書ファイルが見つかりません。一部のテストが失敗する可能性があります。"
        log_info "証明書を生成するには: ./scripts/generate-certs.sh を実行してください"
    else
        log_debug "証明書ファイルが見つかりました"
    fi
    
    log_success "前提条件チェック完了"
}

# 既存イメージのクリーンアップ
cleanup_existing_image() {
    if docker images | grep -q "^${IMAGE_NAME}"; then
        log_warn "既存のイメージ '${FULL_IMAGE_NAME}' を削除しています..."
        docker rmi "${FULL_IMAGE_NAME}" 2>/dev/null || true
        log_info "既存イメージを削除しました"
    fi
}

# Goアプリケーションのビルドテスト
test_build() {
    # 環境変数の値を表示（デバッグ情報の改善）
    log_info "テスト設定: SKIP_TESTS=${SKIP_TESTS}, FORCE_BUILD=${FORCE_BUILD}"
    
    if [[ "${SKIP_TESTS}" == "true" ]]; then
        log_warn "テストをスキップしています（--skip-tests オプションまたは環境変数 SKIP_TESTS=true が指定されました）"
        return 0
    fi
    
    log_info "Goアプリケーションのビルドテストを実行しています..."
    
    # テストの実行
    local test_failed=false
    local test_output
    local failed_tests=""
    local test_command="go test"
    local test_args="-v"
    local test_log_file="/tmp/build-image-test-log-$(date +%Y%m%d_%H%M%S).txt"
    
    # テストタイムアウトの設定
    if [[ -n "${TEST_TIMEOUT}" ]]; then
        test_args="${test_args} -timeout=${TEST_TIMEOUT}"
    fi
    
    # テストカテゴリの設定
    if [[ -n "${TEST_CATEGORIES}" ]]; then
        log_info "指定されたテストカテゴリのみを実行します: ${TEST_CATEGORIES}"
        # カンマ区切りのカテゴリを処理
        local categories
        IFS=',' read -ra categories <<< "${TEST_CATEGORIES}"
        for category in "${categories[@]}"; do
            # カテゴリに基づいてテストパッケージを選択
            case "${category}" in
                "unit")
                    test_args="${test_args} ./internal/..."
                    ;;
                "integration")
                    test_args="${test_args} -tags=integration ./internal/..."
                    ;;
                "monitoring")
                    test_args="${test_args} ./internal/metrics/..."
                    ;;
                *)
                    log_warn "不明なテストカテゴリ: ${category}"
                    ;;
            esac
        done
    else
        # すべてのテストを実行
        test_args="${test_args} ./..."
    fi
    
    log_info "テストを実行中... コマンド: ${test_command} ${test_args}"
    log_debug "テストログは ${test_log_file} に保存されます"
    
    # テスト実行とログ保存
    if test_output=$(${test_command} ${test_args} 2>&1 | tee "${test_log_file}"); then
        log_info "すべてのテストが成功しました"
        echo "${test_output}" | grep -E "(PASS|ok)" | head -5
    else
        test_failed=true
        log_error "テストが失敗しました"
        echo "--- テスト出力 ---"
        echo "${test_output}" | tail -20
        echo "--- テスト出力終了 ---"
        
        # テストエラー分析の実行
        log_info "テストエラーを分析しています..."
        if [[ -f "scripts/test-error-analyzer.sh" ]]; then
            chmod +x scripts/test-error-analyzer.sh
            scripts/test-error-analyzer.sh "${test_log_file}"
        else
            log_warn "テストエラー分析スクリプトが見つかりません"
        fi
        
        # 失敗したテストを特定（エラーメッセージの改善）
        failed_tests=$(echo "${test_output}" | grep -E "FAIL:" | awk '{print $2}')
        log_error "失敗したテスト:"
        for test in ${failed_tests}; do
            log_error "  - ${test}"
        done
        
        # テスト失敗の詳細分析
        log_info "📊 テスト失敗の詳細分析:"
        
        # 失敗したテストの数をカウント
        local failed_count=$(echo "${failed_tests}" | wc -l)
        log_info "  失敗したテスト数: ${failed_count}"
        
        # テスト失敗パターンの検出と解決策の提案
        local cert_issues=false
        local conn_issues=false
        local timeout_issues=false
        local permission_issues=false
        local resource_issues=false
        
        # 証明書関連の問題
        if echo "${test_output}" | grep -q -E "(certificate|x509|tls)"; then
            cert_issues=true
            log_error "🔒 証明書関連のテストが失敗しています"
            log_info "💡 解決策:"
            log_info "  1. ./scripts/generate-certs.sh を実行して証明書を再生成してください"
            log_info "  2. 証明書の配置を確認: ls -la certs/"
            log_info "  3. 証明書の権限を確認: chmod 600 certs/tls.key"
            
            # 証明書ファイルの存在確認
            if [[ ! -f "certs/tls.crt" || ! -f "certs/tls.key" ]]; then
                log_error "  証明書ファイルが見つかりません"
                log_info "  証明書を生成するには: ./scripts/generate-certs.sh を実行してください"
            else
                log_info "  証明書ファイルは存在しますが、内容が無効または期限切れの可能性があります"
            fi
            
            # 特定のテストケースに対する具体的なアドバイス
            if echo "${failed_tests}" | grep -q "internal/cert"; then
                log_info "  証明書マネージャーのテストが失敗しています"
                log_info "  テストスキップ方法: go test -v ./internal/... -skip=TestCertManager"
            fi
            
            show_error_solution "E005" "証明書関連のテストが失敗しています"
        fi
        
        # 接続関連の問題
        if echo "${test_output}" | grep -q -E "(connection refused|dial tcp|network|unreachable)"; then
            conn_issues=true
            log_error "🔌 接続エラーが発生しています"
            log_info "💡 解決策:"
            log_info "  1. テスト用のサービスが起動しているか確認してください"
            log_info "  2. ファイアウォール設定を確認してください"
            log_info "  3. ネットワーク接続を確認: ping -c 3 8.8.8.8"
            
            # 特定のテストケースに対する具体的なアドバイス
            if echo "${failed_tests}" | grep -q "webhook"; then
                log_info "  Webhookサーバーのテストが失敗しています"
                log_info "  ローカルポートの使用状況を確認: lsof -i :8443"
            fi
            
            show_error_solution "E009" "ネットワーク接続エラーが発生しています"
        fi
        
        # タイムアウト関連の問題
        if echo "${test_output}" | grep -q -E "(timeout|deadline exceeded|context deadline)"; then
            timeout_issues=true
            log_error "⏱️ テストがタイムアウトしています"
            log_info "💡 解決策:"
            log_info "  1. TEST_TIMEOUT 環境変数を増やしてください (現在: ${TEST_TIMEOUT})"
            log_info "  2. システムリソースの使用状況を確認してください"
            log_info "  3. テストを個別に実行してみてください"
            
            # タイムアウトしたテストの特定
            local timeout_tests=$(echo "${test_output}" | grep -E "timeout" | grep -oE "[^ ]+_test.go:[0-9]+" | sort | uniq)
            if [[ -n "${timeout_tests}" ]]; then
                log_info "  タイムアウトしたテストファイル:"
                echo "${timeout_tests}" | while read -r test_file; do
                    log_info "    - ${test_file}"
                done
            fi
        fi
        
        # 権限関連の問題
        if echo "${test_output}" | grep -q -E "(permission denied|access denied|cannot access)"; then
            permission_issues=true
            log_error "🔐 権限エラーが発生しています"
            log_info "💡 解決策:"
            log_info "  1. ファイルの権限を確認してください"
            log_info "  2. 必要に応じて sudo を使用してください"
            log_info "  3. Docker グループに所属しているか確認: groups"
            
            # 権限エラーのあるファイルの特定
            local perm_files=$(echo "${test_output}" | grep -E "permission denied" | grep -oE "[^ ]+\.[a-zA-Z]+" | sort | uniq)
            if [[ -n "${perm_files}" ]]; then
                log_info "  権限エラーのあるファイル:"
                echo "${perm_files}" | while read -r file; do
                    if [[ -f "${file}" ]]; then
                        log_info "    - ${file} (権限: $(ls -la "${file}" | awk '{print $1}'))"
                    else
                        log_info "    - ${file} (ファイルが存在しません)"
                    fi
                done
            fi
        fi
        
        # リソース関連の問題
        if echo "${test_output}" | grep -q -E "(resource temporarily unavailable|out of memory|cannot allocate)"; then
            resource_issues=true
            log_error "💻 システムリソースの問題が発生しています"
            log_info "💡 解決策:"
            log_info "  1. 不要なプロセスを終了してメモリを解放してください"
            log_info "  2. Docker リソース制限を確認してください"
            log_info "  3. システムリソースの使用状況を確認: top"
        fi
        
        # 監視テスト関連の問題
        if echo "${failed_tests}" | grep -q "monitoring"; then
            log_error "📊 監視テストが失敗しています"
            log_info "💡 解決策:"
            log_info "  1. メトリクスエンドポイントの設定を確認してください"
            log_info "  2. 監視テストをスキップするには: TEST_CATEGORIES=unit,integration $0"
            log_info "  3. 監視テストの詳細: cat test/monitoring/monitoring_test.go"
            
            show_error_solution "E008" "監視テストが失敗しています"
        fi
        
        # その他のエラーパターン
        if ! $cert_issues && ! $conn_issues && ! $timeout_issues && ! $permission_issues && ! $resource_issues; then
            log_error "🔍 テスト失敗の原因を特定できませんでした"
            log_info "💡 一般的な解決策:"
            log_info "  1. go clean -testcache でテストキャッシュをクリアしてください"
            log_info "  2. go mod tidy で依存関係を更新してください"
            log_info "  3. DEBUG=true $0 で詳細なデバッグ情報を確認してください"
            
            # テストログの詳細な分析
            log_info "  テストログの詳細分析:"
            local error_lines=$(echo "${test_output}" | grep -E "Error:|error:|fatal:|panic:" | head -5)
            if [[ -n "${error_lines}" ]]; then
                log_info "  エラーメッセージ抜粋:"
                echo "${error_lines}" | while read -r line; do
                    log_info "    - ${line}"
                done
            fi
        fi
        
        if [[ "${FORCE_BUILD}" == "true" ]]; then
            log_warn "強制ビルドモードが有効なため、テスト失敗を無視して続行します"
            log_warn "⚠️  警告: 本番環境では必ずテストを通してからデプロイしてください"
            log_info "テスト失敗の詳細については上記の出力を確認してください"
        else
            log_error "ビルドを中止します"
            log_info "💡 解決方法:"
            log_info "  1. テストエラーを修正してから再実行"
            log_info "  2. テスト失敗を無視する場合: $0 --force-build または FORCE_BUILD=true $0"
            log_info "  3. テストをスキップする場合: $0 --skip-tests または SKIP_TESTS=true $0"
            exit 1
        fi
    fi
    
    # ビルドテスト
    log_info "アプリケーションのビルドテストを実行中..."
    local build_output
    if build_output=$(go build -o /tmp/webhook-test ./cmd/webhook 2>&1); then
        log_info "アプリケーションのビルドテストが成功しました"
    else
        log_error "Goアプリケーションのビルドに失敗しました"
        echo "--- ビルドエラー出力 ---"
        echo "${build_output}"
        echo "--- ビルドエラー出力終了 ---"
        
        # エラーメッセージの改善
        log_error "💡 考えられる原因:"
        if echo "${build_output}" | grep -q "cannot find package"; then
            log_error "  - 依存パッケージが見つかりません"
            log_info "    解決策: go mod tidy を実行して依存関係を更新してください"
        elif echo "${build_output}" | grep -q "undefined:"; then
            log_error "  - 未定義の関数または変数が使用されています"
            log_info "    解決策: インポートが正しいか、関数名のタイプミスがないか確認してください"
        elif echo "${build_output}" | grep -q "syntax error"; then
            log_error "  - 構文エラーがあります"
            log_info "    解決策: エラーメッセージに示された行を確認してください"
        else
            log_error "  - コードに構文エラーがある"
            log_error "  - 依存関係の問題がある"
            log_error "  - go.mod または go.sum が破損している"
        fi
        
        log_info "💡 一般的な解決方法:"
        log_info "  1. go mod tidy を実行して依存関係を整理"
        log_info "  2. コードの構文エラーを修正"
        log_info "  3. go clean -cache でビルドキャッシュをクリア"
        
        # 強制ビルドオプションの場合はエラーを無視
        if [[ "${FORCE_BUILD}" == "true" ]]; then
            log_warn "強制ビルドモードが有効なため、ビルド失敗を無視して続行します"
            log_warn "⚠️  警告: ビルドに失敗したため、イメージが正常に動作しない可能性があります"
        else
            exit 1
        fi
    fi
    
    # テスト用バイナリの削除
    rm -f /tmp/webhook-test
    
    if [[ "${test_failed}" == "true" ]]; then
        log_warn "ビルドテスト完了（テストは失敗しましたが強制続行）"
    else
        log_info "ビルドテスト完了"
    fi
}

# Dockerイメージのビルド
build_image() {
    log_info "Dockerイメージ '${FULL_IMAGE_NAME}' をビルドしています..."
    
    # ビルド開始時刻を記録
    local start_time=$(date +%s)
    
    # Dockerイメージのビルド
    if docker build -t "${FULL_IMAGE_NAME}" -f "${DOCKERFILE}" .; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        log_info "イメージビルド完了 (所要時間: ${duration}秒)"
    else
        log_error "Dockerイメージのビルドに失敗しました"
        exit 1
    fi
}

# イメージの検証
verify_image() {
    log_info "ビルドしたイメージを検証しています..."
    
    # イメージの存在確認
    if ! docker images "${FULL_IMAGE_NAME}" | grep -q "${IMAGE_NAME}"; then
        log_error "ビルドしたイメージが見つかりません"
        log_info "利用可能なイメージ:"
        docker images | grep "${IMAGE_NAME}" || echo "該当するイメージがありません"
        exit 1
    fi
    
    # イメージサイズの確認
    local image_size=$(docker images --format "table {{.Size}}" "${FULL_IMAGE_NAME}" | tail -n 1)
    log_info "イメージサイズ: ${image_size}"
    
    # イメージの基本的な動作確認
    log_info "イメージの基本動作を確認しています..."
    if docker run --rm "${FULL_IMAGE_NAME}" --help &> /dev/null; then
        log_info "イメージの基本動作確認完了"
    else
        log_warn "イメージの基本動作確認でエラーが発生しました（証明書が必要な可能性があります）"
    fi
}

# イメージ情報の表示
show_image_info() {
    log_info "ビルドしたイメージ情報:"
    echo "----------------------------------------"
    docker images "${IMAGE_NAME}"
    echo "----------------------------------------"
    echo "イメージ名: ${FULL_IMAGE_NAME}"
    echo "ビルド日時: $(date)"
    echo "----------------------------------------"
}

# システム情報の表示（デバッグ情報の改善）
show_system_info() {
    log_info "システム情報:"
    echo "----------------------------------------"
    echo "OS: $(uname -s) $(uname -r) $(uname -m)"
    echo "Docker バージョン: $(docker --version)"
    echo "Go バージョン: $(go version)"
    echo "ホスト名: $(hostname)"
    echo "実行ユーザー: $(whoami)"
    echo "作業ディレクトリ: $(pwd)"
    
    # 環境変数の表示
    echo "環境変数:"
    echo "  SKIP_TESTS: ${SKIP_TESTS:-false}"
    echo "  FORCE_BUILD: ${FORCE_BUILD:-false}"
    echo "  TEST_CATEGORIES: ${TEST_CATEGORIES:-全て}"
    echo "  TEST_TIMEOUT: ${TEST_TIMEOUT:-30s}"
    echo "  DEBUG: ${DEBUG:-false}"
    
    # システムリソース情報
    echo "システムリソース:"
    if command -v free &> /dev/null; then
        echo "  メモリ使用状況:"
        free -h | grep -v total | awk '{print "    " $1 ": " $2 " 合計, " $3 " 使用中, " $4 " 空き"}'
    fi
    
    if command -v df &> /dev/null; then
        echo "  ディスク使用状況:"
        df -h . | grep -v Filesystem | awk '{print "    " $1 ": " $2 " 合計, " $3 " 使用中, " $4 " 空き (" $5 ")"}'
    fi
    
    # Docker情報
    echo "Docker情報:"
    if docker info &> /dev/null; then
        echo "  イメージ数: $(docker images -q | wc -l)"
        echo "  コンテナ数: $(docker ps -a -q | wc -l)"
        echo "  実行中コンテナ: $(docker ps -q | wc -l)"
        
        # Dockerディスク使用量
        if docker system df &> /dev/null; then
            echo "  ディスク使用量:"
            docker system df | grep -v TYPE | awk '{print "    " $1 ": " $2 " 合計, " $3 " 使用中, " $4 " 空き"}'
        fi
    else
        echo "  Docker情報を取得できません。Dockerが実行中か確認してください。"
    fi
    
    # Goモジュール情報
    echo "Goモジュール情報:"
    if [[ -f "go.mod" ]]; then
        echo "  モジュール名: $(grep "^module" go.mod | awk '{print $2}')"
        echo "  Go バージョン: $(grep "^go" go.mod | awk '{print $2}')"
        echo "  依存モジュール数: $(grep -c "^[[:space:]]*[a-z]" go.mod)"
    else
        echo "  go.modファイルが見つかりません"
    fi
    
    echo "----------------------------------------"
    
    # 詳細なデバッグ情報（DEBUGモードの場合のみ）
    if [[ "${DEBUG:-false}" == "true" ]]; then
        log_debug "詳細なデバッグ情報:"
        
        # プロセス情報
        log_debug "実行中のプロセス (top 5):"
        if command -v ps &> /dev/null; then
            ps aux | sort -rn -k 3,3 | head -5 | awk '{print "  " $1 " " $2 " " $3 "% " $4 "% " $11}'
        fi
        
        # ネットワーク情報
        log_debug "ネットワーク接続:"
        if command -v netstat &> /dev/null; then
            netstat -tuln | grep LISTEN | head -5 | awk '{print "  " $4}'
        fi
        
        # 環境変数
        log_debug "すべての環境変数:"
        env | sort | head -10 | awk '{print "  " $0}'
        log_debug "  ... (省略) ..."
    fi
}

# メイン処理
main() {
    parse_arguments "$@"
    
    log_info "HPA Deployment Validator Dockerイメージのビルドを開始します"
    
    # ビルドモードの表示（デバッグ情報の改善）
    if [[ "${SKIP_TESTS}" == "true" ]]; then
        log_info "モード: テストスキップビルド"
    elif [[ "${FORCE_BUILD}" == "true" ]]; then
        log_info "モード: 強制ビルド（テスト失敗時も続行）"
    else
        log_info "モード: 通常ビルド（テスト必須）"
    fi
    
    # システム情報の表示（デバッグ用）
    show_system_info
    
    check_prerequisites
    cleanup_existing_image
    test_build
    build_image
    verify_image
    show_image_info
    
    log_info "Dockerイメージのビルドが完了しました！"
    log_info "次のステップ:"
    log_info "  1. ./scripts/deploy-webhook.sh を実行してkind環境にデプロイ"
    log_info "  2. ./scripts/run-e2e-tests.sh を実行してE2Eテストを実行"
    log_info "  3. ./scripts/verify-webhook.sh でWebhookの動作を確認"
}

# スクリプト実行
main "$@"