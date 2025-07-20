### 4. マニフェストの編集

デプロイ前に、マニフェストファイルを環境に合わせて編集する必要があります。

```bash
# イメージのパスを更新
sed -i 's|image: k8s-deployment-hpa-validator:latest|image: your-registry.com/k8s-deployment-hpa-validator:latest|g' manifests/base/deployment.yaml

# 環境に合わせて設定を調整（必要に応じて）
vi configs/production.yaml
```

**configs/production.yaml の主な設定項目：**

```yaml
# 本番環境設定
environment: production

# サーバー設定
port: 8443
timeout: 30s

# ログ設定
log_level: warn
log_format: json

# バリデーション設定
skip_namespaces:
  - kube-system
  - kube-public
  - kube-node-lease
  - cert-manager
  - monitoring
  - istio-system

# 失敗ポリシー
failure_policy: Fail
```

### 5. Kubernetesリソースのデプロイ

```bash
# 名前空間の作成
kubectl create namespace webhook-system

# TLS証明書のシークレットを作成
kubectl create secret tls webhook-tls -n webhook-system --cert=certs/tls.crt --key=certs/tls.key

# 設定のConfigMapを作成
kubectl create configmap webhook-config -n webhook-system --from-file=configs/production.yaml

# マニフェストのデプロイ
kubectl apply -k manifests/overlays/production
```

**デプロイされるリソース：**
- Deployment: Webhookサーバーのポッド
- Service: Webhookサーバーへのネットワークアクセス
- ServiceAccount: Webhookサーバーの認証
- ClusterRole & ClusterRoleBinding: 必要な権限
- ValidatingWebhookConfiguration: Webhookの登録

### 6. デプロイの確認

```bash
# Podの状態確認
kubectl get pods -n webhook-system

# Webhookの設定確認
kubectl get validatingwebhookconfigurations

# ログの確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator
```

### 7. 動作確認

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

# エラーメッセージを確認
# "1 replicaのDeploymentを対象とするHPAは作成できません" というメッセージが表示されるはず
```