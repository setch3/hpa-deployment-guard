# マルチステージビルドでGoアプリケーションをビルド
FROM golang:1.24.2-alpine AS builder

# 作業ディレクトリを設定
WORKDIR /app

# Go modulesファイルをコピー
COPY go.mod go.sum ./

# 依存関係をダウンロード
RUN go mod download

# ソースコードをコピー
COPY . .

# アプリケーションをビルド
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o webhook ./cmd/webhook

# 実行用の軽量イメージ
FROM alpine:latest

# セキュリティアップデートとCA証明書をインストール
RUN apk --no-cache add ca-certificates

# 非rootユーザーを作成
RUN adduser -D -s /bin/sh webhook

# 作業ディレクトリを設定
WORKDIR /home/webhook

# ビルドしたバイナリをコピーして実行権限を付与
COPY --from=builder /app/webhook ./webhook
RUN chmod +x ./webhook && chown webhook:webhook ./webhook

USER webhook

# ポートを公開
EXPOSE 8443 8080

# アプリケーションを実行
ENTRYPOINT ["./webhook"]