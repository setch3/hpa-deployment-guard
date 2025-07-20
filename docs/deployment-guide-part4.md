### 4. ArgoCDダッシュボードでの確認

1. ArgoCDのWebインターフェースにアクセス
   ```bash
   # ポートフォワードを使用する場合
   kubectl port-forward svc/argocd-server -n argocd 8080:443
   
   # ブラウザで https://localhost:8080 にアクセス
   ```

2. `k8s-deployment-hpa-validator` アプリケーションが表示されていることを確認
3. 同期ステータスが「Synced」になっていることを確認
4. ヘルスステータスが「Healthy」になっていることを確認

![ArgoCD Dashboard](https://argoproj.github.io/argo-cd/assets/argocd-ui.png)

### 5. 同期の手動トリガー（必要な場合）

```bash
# ArgoCDコマンドラインツールを使用
argocd app sync k8s-deployment-hpa-validator

# または、ArgoCDのWebインターフェースで「SYNC」ボタンをクリック
```

### 6. デプロイの確認

```bash
# Podの状態確認
kubectl get pods -n webhook-system

# Webhookの設定確認
kubectl get validatingwebhookconfigurations

# ログの確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator
```

### 7. 更新の適用

コードや設定を更新する場合：

```bash
# 変更を加える
vi configs/production.yaml

# 変更をコミットしてプッシュ
git add configs/production.yaml
git commit -m "Update production configuration"
git push

# ArgoCDが自動的に変更を検出して適用（自動同期が有効な場合）
# または手動で同期をトリガー
argocd app sync k8s-deployment-hpa-validator
```

## デプロイ後の確認

### 1. Webhookの動作確認

```bash
# テスト用のDeploymentを作成（1レプリカ）
kubectl create deployment test-deployment --image=nginx --replicas=1

# HPAを作成（拒否されるはず）
cat <<EOF | kubectl apply -f -
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: test-hpa
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
        averageUtilization: 80
EOF
```

エラーメッセージが表示されれば、Webhookは正常に動作しています。