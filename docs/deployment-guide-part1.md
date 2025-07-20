# Kubernetes Deployment-HPA Validator デプロイガイド

このガイドでは、Kubernetes Deployment-HPA Validatorを本番環境にデプロイする手順を説明します。

## 目次

1. [前提条件](#前提条件)
2. [kubectl を使用した直接デプロイ](#kubectl-を使用した直接デプロイ)
3. [ArgoCDを使用した自動デプロイ](#argocdを使用した自動デプロイ)
4. [デプロイ後の確認](#デプロイ後の確認)
5. [トラブルシューティング](#トラブルシューティング)

## 前提条件

### 必要なツール

- `kubectl` コマンドラインツール（バージョン1.18以上）
- `docker` または `podman` などのコンテナビルドツール
- （ArgoCDを使用する場合）`argocd` コマンドラインツール

### アクセス権

- Kubernetesクラスターへの管理者アクセス権
- コンテナレジストリへのプッシュ権限
- （GitOpsを使用する場合）Gitリポジトリへの書き込み権限

## kubectl を使用した直接デプロイ

### 1. リポジトリのクローン

```bash
# リポジトリをクローン
git clone https://github.com/example/k8s-deployment-hpa-validator.git
cd k8s-deployment-hpa-validator
```

### 2. TLS証明書の生成

Webhookサーバーには、Kubernetes APIサーバーとの安全な通信のためにTLS証明書が必要です。

```bash
# 証明書生成スクリプトを実行
./scripts/generate-certs.sh

# 証明書が生成されたことを確認
ls -la certs/
```

このスクリプトは以下のファイルを生成します：
- `certs/tls.crt`: TLS証明書
- `certs/tls.key`: TLS秘密鍵
- `certs/ca.crt`: CA証明書（ValidatingWebhookConfigurationで使用）

### 3. コンテナイメージのビルドとプッシュ

```bash
# イメージをビルド
./scripts/build-image.sh

# イメージをレジストリにプッシュ（必要に応じてタグを変更）
docker tag k8s-deployment-hpa-validator:latest your-registry.com/k8s-deployment-hpa-validator:latest
docker push your-registry.com/k8s-deployment-hpa-validator:latest
```

**注意**: `your-registry.com` は実際のコンテナレジストリのアドレスに置き換えてください。