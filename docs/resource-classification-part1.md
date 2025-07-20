# 本番環境で使用されるリソース

このドキュメントでは、Kubernetes Deployment-HPA Validatorの本番環境で実際に使用されるリソースについて説明します。

## 1. コアコンポーネント

| リソース | 説明 | パス |
|---------|------|------|
| Webhookバイナリ | コンパイルされたGoバイナリ | `webhook` |
| コンテナイメージ | Webhookサーバーのコンテナイメージ | `k8s-deployment-hpa-validator:latest` |

## 2. Kubernetesマニフェスト

```
manifests/
├── base/                  # 基本設定
│   ├── deployment.yaml    # Webhookのデプロイメント定義
│   ├── kustomization.yaml # Kustomize設定
│   ├── rbac.yaml          # 権限設定
│   ├── service.yaml       # サービス定義
│   └── webhook.yaml       # ValidatingWebhookConfiguration
└── overlays/              # 環境別オーバーレイ
    ├── development/       # 開発環境設定
    ├── production/        # 本番環境設定
    └── staging/           # ステージング環境設定
```

### 2.1 主要なファイル

- **deployment.yaml**: Webhookサーバーのポッド設定（コンテナイメージ、リソース制限など）
- **service.yaml**: Webhookサーバーへのネットワークアクセス設定
- **webhook.yaml**: ValidatingWebhookConfigurationリソース（どのリクエストをチェックするか）
- **rbac.yaml**: 必要な権限設定（ServiceAccount、Role、RoleBinding）

## 3. 設定ファイル

```
configs/
├── development.yaml  # 開発環境設定
├── production.yaml   # 本番環境設定
└── staging.yaml      # ステージング環境設定
```

各環境ごとに異なる設定を定義しています：

- **production.yaml**: 本番環境用の厳格な設定（例：ログレベル=warn、失敗ポリシー=Fail）
- **staging.yaml**: ステージング環境用の設定（本番に近いが一部緩和）
- **development.yaml**: 開発環境用の設定（デバッグ情報多め、失敗ポリシー=Ignore）

## 4. TLS証明書

```
certs/
├── tls.crt  # TLS証明書
└── tls.key  # TLS秘密鍵
```

Webhookサーバーとの安全な通信に使用されます。

## 5. ArgoCD設定

```
argocd/
├── application.yaml              # 基本のArgoCD Application定義
├── application-development.yaml  # 開発環境用Application定義
├── application-staging.yaml      # ステージング環境用Application定義
└── application-production.yaml   # 本番環境用Application定義
```

GitOpsによる自動デプロイのための設定ファイルです。