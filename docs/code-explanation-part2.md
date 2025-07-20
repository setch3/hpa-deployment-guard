## 4. 設定管理 (internal/config/config.go)

```go
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
```

**解説：**
- `WebhookConfig` 構造体に設定項目を定義
- 各フィールドに `yaml` タグ、`env` タグ、`default` タグを設定
  - `yaml`: YAMLファイルからの読み込み時のキー名
  - `env`: 環境変数からの読み込み時の変数名
  - `default`: デフォルト値

```go
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
```

**解説：**
- 設定の読み込み順序（優先度が低い順）：
  1. デフォルト値
  2. YAMLファイル
  3. ConfigMap
  4. 環境変数
- 最後に `validateConfig()` で設定の妥当性を検証