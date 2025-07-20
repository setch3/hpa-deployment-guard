### 2. メトリクスの確認

Webhookはメトリクスを公開しています。これらを確認するには：

```bash
# ポートフォワードを設定
kubectl port-forward -n webhook-system svc/k8s-deployment-hpa-validator-metrics 8080:8080

# メトリクスを取得
curl http://localhost:8080/metrics
```

主なメトリクス：
- `webhook_requests_total`: リクエスト数
- `webhook_request_duration_seconds`: リクエスト処理時間
- `webhook_validation_errors_total`: バリデーションエラー数
- `webhook_certificate_expiry_days`: 証明書の有効期限までの日数

### 3. ヘルスチェックの確認

```bash
# ポートフォワードを設定
kubectl port-forward -n webhook-system svc/k8s-deployment-hpa-validator-metrics 8080:8080

# ヘルスチェックエンドポイントにアクセス
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

両方のエンドポイントが `OK` を返せば、Webhookは正常に動作しています。

## トラブルシューティング

### 一般的な問題と解決策

#### 1. Webhookが応答しない

**症状**: リソースの作成・更新時にタイムアウトが発生する

**確認方法**:
```bash
# Podの状態確認
kubectl get pods -n webhook-system

# ログの確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator
```

**解決策**:
- Podが実行中であることを確認
- サービスとエンドポイントが正しく設定されていることを確認
- TLS証明書が正しく設定されていることを確認

#### 2. 証明書エラー

**症状**: `x509: certificate signed by unknown authority` エラーが発生

**確認方法**:
```bash
# 証明書の内容確認
kubectl get secret webhook-tls -n webhook-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text

# ValidatingWebhookConfigurationのCABundle確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml
```

**解決策**:
- 証明書を再生成
- ValidatingWebhookConfigurationのcaBundle値を更新

#### 3. 権限エラー

**症状**: `forbidden: user cannot list resource` エラーがログに表示される

**確認方法**:
```bash
# RBAC設定の確認
kubectl get clusterrole k8s-deployment-hpa-validator -o yaml
kubectl get clusterrolebinding k8s-deployment-hpa-validator -o yaml
```

**解決策**:
- RBACマニフェストを修正して必要な権限を追加
- ServiceAccountが正しく設定されていることを確認

#### 4. 設定の問題

**症状**: 期待通りの動作をしない（例：特定のnamespaceでWebhookが動作しない）

**確認方法**:
```bash
# 設定の確認
kubectl get configmap webhook-config -n webhook-system -o yaml

# Webhookの設定確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml
```

**解決策**:
- ConfigMapの内容を修正
- ValidatingWebhookConfigurationのrules設定を確認・修正