package validator

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

// Validator defines the interface for validating Kubernetes resources
type Validator interface {
	ValidateDeployment(ctx context.Context, deployment *appsv1.Deployment) error
	ValidateHPA(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler) error
	ValidateResource(ctx context.Context, resourceType string, resource interface{}) ValidationResult
}

// ValidationRequest represents an incoming admission request
type ValidationRequest struct {
	AdmissionRequest *admissionv1.AdmissionRequest
}

// ValidationResponse represents the response to an admission request
type ValidationResponse struct {
	AdmissionResponse *admissionv1.AdmissionResponse
}

// DeploymentValidationData contains data needed for Deployment validation
type DeploymentValidationData struct {
	Deployment *appsv1.Deployment
	Namespace  string
	Operation  string // CREATE, UPDATE
}

// HPAValidationData contains data needed for HPA validation
type HPAValidationData struct {
	HPA       *autoscalingv2.HorizontalPodAutoscaler
	Namespace string
	Operation string // CREATE, UPDATE
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Allowed bool
	Message string
	Code    int32
}

// Error messages in Japanese
const (
	ErrDeploymentWithHPA    = "1 replicaのDeploymentにHPAが設定されています。HPAを削除するか、replicasを2以上に設定してください。"
	ErrHPAWithSingleReplica = "1 replicaのDeploymentを対象とするHPAは作成できません。Deploymentのreplicasを2以上に設定してください。"
	ErrSystemFailure        = "システムエラーが発生しました。管理者に連絡してください。"
)