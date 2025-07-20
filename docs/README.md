# Kubernetes Deployment-HPA Validator ドキュメント

このディレクトリには、Kubernetes Deployment-HPA Validatorの詳細なドキュメントが含まれています。

## 主要ドキュメント

### [ユーザーガイド](user-guide.md)
概要、リソース区分け、Webhookの仕組み、デプロイ手順の概要を説明しています。初めての方はこちらから読むことをお勧めします。

### [コード解説](code-explanation.md)
Webhookのコードを初学者にもわかりやすく解説しています。全体構造、処理の流れ、各コンポーネントの役割と実装を詳しく説明しています。

### [デプロイガイド](deployment-guide.md)
本番環境へのデプロイ手順を説明しています。kubectlを使用した直接デプロイとArgoCDを使用した自動デプロイの両方の手順を含みます。

### [リソース分類](resource-classification.md)
本番環境で使用されるリソースとテスト用リソースの区分けを説明しています。各リソースの役割と配置場所を整理しています。

## 詳細ドキュメント

各主要ドキュメントは、読みやすさのために複数のパートに分割されています：

### コード解説
- [コード解説 - パート1](code-explanation-part1.md) - 全体構造、処理の流れ、メインエントリーポイント
- [コード解説 - パート2](code-explanation-part2.md) - 設定管理
- [コード解説 - パート3](code-explanation-part3.md) - バリデーションロジック（HPA）
- [コード解説 - パート4](code-explanation-part4.md) - バリデーションロジック（Deployment）、Webhookサーバー構造
- [コード解説 - パート5](code-explanation-part5.md) - サーバー初期化
- [コード解説 - パート6](code-explanation-part6.md) - サーバー起動、リクエスト処理
- [コード解説 - パート7](code-explanation-part7.md) - HPA検証処理
- [コード解説 - パート8](code-explanation-part8.md) - 証明書管理
- [コード解説 - パート9](code-explanation-part9.md) - メトリクス収集
- [コード解説 - パート10](code-explanation-part10.md) - リクエストメトリクス、ロギング

### デプロイガイド
- [デプロイガイド - パート1](deployment-guide-part1.md) - 前提条件、kubectl直接デプロイ（準備）
- [デプロイガイド - パート2](deployment-guide-part2.md) - kubectl直接デプロイ（実行）
- [デプロイガイド - パート3](deployment-guide-part3.md) - ArgoCDを使用した自動デプロイ（準備）
- [デプロイガイド - パート4](deployment-guide-part4.md) - ArgoCDを使用した自動デプロイ（実行）
- [デプロイガイド - パート5](deployment-guide-part5.md) - デプロイ後の確認、トラブルシューティング

### リソース分類
- [リソース分類 - パート1](resource-classification.md) - 本番環境で使用されるリソース
- [リソース分類 - パート2](resource-classification-part2.md) - テスト用リソース、区分けの基準

## クイックスタート

### 直接デプロイ

```bash
# リポジトリのクローン
git clone https://github.com/example/k8s-deployment-hpa-validator.git
cd k8s-deployment-hpa-validator

# 証明書生成
./scripts/generate-certs.sh

# デプロイ
kubectl create namespace webhook-system
kubectl create secret tls webhook-tls -n webhook-system --cert=certs/tls.crt --key=certs/tls.key
kubectl apply -k manifests/overlays/production
```

### ArgoCDデプロイ

```bash
# 証明書生成
./scripts/generate-certs.sh

# 証明書をシークレットとして作成
kubectl create namespace webhook-system
kubectl create secret tls webhook-tls -n webhook-system --cert=certs/tls.crt --key=certs/tls.key

# ArgoCDアプリケーションをデプロイ
kubectl apply -f argocd/application.yaml
```

詳細な手順については、各ドキュメントを参照してください。