package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// WebhookConfig webhook設定構造体
type WebhookConfig struct {
	// サーバー設定
	Port        int           `yaml:"port" env:"WEBHOOK_PORT" default:"8443"`
	TLSCertFile string        `yaml:"tls_cert_file" env:"TLS_CERT_FILE" default:"/etc/certs/tls.crt"`
	TLSKeyFile  string        `yaml:"tls_key_file" env:"TLS_KEY_FILE" default:"/etc/certs/tls.key"`
	Timeout     time.Duration `yaml:"timeout" env:"WEBHOOK_TIMEOUT" default:"10s"`

	// ログ設定
	LogLevel  string `yaml:"log_level" env:"LOG_LEVEL" default:"info"`
	LogFormat string `yaml:"log_format" env:"LOG_FORMAT" default:"json"`

	// バリデーション設定
	SkipNamespaces []string `yaml:"skip_namespaces" env:"SKIP_NAMESPACES"`
	SkipLabels     []string `yaml:"skip_labels" env:"SKIP_LABELS"`

	// 監視設定
	MetricsEnabled bool `yaml:"metrics_enabled" env:"METRICS_ENABLED" default:"true"`
	MetricsPort    int  `yaml:"metrics_port" env:"METRICS_PORT" default:"8080"`
	HealthEnabled  bool `yaml:"health_enabled" env:"HEALTH_ENABLED" default:"true"`

	// 環境情報
	Environment string `yaml:"environment" env:"ENVIRONMENT" default:"development"`
	ClusterName string `yaml:"cluster_name" env:"CLUSTER_NAME"`

	// 失敗ポリシー
	FailurePolicy string `yaml:"failure_policy" env:"FAILURE_POLICY" default:"Fail"`
}

// ConfigLoader 設定ローダー
type ConfigLoader struct {
	configMapData map[string]string
	configFile    string
}

// NewConfigLoader 新しい設定ローダーを作成
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		configMapData: make(map[string]string),
	}
}

// NewConfigLoaderWithConfigMap ConfigMapデータを使用して設定ローダーを作成
func NewConfigLoaderWithConfigMap(configMapData map[string]string) *ConfigLoader {
	return &ConfigLoader{
		configMapData: configMapData,
	}
}

// NewConfigLoaderWithFile 設定ファイルを指定して設定ローダーを作成
func NewConfigLoaderWithFile(configFile string) *ConfigLoader {
	return &ConfigLoader{
		configMapData: make(map[string]string),
		configFile:    configFile,
	}
}

// NewConfigLoaderWithFileAndConfigMap 設定ファイルとConfigMapデータを使用して設定ローダーを作成
func NewConfigLoaderWithFileAndConfigMap(configFile string, configMapData map[string]string) *ConfigLoader {
	return &ConfigLoader{
		configMapData: configMapData,
		configFile:    configFile,
	}
}

// LoadConfig 設定を読み込み
func (cl *ConfigLoader) LoadConfig() (*WebhookConfig, error) {
	config := &WebhookConfig{}

	// デフォルト値を設定
	if err := cl.setDefaults(config); err != nil {
		return nil, fmt.Errorf("デフォルト値の設定に失敗しました: %w", err)
	}

	// YAMLファイルから設定を読み込み
	if err := cl.loadFromYAMLFile(config); err != nil {
		return nil, fmt.Errorf("YAMLファイルからの設定読み込みに失敗しました: %w", err)
	}

	// ConfigMapから設定を読み込み
	if err := cl.loadFromConfigMap(config); err != nil {
		return nil, fmt.Errorf("ConfigMapからの設定読み込みに失敗しました: %w", err)
	}

	// 環境変数から設定を読み込み（優先度が高い）
	if err := cl.loadFromEnv(config); err != nil {
		return nil, fmt.Errorf("環境変数からの設定読み込みに失敗しました: %w", err)
	}

	// 設定の妥当性を検証
	if err := cl.validateConfig(config); err != nil {
		return nil, fmt.Errorf("設定の検証に失敗しました: %w", err)
	}

	return config, nil
}

// setDefaults デフォルト値を設定
func (cl *ConfigLoader) setDefaults(config *WebhookConfig) error {
	config.Port = 8443
	config.TLSCertFile = "/etc/certs/tls.crt"
	config.TLSKeyFile = "/etc/certs/tls.key"
	config.Timeout = 10 * time.Second
	config.LogLevel = "info"
	config.LogFormat = "json"
	config.MetricsEnabled = true
	config.MetricsPort = 8080
	config.HealthEnabled = true
	config.Environment = "development"
	config.FailurePolicy = "Fail"
	config.SkipNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}
	config.SkipLabels = []string{"k8s-deployment-hpa-validator.io/skip-validation=true"}

	return nil
}

// loadFromYAMLFile YAMLファイルから設定を読み込み
func (cl *ConfigLoader) loadFromYAMLFile(config *WebhookConfig) error {
	if cl.configFile == "" {
		// 環境変数からファイルパスを取得
		if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
			cl.configFile = configFile
		} else {
			// 環境に基づいてデフォルトファイルを決定
			environment := os.Getenv("ENVIRONMENT")
			if environment == "" {
				environment = "development"
			}
			cl.configFile = filepath.Join("configs", environment+".yaml")
		}
	}

	// ファイルが存在しない場合はスキップ
	if _, err := os.Stat(cl.configFile); os.IsNotExist(err) {
		// テスト環境では相対パスを試す
		if !filepath.IsAbs(cl.configFile) {
			// 現在のディレクトリから相対パスを試す
			if _, err := os.Stat(cl.configFile); os.IsNotExist(err) {
				return nil // ファイルが存在しない場合はエラーにしない
			}
		} else {
			return nil // ファイルが存在しない場合はエラーにしない
		}
	}

	// YAMLファイルを読み込み
	data, err := os.ReadFile(cl.configFile)
	if err != nil {
		return fmt.Errorf("設定ファイルの読み込みに失敗しました (%s): %w", cl.configFile, err)
	}

	// YAMLをパース
	var yamlConfig WebhookConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("YAMLの解析に失敗しました (%s): %w", cl.configFile, err)
	}

	// 設定をマージ（ゼロ値でない場合のみ上書き）
	if yamlConfig.Port != 0 {
		config.Port = yamlConfig.Port
	}
	if yamlConfig.TLSCertFile != "" {
		config.TLSCertFile = yamlConfig.TLSCertFile
	}
	if yamlConfig.TLSKeyFile != "" {
		config.TLSKeyFile = yamlConfig.TLSKeyFile
	}
	if yamlConfig.Timeout != 0 {
		config.Timeout = yamlConfig.Timeout
	}
	if yamlConfig.LogLevel != "" {
		config.LogLevel = yamlConfig.LogLevel
	}
	if yamlConfig.LogFormat != "" {
		config.LogFormat = yamlConfig.LogFormat
	}
	if len(yamlConfig.SkipNamespaces) > 0 {
		config.SkipNamespaces = append(config.SkipNamespaces, yamlConfig.SkipNamespaces...)
	}
	if len(yamlConfig.SkipLabels) > 0 {
		config.SkipLabels = append(config.SkipLabels, yamlConfig.SkipLabels...)
	}
	if yamlConfig.MetricsPort != 0 {
		config.MetricsPort = yamlConfig.MetricsPort
	}
	if yamlConfig.Environment != "" {
		config.Environment = yamlConfig.Environment
	}
	if yamlConfig.ClusterName != "" {
		config.ClusterName = yamlConfig.ClusterName
	}
	if yamlConfig.FailurePolicy != "" {
		config.FailurePolicy = yamlConfig.FailurePolicy
	}

	// ブール値の処理（YAMLで明示的に設定された場合のみ上書き）
	// YAMLファイルでブール値が設定されている場合は上書き
	config.MetricsEnabled = yamlConfig.MetricsEnabled
	config.HealthEnabled = yamlConfig.HealthEnabled

	return nil
}

// loadFromConfigMap ConfigMapから設定を読み込み
func (cl *ConfigLoader) loadFromConfigMap(config *WebhookConfig) error {
	if len(cl.configMapData) == 0 {
		return nil // ConfigMapデータがない場合はスキップ
	}

	// ポート設定
	if portStr, exists := cl.configMapData["webhook.port"]; exists {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	// TLS証明書設定
	if certFile, exists := cl.configMapData["webhook.tls-cert-file"]; exists {
		config.TLSCertFile = certFile
	}
	if keyFile, exists := cl.configMapData["webhook.tls-key-file"]; exists {
		config.TLSKeyFile = keyFile
	}

	// タイムアウト設定
	if timeoutStr, exists := cl.configMapData["webhook.timeout"]; exists {
		if timeout, err := time.ParseDuration(timeoutStr + "s"); err == nil {
			config.Timeout = timeout
		}
	}

	// ログ設定
	if logLevel, exists := cl.configMapData["log.level"]; exists {
		config.LogLevel = logLevel
	}
	if logFormat, exists := cl.configMapData["log.format"]; exists {
		config.LogFormat = logFormat
	}

	// バリデーション設定
	if skipNamespaces, exists := cl.configMapData["validation.skip-namespaces"]; exists {
		additionalNamespaces := strings.Split(skipNamespaces, ",")
		// 空白を削除
		for i, ns := range additionalNamespaces {
			additionalNamespaces[i] = strings.TrimSpace(ns)
		}
		// デフォルト値に追加
		config.SkipNamespaces = append(config.SkipNamespaces, additionalNamespaces...)
	}
	if skipLabels, exists := cl.configMapData["validation.skip-labels"]; exists {
		additionalLabels := strings.Split(skipLabels, ",")
		// 空白を削除
		for i, label := range additionalLabels {
			additionalLabels[i] = strings.TrimSpace(label)
		}
		// デフォルト値に追加
		config.SkipLabels = append(config.SkipLabels, additionalLabels...)
	}

	// 監視設定
	if metricsEnabled, exists := cl.configMapData["metrics.enabled"]; exists {
		config.MetricsEnabled = strings.ToLower(metricsEnabled) == "true"
	}
	if metricsPortStr, exists := cl.configMapData["metrics.port"]; exists {
		if port, err := strconv.Atoi(metricsPortStr); err == nil {
			config.MetricsPort = port
		}
	}
	if healthEnabled, exists := cl.configMapData["health.enabled"]; exists {
		config.HealthEnabled = strings.ToLower(healthEnabled) == "true"
	}

	// 環境情報
	if environment, exists := cl.configMapData["environment"]; exists {
		config.Environment = environment
	}
	if clusterName, exists := cl.configMapData["cluster.name"]; exists {
		config.ClusterName = clusterName
	}

	// 失敗ポリシー
	if failurePolicy, exists := cl.configMapData["webhook.failure-policy"]; exists {
		config.FailurePolicy = failurePolicy
	}

	return nil
}

// loadFromEnv 環境変数から設定を読み込み
func (cl *ConfigLoader) loadFromEnv(config *WebhookConfig) error {
	// ポート設定
	if portStr := os.Getenv("WEBHOOK_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		} else {
			return fmt.Errorf("無効なWEBHOOK_PORT値: %s", portStr)
		}
	}

	// TLS証明書設定
	if certFile := os.Getenv("TLS_CERT_FILE"); certFile != "" {
		config.TLSCertFile = certFile
	}
	if keyFile := os.Getenv("TLS_KEY_FILE"); keyFile != "" {
		config.TLSKeyFile = keyFile
	}

	// タイムアウト設定
	if timeoutStr := os.Getenv("WEBHOOK_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.Timeout = timeout
		} else {
			return fmt.Errorf("無効なWEBHOOK_TIMEOUT値: %s", timeoutStr)
		}
	}

	// ログ設定
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		config.LogFormat = logFormat
	}

	// バリデーション設定
	if skipNamespaces := os.Getenv("SKIP_NAMESPACES"); skipNamespaces != "" {
		additionalNamespaces := strings.Split(skipNamespaces, ",")
		// 空白を削除
		for i, ns := range additionalNamespaces {
			additionalNamespaces[i] = strings.TrimSpace(ns)
		}
		// デフォルト値に追加
		config.SkipNamespaces = append(config.SkipNamespaces, additionalNamespaces...)
	}
	if skipLabels := os.Getenv("SKIP_LABELS"); skipLabels != "" {
		additionalLabels := strings.Split(skipLabels, ",")
		// 空白を削除
		for i, label := range additionalLabels {
			additionalLabels[i] = strings.TrimSpace(label)
		}
		// デフォルト値に追加
		config.SkipLabels = append(config.SkipLabels, additionalLabels...)
	}

	// 監視設定
	if metricsEnabled := os.Getenv("METRICS_ENABLED"); metricsEnabled != "" {
		config.MetricsEnabled = strings.ToLower(metricsEnabled) == "true"
	}
	if metricsPortStr := os.Getenv("METRICS_PORT"); metricsPortStr != "" {
		if port, err := strconv.Atoi(metricsPortStr); err == nil {
			config.MetricsPort = port
		} else {
			return fmt.Errorf("無効なMETRICS_PORT値: %s", metricsPortStr)
		}
	}
	if healthEnabled := os.Getenv("HEALTH_ENABLED"); healthEnabled != "" {
		config.HealthEnabled = strings.ToLower(healthEnabled) == "true"
	}

	// 環境情報
	if environment := os.Getenv("ENVIRONMENT"); environment != "" {
		config.Environment = environment
	}
	if clusterName := os.Getenv("CLUSTER_NAME"); clusterName != "" {
		config.ClusterName = clusterName
	}

	// 失敗ポリシー
	if failurePolicy := os.Getenv("FAILURE_POLICY"); failurePolicy != "" {
		config.FailurePolicy = failurePolicy
	}

	return nil
}

// validateConfig 設定の妥当性を検証
func (cl *ConfigLoader) validateConfig(config *WebhookConfig) error {
	// 必須設定の検証
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("無効なポート番号: %d", config.Port)
	}

	if config.TLSCertFile == "" {
		return fmt.Errorf("TLS証明書ファイルが指定されていません")
	}

	if config.TLSKeyFile == "" {
		return fmt.Errorf("TLS秘密鍵ファイルが指定されていません")
	}

	// ログレベルの検証
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, strings.ToLower(config.LogLevel)) {
		return fmt.Errorf("無効なログレベル: %s (有効な値: %v)", config.LogLevel, validLogLevels)
	}

	// ログフォーマットの検証
	validLogFormats := []string{"json", "text"}
	if !contains(validLogFormats, strings.ToLower(config.LogFormat)) {
		return fmt.Errorf("無効なログフォーマット: %s (有効な値: %v)", config.LogFormat, validLogFormats)
	}

	// 環境の検証
	validEnvironments := []string{"development", "staging", "production"}
	if !contains(validEnvironments, strings.ToLower(config.Environment)) {
		return fmt.Errorf("無効な環境: %s (有効な値: %v)", config.Environment, validEnvironments)
	}

	// 失敗ポリシーの検証
	validFailurePolicies := []string{"Fail", "Ignore"}
	if !contains(validFailurePolicies, config.FailurePolicy) {
		return fmt.Errorf("無効な失敗ポリシー: %s (有効な値: %v)", config.FailurePolicy, validFailurePolicies)
	}

	// メトリクスポートの検証
	if config.MetricsPort <= 0 || config.MetricsPort > 65535 {
		return fmt.Errorf("無効なメトリクスポート番号: %d", config.MetricsPort)
	}

	// ポートの重複チェック
	if config.Port == config.MetricsPort {
		return fmt.Errorf("webhookポートとメトリクスポートが重複しています: %d", config.Port)
	}

	return nil
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

// GetConfigSummary 設定の要約を取得（デバッグ用）
func (config *WebhookConfig) GetConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"port":             config.Port,
		"tls_cert_file":    config.TLSCertFile,
		"tls_key_file":     config.TLSKeyFile,
		"timeout":          config.Timeout.String(),
		"log_level":        config.LogLevel,
		"log_format":       config.LogFormat,
		"skip_namespaces":  config.SkipNamespaces,
		"skip_labels":      config.SkipLabels,
		"metrics_enabled":  config.MetricsEnabled,
		"metrics_port":     config.MetricsPort,
		"health_enabled":   config.HealthEnabled,
		"environment":      config.Environment,
		"cluster_name":     config.ClusterName,
		"failure_policy":   config.FailurePolicy,
	}
}

// IsProductionEnvironment 本番環境かどうかを判定
func (config *WebhookConfig) IsProductionEnvironment() bool {
	return strings.ToLower(config.Environment) == "production"
}

// IsDevelopmentEnvironment 開発環境かどうかを判定
func (config *WebhookConfig) IsDevelopmentEnvironment() bool {
	return strings.ToLower(config.Environment) == "development"
}

// ShouldSkipNamespace 指定されたnamespaceをスキップするかどうかを判定
func (config *WebhookConfig) ShouldSkipNamespace(namespace string) bool {
	for _, skipNs := range config.SkipNamespaces {
		if skipNs == namespace {
			return true
		}
	}
	return false
}

// ShouldSkipByLabel 指定されたラベルでスキップするかどうかを判定
func (config *WebhookConfig) ShouldSkipByLabel(labels map[string]string) bool {
	for _, skipLabel := range config.SkipLabels {
		parts := strings.SplitN(skipLabel, "=", 2)
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			if labelValue, exists := labels[key]; exists && labelValue == value {
				return true
			}
		}
	}
	return false
}

// LoadConfigForEnvironment 指定された環境の設定を読み込み
func LoadConfigForEnvironment(environment string) (*WebhookConfig, error) {
	configFile := filepath.Join("configs", environment+".yaml")
	loader := NewConfigLoaderWithFile(configFile)
	return loader.LoadConfig()
}

// LoadConfigWithFile 指定されたファイルから設定を読み込み
func LoadConfigWithFile(configFile string) (*WebhookConfig, error) {
	loader := NewConfigLoaderWithFile(configFile)
	return loader.LoadConfig()
}

// LoadConfigWithDefaults デフォルト設定を読み込み（環境変数とConfigMapのみ）
func LoadConfigWithDefaults() (*WebhookConfig, error) {
	loader := NewConfigLoader()
	return loader.LoadConfig()
}

// ValidateConfig 設定の妥当性を検証（外部からアクセス可能）
func (cl *ConfigLoader) ValidateConfig(config *WebhookConfig) error {
	return cl.validateConfig(config)
}