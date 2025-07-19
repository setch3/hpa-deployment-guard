package validator

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidateDeployment(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *appsv1.Deployment
		existingHPAs   []autoscalingv2.HorizontalPodAutoscaler
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "Valid deployment with 2 replicas and HPA",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
				},
			},
			existingHPAs: []autoscalingv2.HorizontalPodAutoscaler{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind: "Deployment",
							Name: "test-deployment",
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Invalid deployment with 1 replica and HPA",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			existingHPAs: []autoscalingv2.HorizontalPodAutoscaler{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind: "Deployment",
							Name: "test-deployment",
						},
					},
				},
			},
			expectedError:  true,
			expectedErrMsg: ErrDeploymentWithHPA,
		},
		{
			name: "Valid deployment with 1 replica but no HPA",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			existingHPAs:  []autoscalingv2.HorizontalPodAutoscaler{},
			expectedError: false,
		},
		{
			name: "Valid deployment with 1 replica and HPA targeting different deployment",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			existingHPAs: []autoscalingv2.HorizontalPodAutoscaler{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind: "Deployment",
							Name: "other-deployment",
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Deployment with nil replicas (defaults to 1) and HPA",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: nil, // This should default to 1
				},
			},
			existingHPAs: []autoscalingv2.HorizontalPodAutoscaler{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind: "Deployment",
							Name: "test-deployment",
						},
					},
				},
			},
			expectedError: false, // nil replicas should not trigger validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with existing HPAs
			fakeClient := fake.NewSimpleClientset()
			for _, hpa := range tt.existingHPAs {
				_, err := fakeClient.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(
					context.Background(), &hpa, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test HPA: %v", err)
				}
			}

			validator := NewDeploymentHPAValidator(fakeClient)
			err := validator.ValidateDeployment(context.Background(), tt.deployment)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateHPA(t *testing.T) {
	tests := []struct {
		name               string
		hpa                *autoscalingv2.HorizontalPodAutoscaler
		existingDeployment *appsv1.Deployment
		expectedError      bool
		expectedErrMsg     string
	}{
		{
			name: "Valid HPA targeting deployment with 2 replicas",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
				},
			},
			existingDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
				},
			},
			expectedError: false,
		},
		{
			name: "Invalid HPA targeting deployment with 1 replica",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
				},
			},
			existingDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			expectedError:  true,
			expectedErrMsg: ErrHPAWithSingleReplica,
		},
		{
			name: "HPA targeting non-existent deployment",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "non-existent-deployment",
					},
				},
			},
			existingDeployment: nil,
			expectedError:      false, // Should not error if deployment doesn't exist
		},
		{
			name: "HPA targeting non-Deployment resource",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "StatefulSet",
						Name: "test-statefulset",
					},
				},
			},
			existingDeployment: nil,
			expectedError:      false, // Should not validate non-Deployment targets
		},
		{
			name: "HPA targeting deployment with nil replicas",
			hpa: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
				},
			},
			existingDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: nil, // nil replicas should not trigger validation
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fake.NewSimpleClientset()

			// Create existing deployment if provided
			if tt.existingDeployment != nil {
				_, err := fakeClient.AppsV1().Deployments(tt.existingDeployment.Namespace).Create(
					context.Background(), tt.existingDeployment, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test deployment: %v", err)
				}
			}

			validator := NewDeploymentHPAValidator(fakeClient)
			err := validator.ValidateHPA(context.Background(), tt.hpa)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.expectedErrMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCreateValidationResult(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedResult ValidationResult
	}{
		{
			name: "Success case - no error",
			err:  nil,
			expectedResult: ValidationResult{
				Allowed: true,
				Message: "",
				Code:    200,
			},
		},
		{
			name: "Error case - deployment with HPA",
			err:  fmt.Errorf(ErrDeploymentWithHPA),
			expectedResult: ValidationResult{
				Allowed: false,
				Message: ErrDeploymentWithHPA,
				Code:    400,
			},
		},
		{
			name: "Error case - HPA with single replica",
			err:  fmt.Errorf(ErrHPAWithSingleReplica),
			expectedResult: ValidationResult{
				Allowed: false,
				Message: ErrHPAWithSingleReplica,
				Code:    400,
			},
		},
		{
			name: "Error case - system failure",
			err:  fmt.Errorf(ErrSystemFailure),
			expectedResult: ValidationResult{
				Allowed: false,
				Message: ErrSystemFailure,
				Code:    400,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateValidationResult(tt.err)

			if result.Allowed != tt.expectedResult.Allowed {
				t.Errorf("Expected Allowed=%v, got %v", tt.expectedResult.Allowed, result.Allowed)
			}
			if result.Message != tt.expectedResult.Message {
				t.Errorf("Expected Message='%s', got '%s'", tt.expectedResult.Message, result.Message)
			}
			if result.Code != tt.expectedResult.Code {
				t.Errorf("Expected Code=%d, got %d", tt.expectedResult.Code, result.Code)
			}
		})
	}
}

func TestCreateValidationResponse(t *testing.T) {
	tests := []struct {
		name           string
		uid            string
		result         ValidationResult
		expectedAllowed bool
		expectedMessage string
		expectedCode    int32
	}{
		{
			name: "Success response",
			uid:  "test-uid-123",
			result: ValidationResult{
				Allowed: true,
				Message: "",
				Code:    200,
			},
			expectedAllowed: true,
			expectedMessage: "",
			expectedCode:    0, // No status set for success
		},
		{
			name: "Error response",
			uid:  "test-uid-456",
			result: ValidationResult{
				Allowed: false,
				Message: ErrDeploymentWithHPA,
				Code:    400,
			},
			expectedAllowed: false,
			expectedMessage: ErrDeploymentWithHPA,
			expectedCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := CreateValidationResponse(tt.uid, tt.result)

			if string(response.UID) != tt.uid {
				t.Errorf("Expected UID='%s', got '%s'", tt.uid, string(response.UID))
			}
			if response.Allowed != tt.expectedAllowed {
				t.Errorf("Expected Allowed=%v, got %v", tt.expectedAllowed, response.Allowed)
			}

			if tt.expectedAllowed {
				if response.Result != nil {
					t.Errorf("Expected Result to be nil for success case, got %v", response.Result)
				}
			} else {
				if response.Result == nil {
					t.Errorf("Expected Result to be set for error case")
				} else {
					if response.Result.Message != tt.expectedMessage {
						t.Errorf("Expected Result.Message='%s', got '%s'", tt.expectedMessage, response.Result.Message)
					}
					if response.Result.Code != tt.expectedCode {
						t.Errorf("Expected Result.Code=%d, got %d", tt.expectedCode, response.Result.Code)
					}
				}
			}
		})
	}
}

func TestValidateResource(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		resource       interface{}
		existingHPAs   []autoscalingv2.HorizontalPodAutoscaler
		existingDeps   []appsv1.Deployment
		expectedResult ValidationResult
	}{
		{
			name:         "Valid Deployment resource",
			resourceType: "Deployment",
			resource: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(2),
				},
			},
			expectedResult: ValidationResult{
				Allowed: true,
				Message: "",
				Code:    200,
			},
		},
		{
			name:         "Invalid Deployment resource",
			resourceType: "Deployment",
			resource: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
				},
			},
			existingHPAs: []autoscalingv2.HorizontalPodAutoscaler{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-hpa",
						Namespace: "default",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							Kind: "Deployment",
							Name: "test-deployment",
						},
					},
				},
			},
			expectedResult: ValidationResult{
				Allowed: false,
				Message: ErrDeploymentWithHPA,
				Code:    400,
			},
		},
		{
			name:         "Valid HPA resource",
			resourceType: "HorizontalPodAutoscaler",
			resource: &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						Kind: "Deployment",
						Name: "test-deployment",
					},
				},
			},
			existingDeps: []appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deployment",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(2),
					},
				},
			},
			expectedResult: ValidationResult{
				Allowed: true,
				Message: "",
				Code:    200,
			},
		},
		{
			name:         "Unsupported resource type",
			resourceType: "Service",
			resource:     nil,
			expectedResult: ValidationResult{
				Allowed: true,
				Message: "",
				Code:    200,
			},
		},
		{
			name:         "Invalid resource type casting",
			resourceType: "Deployment",
			resource:     "invalid-resource",
			expectedResult: ValidationResult{
				Allowed: false,
				Message: ErrSystemFailure + ": invalid deployment resource",
				Code:    400,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			fakeClient := fake.NewSimpleClientset()

			// Create existing resources
			for _, hpa := range tt.existingHPAs {
				_, err := fakeClient.AutoscalingV2().HorizontalPodAutoscalers(hpa.Namespace).Create(
					context.Background(), &hpa, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test HPA: %v", err)
				}
			}
			for _, dep := range tt.existingDeps {
				_, err := fakeClient.AppsV1().Deployments(dep.Namespace).Create(
					context.Background(), &dep, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test Deployment: %v", err)
				}
			}

			validator := NewDeploymentHPAValidator(fakeClient)
			result := validator.ValidateResource(context.Background(), tt.resourceType, tt.resource)

			if result.Allowed != tt.expectedResult.Allowed {
				t.Errorf("Expected Allowed=%v, got %v", tt.expectedResult.Allowed, result.Allowed)
			}
			if result.Message != tt.expectedResult.Message {
				t.Errorf("Expected Message='%s', got '%s'", tt.expectedResult.Message, result.Message)
			}
			if result.Code != tt.expectedResult.Code {
				t.Errorf("Expected Code=%d, got %d", tt.expectedResult.Code, result.Code)
			}
		})
	}
}

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}