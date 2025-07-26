package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"k8s-deployment-hpa-validator/internal/config"
	"k8s-deployment-hpa-validator/internal/logging"
	"k8s-deployment-hpa-validator/internal/validator"
)

func TestServer_validateAdmissionRequest(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	// Create test config
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	// Create server with fake client
	server := &Server{
		client:       fakeClient,
		validator:    validator.NewDeploymentHPAValidator(fakeClient),
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
	}

	tests := []struct {
		name     string
		request  *admissionv1.AdmissionRequest
		expected bool
	}{
		{
			name: "nil request",
			request: nil,
			expected: false,
		},
		{
			name: "valid deployment with 2 replicas",
			request: createDeploymentAdmissionRequest("test-deployment", "default", 2),
			expected: true,
		},
		{
			name: "deployment with 1 replica (no HPA)",
			request: createDeploymentAdmissionRequest("test-deployment", "default", 1),
			expected: true,
		},
		{
			name: "unsupported resource type",
			request: &admissionv1.AdmissionRequest{
				UID: types.UID("test-uid"),
				Kind: metav1.GroupVersionKind{
					Kind: "ConfigMap",
				},
				Name:      "test-configmap",
				Namespace: "default",
				Operation: admissionv1.Create,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := server.validateAdmissionRequest(context.Background(), tt.request)
			if response.Allowed != tt.expected {
				t.Errorf("validateAdmissionRequest() allowed = %v, expected %v", response.Allowed, tt.expected)
			}
		})
	}
}

func TestServer_handleHealth(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	server := &Server{
		client:       fakeClient,
		validator:    validator.NewDeploymentHPAValidator(fakeClient),
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
		certManager:  nil, // テスト用にnilに設定
	}
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	server.handleHealth(w, req)
	
	// certManagerがnilの場合はエラーになるため、503を期待
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("handleHealth() status = %v, expected %v", w.Code, http.StatusServiceUnavailable)
	}
	
	// レスポンスボディの確認
	var health HealthStatus
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Errorf("Failed to unmarshal health response: %v", err)
	}
	
	if health.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", health.Status)
	}
}

func TestServer_handleHealthz(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	server := &Server{
		client:       fakeClient,
		validator:    validator.NewDeploymentHPAValidator(fakeClient),
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
		certManager: nil, // テスト用にnilに設定
	}
	
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	
	server.handleHealthz(w, req)
	
	// certManagerがnilの場合はエラーになるため、503を期待
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("handleHealthz() status = %v, expected %v", w.Code, http.StatusServiceUnavailable)
	}
	
	// レスポンスボディの確認
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal healthz response: %v", err)
	}
	
	if response["status"] != "error" {
		t.Errorf("Expected status 'error', got '%v'", response["status"])
	}
}

func TestServer_handleLiveness(t *testing.T) {
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	server := &Server{
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
	}
	
	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()
	
	server.handleLiveness(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("handleLiveness() status = %v, expected %v", w.Code, http.StatusOK)
	}
}

func TestServer_handleReadiness(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	server := &Server{
		client:       fakeClient,
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
		certManager:  nil, // テスト用にnilに設定
	}
	
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	
	server.handleReadiness(w, req)
	
	// certManagerがnilの場合はエラーになるため、503を期待
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("handleReadiness() status = %v, expected %v", w.Code, http.StatusServiceUnavailable)
	}
}

func TestServer_withMiddleware(t *testing.T) {
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	server := &Server{
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
	}
	
	// Test middleware with valid POST request to /validate
	handler := server.withMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	
	req := httptest.NewRequest("POST", "/validate", nil)
	w := httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("withMiddleware() status = %v, expected %v", w.Code, http.StatusOK)
	}
	
	// Test middleware with invalid GET request to /validate
	req = httptest.NewRequest("GET", "/validate", nil)
	w = httptest.NewRecorder()
	
	handler(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("withMiddleware() status = %v, expected %v", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestServer_handleValidate(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	
	cfg := &config.WebhookConfig{
		Environment: "development",
		LogLevel:    "info",
		LogFormat:   "json",
	}
	
	logger := logging.NewLogger("test-webhook")
	
	// Create server with fake client
	server := &Server{
		client:       fakeClient,
		validator:    validator.NewDeploymentHPAValidator(fakeClient),
		logger:       logger,
		config:       cfg,
		errorHandler: NewErrorHandler(cfg, logger),
	}

	// Create admission review request
	admissionReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: createDeploymentAdmissionRequest("test-deployment", "default", 2),
	}

	body, err := json.Marshal(admissionReview)
	if err != nil {
		t.Fatalf("Failed to marshal admission review: %v", err)
	}

	req := httptest.NewRequest("POST", "/validate", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleValidate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleValidate() status = %v, expected %v", w.Code, http.StatusOK)
	}

	// Parse response
	var responseReview admissionv1.AdmissionReview
	if err := json.Unmarshal(w.Body.Bytes(), &responseReview); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !responseReview.Response.Allowed {
		t.Errorf("handleValidate() allowed = %v, expected %v", responseReview.Response.Allowed, true)
	}
}

// Helper function to create deployment admission request
func createDeploymentAdmissionRequest(name, namespace string, replicas int32) *admissionv1.AdmissionRequest {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}

	deploymentBytes, _ := json.Marshal(deployment)

	return &admissionv1.AdmissionRequest{
		UID: types.UID("test-uid"),
		Kind: metav1.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		},
		Name:      name,
		Namespace: namespace,
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: deploymentBytes,
		},
	}
}

// Helper function to create HPA admission request
func createHPAAdmissionRequest(name, namespace, targetName string) *admissionv1.AdmissionRequest {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment",
				Name: targetName,
			},
		},
	}

	hpaBytes, _ := json.Marshal(hpa)

	return &admissionv1.AdmissionRequest{
		UID: types.UID("test-uid"),
		Kind: metav1.GroupVersionKind{
			Group:   "autoscaling",
			Version: "v2",
			Kind:    "HorizontalPodAutoscaler",
		},
		Name:      name,
		Namespace: namespace,
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: hpaBytes,
		},
	}
}