## ArgoCDを使用した自動デプロイ

ArgoCDを使用すると、GitOpsアプローチでWebhookをデプロイできます。これにより、Gitリポジトリの変更が自動的にクラスターに反映されます。

### 1. リポジトリの準備

```bash
# リポジトリをクローン
git clone https://github.com/example/k8s-deployment-hpa-validator.git
cd k8s-deployment-hpa-validator

# イメージのパスを更新
sed -i 's|image: k8s-deployment-hpa-validator:latest|image: your-registry.com/k8s-deployment-hpa-validator:latest|g' manifests/base/deployment.yaml

# 変更をコミットしてプッシュ
git add manifests/base/deployment.yaml
git commit -m "Update image path for production"
git push
```

### 2. TLS証明書の準備

```bash
# 証明書生成スクリプトを実行
./scripts/generate-certs.sh

# 証明書をKubernetesシークレットとして作成
kubectl create namespace webhook-system
kubectl create secret tls webhook-tls -n webhook-system --cert=certs/tls.crt --key=certs/tls.key
```

### 3. ArgoCD Application定義の作成

```bash
# argocdディレクトリを作成
mkdir -p argocd

# Application定義ファイルを作成
cat > argocd/application.yaml <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: k8s-deployment-hpa-validator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/example/k8s-deployment-hpa-validator.git
    targetRevision: HEAD
    path: manifests/overlays/production
  destination:
    server: https://kubernetes.default.svc
    namespace: webhook-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
EOF

# Application定義をデプロイ
kubectl apply -f argocd/application.yaml
```

**application.yaml の主な設定項目：**

- `repoURL`: Gitリポジトリのアドレス
- `targetRevision`: 使用するブランチまたはタグ（例：`HEAD`, `main`, `v1.0.0`）
- `path`: マニフェストファイルのパス
- `destination.namespace`: デプロイ先の名前空間
- `syncPolicy.automated`: 自動同期の設定
  - `prune`: 不要になったリソースを削除
  - `selfHeal`: クラスター内の手動変更を自動修正
- `syncOptions`: 同期オプション
  - `CreateNamespace=true`: 名前空間が存在しない場合に作成