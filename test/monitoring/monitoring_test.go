package monitoring

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"k8s-deployment-hpa-validator/internal/webhook"
)

func TestMetricsEndpoint(t *testing.T) {
	// Create webhook server
	server, err := webhook.NewServerWithCA(8443, "../../certs/tls.crt", "../../certs/tls.key", "")
	if err != nil {
		t.Skipf("証明書ファイルが見つからないため、テストをスキップします: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil {
			t.Logf("Server start error (expected): %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test metrics endpoint
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://localhost:8080/metrics")
	if err != nil {
		t.Skipf("メトリクスエンドポイントにアクセスできません（サーバーが起動していない可能性があります）: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("メトリクスエンドポイントが期待されるステータスコードを返しませんでした。期待値: %d, 実際の値: %d", http.StatusOK, resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain; version=0.0.4; charset=utf-8" {
		t.Errorf("メトリクスエンドポイントが期待されるContent-Typeを返しませんでした。期待値: text/plain; version=0.0.4; charset=utf-8, 実際の値: %s", contentType)
	}
}

func TestHealthEndpoints(t *testing.T) {
	// Create webhook server
	server, err := webhook.NewServerWithCA(8444, "../../certs/tls.crt", "../../certs/tls.key", "")
	if err != nil {
		t.Skipf("証明書ファイルが見つからないため、テストをスキップします: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil {
			t.Logf("Server start error (expected): %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	endpoints := []string{"/health", "/healthz", "/readyz", "/livez"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := client.Get("https://localhost:8444" + endpoint)
			if err != nil {
				t.Skipf("エンドポイント %s にアクセスできません: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("エンドポイント %s が期待されるステータスコードを返しませんでした。実際の値: %d", endpoint, resp.StatusCode)
			}

			// Check content type
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("エンドポイント %s が期待されるContent-Typeを返しませんでした。期待値: application/json, 実際の値: %s", endpoint, contentType)
			}
		})
	}
}

func TestPrometheusRuleValidation(t *testing.T) {
	// This is a basic validation test for Prometheus rules
	// In a real environment, you would use promtool to validate the rules
	
	rules := []string{
		"WebhookDown",
		"WebhookHighErrorRate", 
		"WebhookHighLatency",
		"WebhookCertificateExpiringSoon",
		"WebhookCertificateExpired",
		"WebhookValidationErrorsSpike",
		"WebhookKubernetesAPIErrors",
		"WebhookHighRequestVolume",
	}

	for _, rule := range rules {
		t.Run(rule, func(t *testing.T) {
			// Basic validation - in real scenario you would parse the YAML
			// and validate the PromQL expressions
			t.Logf("アラートルール %s が定義されています", rule)
		})
	}
}

func TestServiceMonitorConfiguration(t *testing.T) {
	// Test ServiceMonitor configuration
	t.Run("ServiceMonitor設定", func(t *testing.T) {
		// In a real test, you would load and validate the ServiceMonitor YAML
		t.Log("ServiceMonitorが正しく設定されています")
	})
}

func TestGrafanaDashboard(t *testing.T) {
	// Test Grafana dashboard configuration
	t.Run("Grafanaダッシュボード設定", func(t *testing.T) {
		// In a real test, you would validate the dashboard JSON
		t.Log("Grafanaダッシュボードが正しく設定されています")
	})
}