#!/bin/bash

# 新しい環境でのプロジェクトセットアップスクリプト
# このスクリプトは新しいマシンでプロジェクトを初期化する際に使用します

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

# スクリプトのディレクトリを取得
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

log_info "新しい環境でのプロジェクトセットアップを開始します"
log_info "プロジェクトルート: ${PROJECT_ROOT}"

# プロジェクトルートに移動
cd "${PROJECT_ROOT}"

# 1. 基本的な前提条件チェック
log_info "📋 ステップ1: 前提条件をチェック中..."

# 必要なコマンドの存在確認
required_commands=("git" "go" "docker" "kubectl" "kind")
missing_commands=()

for cmd in "${required_commands[@]}"; do
    if ! command -v "$cmd" &> /dev/null; then
        missing_commands+=("$cmd")
    fi
done

if [[ ${#missing_commands[@]} -gt 0 ]]; then
    log_error "以下のコマンドが見つかりません:"
    for cmd in "${missing_commands[@]}"; do
        log_error "  - $cmd"
    done
    log_info "💡 インストール方法:"
    log_info "  macOS: brew install git go docker kubectl kind"
    log_info "  Ubuntu: apt-get install git golang-go docker.io kubectl"
    log_info "  kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

log_success "必要なコマンドが全て利用可能です"

# 2. Gitリポジトリの状態確認
log_info "📋 ステップ2: Gitリポジトリの状態を確認中..."

if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    log_error "現在のディレクトリはGitリポジトリではありません"
    log_info "💡 解決策:"
    log_info "  1. 正しいプロジェクトディレクトリに移動してください"
    log_info "  2. または git clone でリポジトリをクローンしてください"
    exit 1
fi

# リポジトリの状態確認
log_info "Git状態:"
log_info "  ブランチ: $(git branch --show-current)"
log_info "  最新コミット: $(git log -1 --oneline)"
log_info "  リモートURL: $(git remote get-url origin 2>/dev/null || echo '設定されていません')"

# 3. プロジェクト構造の確認と修復
log_info "📋 ステップ3: プロジェクト構造を確認中..."

# 必要なディレクトリとファイルのリスト
required_paths=(
    "go.mod"
    "Makefile"
    "Dockerfile"
    "cmd"
    "cmd/webhook"
    "cmd/webhook/main.go"
    "internal"
    "internal/webhook"
    "internal/validator"
    "internal/cert"
    "scripts"
    "manifests"
    "test"
)

missing_paths=()
for path in "${required_paths[@]}"; do
    if [[ ! -e "$path" ]]; then
        missing_paths+=("$path")
    fi
done

if [[ ${#missing_paths[@]} -gt 0 ]]; then
    log_warning "以下のファイル/ディレクトリが見つかりません:"
    for path in "${missing_paths[@]}"; do
        log_warning "  - $path"
    done
    
    # Gitから復元を試行
    log_info "Gitリポジトリからの復元を試行中..."
    
    restored_paths=()
    failed_paths=()
    
    for path in "${missing_paths[@]}"; do
        if git ls-files | grep -q "^${path}"; then
            if git checkout HEAD -- "$path" 2>/dev/null; then
                restored_paths+=("$path")
                log_success "復元成功: $path"
            else
                failed_paths+=("$path")
                log_error "復元失敗: $path"
            fi
        else
            log_warning "Gitで追跡されていません: $path"
        fi
    done
    
    if [[ ${#failed_paths[@]} -gt 0 ]]; then
        log_error "以下のファイル/ディレクトリの復元に失敗しました:"
        for path in "${failed_paths[@]}"; do
            log_error "  - $path"
        done
        log_info "💡 解決策:"
        log_info "  1. git reset --hard HEAD を実行"
        log_info "  2. git pull origin main を実行"
        log_info "  3. リポジトリを再クローン"
        exit 1
    fi
    
    if [[ ${#restored_paths[@]} -gt 0 ]]; then
        log_success "復元されたファイル/ディレクトリ: ${#restored_paths[@]}個"
    fi
fi

log_success "プロジェクト構造の確認完了"

# 4. Go依存関係の確認
log_info "📋 ステップ4: Go依存関係を確認中..."

# go.modの確認
if [[ -f "go.mod" ]]; then
    module_name=$(grep "^module" go.mod | awk '{print $2}')
    go_version=$(grep "^go" go.mod | awk '{print $2}')
    log_info "Goモジュール: $module_name"
    log_info "Go要求バージョン: $go_version"
    
    # 現在のGoバージョンとの比較
    current_go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    log_info "現在のGoバージョン: $current_go_version"
else
    log_error "go.modファイルが見つかりません"
    exit 1
fi

# 依存関係の更新
log_info "Go依存関係を更新中..."
if go mod tidy; then
    log_success "Go依存関係の更新完了"
else
    log_error "Go依存関係の更新に失敗しました"
    log_info "💡 解決策:"
    log_info "  1. ネットワーク接続を確認"
    log_info "  2. go clean -modcache を実行"
    log_info "  3. GOPROXY設定を確認: go env GOPROXY"
    exit 1
fi

# 依存関係の検証
if go mod verify; then
    log_success "Go依存関係の検証完了"
else
    log_warning "Go依存関係の検証で警告が発生しました"
fi

# 5. ビルドテスト
log_info "📋 ステップ5: ビルドテストを実行中..."

if go build -o /tmp/webhook-setup-test ./cmd/webhook; then
    log_success "ビルドテスト成功"
    rm -f /tmp/webhook-setup-test
else
    log_error "ビルドテストに失敗しました"
    log_info "💡 解決策:"
    log_info "  1. go mod tidy を再実行"
    log_info "  2. ソースコードの構文エラーを確認"
    log_info "  3. 詳細なエラー情報: go build -v ./cmd/webhook"
    exit 1
fi

# 6. Docker環境の確認
log_info "📋 ステップ6: Docker環境を確認中..."

if docker info &>/dev/null; then
    log_success "Docker環境が利用可能です"
    
    # Dockerディスク使用量の確認
    docker_usage=$(docker system df --format "table {{.Type}}\t{{.TotalCount}}\t{{.Size}}" 2>/dev/null || echo "取得できませんでした")
    log_info "Dockerディスク使用量:"
    echo "$docker_usage" | while read -r line; do
        log_info "  $line"
    done
else
    log_warning "Dockerが利用できません"
    log_info "💡 解決策:"
    log_info "  1. Dockerデーモンを起動: sudo systemctl start docker"
    log_info "  2. Dockerグループに追加: sudo usermod -aG docker $USER"
    log_info "  3. セッションを再開: newgrp docker"
fi

# 7. 証明書ディレクトリの準備
log_info "📋 ステップ7: 証明書ディレクトリを準備中..."

if [[ ! -d "certs" ]]; then
    mkdir -p certs
    log_info "certsディレクトリを作成しました"
fi

# 8. テストレポートディレクトリの準備
log_info "📋 ステップ8: テストレポートディレクトリを準備中..."

if [[ ! -d "test-reports" ]]; then
    mkdir -p test-reports
    log_info "test-reportsディレクトリを作成しました"
fi

# 9. スクリプトの実行権限確認
log_info "📋 ステップ9: スクリプトの実行権限を確認中..."

scripts_to_check=(
    "scripts/build-image.sh"
    "scripts/setup-kind-cluster.sh"
    "scripts/deploy-webhook.sh"
    "scripts/run-e2e-tests.sh"
    "scripts/generate-certs.sh"
    "scripts/verify-webhook.sh"
    "scripts/verify-rbac.sh"
    "scripts/verify-deployment.sh"
    "scripts/cleanup-test-environment.sh"
)

for script in "${scripts_to_check[@]}"; do
    if [[ -f "$script" ]]; then
        if [[ ! -x "$script" ]]; then
            chmod +x "$script"
            log_info "実行権限を付与: $script"
        fi
    else
        log_warning "スクリプトが見つかりません: $script"
    fi
done

# 10. 環境変数の確認
log_info "📋 ステップ10: 環境変数を確認中..."

log_info "重要な環境変数:"
log_info "  HOME: ${HOME:-未設定}"
log_info "  USER: ${USER:-未設定}"
log_info "  GOPATH: ${GOPATH:-未設定}"
log_info "  GOROOT: ${GOROOT:-未設定}"
log_info "  PATH: ${PATH:0:100}..."

# 11. セットアップ完了の確認
log_info "📋 ステップ11: セットアップ完了の確認..."

log_success "🎉 新しい環境でのプロジェクトセットアップが完了しました！"

log_info "📚 次のステップ:"
log_info "  1. 証明書の生成: ./scripts/generate-certs.sh"
log_info "  2. Dockerイメージのビルド: ./scripts/build-image.sh"
log_info "  3. kind環境のセットアップ: make setup-kind"
log_info "  4. E2Eテストの実行: make e2e-full"

log_info "🔧 利用可能なコマンド:"
log_info "  make help          - 利用可能なターゲットを表示"
log_info "  make test-unit     - 単体テストを実行"
log_info "  make build         - バイナリをビルド"
log_info "  make e2e-full-auto - 完全なE2Eテスト（自動ビルド付き）"

log_info "📖 ドキュメント:"
log_info "  README.md                     - プロジェクト概要と使用方法"
log_info "  docs/troubleshooting-guide.md - トラブルシューティングガイド"

log_info "🆘 問題が発生した場合:"
log_info "  1. docs/troubleshooting-guide.md を確認"
log_info "  2. DEBUG=true ./scripts/build-image.sh でデバッグ情報を表示"
log_info "  3. ./scripts/check-test-environment.sh で環境状態を確認"

echo ""
log_success "セットアップ完了！開発を開始できます。"