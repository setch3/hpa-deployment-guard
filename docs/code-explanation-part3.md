## 5. バリデーションロジック (internal/validator/validator.go)

バリデーションロジックは、このWebhookの核心部分です。

### 5.1 バリデーター構造体

```go
// DeploymentHPAValidator implements the Validator interface
type DeploymentHPAValidator struct {
    client kubernetes.Interface
}

// NewDeploymentHPAValidator creates a new validator instance
func NewDeploymentHPAValidator(client kubernetes.Interface) *DeploymentHPAValidator {
    return &DeploymentHPAValidator{
        client: client,
    }
}
```

**解説：**
- `DeploymentHPAValidator` 構造体は Kubernetes クライアントを保持
- `NewDeploymentHPAValidator` 関数でバリデーターのインスタンスを作成

### 5.2 HPA検証ロジック

```go
// ValidateHPA はHPAリソースを検証します
func (v *DeploymentHPAValidator) ValidateHPA(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
    // HPAのターゲットがDeploymentかどうか確認
    if hpa.Spec.ScaleTargetRef.Kind != "Deployment" {
        return nil // Deployment以外はチェックしない
    }
    
    // ターゲットDeploymentを取得
    deployment, err := v.client.AppsV1().Deployments(hpa.Namespace).Get(
        ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
    
    // Deploymentが見つからない場合はエラーにしない（後で作成される可能性）
    if err != nil && errors.IsNotFound(err) {
        return nil
    }
    
    // その他のエラーの場合
    if err != nil {
        return fmt.Errorf("ターゲットDeploymentの取得に失敗しました: %w", err)
    }
    
    // Deploymentのレプリカ数が1の場合はエラー
    if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 1 {
        return fmt.Errorf("1 replicaのDeploymentを対象とするHPAは作成できません。" +
            "Deploymentのreplicasを2以上に設定してください。")
    }
    
    return nil // 問題なし
}
```

**解説：**
1. HPAのターゲットがDeploymentかどうか確認
2. ターゲットのDeploymentをKubernetes APIから取得
3. Deploymentが見つからない場合は許可（後で作成される可能性）
4. Deploymentのレプリカ数が1の場合はエラーを返す
5. 問題なければnilを返す（許可）