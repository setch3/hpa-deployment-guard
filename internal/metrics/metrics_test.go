package metrics

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func init() {
	// テストモードを有効にしてメトリクスの重複登録を防ぐ
	EnableTestMode()
}

func TestRequestMetrics(t *testing.T) {
	// メトリクスをリセット
	WebhookRequestsTotal.Reset()
	WebhookRequestDuration.Reset()
	WebhookValidationErrors.Reset()

	t.Run("成功したリクエストのメトリクス記録", func(t *testing.T) {
		rm := NewRequestMetrics("POST", "Deployment")
		
		// 少し待機してから成功を記録
		time.Sleep(10 * time.Millisecond)
		rm.RecordSuccess()

		// メトリクスの確認
		metric := &dto.Metric{}
		WebhookRequestsTotal.WithLabelValues("POST", "success", "Deployment").Write(metric)
		
		if metric.Counter.GetValue() != 1 {
			t.Errorf("期待値: 1, 実際の値: %f", metric.Counter.GetValue())
		}
	})

	t.Run("エラーが発生したリクエストのメトリクス記録", func(t *testing.T) {
		rm := NewRequestMetrics("POST", "HorizontalPodAutoscaler")
		
		// エラーを記録
		rm.RecordError("validation_failed")

		// エラーメトリクスの確認
		metric := &dto.Metric{}
		WebhookValidationErrors.WithLabelValues("validation_failed", "HorizontalPodAutoscaler").Write(metric)
		
		if metric.Counter.GetValue() != 1 {
			t.Errorf("期待値: 1, 実際の値: %f", metric.Counter.GetValue())
		}
	})

	t.Run("バリデーションエラーの記録", func(t *testing.T) {
		RecordValidationError("hpa_conflict", "Deployment")

		metric := &dto.Metric{}
		WebhookValidationErrors.WithLabelValues("hpa_conflict", "Deployment").Write(metric)
		
		if metric.Counter.GetValue() != 1 {
			t.Errorf("期待値: 1, 実際の値: %f", metric.Counter.GetValue())
		}
	})
}

func TestCertificateMetrics(t *testing.T) {
	t.Run("証明書の有効期限メトリクス更新", func(t *testing.T) {
		UpdateCertificateExpiry(30)

		metric := &dto.Metric{}
		WebhookCertificateExpiryDays.Write(metric)
		
		if metric.Gauge.GetValue() != 30 {
			t.Errorf("期待値: 30, 実際の値: %f", metric.Gauge.GetValue())
		}
	})
}

func TestKubernetesAPIMetrics(t *testing.T) {
	// メトリクスをリセット
	WebhookKubernetesAPIRequests.Reset()

	t.Run("Kubernetes APIリクエストメトリクス記録", func(t *testing.T) {
		RecordKubernetesAPIRequest("GET", "deployments", "success")

		metric := &dto.Metric{}
		WebhookKubernetesAPIRequests.WithLabelValues("GET", "deployments", "success").Write(metric)
		
		if metric.Counter.GetValue() != 1 {
			t.Errorf("期待値: 1, 実際の値: %f", metric.Counter.GetValue())
		}
	})
}

func TestWebhookUpMetrics(t *testing.T) {
	t.Run("webhook稼働状態の設定", func(t *testing.T) {
		// 稼働中に設定
		SetWebhookUp(true)
		
		metric := &dto.Metric{}
		WebhookUp.Write(metric)
		
		if metric.Gauge.GetValue() != 1 {
			t.Errorf("期待値: 1, 実際の値: %f", metric.Gauge.GetValue())
		}

		// 停止中に設定
		SetWebhookUp(false)
		
		WebhookUp.Write(metric)
		
		if metric.Gauge.GetValue() != 0 {
			t.Errorf("期待値: 0, 実際の値: %f", metric.Gauge.GetValue())
		}
	})
}

func TestNewRequestMetrics(t *testing.T) {
	t.Run("RequestMetricsインスタンスの作成", func(t *testing.T) {
		rm := NewRequestMetrics("POST", "Deployment")
		
		if rm.Method != "POST" {
			t.Errorf("期待値: POST, 実際の値: %s", rm.Method)
		}
		
		if rm.ResourceType != "Deployment" {
			t.Errorf("期待値: Deployment, 実際の値: %s", rm.ResourceType)
		}
		
		if rm.StartTime.IsZero() {
			t.Error("StartTimeが設定されていません")
		}
	})
}

// ベンチマークテスト
func BenchmarkRecordSuccess(b *testing.B) {
	rm := NewRequestMetrics("POST", "Deployment")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordSuccess()
	}
}

func BenchmarkRecordError(b *testing.B) {
	rm := NewRequestMetrics("POST", "Deployment")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordError("validation_failed")
	}
}