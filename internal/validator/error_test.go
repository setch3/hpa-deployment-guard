package validator

import (
	"testing"
)

func TestWebhookError(t *testing.T) {
	tests := []struct {
		name           string
		errorType      ErrorType
		code           string
		message        string
		expectedStatus int
		isRetryable    bool
	}{
		{
			name:           "バリデーションエラー",
			errorType:      ErrorTypeValidation,
			code:           CodeDeploymentHPAConflict,
			message:        "バリデーションエラーです",
			expectedStatus: 400,
			isRetryable:    false,
		},
		{
			name:           "Kubernetes APIエラー",
			errorType:      ErrorTypeKubernetesAPI,
			code:           CodeAPIConnection,
			message:        "API接続エラーです",
			expectedStatus: 502,
			isRetryable:    true,
		},
		{
			name:           "内部エラー",
			errorType:      ErrorTypeInternal,
			code:           CodeInternalUnknown,
			message:        "内部エラーです",
			expectedStatus: 500,
			isRetryable:    false,
		},
		{
			name:           "証明書エラー",
			errorType:      ErrorTypeCertificate,
			code:           CodeCertExpired,
			message:        "証明書エラーです",
			expectedStatus: 495,
			isRetryable:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewWebhookError(tt.errorType, tt.code, tt.message)

			// エラーメッセージの確認
			if err.Error() != tt.message {
				t.Errorf("Error() = %v, want %v", err.Error(), tt.message)
			}

			// HTTPステータスコードの確認
			if status := err.GetHTTPStatusCode(); status != tt.expectedStatus {
				t.Errorf("GetHTTPStatusCode() = %v, want %v", status, tt.expectedStatus)
			}

			// 再試行可能性の確認
			if retryable := err.IsRetryable(); retryable != tt.isRetryable {
				t.Errorf("IsRetryable() = %v, want %v", retryable, tt.isRetryable)
			}

			// タイムスタンプの確認
			if err.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}

			// エラータイプとコードの確認
			if err.Type != tt.errorType {
				t.Errorf("Type = %v, want %v", err.Type, tt.errorType)
			}

			if err.Code != tt.code {
				t.Errorf("Code = %v, want %v", err.Code, tt.code)
			}
		})
	}
}

func TestWebhookErrorWithContext(t *testing.T) {
	err := NewWebhookError(ErrorTypeValidation, CodeDeploymentHPAConflict, "テストエラー")
	
	// コンテキスト情報を追加
	err = err.WithContext("req-123", "Deployment", "test-deployment", "default")

	// コンテキスト情報の確認
	if err.RequestID != "req-123" {
		t.Errorf("RequestID = %v, want %v", err.RequestID, "req-123")
	}

	if err.ResourceType != "Deployment" {
		t.Errorf("ResourceType = %v, want %v", err.ResourceType, "Deployment")
	}

	if err.ResourceName != "test-deployment" {
		t.Errorf("ResourceName = %v, want %v", err.ResourceName, "test-deployment")
	}

	if err.Namespace != "default" {
		t.Errorf("Namespace = %v, want %v", err.Namespace, "default")
	}
}

func TestWebhookErrorWithSuggestions(t *testing.T) {
	suggestions := []string{"提案1", "提案2"}
	err := NewWebhookError(ErrorTypeValidation, CodeDeploymentHPAConflict, "テストエラー")
	err = err.WithSuggestions(suggestions)

	if len(err.Suggestions) != 2 {
		t.Errorf("Suggestions length = %v, want %v", len(err.Suggestions), 2)
	}

	for i, suggestion := range suggestions {
		if err.Suggestions[i] != suggestion {
			t.Errorf("Suggestions[%d] = %v, want %v", i, err.Suggestions[i], suggestion)
		}
	}
}

func TestPredefinedErrors(t *testing.T) {
	t.Run("DeploymentHPAConflictError", func(t *testing.T) {
		err := NewDeploymentHPAConflictError()

		if err.Type != ErrorTypeValidation {
			t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
		}

		if err.Code != CodeDeploymentHPAConflict {
			t.Errorf("Code = %v, want %v", err.Code, CodeDeploymentHPAConflict)
		}

		if len(err.Suggestions) == 0 {
			t.Error("Suggestions should not be empty")
		}
	})

	t.Run("HPASingleReplicaError", func(t *testing.T) {
		err := NewHPASingleReplicaError()

		if err.Type != ErrorTypeValidation {
			t.Errorf("Type = %v, want %v", err.Type, ErrorTypeValidation)
		}

		if err.Code != CodeHPASingleReplica {
			t.Errorf("Code = %v, want %v", err.Code, CodeHPASingleReplica)
		}

		if len(err.Suggestions) == 0 {
			t.Error("Suggestions should not be empty")
		}
	})
}

func TestGetProductionMessage(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		wantProd  bool // 本番環境でメッセージが制限されるかどうか
	}{
		{
			name:      "バリデーションエラー",
			errorType: ErrorTypeValidation,
			message:   "詳細なバリデーションエラー",
			wantProd:  false, // バリデーションエラーは詳細を表示
		},
		{
			name:      "内部エラー",
			errorType: ErrorTypeInternal,
			message:   "詳細な内部エラー情報",
			wantProd:  true, // 内部エラーは制限される
		},
		{
			name:      "設定エラー",
			errorType: ErrorTypeConfiguration,
			message:   "詳細な設定エラー情報",
			wantProd:  true, // 設定エラーは制限される
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewWebhookError(tt.errorType, "TEST_CODE", tt.message)
			prodMessage := err.GetProductionMessage()

			if tt.wantProd {
				// 本番環境では詳細情報が制限される
				if prodMessage == tt.message {
					t.Errorf("Production message should be different from original message")
				}
			} else {
				// バリデーションエラーは詳細を表示
				if prodMessage != tt.message {
					t.Errorf("Production message should be same as original message for validation errors")
				}
			}

			// 本番環境メッセージは空でないことを確認
			if prodMessage == "" {
				t.Error("Production message should not be empty")
			}
		})
	}
}

func TestErrorTypeRetryability(t *testing.T) {
	retryableTypes := []ErrorType{
		ErrorTypeNetwork,
		ErrorTypeKubernetesAPI,
		ErrorTypeResource,
	}

	nonRetryableTypes := []ErrorType{
		ErrorTypeValidation,
		ErrorTypeConfiguration,
		ErrorTypeCertificate,
		ErrorTypeAuth,
	}

	for _, errorType := range retryableTypes {
		t.Run(string(errorType)+"_should_be_retryable", func(t *testing.T) {
			err := NewWebhookError(errorType, "TEST_CODE", "テストエラー")
			if !err.IsRetryable() {
				t.Errorf("ErrorType %v should be retryable", errorType)
			}
		})
	}

	for _, errorType := range nonRetryableTypes {
		t.Run(string(errorType)+"_should_not_be_retryable", func(t *testing.T) {
			err := NewWebhookError(errorType, "TEST_CODE", "テストエラー")
			if err.IsRetryable() {
				t.Errorf("ErrorType %v should not be retryable", errorType)
			}
		})
	}
}

func TestGetTypeAndGetResourceType(t *testing.T) {
	err := NewWebhookError(ErrorTypeValidation, CodeDeploymentHPAConflict, "テストエラー")
	err = err.WithContext("req-123", "Deployment", "test-deployment", "default")

	// GetType メソッドのテスト
	if err.GetType() != string(ErrorTypeValidation) {
		t.Errorf("GetType() = %v, want %v", err.GetType(), string(ErrorTypeValidation))
	}

	// GetResourceType メソッドのテスト
	if err.GetResourceType() != "Deployment" {
		t.Errorf("GetResourceType() = %v, want %v", err.GetResourceType(), "Deployment")
	}

	// ResourceTypeが空の場合のテスト
	err2 := NewWebhookError(ErrorTypeValidation, CodeDeploymentHPAConflict, "テストエラー")
	if err2.GetResourceType() != "unknown" {
		t.Errorf("GetResourceType() = %v, want %v", err2.GetResourceType(), "unknown")
	}
}