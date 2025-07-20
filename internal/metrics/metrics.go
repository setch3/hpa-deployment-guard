package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	metricsInitialized bool
	metricsInitMutex   sync.Mutex
)

// WebhookError は循環インポートを避けるためのインターフェース
type WebhookError interface {
	Error() string
	GetType() string
	GetResourceType() string
}

// メトリクス定義
var (
	// webhook_requests_total - webhookリクエストの総数
	WebhookRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_total",
			Help: "webhookリクエストの総数",
		},
		[]string{"method", "status", "resource_type"},
	)

	// webhook_request_duration_seconds - webhookリクエストの処理時間
	WebhookRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webhook_request_duration_seconds",
			Help:    "webhookリクエストの処理時間（秒）",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "resource_type"},
	)

	// webhook_validation_errors_total - バリデーションエラーの総数
	WebhookValidationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_validation_errors_total",
			Help: "バリデーションエラーの総数",
		},
		[]string{"error_type", "resource_type"},
	)

	// webhook_certificate_expiry_days - 証明書の有効期限までの日数
	WebhookCertificateExpiryDays = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "webhook_certificate_expiry_days",
			Help: "証明書の有効期限までの日数",
		},
	)

	// webhook_kubernetes_api_requests_total - Kubernetes APIリクエストの総数
	WebhookKubernetesAPIRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_kubernetes_api_requests_total",
			Help: "Kubernetes APIリクエストの総数",
		},
		[]string{"method", "resource", "status"},
	)

	// webhook_up - webhookサービスの稼働状態
	WebhookUp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "webhook_up",
			Help: "webhookサービスの稼働状態（1=稼働中、0=停止中）",
		},
	)
)

// RequestMetrics はリクエストメトリクスを記録するための構造体
type RequestMetrics struct {
	Method       string
	ResourceType string
	StartTime    time.Time
}

// NewRequestMetrics は新しいRequestMetricsインスタンスを作成
func NewRequestMetrics(method, resourceType string) *RequestMetrics {
	return &RequestMetrics{
		Method:       method,
		ResourceType: resourceType,
		StartTime:    time.Now(),
	}
}

// RecordSuccess は成功したリクエストのメトリクスを記録
func (rm *RequestMetrics) RecordSuccess() {
	duration := time.Since(rm.StartTime).Seconds()
	
	WebhookRequestsTotal.WithLabelValues(rm.Method, "success", rm.ResourceType).Inc()
	WebhookRequestDuration.WithLabelValues(rm.Method, rm.ResourceType).Observe(duration)
}

// RecordError はエラーが発生したリクエストのメトリクスを記録
func (rm *RequestMetrics) RecordError(errorType string) {
	duration := time.Since(rm.StartTime).Seconds()
	
	WebhookRequestsTotal.WithLabelValues(rm.Method, "error", rm.ResourceType).Inc()
	WebhookRequestDuration.WithLabelValues(rm.Method, rm.ResourceType).Observe(duration)
	WebhookValidationErrors.WithLabelValues(errorType, rm.ResourceType).Inc()
}

// RecordWebhookError はWebhookErrorが発生したリクエストのメトリクスを記録
func (rm *RequestMetrics) RecordWebhookError(webhookErr WebhookError) {
	duration := time.Since(rm.StartTime).Seconds()
	
	WebhookRequestsTotal.WithLabelValues(rm.Method, "error", rm.ResourceType).Inc()
	WebhookRequestDuration.WithLabelValues(rm.Method, rm.ResourceType).Observe(duration)
	WebhookValidationErrors.WithLabelValues(webhookErr.GetType(), webhookErr.GetResourceType()).Inc()
}

// RecordValidationError はバリデーションエラーのメトリクスを記録
func RecordValidationError(errorType, resourceType string) {
	WebhookValidationErrors.WithLabelValues(errorType, resourceType).Inc()
}

// UpdateCertificateExpiry は証明書の有効期限メトリクスを更新
func UpdateCertificateExpiry(daysUntilExpiry int) {
	WebhookCertificateExpiryDays.Set(float64(daysUntilExpiry))
}

// RecordKubernetesAPIRequest はKubernetes APIリクエストのメトリクスを記録
func RecordKubernetesAPIRequest(method, resource, status string) {
	WebhookKubernetesAPIRequests.WithLabelValues(method, resource, status).Inc()
}

// SetWebhookUp はwebhookの稼働状態を設定
func SetWebhookUp(up bool) {
	if up {
		WebhookUp.Set(1)
	} else {
		WebhookUp.Set(0)
	}
}

// InitMetrics はメトリクスの初期化を行う（テスト用）
func InitMetrics() {
	metricsInitMutex.Lock()
	defer metricsInitMutex.Unlock()
	
	if metricsInitialized {
		return
	}
	
	// 初期状態でwebhookを稼働中に設定
	SetWebhookUp(true)
	metricsInitialized = true
}

// ResetMetrics はメトリクスをリセットする（テスト用）
func ResetMetrics() {
	metricsInitMutex.Lock()
	defer metricsInitMutex.Unlock()
	
	metricsInitialized = false
}

// init はメトリクスの初期化を行う
func init() {
	InitMetrics()
}

// RecordWebhookError WebhookErrorのメトリクスを記録
func RecordWebhookError(webhookErr WebhookError) {
	if webhookErr == nil {
		return
	}
	
	WebhookValidationErrors.WithLabelValues(
		webhookErr.GetType(),
		webhookErr.GetResourceType(),
	).Inc()
}

// RecordValidationErrorWithType エラータイプ付きでバリデーションエラーを記録
func RecordValidationErrorWithType(errorType, resourceType string) {
	WebhookValidationErrors.WithLabelValues(errorType, resourceType).Inc()
}

// GetErrorTypeFromError エラーからエラータイプを取得
func GetErrorTypeFromError(err error) string {
	if webhookErr, ok := err.(WebhookError); ok {
		return webhookErr.GetType()
	}
	return "unknown"
}