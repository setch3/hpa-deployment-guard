package webhook

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s-deployment-hpa-validator/internal/config"
	"k8s-deployment-hpa-validator/internal/logging"
	"k8s-deployment-hpa-validator/internal/validator"
)

func TestErrorHandler_HandleError(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		err         error
		req         *admissionv1.AdmissionRequest
		wantAllowed bool
		wantCode    int32
	}{
		{
			name:        "成功ケース",
			environment: "development",
			err:         nil,
			req: &admissionv1.AdmissionRequest{
				UID: types.UID("test-uid"),
			},
			wantAllowed: true,
			wantCode:    0,
		},
		{
			name:        "バリデーションエラー（開発環境）",
			environment: "development",
			err:         validator.NewDeploymentHPAConflictError(),
			req: &admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Kind: "Deployment"},
				Name:      "test-deployment",
				Namespace: "default",
			},
			wantAllowed: false,
			wantCode:    400,
		},
		{
			name:        "バリデーションエラー（本番環境）",
			environment: "production",
			err:         validator.NewDeploymentHPAConflictError(),
			req: &admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Kind: "Deployment"},
				Name:      "test-deployment",
				Namespace: "default",
			},
			wantAllowed: false,
			wantCode:    400,
		},
		{
			name:        "内部エラー（開発環境）",
			environment: "development",
			err:         validator.NewInternalError("test", fmt.Errorf("テストエラー")),
			req: &admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Kind: "Deployment"},
				Name:      "test-deployment",
				Namespace: "default",
			},
			wantAllowed: false,
			wantCode:    500,
		},
		{
			name:        "内部エラー（本番環境）",
			environment: "production",
			err:         validator.NewInternalError("test", fmt.Errorf("テストエラー")),
			req: &admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Kind: "Deployment"},
				Name:      "test-deployment",
				Namespace: "default",
			},
			wantAllowed: false,
			wantCode:    500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.WebhookConfig{
				Environment: tt.environment,
				LogLevel:    "info",
				LogFormat:   "json",
			}
			logger := logging.NewLogger("test")
			handler := NewErrorHandler(cfg, logger)

			ctx := context.Background()
			response := handler.HandleError(ctx, tt.err, tt.req)

			if response.Allowed != tt.wantAllowed {
				t.Errorf("HandleError() Allowed = %v, want %v", response.Allowed, tt.wantAllowed)
			}

			if tt.wantCode > 0 && response.Result != nil {
				if response.Result.Code != tt.wantCode {
					t.Errorf("HandleError() Code = %v, want %v", response.Result.Code, tt.wantCode)
				}
			}

			if tt.req != nil && response.UID != tt.req.UID {
				t.Errorf("HandleError() UID = %v, want %v", response.UID, tt.req.UID)
			}
		})
	}
}

func TestErrorHandler_ProductionMessageRestriction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		err         *validator.WebhookError
		wantRestricted bool
	}{
		{
			name:        "バリデーションエラー（本番環境）",
			environment: "production",
			err:         validator.NewDeploymentHPAConflictError(),
			wantRestricted: false, // バリデーションエラーは詳細を表示
		},
		{
			name:        "内部エラー（本番環境）",
			environment: "production",
			err:         validator.NewInternalError("test", fmt.Errorf("詳細な内部エラー情報")),
			wantRestricted: true, // 内部エラーは制限される
		},
		{
			name:        "設定エラー（本番環境）",
			environment: "production",
			err:         validator.NewConfigurationError("test", fmt.Errorf("詳細な設定エラー情報")),
			wantRestricted: true, // 設定エラーは制限される
		},
		{
			name:        "内部エラー（開発環境）",
			environment: "development",
			err:         validator.NewInternalError("test", fmt.Errorf("詳細な内部エラー情報")),
			wantRestricted: false, // 開発環境では制限されない
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.WebhookConfig{
				Environment: tt.environment,
				LogLevel:    "info",
				LogFormat:   "json",
			}
			logger := logging.NewLogger("test")
			handler := NewErrorHandler(cfg, logger)

			req := &admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Kind:      metav1.GroupVersionKind{Kind: "Deployment"},
				Name:      "test-deployment",
				Namespace: "default",
			}

			ctx := context.Background()
			response := handler.HandleError(ctx, tt.err, req)

			originalMessage := tt.err.Message
			responseMessage := response.Result.Message

			if tt.wantRestricted {
				// 本番環境で制限される場合、メッセージが変更されているはず
				if responseMessage == originalMessage {
					t.Errorf("Expected message to be restricted in production, but got original message: %s", responseMessage)
				}
			} else {
				// 制限されない場合、元のメッセージまたは詳細情報を含むはず
				if !strings.Contains(responseMessage, originalMessage) {
					t.Errorf("Expected message to contain original message, got: %s", responseMessage)
				}
			}
		})
	}
}

func TestErrorHandler_ShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		errorType   validator.ErrorType
		wantRetry   bool
	}{
		{
			name:        "ネットワークエラー（本番環境）",
			environment: "production",
			errorType:   validator.ErrorTypeNetwork,
			wantRetry:   true,
		},
		{
			name:        "Kubernetes APIエラー（本番環境）",
			environment: "production",
			errorType:   validator.ErrorTypeKubernetesAPI,
			wantRetry:   true,
		},
		{
			name:        "バリデーションエラー（本番環境）",
			environment: "production",
			errorType:   validator.ErrorTypeValidation,
			wantRetry:   false,
		},
		{
			name:        "内部エラー（本番環境）",
			environment: "production",
			errorType:   validator.ErrorTypeInternal,
			wantRetry:   false,
		},
		{
			name:        "ネットワークエラー（開発環境）",
			environment: "development",
			errorType:   validator.ErrorTypeNetwork,
			wantRetry:   true,
		},
		{
			name:        "リソースエラー（開発環境）",
			environment: "development",
			errorType:   validator.ErrorTypeResource,
			wantRetry:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.WebhookConfig{
				Environment: tt.environment,
				LogLevel:    "info",
				LogFormat:   "json",
			}
			logger := logging.NewLogger("test")
			handler := NewErrorHandler(cfg, logger)

			webhookErr := validator.NewWebhookError(tt.errorType, "TEST_CODE", "テストエラー")
			shouldRetry := handler.shouldRetry(webhookErr)

			if shouldRetry != tt.wantRetry {
				t.Errorf("shouldRetry() = %v, want %v", shouldRetry, tt.wantRetry)
			}
		})
	}
}

func TestWithRetry(t *testing.T) {
	t.Run("成功ケース", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.MaxRetries = 2

		callCount := 0
		err := WithRetry(context.Background(), config, func() error {
			callCount++
			return nil
		})

		if err != nil {
			t.Errorf("WithRetry() error = %v, want nil", err)
		}

		if callCount != 1 {
			t.Errorf("Function called %d times, want 1", callCount)
		}
	})

	t.Run("再試行後成功", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.MaxRetries = 3
		config.BaseDelay = 1 * time.Millisecond // テスト用に短縮

		callCount := 0
		err := WithRetry(context.Background(), config, func() error {
			callCount++
			if callCount < 3 {
				return validator.NewKubernetesAPIError("test", fmt.Errorf("一時的エラー"))
			}
			return nil
		})

		if err != nil {
			t.Errorf("WithRetry() error = %v, want nil", err)
		}

		if callCount != 3 {
			t.Errorf("Function called %d times, want 3", callCount)
		}
	})

	t.Run("再試行不可能エラー", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.MaxRetries = 3

		callCount := 0
		validationErr := validator.NewDeploymentHPAConflictError()
		err := WithRetry(context.Background(), config, func() error {
			callCount++
			return validationErr
		})

		if err != validationErr {
			t.Errorf("WithRetry() error = %v, want %v", err, validationErr)
		}

		if callCount != 1 {
			t.Errorf("Function called %d times, want 1", callCount)
		}
	})

	t.Run("最大再試行回数到達", func(t *testing.T) {
		config := DefaultRetryConfig()
		config.MaxRetries = 2
		config.BaseDelay = 1 * time.Millisecond // テスト用に短縮

		callCount := 0
		retryableErr := validator.NewKubernetesAPIError("test", fmt.Errorf("一時的エラー"))
		err := WithRetry(context.Background(), config, func() error {
			callCount++
			return retryableErr
		})

		if err == nil {
			t.Error("WithRetry() error = nil, want error")
		}

		if callCount != 3 { // 初回 + 2回の再試行
			t.Errorf("Function called %d times, want 3", callCount)
		}

		if !strings.Contains(err.Error(), "最大再試行回数") {
			t.Errorf("Error message should contain retry limit info, got: %s", err.Error())
		}
	})
}