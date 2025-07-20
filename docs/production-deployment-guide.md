# 本番デプロイメントガイド

## 概要

このドキュメントは、Kubernetes validating admission webhook（k8s-deployment-hpa-validator）をArgoCDを使用して本番環境にデプロイする手順を説明します。

## 前提条件

### 必要なツール
- `kubectl` - Kubernetesクラスターへのアクセス
- `argocd` CLI - ArgoCDの操作
- `git` - ソースコード管理

### 必要な権限
- Kubernetesクラスターの管理者権限
- ArgoCDプロジェクトへのアクセス権限
- Gitリポジトリへの読み取り権限

### 環境要件
- Kubernetes 1.20以上
- ArgoCD 2.0以上
- cert-manager（TLS証明書の自動管理用）

## デプロイ手順

### ステップ1: 事前準備

#### 1.1 Kubernetesクラスターの確認
```bash
# クラスターの接続確認
kubectl cluster-info

# ノードの状態確認
kubectl get nodes

# cert-managerの確認
kubectl get pods -n cert-manager
```

#### 1.2 ArgoCDの確認
```bash
# ArgoCDサーバーへのログイン
argocd login <argocd-server-url>

# プロジェクトの確認
argocd proj list
```

#### 1.3 Gitリポジトリの準備
```bash
# リポジトリのクローン
git clone <repository-url>
cd k8s-deployment-hpa-validator

# 最新のコードを取得
git pull origin main
```

### ステップ2: 名前空間の作成

```bash
# webhook用の名前空間を作成
kubectl create namespace webhook-system

# 名前空間にラベルを追加（監視用）
kubectl label namespace webhook-system monitoring=enabled
```

### ステップ3: ArgoCDアプリケーションの作成

#### 3.1 アプリケーション定義の確認
```bash
# アプリケーション定義ファイルの確認
cat argocd/application.yaml
```

#### 3.2 アプリケーションの作成
```bash
# ArgoCDアプリケーションを作成
kubectl apply -f argocd/application.yaml

# または、argocd CLIを使用
argocd app create k8s-deployment-hpa-validator \
  --repo <repository-url> \
  --path manifests/overlays/production \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace webhook-system \
  --sync-policy automated \
  --auto-prune \
  --self-heal
```

### ステップ4: デプロイメントの実行

#### 4.1 初回同期の実行
```bash
# アプリケーションの同期を実行
argocd app sync k8s-deployment-hpa-validator

# 同期完了まで待機
argocd app wait k8s-deployment-hpa-validator --timeout 300
```

#### 4.2 デプロイメント状態の確認
```bash
# ArgoCDでの状態確認
argocd app get k8s-deployment-hpa-validator

# Kubernetesでの状態確認
kubectl get all -n webhook-system
```

### ステップ5: デプロイメント検証

#### 5.1 Podの状態確認
```bash
# Podの状態を確認
kubectl get pods -n webhook-system -l app=k8s-deployment-hpa-validator

# Podのログを確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --tail=50
```

#### 5.2 サービスの確認
```bash
# サービスの状態確認
kubectl get svc -n webhook-system

# エンドポイントの確認
kubectl get endpoints -n webhook-system
```

#### 5.3 ValidatingWebhookConfigurationの確認
```bash
# Webhook設定の確認
kubectl get validatingwebhookconfiguration k8s-deployment-hpa-validator

# Webhook設定の詳細確認
kubectl describe validatingwebhookconfiguration k8s-deployment-hpa-validator
```

#### 5.4 TLS証明書の確認
```bash
# 証明書Secretの確認
kubectl get secret -n webhook-system webhook-tls-secret

# 証明書の有効期限確認
kubectl get certificate -n webhook-system webhook-tls
```

### ステップ6: 機能テスト

#### 6.1 基本的な機能テスト
```bash
# テスト用のDeploymentを作成（1レプリカ）
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
EOF

# HPAを作成してwebhookが動作することを確認
kubectl apply -f - <<EOF
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
  namespace: default
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: test-deployment
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
EOF
```

期待される結果：HPAの作成が拒否され、適切なエラーメッセージが表示される

#### 6.2 テストリソースのクリーンアップ
```bash
# テストリソースを削除
kubectl delete hpa test-hpa -n default --ignore-not-found
kubectl delete deployment test-deployment -n default --ignore-not-found
```

## 段階的ロールアウト手順

### フェーズ1: 開発環境でのテスト
1. 開発環境でのデプロイメント実行
2. 基本機能テストの実行
3. ログとメトリクスの確認

### フェーズ2: ステージング環境でのテスト
1. ステージング環境でのデプロイメント実行
2. 負荷テストの実行
3. 統合テストの実行

### フェーズ3: 本番環境でのデプロイメント
1. メンテナンス時間の設定
2. 本番環境でのデプロイメント実行
3. 段階的な検証の実行
4. 監視とアラートの確認

## ロールバック手順

### 緊急時のロールバック
```bash
# ArgoCDでの前のバージョンへのロールバック
argocd app rollback k8s-deployment-hpa-validator <previous-revision>

# または、Webhook設定を無効化
kubectl patch validatingwebhookconfiguration k8s-deployment-hpa-validator \
  --type='json' \
  -p='[{"op": "replace", "path": "/webhooks/0/failurePolicy", "value": "Ignore"}]'
```

### 完全な削除
```bash
# ArgoCDアプリケーションの削除
argocd app delete k8s-deployment-hpa-validator --cascade

# 手動でのリソース削除
kubectl delete validatingwebhookconfiguration k8s-deployment-hpa-validator
kubectl delete namespace webhook-system
```

## 検証チェックリスト

### デプロイメント後の必須確認項目
- [ ] Podが正常に起動している
- [ ] サービスが正しく作成されている
- [ ] ValidatingWebhookConfigurationが適用されている
- [ ] TLS証明書が有効である
- [ ] ヘルスチェックエンドポイントが応答する
- [ ] メトリクスエンドポイントが応答する
- [ ] ログが正常に出力されている
- [ ] 基本的な機能テストが成功する

### 本番環境固有の確認項目
- [ ] リソース制限が適切に設定されている
- [ ] セキュリティ設定が適用されている
- [ ] 監視とアラートが設定されている
- [ ] バックアップとリストア手順が準備されている

## トラブルシューティング

### よくある問題と解決方法

#### Podが起動しない
```bash
# Podの詳細確認
kubectl describe pod -n webhook-system -l app=k8s-deployment-hpa-validator

# イベントの確認
kubectl get events -n webhook-system --sort-by='.lastTimestamp'
```

#### TLS証明書の問題
```bash
# 証明書の状態確認
kubectl describe certificate -n webhook-system webhook-tls

# cert-managerのログ確認
kubectl logs -n cert-manager -l app=cert-manager
```

#### Webhookが応答しない
```bash
# サービスの接続確認
kubectl port-forward -n webhook-system svc/k8s-deployment-hpa-validator 8443:443

# 別のターミナルで接続テスト
curl -k https://localhost:8443/healthz
```

## 参考資料

- [ArgoCD公式ドキュメント](https://argo-cd.readthedocs.io/)
- [Kubernetes Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
- [cert-manager Documentation](https://cert-manager.io/docs/)