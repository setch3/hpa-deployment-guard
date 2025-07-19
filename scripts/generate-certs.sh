#!/bin/bash

# TLS証明書生成スクリプト
# k8s-deployment-hpa-validator用の自己署名証明書を生成

set -euo pipefail

# 設定
SERVICE_NAME="k8s-deployment-hpa-validator"
NAMESPACE="default"
CERT_DIR="./certs"
DAYS_VALID=365

# 色付きログ出力
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 証明書ディレクトリの作成
create_cert_dir() {
    log_info "証明書ディレクトリを作成中: $CERT_DIR"
    mkdir -p "$CERT_DIR"
}

# CA証明書の生成
generate_ca() {
    log_info "CA証明書を生成中..."
    
    # CA秘密鍵の生成
    openssl genrsa -out "$CERT_DIR/ca.key" 2048
    
    # CA証明書の生成
    openssl req -new -x509 -days $DAYS_VALID -key "$CERT_DIR/ca.key" \
        -out "$CERT_DIR/ca.crt" \
        -subj "/C=JP/ST=Tokyo/L=Tokyo/O=k8s-deployment-hpa-validator/CN=ca"
    
    log_info "CA証明書が生成されました: $CERT_DIR/ca.crt"
}

# サーバー証明書の生成
generate_server_cert() {
    log_info "サーバー証明書を生成中..."
    
    # サーバー秘密鍵の生成
    openssl genrsa -out "$CERT_DIR/tls.key" 2048
    
    # CSR設定ファイルの作成
    cat > "$CERT_DIR/server.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = JP
ST = Tokyo
L = Tokyo
O = k8s-deployment-hpa-validator
CN = $SERVICE_NAME.$NAMESPACE.svc.cluster.local

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $SERVICE_NAME
DNS.2 = $SERVICE_NAME.$NAMESPACE
DNS.3 = $SERVICE_NAME.$NAMESPACE.svc
DNS.4 = $SERVICE_NAME.$NAMESPACE.svc.cluster.local
DNS.5 = localhost
IP.1 = 127.0.0.1
EOF
    
    # CSRの生成
    openssl req -new -key "$CERT_DIR/tls.key" \
        -out "$CERT_DIR/server.csr" \
        -config "$CERT_DIR/server.conf"
    
    # サーバー証明書の生成
    openssl x509 -req -in "$CERT_DIR/server.csr" \
        -CA "$CERT_DIR/ca.crt" \
        -CAkey "$CERT_DIR/ca.key" \
        -CAcreateserial \
        -out "$CERT_DIR/tls.crt" \
        -days $DAYS_VALID \
        -extensions v3_req \
        -extfile "$CERT_DIR/server.conf"
    
    log_info "サーバー証明書が生成されました: $CERT_DIR/tls.crt"
}

# 証明書の検証
verify_certificates() {
    log_info "証明書を検証中..."
    
    # CA証明書の検証
    if openssl x509 -in "$CERT_DIR/ca.crt" -text -noout > /dev/null 2>&1; then
        log_info "CA証明書の検証: OK"
    else
        log_error "CA証明書の検証: 失敗"
        return 1
    fi
    
    # サーバー証明書の検証
    if openssl x509 -in "$CERT_DIR/tls.crt" -text -noout > /dev/null 2>&1; then
        log_info "サーバー証明書の検証: OK"
    else
        log_error "サーバー証明書の検証: 失敗"
        return 1
    fi
    
    # 証明書チェーンの検証
    if openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/tls.crt" > /dev/null 2>&1; then
        log_info "証明書チェーンの検証: OK"
    else
        log_error "証明書チェーンの検証: 失敗"
        return 1
    fi
    
    # 証明書の詳細表示
    log_info "証明書の詳細:"
    echo "CA証明書:"
    openssl x509 -in "$CERT_DIR/ca.crt" -text -noout | grep -E "(Subject:|Not Before|Not After)"
    echo ""
    echo "サーバー証明書:"
    openssl x509 -in "$CERT_DIR/tls.crt" -text -noout | grep -E "(Subject:|Not Before|Not After|DNS:|IP Address)"
}

# kind環境用のCA証明書をbase64エンコード
generate_ca_bundle() {
    log_info "kind環境用のCA証明書バンドルを生成中..."
    
    CA_BUNDLE=$(base64 < "$CERT_DIR/ca.crt" | tr -d '\n')
    
    cat > "$CERT_DIR/ca-bundle.yaml" <<EOF
# ValidatingAdmissionWebhookConfiguration用のCA証明書バンドル
# 以下の値をcaBundle フィールドにコピーしてください
caBundle: $CA_BUNDLE
EOF
    
    log_info "CA証明書バンドルが生成されました: $CERT_DIR/ca-bundle.yaml"
}

# 証明書ファイルの権限設定
set_permissions() {
    log_info "証明書ファイルの権限を設定中..."
    
    chmod 600 "$CERT_DIR"/*.key
    chmod 644 "$CERT_DIR"/*.crt
    chmod 644 "$CERT_DIR"/*.yaml
    
    log_info "権限設定完了"
}

# クリーンアップ関数
cleanup() {
    log_info "一時ファイルをクリーンアップ中..."
    rm -f "$CERT_DIR/server.csr" "$CERT_DIR/server.conf" "$CERT_DIR/ca.srl"
}

# メイン処理
main() {
    log_info "TLS証明書生成を開始します..."
    log_info "サービス名: $SERVICE_NAME"
    log_info "名前空間: $NAMESPACE"
    log_info "有効期間: ${DAYS_VALID}日"
    
    # 既存の証明書ディレクトリがある場合の確認
    if [ -d "$CERT_DIR" ] && [ "$(ls -A $CERT_DIR 2>/dev/null)" ]; then
        log_warn "既存の証明書が見つかりました: $CERT_DIR"
        read -p "既存の証明書を削除して新しく生成しますか? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CERT_DIR"
        else
            log_info "証明書生成をキャンセルしました"
            exit 0
        fi
    fi
    
    create_cert_dir
    generate_ca
    generate_server_cert
    verify_certificates
    generate_ca_bundle
    set_permissions
    cleanup
    
    log_info "TLS証明書の生成が完了しました!"
    log_info "生成されたファイル:"
    ls -la "$CERT_DIR"
}

# スクリプト実行
main "$@"