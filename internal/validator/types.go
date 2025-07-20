package validator

import (
	"context"
	"fmt"
	"time"

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

// ErrorType エラーの種類を表す列挙型
type ErrorType string

const (
	// バリデーションエラー（ユーザーの設定に起因）
	ErrorTypeValidation ErrorType = "validation"
	// 設定エラー（webhook自体の設定に起因）
	ErrorTypeConfiguration ErrorType = "configuration"
	// ネットワークエラー（通信に起因）
	ErrorTypeNetwork ErrorType = "network"
	// 証明書エラー（TLS証明書に起因）
	ErrorTypeCertificate ErrorType = "certificate"
	// 内部エラー（システム内部の問題）
	ErrorTypeInternal ErrorType = "internal"
	// Kubernetes APIエラー（API呼び出しに起因）
	ErrorTypeKubernetesAPI ErrorType = "kubernetes_api"
	// 認証・認可エラー
	ErrorTypeAuth ErrorType = "auth"
	// リソース不足エラー
	ErrorTypeResource ErrorType = "resource"
)

// WebhookError webhook固有のエラー構造体
type WebhookError struct {
	Type        ErrorType `json:"type"`
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Details     string    `json:"details,omitempty"`
	Suggestions []string  `json:"suggestions,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	// 内部情報（ログ用、レスポンスには含めない）
	InternalError error  `json:"-"`
	RequestID     string `json:"-"`
	ResourceType  string `json:"-"`
	ResourceName  string `json:"-"`
	Namespace     string `json:"-"`
}

// Error はerrorインターフェースを実装
func (e *WebhookError) Error() string {
	return e.Message
}

// IsRetryable エラーが再試行可能かどうかを判定
func (e *WebhookError) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeNetwork, ErrorTypeKubernetesAPI, ErrorTypeResource:
		return true
	case ErrorTypeValidation, ErrorTypeConfiguration, ErrorTypeCertificate, ErrorTypeAuth:
		return false
	case ErrorTypeInternal:
		// 内部エラーは一部再試行可能
		return e.Code == "INTERNAL_TEMPORARY"
	default:
		return false
	}
}

// GetHTTPStatusCode エラーに対応するHTTPステータスコードを取得
func (e *WebhookError) GetHTTPStatusCode() int {
	switch e.Type {
	case ErrorTypeValidation:
		return 400 // Bad Request
	case ErrorTypeAuth:
		return 403 // Forbidden
	case ErrorTypeConfiguration:
		return 422 // Unprocessable Entity
	case ErrorTypeNetwork, ErrorTypeKubernetesAPI:
		return 502 // Bad Gateway
	case ErrorTypeResource:
		return 503 // Service Unavailable
	case ErrorTypeCertificate:
		return 495 // SSL Certificate Error (nginx extension)
	case ErrorTypeInternal:
		return 500 // Internal Server Error
	default:
		return 500
	}
}

// GetProductionMessage 本番環境用の制限されたメッセージを取得
func (e *WebhookError) GetProductionMessage() string {
	switch e.Type {
	case ErrorTypeValidation:
		return e.Message // バリデーションエラーは詳細を表示
	case ErrorTypeConfiguration:
		return "設定に問題があります。管理者に連絡してください。"
	case ErrorTypeNetwork:
		return "ネットワークエラーが発生しました。しばらく待ってから再試行してください。"
	case ErrorTypeCertificate:
		return "証明書に問題があります。管理者に連絡してください。"
	case ErrorTypeKubernetesAPI:
		return "Kubernetes APIとの通信に問題があります。しばらく待ってから再試行してください。"
	case ErrorTypeAuth:
		return "認証に失敗しました。権限を確認してください。"
	case ErrorTypeResource:
		return "リソースが不足しています。しばらく待ってから再試行してください。"
	case ErrorTypeInternal:
		return "内部エラーが発生しました。管理者に連絡してください。"
	default:
		return "システムエラーが発生しました。管理者に連絡してください。"
	}
}

// Error messages in Japanese
const (
	ErrDeploymentWithHPA    = "1 replicaのDeploymentにHPAが設定されています。HPAを削除するか、replicasを2以上に設定してください。"
	ErrHPAWithSingleReplica = "1 replicaのDeploymentを対象とするHPAは作成できません。Deploymentのreplicasを2以上に設定してください。"
	ErrSystemFailure        = "システムエラーが発生しました。管理者に連絡してください。"
)

// エラーコード定数
const (
	// バリデーションエラーコード
	CodeDeploymentHPAConflict = "VALIDATION_DEPLOYMENT_HPA_CONFLICT"
	CodeHPASingleReplica      = "VALIDATION_HPA_SINGLE_REPLICA"
	CodeInvalidResource       = "VALIDATION_INVALID_RESOURCE"

	// 設定エラーコード
	CodeInvalidConfig     = "CONFIG_INVALID"
	CodeMissingConfig     = "CONFIG_MISSING"
	CodeConfigValidation  = "CONFIG_VALIDATION_FAILED"

	// ネットワークエラーコード
	CodeNetworkTimeout    = "NETWORK_TIMEOUT"
	CodeNetworkConnection = "NETWORK_CONNECTION_FAILED"
	CodeNetworkDNS        = "NETWORK_DNS_RESOLUTION_FAILED"

	// 証明書エラーコード
	CodeCertExpired       = "CERT_EXPIRED"
	CodeCertInvalid       = "CERT_INVALID"
	CodeCertNotFound      = "CERT_NOT_FOUND"
	CodeCertChainInvalid  = "CERT_CHAIN_INVALID"

	// Kubernetes APIエラーコード
	CodeAPITimeout        = "API_TIMEOUT"
	CodeAPIConnection     = "API_CONNECTION_FAILED"
	CodeAPINotFound       = "API_RESOURCE_NOT_FOUND"
	CodeAPIConflict       = "API_CONFLICT"
	CodeAPIForbidden      = "API_FORBIDDEN"

	// 内部エラーコード
	CodeInternalPanic     = "INTERNAL_PANIC"
	CodeInternalTemporary = "INTERNAL_TEMPORARY"
	CodeInternalUnknown   = "INTERNAL_UNKNOWN"

	// 認証エラーコード
	CodeAuthFailed        = "AUTH_FAILED"
	CodeAuthInsufficientPermissions = "AUTH_INSUFFICIENT_PERMISSIONS"

	// リソースエラーコード
	CodeResourceExhausted = "RESOURCE_EXHAUSTED"
	CodeResourceUnavailable = "RESOURCE_UNAVAILABLE"
)

// NewWebhookError 新しいWebhookErrorを作成
func NewWebhookError(errorType ErrorType, code, message string) *WebhookError {
	return &WebhookError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewWebhookErrorWithDetails 詳細情報付きのWebhookErrorを作成
func NewWebhookErrorWithDetails(errorType ErrorType, code, message, details string, suggestions []string) *WebhookError {
	return &WebhookError{
		Type:        errorType,
		Code:        code,
		Message:     message,
		Details:     details,
		Suggestions: suggestions,
		Timestamp:   time.Now(),
	}
}

// NewWebhookErrorFromError 既存のerrorからWebhookErrorを作成
func NewWebhookErrorFromError(errorType ErrorType, code string, err error) *WebhookError {
	return &WebhookError{
		Type:          errorType,
		Code:          code,
		Message:       err.Error(),
		Timestamp:     time.Now(),
		InternalError: err,
	}
}

// WithContext WebhookErrorにコンテキスト情報を追加
func (e *WebhookError) WithContext(requestID, resourceType, resourceName, namespace string) *WebhookError {
	e.RequestID = requestID
	e.ResourceType = resourceType
	e.ResourceName = resourceName
	e.Namespace = namespace
	return e
}

// WithInternalError WebhookErrorに内部エラーを追加
func (e *WebhookError) WithInternalError(err error) *WebhookError {
	e.InternalError = err
	return e
}

// WithSuggestions WebhookErrorに提案を追加
func (e *WebhookError) WithSuggestions(suggestions []string) *WebhookError {
	e.Suggestions = suggestions
	return e
}

// 事前定義されたエラー作成関数

// NewDeploymentHPAConflictError Deployment-HPA競合エラーを作成
func NewDeploymentHPAConflictError() *WebhookError {
	return NewWebhookErrorWithDetails(
		ErrorTypeValidation,
		CodeDeploymentHPAConflict,
		ErrDeploymentWithHPA,
		"HPAが正常に動作するためには、対象のDeploymentのreplica数が2以上である必要があります。",
		[]string{
			"Deploymentのspec.replicasを2以上に設定してください",
			"または、HPAを削除してください",
		},
	)
}

// NewHPASingleReplicaError HPA単一レプリカエラーを作成
func NewHPASingleReplicaError() *WebhookError {
	return NewWebhookErrorWithDetails(
		ErrorTypeValidation,
		CodeHPASingleReplica,
		ErrHPAWithSingleReplica,
		"HPAは最低2つのレプリカが必要です。1つのレプリカでは自動スケーリングが機能しません。",
		[]string{
			"対象のDeploymentのspec.replicasを2以上に設定してください",
			"HPAのspec.minReplicasを2以上に設定してください",
		},
	)
}

// NewKubernetesAPIError Kubernetes APIエラーを作成
func NewKubernetesAPIError(operation string, err error) *WebhookError {
	return NewWebhookErrorFromError(
		ErrorTypeKubernetesAPI,
		CodeAPIConnection,
		err,
	).WithSuggestions([]string{
		"Kubernetes APIサーバーが利用可能であることを確認してください",
		"ネットワーク接続を確認してください",
		"しばらく待ってから再試行してください",
	})
}

// NewConfigurationError 設定エラーを作成
func NewConfigurationError(configItem string, err error) *WebhookError {
	return NewWebhookErrorFromError(
		ErrorTypeConfiguration,
		CodeInvalidConfig,
		err,
	).WithSuggestions([]string{
		fmt.Sprintf("%sの設定を確認してください", configItem),
		"設定ファイルまたは環境変数を確認してください",
		"管理者に連絡してください",
	})
}

// NewCertificateError 証明書エラーを作成
func NewCertificateError(certType string, err error) *WebhookError {
	return NewWebhookErrorFromError(
		ErrorTypeCertificate,
		CodeCertInvalid,
		err,
	).WithSuggestions([]string{
		fmt.Sprintf("%s証明書のパスと内容を確認してください", certType),
		"証明書の有効期限を確認してください",
		"証明書を再生成してください",
	})
}

// NewInternalError 内部エラーを作成
func NewInternalError(component string, err error) *WebhookError {
	return NewWebhookErrorFromError(
		ErrorTypeInternal,
		CodeInternalUnknown,
		err,
	).WithSuggestions([]string{
		"管理者に連絡してください",
		"ログを確認してください",
		"webhookを再起動してください",
	})
}

// GetType エラータイプを取得（メトリクス用）
func (e *WebhookError) GetType() string {
	return string(e.Type)
}

// GetResourceType リソースタイプを取得（メトリクス用）
func (e *WebhookError) GetResourceType() string {
	if e.ResourceType == "" {
		return "unknown"
	}
	return e.ResourceType
}