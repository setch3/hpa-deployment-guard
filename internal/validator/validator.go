package validator

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

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

// ValidateDeployment validates a Deployment resource
func (v *DeploymentHPAValidator) ValidateDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	// Check if deployment has 1 replica
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 1 {
		// Search for HPAs that target this deployment
		hpa, err := v.findHPAForDeployment(ctx, deployment)
		if err != nil {
			return NewKubernetesAPIError("HPA検索", err).WithContext(
				"", "Deployment", deployment.Name, deployment.Namespace,
			)
		}
		if hpa != nil {
			return NewDeploymentHPAConflictError().WithContext(
				"", "Deployment", deployment.Name, deployment.Namespace,
			)
		}
	}
	return nil
}

// ValidateHPA validates an HPA resource
func (v *DeploymentHPAValidator) ValidateHPA(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	// Check if HPA targets a Deployment
	if hpa.Spec.ScaleTargetRef.Kind != "Deployment" {
		return nil // Only validate HPAs that target Deployments
	}

	// Get the target deployment
	deployment, err := v.getTargetDeployment(ctx, hpa)
	if err != nil {
		return NewKubernetesAPIError("Deployment取得", err).WithContext(
			"", "HorizontalPodAutoscaler", hpa.Name, hpa.Namespace,
		)
	}

	// Check if target deployment has 1 replica
	if deployment != nil && deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 1 {
		return NewHPASingleReplicaError().WithContext(
			"", "HorizontalPodAutoscaler", hpa.Name, hpa.Namespace,
		)
	}

	return nil
}

// findHPAForDeployment searches for HPAs that target the given deployment
func (v *DeploymentHPAValidator) findHPAForDeployment(ctx context.Context, deployment *appsv1.Deployment) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	hpaList, err := v.client.AutoscalingV2().HorizontalPodAutoscalers(deployment.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" &&
			hpa.Spec.ScaleTargetRef.Name == deployment.Name {
			return &hpa, nil
		}
	}
	return nil, nil
}

// getTargetDeployment retrieves the deployment targeted by the given HPA
func (v *DeploymentHPAValidator) getTargetDeployment(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) (*appsv1.Deployment, error) {
	deployment, err := v.client.AppsV1().Deployments(hpa.Namespace).Get(ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		// If deployment doesn't exist, return nil without error (it might be created later)
		return nil, nil
	}
	return deployment, nil
}

// CreateValidationResult creates a ValidationResult based on error
func CreateValidationResult(err error) ValidationResult {
	if err != nil {
		// WebhookErrorの場合は適切なHTTPステータスコードを使用
		if webhookErr, ok := err.(*WebhookError); ok {
			return ValidationResult{
				Allowed: false,
				Message: webhookErr.Error(),
				Code:    int32(webhookErr.GetHTTPStatusCode()),
			}
		}
		// 通常のerrorの場合はデフォルトの400を使用
		return ValidationResult{
			Allowed: false,
			Message: err.Error(),
			Code:    400,
		}
	}
	return ValidationResult{
		Allowed: true,
		Message: "",
		Code:    200,
	}
}

// CreateValidationResponse creates an AdmissionResponse from ValidationResult
func CreateValidationResponse(uid string, result ValidationResult) *admissionv1.AdmissionResponse {
	response := &admissionv1.AdmissionResponse{
		UID:     types.UID(uid),
		Allowed: result.Allowed,
	}

	if !result.Allowed {
		response.Result = &metav1.Status{
			Code:    result.Code,
			Message: result.Message,
		}
	}

	return response
}

// ValidateResource validates a resource based on its type and returns ValidationResult
func (v *DeploymentHPAValidator) ValidateResource(ctx context.Context, resourceType string, resource interface{}) ValidationResult {
	var err error

	switch resourceType {
	case "Deployment":
		if deployment, ok := resource.(*appsv1.Deployment); ok {
			err = v.ValidateDeployment(ctx, deployment)
		} else {
			err = NewWebhookError(
				ErrorTypeInternal,
				CodeInvalidResource,
				"無効なDeploymentリソースです",
			)
		}
	case "HorizontalPodAutoscaler":
		if hpa, ok := resource.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			err = v.ValidateHPA(ctx, hpa)
		} else {
			err = NewWebhookError(
				ErrorTypeInternal,
				CodeInvalidResource,
				"無効なHPAリソースです",
			)
		}
	default:
		// For unsupported resource types, allow by default
		return ValidationResult{
			Allowed: true,
			Message: "",
			Code:    200,
		}
	}

	return CreateValidationResult(err)
}