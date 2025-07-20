### 6.2 サーバー初期化

```go
// NewServerWithConfig creates a new webhook server instance with configuration
func NewServerWithConfig(port int, certFile, keyFile, caFile string, cfg *config.WebhookConfig) (*Server, error) {
    // デフォルト設定を作成（設定が提供されていない場合）
    if cfg == nil {
        var err error
        cfg, err = config.LoadConfigWithDefaults()
        if err != nil {
            return nil, fmt.Errorf("デフォルト設定の読み込みに失敗しました: %w", err)
        }
    }

    // ロガーを初期化
    logger := logging.NewLogger("webhook")

    // Kubernetesクライアントを作成
    client, err := createKubernetesClient(logger)
    if err != nil {
        return nil, fmt.Errorf("Kubernetesクライアントの作成に失敗しました: %w", err)
    }

    // 証明書マネージャーを作成
    certManager := cert.NewManager(certFile, keyFile, caFile)

    // スキームとコーデックを初期化
    scheme := runtime.NewScheme()
    codecs := serializer.NewCodecFactory(scheme)

    // バリデーターを作成
    validator := validator.NewDeploymentHPAValidator(client)

    // エラーハンドラーを作成
    errorHandler := NewErrorHandler(logger)

    // サーバーインスタンスを作成
    s := &Server{
        client:       client,
        validator:    validator,
        scheme:       scheme,
        codecs:       codecs,
        certManager:  certManager,
        logger:       logger,
        config:       cfg,
        errorHandler: errorHandler,
    }

    // HTTPサーバーを設定
    s.server = &http.Server{
        Addr:      fmt.Sprintf(":%d", port),
        TLSConfig: &tls.Config{
            GetCertificate: certManager.GetCertificate,
            MinVersion:     tls.VersionTLS12,
        },
        ReadHeaderTimeout: 10 * time.Second,
        WriteTimeout:      cfg.Timeout,
        ReadTimeout:       cfg.Timeout,
    }

    return s, nil
}
```

**解説：**
1. 設定が提供されていない場合はデフォルト設定を読み込み
2. ロガーを初期化
3. Kubernetesクライアントを作成
4. 証明書マネージャーを作成
5. スキームとコーデックを初期化（Kubernetesオブジェクトの変換用）
6. バリデーターを作成
7. エラーハンドラーを作成
8. サーバーインスタンスを作成
9. HTTPサーバーを設定（TLS、タイムアウトなど）