# Kubernetes Deployment-HPA Validator ドキュメント

## はじめに

Kubernetes Deployment-HPA Validatorは、DeploymentとHorizontalPodAutoscaler(HPA)の間の設定の不整合を防ぐためのValidating Admission Webhookです。このドキュメントでは、Webhookの使い方、内部の仕組み、デプロイ方法について説明します。

## 主要ドキュメント

### [ユーザーガイド](user-guide.md)
概要、リソース区分け、Webhookの仕組み、デプロイ手順の概要を説明しています。初めての方はこちらから読むことをお勧めします。

### [コード解説](code-explanation.md)
Webhookのコードを初学者にもわかりやすく解説しています。全体構造、処理の流れ、各コンポーネントの役割と実装を詳しく説明しています。

### [デプロイガイド](deployment-guide.md)
本番環境へのデプロイ手順を説明しています。kubectlを使用した直接デプロイとArgoCDを使用した自動デプロイの両方の手順を含みます。

### [リソース分類](resource-classification.md)
本番環境で使用されるリソースとテスト用リソースの区分けを説明しています。各リソースの役割と配置場所を整理しています。

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