package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigLoader_LoadConfig_Defaults(t *testing.T) {
	loader := NewConfigLoader()
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// デフォルト値の検証
	if config.Port != 8443 {
		t.Errorf("期待されるポート: 8443, 実際: %d", config.Port)
	}
	if config.TLSCertFile != "/etc/certs/tls.crt" {
		t.Errorf("期待される証明書ファイル: /etc/certs/tls.crt, 実際: %s", config.TLSCertFile)
	}
	if config.TLSKeyFile != "/etc/certs/tls.key" {
		t.Errorf("期待される秘密鍵ファイル: /etc/certs/tls.key, 実際: %s", config.TLSKeyFile)
	}
	if config.Timeout != 10*time.Second {
		t.Errorf("期待されるタイムアウト: 10s, 実際: %v", config.Timeout)
	}
	if config.LogLevel != "info" {
		t.Errorf("期待されるログレベル: info, 実際: %s", config.LogLevel)
	}
	if config.LogFormat != "json" {
		t.Errorf("期待されるログフォーマット: json, 実際: %s", config.LogFormat)
	}
	if !config.MetricsEnabled {
		t.Error("メトリクスが有効になっていません")
	}
	if config.MetricsPort != 8080 {
		t.Errorf("期待されるメトリクスポート: 8080, 実際: %d", config.MetricsPort)
	}
	if !config.HealthEnabled {
		t.Error("ヘルスチェックが有効になっていません")
	}
	if config.Environment != "development" {
		t.Errorf("期待される環境: development, 実際: %s", config.Environment)
	}
	if config.FailurePolicy != "Fail" {
		t.Errorf("期待される失敗ポリシー: Fail, 実際: %s", config.FailurePolicy)
	}
}

func TestConfigLoader_LoadConfig_FromEnv(t *testing.T) {
	// 環境変数を設定
	os.Setenv("WEBHOOK_PORT", "9443")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("METRICS_ENABLED", "false")
	defer func() {
		os.Unsetenv("WEBHOOK_PORT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("METRICS_ENABLED")
	}()

	loader := NewConfigLoader()
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// 環境変数からの値の検証
	if config.Port != 9443 {
		t.Errorf("期待されるポート: 9443, 実際: %d", config.Port)
	}
	if config.LogLevel != "debug" {
		t.Errorf("期待されるログレベル: debug, 実際: %s", config.LogLevel)
	}
	if config.Environment != "production" {
		t.Errorf("期待される環境: production, 実際: %s", config.Environment)
	}
	if config.MetricsEnabled {
		t.Error("メトリクスが無効になっていません")
	}
}

func TestConfigLoader_LoadConfig_FromConfigMap(t *testing.T) {
	configMapData := map[string]string{
		"webhook.port":                "7443",
		"log.level":                   "warn",
		"validation.skip-namespaces":  "test-ns1,test-ns2",
		"validation.skip-labels":      "skip=true,test=skip",
		"metrics.port":                "9090",
		"environment":                 "staging",
		"webhook.failure-policy":      "Ignore",
	}

	loader := NewConfigLoaderWithConfigMap(configMapData)
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// ConfigMapからの値の検証
	if config.Port != 7443 {
		t.Errorf("期待されるポート: 7443, 実際: %d", config.Port)
	}
	if config.LogLevel != "warn" {
		t.Errorf("期待されるログレベル: warn, 実際: %s", config.LogLevel)
	}
	if len(config.SkipNamespaces) != 5 { // デフォルト3つ + 追加2つ
		t.Errorf("期待されるスキップnamespace数: 5, 実際: %d", len(config.SkipNamespaces))
	}
	if len(config.SkipLabels) != 3 { // デフォルト1つ + 追加2つ
		t.Errorf("期待されるスキップラベル数: 3, 実際: %d", len(config.SkipLabels))
	}
	if config.MetricsPort != 9090 {
		t.Errorf("期待されるメトリクスポート: 9090, 実際: %d", config.MetricsPort)
	}
	if config.Environment != "staging" {
		t.Errorf("期待される環境: staging, 実際: %s", config.Environment)
	}
	if config.FailurePolicy != "Ignore" {
		t.Errorf("期待される失敗ポリシー: Ignore, 実際: %s", config.FailurePolicy)
	}
}

func TestConfigLoader_LoadConfig_EnvOverridesConfigMap(t *testing.T) {
	// ConfigMapデータを設定
	configMapData := map[string]string{
		"webhook.port": "7443",
		"log.level":    "warn",
	}

	// 環境変数を設定（ConfigMapより優先される）
	os.Setenv("WEBHOOK_PORT", "9443")
	os.Setenv("LOG_LEVEL", "error")
	defer func() {
		os.Unsetenv("WEBHOOK_PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	loader := NewConfigLoaderWithConfigMap(configMapData)
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	// 環境変数がConfigMapを上書きしていることを検証
	if config.Port != 9443 {
		t.Errorf("環境変数が優先されていません。期待: 9443, 実際: %d", config.Port)
	}
	if config.LogLevel != "error" {
		t.Errorf("環境変数が優先されていません。期待: error, 実際: %s", config.LogLevel)
	}
}

func TestConfigLoader_ValidateConfig_InvalidValues(t *testing.T) {
	testCases := []struct {
		name        string
		setupConfig func(*WebhookConfig)
		expectError bool
	}{
		{
			name: "無効なポート番号",
			setupConfig: func(c *WebhookConfig) {
				c.Port = -1
			},
			expectError: true,
		},
		{
			name: "空の証明書ファイル",
			setupConfig: func(c *WebhookConfig) {
				c.TLSCertFile = ""
			},
			expectError: true,
		},
		{
			name: "空の秘密鍵ファイル",
			setupConfig: func(c *WebhookConfig) {
				c.TLSKeyFile = ""
			},
			expectError: true,
		},
		{
			name: "無効なログレベル",
			setupConfig: func(c *WebhookConfig) {
				c.LogLevel = "invalid"
			},
			expectError: true,
		},
		{
			name: "無効なログフォーマット",
			setupConfig: func(c *WebhookConfig) {
				c.LogFormat = "invalid"
			},
			expectError: true,
		},
		{
			name: "無効な環境",
			setupConfig: func(c *WebhookConfig) {
				c.Environment = "invalid"
			},
			expectError: true,
		},
		{
			name: "無効な失敗ポリシー",
			setupConfig: func(c *WebhookConfig) {
				c.FailurePolicy = "invalid"
			},
			expectError: true,
		},
		{
			name: "ポートの重複",
			setupConfig: func(c *WebhookConfig) {
				c.Port = 8080
				c.MetricsPort = 8080
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			loader := NewConfigLoader()
			config, err := loader.LoadConfig()
			if err != nil {
				t.Fatalf("初期設定の読み込みに失敗しました: %v", err)
			}

			// テストケース固有の設定を適用
			tc.setupConfig(config)

			// 検証を実行
			err = loader.validateConfig(config)
			if tc.expectError && err == nil {
				t.Error("エラーが期待されましたが、エラーが発生しませんでした")
			}
			if !tc.expectError && err != nil {
				t.Errorf("エラーが期待されませんでしたが、エラーが発生しました: %v", err)
			}
		})
	}
}

func TestWebhookConfig_HelperMethods(t *testing.T) {
	config := &WebhookConfig{
		Environment:    "production",
		SkipNamespaces: []string{"kube-system", "test-ns"},
		SkipLabels:     []string{"skip=true", "test=skip"},
	}

	// 環境判定のテスト
	if !config.IsProductionEnvironment() {
		t.Error("本番環境として認識されませんでした")
	}
	if config.IsDevelopmentEnvironment() {
		t.Error("開発環境として誤認識されました")
	}

	// namespace スキップ判定のテスト
	if !config.ShouldSkipNamespace("kube-system") {
		t.Error("kube-system namespaceがスキップされませんでした")
	}
	if config.ShouldSkipNamespace("default") {
		t.Error("default namespaceが誤ってスキップされました")
	}

	// ラベルスキップ判定のテスト
	labels1 := map[string]string{"skip": "true"}
	if !config.ShouldSkipByLabel(labels1) {
		t.Error("skip=trueラベルでスキップされませんでした")
	}

	labels2 := map[string]string{"other": "value"}
	if config.ShouldSkipByLabel(labels2) {
		t.Error("無関係なラベルで誤ってスキップされました")
	}
}

func TestConfigLoader_LoadConfig_InvalidEnvValues(t *testing.T) {
	testCases := []struct {
		name   string
		envVar string
		value  string
	}{
		{"無効なポート", "WEBHOOK_PORT", "invalid"},
		{"無効なタイムアウト", "WEBHOOK_TIMEOUT", "invalid"},
		{"無効なメトリクスポート", "METRICS_PORT", "invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.envVar, tc.value)
			defer os.Unsetenv(tc.envVar)

			loader := NewConfigLoader()
			_, err := loader.LoadConfig()
			if err == nil {
				t.Error("無効な環境変数値でエラーが発生しませんでした")
			}
		})
	}
}

func TestWebhookConfig_GetConfigSummary(t *testing.T) {
	config := &WebhookConfig{
		Port:           8443,
		LogLevel:       "info",
		Environment:    "development",
		MetricsEnabled: true,
	}

	summary := config.GetConfigSummary()
	if summary["port"] != 8443 {
		t.Errorf("サマリーのポートが正しくありません: %v", summary["port"])
	}
	if summary["log_level"] != "info" {
		t.Errorf("サマリーのログレベルが正しくありません: %v", summary["log_level"])
	}
	if summary["environment"] != "development" {
		t.Errorf("サマリーの環境が正しくありません: %v", summary["environment"])
	}
	if summary["metrics_enabled"] != true {
		t.Errorf("サマリーのメトリクス設定が正しくありません: %v", summary["metrics_enabled"])
	}
}

func TestLoadConfigForEnvironment(t *testing.T) {
	// テスト用の一時的な設定ファイルを作成
	testConfigDir := "test_configs"
	os.MkdirAll(testConfigDir, 0755)
	defer os.RemoveAll(testConfigDir)

	// 開発環境設定ファイルを作成
	devConfig := `environment: development
log_level: debug
failure_policy: Ignore
cluster_name: "dev-cluster"
metrics_enabled: true
health_enabled: true`
	
	devConfigPath := filepath.Join(testConfigDir, "development.yaml")
	if err := os.WriteFile(devConfigPath, []byte(devConfig), 0644); err != nil {
		t.Fatalf("テスト設定ファイルの作成に失敗しました: %v", err)
	}

	// ステージング環境設定ファイルを作成
	stagingConfig := `environment: staging
log_level: info
failure_policy: Fail
cluster_name: "staging-cluster"
metrics_enabled: true
health_enabled: true`
	
	stagingConfigPath := filepath.Join(testConfigDir, "staging.yaml")
	if err := os.WriteFile(stagingConfigPath, []byte(stagingConfig), 0644); err != nil {
		t.Fatalf("テスト設定ファイルの作成に失敗しました: %v", err)
	}

	// 本番環境設定ファイルを作成
	prodConfig := `environment: production
log_level: warn
failure_policy: Fail
cluster_name: "prod-cluster"
metrics_enabled: true
health_enabled: true`
	
	prodConfigPath := filepath.Join(testConfigDir, "production.yaml")
	if err := os.WriteFile(prodConfigPath, []byte(prodConfig), 0644); err != nil {
		t.Fatalf("テスト設定ファイルの作成に失敗しました: %v", err)
	}

	// 開発環境設定のテスト
	loader := NewConfigLoaderWithFile(devConfigPath)
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("開発環境設定の読み込みに失敗しました: %v", err)
	}

	if config.Environment != "development" {
		t.Errorf("期待される環境: development, 実際: %s", config.Environment)
	}
	if config.LogLevel != "debug" {
		t.Errorf("期待されるログレベル: debug, 実際: %s", config.LogLevel)
	}
	if config.FailurePolicy != "Ignore" {
		t.Errorf("期待される失敗ポリシー: Ignore, 実際: %s", config.FailurePolicy)
	}

	// ステージング環境設定のテスト
	loader = NewConfigLoaderWithFile(stagingConfigPath)
	config, err = loader.LoadConfig()
	if err != nil {
		t.Fatalf("ステージング環境設定の読み込みに失敗しました: %v", err)
	}

	if config.Environment != "staging" {
		t.Errorf("期待される環境: staging, 実際: %s", config.Environment)
	}
	if config.LogLevel != "info" {
		t.Errorf("期待されるログレベル: info, 実際: %s", config.LogLevel)
	}
	if config.FailurePolicy != "Fail" {
		t.Errorf("期待される失敗ポリシー: Fail, 実際: %s", config.FailurePolicy)
	}

	// 本番環境設定のテスト
	loader = NewConfigLoaderWithFile(prodConfigPath)
	config, err = loader.LoadConfig()
	if err != nil {
		t.Fatalf("本番環境設定の読み込みに失敗しました: %v", err)
	}

	if config.Environment != "production" {
		t.Errorf("期待される環境: production, 実際: %s", config.Environment)
	}
	if config.LogLevel != "warn" {
		t.Errorf("期待されるログレベル: warn, 実際: %s", config.LogLevel)
	}
	if config.FailurePolicy != "Fail" {
		t.Errorf("期待される失敗ポリシー: Fail, 実際: %s", config.FailurePolicy)
	}
}

func TestConfigLoader_LoadFromYAMLFile_NonExistentFile(t *testing.T) {
	loader := NewConfigLoaderWithFile("non-existent-file.yaml")
	config, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("存在しないファイルでエラーが発生しました: %v", err)
	}

	// デフォルト値が使用されることを確認
	if config.Environment != "development" {
		t.Errorf("期待される環境: development, 実際: %s", config.Environment)
	}
}