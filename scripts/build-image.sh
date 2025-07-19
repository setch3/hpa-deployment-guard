#!/bin/bash

# Docker イメージビルドスクリプト
# HPA Deployment Validator用のコンテナイメージを作成

set -euo pipefail

# 設定
IMAGE_NAME="hpa-validator"
IMAGE_TAG="latest"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
DOCKERFILE="Dockerfile"

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
    
    # Dockerの動作確認
    if ! docker info &> /dev/null; then
        log_error "Dockerが動作していません。Dockerを起動してください。"
        exit 1
    fi
    
    # Dockerfileの存在確認
    if [[ ! -f "${DOCKERFILE}" ]]; then
        log_error "Dockerfile が見つかりません。プロジェクトルートで実行してください。"
        exit 1
    fi
    
    # go.modの存在確認
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod が見つかりません。プロジェクトルートで実行してください。"
        exit 1
    fi
    
    log_info "前提条件チェック完了"
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
    log_info "Goアプリケーションのビルドテストを実行しています..."
    
    # テストの実行
    if ! go test ./...; then
        log_error "テストが失敗しました。ビルドを中止します。"
        exit 1
    fi
    
    # ビルドテスト
    if ! go build -o /tmp/webhook-test ./cmd/webhook; then
        log_error "Goアプリケーションのビルドに失敗しました。"
        exit 1
    fi
    
    # テスト用バイナリの削除
    rm -f /tmp/webhook-test
    
    log_info "ビルドテスト完了"
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

# メイン処理
main() {
    log_info "HPA Deployment Validator Dockerイメージのビルドを開始します"
    
    check_prerequisites
    cleanup_existing_image
    test_build
    build_image
    verify_image
    show_image_info
    
    log_info "Dockerイメージのビルドが完了しました！"
    log_info "次のステップ: ./scripts/deploy-webhook.sh を実行してkind環境にデプロイしてください"
}

# スクリプト実行
main "$@"