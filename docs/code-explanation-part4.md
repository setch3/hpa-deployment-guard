### 5.3 Deployment検証ロジック

```go
// ValidateDeployment はDeploymentリソースを検証します
func (v *DeploymentHPAValidator) ValidateDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
    // レプリカ数が1でない場合はチェック不要
    if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
        return nil
    }
    
    // このDeploymentをターゲットとするHPAを検索
    hpaList, err := v.client.AutoscalingV2().HorizontalPodAutoscalers("").List(
        ctx, metav1.ListOptions{})
    if err != nil {
        return fmt.Errorf("HPAリストの取得に失敗しました: %w", err)
    }
    
    // 各HPAをチェック
    for _, hpa := range hpaList.Items {
        // このDeploymentをターゲットとするHPAが見つかった場合
        if hpa.Spec.ScaleTargetRef.Kind == "Deployment" &&
           hpa.Spec.ScaleTargetRef.Name == deployment.Name &&
           hpa.Namespace == deployment.Namespace {
            return fmt.Errorf("HPAが存在するDeploymentのreplicasを1に設定することはできません。" +
                "HPAを削除するか、replicas数を2以上に設定してください。")
        }
    }
    
    return nil // 問題なし
}
```

**解説：**
1. Deploymentのレプリカ数が1でない場合は許可（チェック不要）
2. クラスター内の全HPAをリスト
3. 各HPAをチェックし、このDeploymentをターゲットとするものがあるか確認
4. HPAが見つかった場合はエラーを返す
5. 問題なければnilを返す（許可）

## 6. Webhookサーバー (internal/webhook/server.go)

### 6.1 サーバー構造体

```go
// Server represents the webhook server
type Server struct {
    server       *http.Server
    client       kubernetes.Interface
    validator    validator.Validator
    scheme       *runtime.Scheme
    codecs       serializer.CodecFactory
    certManager  *cert.Manager
    logger       *logging.Logger
    config       *config.WebhookConfig
    errorHandler *ErrorHandler
}
```

**解説：**
- `server`: HTTPサーバー
- `client`: Kubernetesクライアント
- `validator`: バリデーションロジック
- `scheme`, `codecs`: Kubernetesオブジェクトのシリアライズ/デシリアライズ用
- `certManager`: TLS証明書管理
- `logger`: ロガー
- `config`: Webhook設定
- `errorHandler`: エラーハンドリング