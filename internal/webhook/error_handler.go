package webhook

import (
	"context"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s-deployment-hpa-validator/internal/config"
	"k8s-deployment-hpa-validator/internal/logging"
	"k8s-deployment-hpa-validator/internal/metrics"
	"k8s-deployment-hpa-validator/internal/validator"
)

// ErrorHandler 本番環境用エラーハンドラー
type ErrorHandler struct {
	config *config.WebhookConfig
	logger *logging.Logger
}

// NewErrorHandler 新しいErrorHandlerを作成
func NewErrorHandler(config *config.WebhookConfig, logger *logging.Logger) *ErrorHandler {
	return &ErrorHandler{
		config: config,
		logger: logger,
	}
}

// HandleError エラーを処理してAdmissionResponseを作成
func (eh *ErrorHandler) HandleError(ctx context.Context, err error, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if err == nil {
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
		}
	}

	requestID := logging.RequestIDFromContext(ctx)
	requestLogger := eh.logger.WithRequestID(requestID)

	// WebhookErrorの場合は詳細な処理を行う
	if webhookErr, ok := err.(*validator.WebhookError); ok {
		return eh.handleWebhookError(ctx, webhookErr, req, requestLogger)
	}

	// 通常のerrorの場合はデフォルト処理
	return eh.handleGenericError(ctx, err, req, requestLogger)
}

// handleWebhookError WebhookErrorを処理
func (eh *ErrorHandler) handleWebhookError(ctx context.Context, webhookErr *validator.WebhookError, req *admissionv1.AdmissionRequest, logger *logging.RequestLogger) *admissionv1.AdmissionResponse {
	// コンテキスト情報を設定
	if webhookErr.RequestID == "" {
		webhookErr.RequestID = logging.RequestIDFromContext(ctx)
	}
	if webhookErr.ResourceType == "" && req != nil {
		webhookErr.ResourceType = req.Kind.Kind
	}
	if webhookErr.ResourceName == "" && req != nil {
		webhookErr.ResourceName = req.Name
	}
	if webhookErr.Namespace == "" && req != nil {
		webhookErr.Namespace = req.Namespace
	}

	// メトリクス記録
	metrics.RecordWebhookError(webhookErr)

	// ログ出力
	eh.logWebhookError(webhookErr, logger)

	// 再試行可能エラーの場合は再試行を試みる
	if webhookErr.IsRetryable() && eh.shouldRetry(webhookErr) {
		return eh.handleRetryableError(ctx, webhookErr, req, logger)
	}

	// レスポンス作成
	return eh.createErrorResponse(webhookErr, req)
}

// handleGenericError 通常のerrorを処理
func (eh *ErrorHandler) handleGenericError(ctx context.Context, err error, req *admissionv1.AdmissionRequest, logger *logging.RequestLogger) *admissionv1.AdmissionResponse {
	// 通常のエラーを内部エラーとして扱う
	webhookErr := validator.NewInternalError("webhook", err)
	if req != nil {
		webhookErr = webhookErr.WithContext(
			logging.RequestIDFromContext(ctx),
			req.Kind.Kind,
			req.Name,
			req.Namespace,
		)
	}

	return eh.handleWebhookError(ctx, webhookErr, req, logger)
}

// logWebhookError WebhookErrorをログに記録
func (eh *ErrorHandler) logWebhookError(webhookErr *validator.WebhookError, logger *logging.RequestLogger) {
	fields := map[string]interface{}{
		"error_type":     webhookErr.GetType(),
		"error_code":     webhookErr.Code,
		"resource_type":  webhookErr.GetResourceType(),
		"resource_name":  webhookErr.ResourceName,
		"namespace":      webhookErr.Namespace,
		"retryable":      webhookErr.IsRetryable(),
		"http_status":    webhookErr.GetHTTPStatusCode(),
	}

	// 内部エラーがある場合は追加
	if webhookErr.InternalError != nil {
		fields["internal_error"] = webhookErr.InternalError.Error()
	}

	// 提案がある場合は追加
	if len(webhookErr.Suggestions) > 0 {
		fields["suggestions"] = webhookErr.Suggestions
	}

	// エラータイプに応じてログレベルを調整
	switch webhookErr.Type {
	case validator.ErrorTypeValidation:
		logger.Info("バリデーションエラーが発生しました", fields)
	case validator.ErrorTypeConfiguration:
		logger.Error("設定エラーが発生しました", fields)
	case validator.ErrorTypeNetwork, validator.ErrorTypeKubernetesAPI:
		logger.Warn("一時的なエラーが発生しました", fields)
	case validator.ErrorTypeCertificate:
		logger.Error("証明書エラーが発生しました", fields)
	case validator.ErrorTypeAuth:
		logger.Warn("認証エラーが発生しました", fields)
	case validator.ErrorTypeResource:
		logger.Warn("リソース不足エラーが発生しました", fields)
	case validator.ErrorTypeInternal:
		logger.Error("内部エラーが発生しました", fields)
	default:
		logger.Error("不明なエラーが発生しました", fields)
	}
}

// shouldRetry 再試行すべきかどうかを判定
func (eh *ErrorHandler) shouldRetry(webhookErr *validator.WebhookError) bool {
	// 本番環境では再試行を制限
	if eh.config.IsProductionEnvironment() {
		// 本番環境では特定のエラータイプのみ再試行
		switch webhookErr.Type {
		case validator.ErrorTypeNetwork, validator.ErrorTypeKubernetesAPI:
			return true
		default:
			return false
		}
	}

	// 開発環境では再試行可能なエラーは全て再試行
	return webhookErr.IsRetryable()
}

// handleRetryableError 再試行可能エラーを処理
func (eh *ErrorHandler) handleRetryableError(ctx context.Context, webhookErr *validator.WebhookError, req *admissionv1.AdmissionRequest, logger *logging.RequestLogger) *admissionv1.AdmissionResponse {
	// 再試行ロジックは実装しない（Kubernetesが自動的に再試行する）
	// ここでは再試行可能であることをログに記録し、通常のエラーレスポンスを返す
	logger.Info("再試行可能エラーです。Kubernetesが自動的に再試行します", map[string]interface{}{
		"error_type": webhookErr.GetType(),
		"error_code": webhookErr.Code,
	})

	return eh.createErrorResponse(webhookErr, req)
}

// createErrorResponse エラーレスポンスを作成
func (eh *ErrorHandler) createErrorResponse(webhookErr *validator.WebhookError, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	var uid types.UID
	if req != nil {
		uid = req.UID
	}

	// 本番環境では制限されたメッセージを使用
	message := webhookErr.Message
	if eh.config.IsProductionEnvironment() {
		message = webhookErr.GetProductionMessage()
	}

	response := &admissionv1.AdmissionResponse{
		UID:     uid,
		Allowed: false,
		Result: &metav1.Status{
			Code:    int32(webhookErr.GetHTTPStatusCode()),
			Message: message,
		},
	}

	// 開発環境では追加情報を含める
	if eh.config.IsDevelopmentEnvironment() && len(webhookErr.Suggestions) > 0 {
		// 提案を含めたメッセージを作成
		detailedMessage := message
		if webhookErr.Details != "" {
			detailedMessage += fmt.Sprintf("\n詳細: %s", webhookErr.Details)
		}
		if len(webhookErr.Suggestions) > 0 {
			detailedMessage += "\n提案:"
			for i, suggestion := range webhookErr.Suggestions {
				detailedMessage += fmt.Sprintf("\n  %d. %s", i+1, suggestion)
			}
		}
		response.Result.Message = detailedMessage
	}

	return response
}

// RetryConfig 再試行設定
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
}

// DefaultRetryConfig デフォルトの再試行設定
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}
}

// WithRetry 再試行付きで関数を実行
func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 指数バックオフで待機
			delay := time.Duration(float64(config.BaseDelay) * float64(attempt) * config.Multiplier)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			
			// ジッターを追加（オプション）
			if config.Jitter {
				jitter := time.Duration(float64(delay) * 0.1) // 10%のジッター
				delay += time.Duration(float64(jitter) * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))
			}
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// WebhookErrorの場合、再試行可能かチェック
		if webhookErr, ok := err.(*validator.WebhookError); ok {
			if !webhookErr.IsRetryable() {
				return err
			}
		}
	}
	
	return fmt.Errorf("最大再試行回数(%d)に達しました。最後のエラー: %w", config.MaxRetries, lastErr)
}

// IsRetryableError エラーが再試行可能かどうかを判定
func IsRetryableError(err error) bool {
	if webhookErr, ok := err.(*validator.WebhookError); ok {
		return webhookErr.IsRetryable()
	}
	return false
}