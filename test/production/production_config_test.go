// +build production

package production

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s-deployment-hpa-validator/internal/config"
)

// TestProductionConfiguration 本番環境設定のテスト
func TestProductionConfiguration(t *testing.T) {
	// 本番環境設定ファイルのパスを設定
	configPath := filepath.Join("..", "..", "configs", "production.yaml")
	
	// 設定ファイルの存在確認
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("本番環境設定ファイルが見つかりません: %s", configPath)
	}

	// 本番環境設定を読み込み
	loader := config.NewConfigLoaderWithFile(configPath)
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("本番環境設定の読み込みに失敗しました: %v", err)
	}

	t.Run("本番環境設定値の検証", func(t *testing.T) {
		// 環境設定の検証
		if cfg.Environment != "production" {
			t.Errorf("期待される環境: production, 実際: %s", cfg.Environment)
		}

		// ログレベルの検証（本番環境では warn 以上）
		validLogLevels := []string{"warn", "error"}
		if !contains(validLogLevels, cfg.LogLevel) {
			t.Errorf("本番環境では warn または error ログレベルが推奨されます。実際: %s", cfg.LogLevel)
		}

		// ログフォーマットの検証（本番環境では JSON 推奨）
		if cfg.LogFormat != "json" {
			t.Errorf("本番環境では JSON ログフォーマットが推奨されます。実際: %s", cfg.LogFormat)
		}

		// 失敗ポリシーの検証（本番環境では Fail が必須）
		if cfg.FailurePolicy != "Fail" {
			t.Errorf("本番環境では Fail ポリシーが必須です。実際: %s", cfg.FailurePolicy)
		}

		// タイムアウト設定の検証（本番環境では長めのタイムアウト）
		if cfg.Timeout < 15*time.Second {
			t.Errorf("本番環境では15秒以上のタイムアウトが推奨されます。実際: %v", cfg.Timeout)
		}

		// 監視設定の検証
		if !cfg.MetricsEnabled {
			t.Error("本番環境ではメトリクスが有効である必要があります")
		}
		if !cfg.HealthEnabled {
			t.Error("本番環境ではヘルスチェックが有効である必要があります")
		}

		// クラスター名の設定確認
		if cfg.ClusterName == "" {
			t.Error("本番環境ではクラスター名の設定が推奨されます")
		}

		// セキュリティ関連の設定確認
		if len(cfg.SkipNamespaces) == 0 {
			t.Error("本番環境ではスキップするnamespaceが設定されている必要があります")
		}

		// 必要なnamespaceがスキップリストに含まれているか確認
		requiredSkipNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
		for _, required := range requiredSkipNamespaces {
			if !cfg.ShouldSkipNamespace(required) {
				t.Errorf("本番環境では %s namespace がスキップリストに含まれている必要があります", required)
			}
		}
	})

	t.Run("本番環境設定の妥当性検証", func(t *testing.T) {
		// 設定の妥当性を検証
		if cfg == nil {
			t.Fatal("設定が初期化されませんでした")
		}

		// 設定が正しく適用されているか確認
		summary := cfg.GetConfigSummary()
		if summary["environment"] != "production" {
			t.Errorf("サーバー設定の環境が正しくありません: %v", summary["environment"])
		}

		// 本番環境固有の設定項目の確認
		if !cfg.IsProductionEnvironment() {
			t.Error("本番環境として認識されませんでした")
		}

		t.Logf("本番環境設定の妥当性検証が成功しました")
	})

	t.Run("本番環境固有の設定項目", func(t *testing.T) {
		// 本番環境で必要な追加設定の確認
		expectedSkipNamespaces := []string{
			"kube-system",
			"kube-public", 
			"kube-node-lease",
			"cert-manager",
			"monitoring",
			"istio-system",
		}

		for _, ns := range expectedSkipNamespaces {
			if !cfg.ShouldSkipNamespace(ns) {
				t.Errorf("本番環境では %s namespace がスキップされる必要があります", ns)
			}
		}

		// 本番環境のポート設定確認
		if cfg.Port != 8443 {
			t.Errorf("本番環境では標準的なWebhookポート(8443)が推奨されます。実際: %d", cfg.Port)
		}

		if cfg.MetricsPort != 8080 {
			t.Errorf("本番環境では標準的なメトリクスポート(8080)が推奨されます。実際: %d", cfg.MetricsPort)
		}
	})
}

// TestProductionConfigurationWithEnvironmentVariables 環境変数を使用した本番環境設定のテスト
func TestProductionConfigurationWithEnvironmentVariables(t *testing.T) {
	// 本番環境用の環境変数を設定
	envVars := map[string]string{
		"ENVIRONMENT":      "production",
		"LOG_LEVEL":        "warn",
		"LOG_FORMAT":       "json",
		"FAILURE_POLICY":   "Fail",
		"WEBHOOK_TIMEOUT":  "30s",
		"CLUSTER_NAME":     "prod-cluster-01",
		"METRICS_ENABLED":  "true",
		"HEALTH_ENABLED":   "true",
	}

	// 環境変数を設定
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		// テスト後に環境変数をクリーンアップ
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// 設定を読み込み
	loader := config.NewConfigLoader()
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("環境変数からの本番環境設定読み込みに失敗しました: %v", err)
	}

	t.Run("環境変数からの本番環境設定検証", func(t *testing.T) {
		if cfg.Environment != "production" {
			t.Errorf("期待される環境: production, 実際: %s", cfg.Environment)
		}

		if cfg.LogLevel != "warn" {
			t.Errorf("期待されるログレベル: warn, 実際: %s", cfg.LogLevel)
		}

		if cfg.LogFormat != "json" {
			t.Errorf("期待されるログフォーマット: json, 実際: %s", cfg.LogFormat)
		}

		if cfg.FailurePolicy != "Fail" {
			t.Errorf("期待される失敗ポリシー: Fail, 実際: %s", cfg.FailurePolicy)
		}

		if cfg.Timeout != 30*time.Second {
			t.Errorf("期待されるタイムアウト: 30s, 実際: %v", cfg.Timeout)
		}

		if cfg.ClusterName != "prod-cluster-01" {
			t.Errorf("期待されるクラスター名: prod-cluster-01, 実際: %s", cfg.ClusterName)
		}

		if !cfg.MetricsEnabled {
			t.Error("メトリクスが有効になっていません")
		}

		if !cfg.HealthEnabled {
			t.Error("ヘルスチェックが有効になっていません")
		}
	})

	t.Run("本番環境判定メソッドのテスト", func(t *testing.T) {
		if !cfg.IsProductionEnvironment() {
			t.Error("本番環境として認識されませんでした")
		}

		if cfg.IsDevelopmentEnvironment() {
			t.Error("開発環境として誤認識されました")
		}
	})
}

// TestProductionConfigurationValidation 本番環境設定の妥当性検証テスト
func TestProductionConfigurationValidation(t *testing.T) {
	testCases := []struct {
		name        string
		setupConfig func() *config.WebhookConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "有効な本番環境設定",
			setupConfig: func() *config.WebhookConfig {
				return &config.WebhookConfig{
					Environment:    "production",
					Port:           8443,
					TLSCertFile:    "/etc/certs/tls.crt",
					TLSKeyFile:     "/etc/certs/tls.key",
					Timeout:        30 * time.Second,
					LogLevel:       "warn",
					LogFormat:      "json",
					FailurePolicy:  "Fail",
					MetricsEnabled: true,
					MetricsPort:    8080,
					HealthEnabled:  true,
					ClusterName:    "prod-cluster",
					SkipNamespaces: []string{"kube-system", "kube-public"},
				}
			},
			expectError: false,
		},
		{
			name: "本番環境でのIgnore失敗ポリシー（警告）",
			setupConfig: func() *config.WebhookConfig {
				return &config.WebhookConfig{
					Environment:    "production",
					Port:           8443,
					TLSCertFile:    "/etc/certs/tls.crt",
					TLSKeyFile:     "/etc/certs/tls.key",
					Timeout:        30 * time.Second,
					LogLevel:       "warn",
					LogFormat:      "json",
					FailurePolicy:  "Ignore", // 本番環境では推奨されない
					MetricsEnabled: true,
					MetricsPort:    8080,
					HealthEnabled:  true,
					ClusterName:    "prod-cluster",
					SkipNamespaces: []string{"kube-system"},
				}
			},
			expectError: false, // 設定としては有効だが推奨されない
		},
		{
			name: "本番環境でのdebugログレベル（警告）",
			setupConfig: func() *config.WebhookConfig {
				return &config.WebhookConfig{
					Environment:    "production",
					Port:           8443,
					TLSCertFile:    "/etc/certs/tls.crt",
					TLSKeyFile:     "/etc/certs/tls.key",
					Timeout:        30 * time.Second,
					LogLevel:       "debug", // 本番環境では推奨されない
					LogFormat:      "json",
					FailurePolicy:  "Fail",
					MetricsEnabled: true,
					MetricsPort:    8080,
					HealthEnabled:  true,
					ClusterName:    "prod-cluster",
					SkipNamespaces: []string{"kube-system"},
				}
			},
			expectError: false, // 設定としては有効だが推奨されない
		},
		{
			name: "本番環境での短いタイムアウト（警告）",
			setupConfig: func() *config.WebhookConfig {
				return &config.WebhookConfig{
					Environment:    "production",
					Port:           8443,
					TLSCertFile:    "/etc/certs/tls.crt",
					TLSKeyFile:     "/etc/certs/tls.key",
					Timeout:        5 * time.Second, // 本番環境では短すぎる
					LogLevel:       "warn",
					LogFormat:      "json",
					FailurePolicy:  "Fail",
					MetricsEnabled: true,
					MetricsPort:    8080,
					HealthEnabled:  true,
					ClusterName:    "prod-cluster",
					SkipNamespaces: []string{"kube-system"},
				}
			},
			expectError: false, // 設定としては有効だが推奨されない
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.setupConfig()
			loader := config.NewConfigLoader()
			
			// 設定の妥当性を検証
			err := loader.ValidateConfig(cfg)
			
			if tc.expectError && err == nil {
				t.Errorf("エラーが期待されましたが、エラーが発生しませんでした")
			}
			if !tc.expectError && err != nil {
				t.Errorf("エラーが期待されませんでしたが、エラーが発生しました: %v", err)
			}
			
			// 本番環境固有の推奨事項をチェック
			if cfg.IsProductionEnvironment() {
				checkProductionRecommendations(t, cfg)
			}
		})
	}
}

// TestEnvironmentSpecificConfigurations 環境別設定の妥当性検証テスト
func TestEnvironmentSpecificConfigurations(t *testing.T) {
	environments := []string{"development", "staging", "production"}
	
	for _, env := range environments {
		t.Run(fmt.Sprintf("%s環境設定テスト", env), func(t *testing.T) {
			configPath := filepath.Join("..", "..", "configs", env+".yaml")
			
			// 設定ファイルの存在確認
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Skipf("%s環境設定ファイルが見つかりません: %s", env, configPath)
			}

			// 設定を読み込み
			loader := config.NewConfigLoaderWithFile(configPath)
			cfg, err := loader.LoadConfig()
			if err != nil {
				t.Fatalf("%s環境設定の読み込みに失敗しました: %v", env, err)
			}

			// 基本的な設定検証
			if cfg.Environment != env {
				t.Errorf("期待される環境: %s, 実際: %s", env, cfg.Environment)
			}

			// 環境固有の推奨事項をチェック
			switch env {
			case "development":
				checkDevelopmentRecommendations(t, cfg)
			case "staging":
				checkStagingRecommendations(t, cfg)
			case "production":
				checkProductionRecommendations(t, cfg)
			}
		})
	}
}

// checkProductionRecommendations 本番環境の推奨事項をチェック
func checkProductionRecommendations(t *testing.T, cfg *config.WebhookConfig) {
	// ログレベルの推奨事項
	if cfg.LogLevel == "debug" {
		t.Logf("警告: 本番環境でdebugログレベルは推奨されません（現在: %s）", cfg.LogLevel)
	}

	// ログフォーマットの推奨事項
	if cfg.LogFormat != "json" {
		t.Logf("警告: 本番環境ではJSONログフォーマットが推奨されます（現在: %s）", cfg.LogFormat)
	}

	// 失敗ポリシーの推奨事項
	if cfg.FailurePolicy != "Fail" {
		t.Logf("警告: 本番環境ではFailポリシーが推奨されます（現在: %s）", cfg.FailurePolicy)
	}

	// タイムアウトの推奨事項
	if cfg.Timeout < 15*time.Second {
		t.Logf("警告: 本番環境では15秒以上のタイムアウトが推奨されます（現在: %v）", cfg.Timeout)
	}

	// 監視設定の推奨事項
	if !cfg.MetricsEnabled {
		t.Logf("警告: 本番環境ではメトリクスの有効化が推奨されます")
	}

	if !cfg.HealthEnabled {
		t.Logf("警告: 本番環境ではヘルスチェックの有効化が推奨されます")
	}

	// クラスター名の推奨事項
	if cfg.ClusterName == "" {
		t.Logf("警告: 本番環境ではクラスター名の設定が推奨されます")
	}
}

// checkStagingRecommendations ステージング環境の推奨事項をチェック
func checkStagingRecommendations(t *testing.T, cfg *config.WebhookConfig) {
	// ステージング環境では本番環境に近い設定が推奨される
	if cfg.FailurePolicy != "Fail" {
		t.Logf("警告: ステージング環境ではFailポリシーが推奨されます（現在: %s）", cfg.FailurePolicy)
	}

	if cfg.LogFormat != "json" {
		t.Logf("警告: ステージング環境ではJSONログフォーマットが推奨されます（現在: %s）", cfg.LogFormat)
	}
}

// checkDevelopmentRecommendations 開発環境の推奨事項をチェック
func checkDevelopmentRecommendations(t *testing.T, cfg *config.WebhookConfig) {
	// 開発環境では柔軟な設定が許可される
	if cfg.LogLevel != "debug" && cfg.LogLevel != "info" {
		t.Logf("情報: 開発環境ではdebugまたはinfoログレベルが一般的です（現在: %s）", cfg.LogLevel)
	}
}

// contains スライスに指定された値が含まれているかチェック
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}