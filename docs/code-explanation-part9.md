### 7.3 証明書の自動更新

```go
// StartCertificateRenewal starts a goroutine to periodically check and renew the certificate
func (m *Manager) StartCertificateRenewal(reloadFunc func(tls.Certificate)) {
    go func() {
        // 初期証明書を読み込み
        cert, err := m.LoadCertificate()
        if err != nil {
            m.logger.Error("初期証明書の読み込みに失敗しました", map[string]interface{}{
                "error": err.Error(),
            })
            return
        }

        // 証明書の有効期限を取得
        x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
        if err != nil {
            m.logger.Error("X.509証明書の解析に失敗しました", map[string]interface{}{
                "error": err.Error(),
            })
            return
        }

        // 更新チェックの間隔を計算（有効期限の1/4）
        checkInterval := x509Cert.NotAfter.Sub(x509Cert.NotBefore) / 4
        if checkInterval > 24*time.Hour {
            checkInterval = 24 * time.Hour // 最大1日
        }
        if checkInterval < time.Hour {
            checkInterval = time.Hour // 最小1時間
        }

        ticker := time.NewTicker(checkInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                // ファイルの更新を確認
                newCert, err := m.LoadCertificate()
                if err != nil {
                    m.logger.Error("証明書の更新チェックに失敗しました", map[string]interface{}{
                        "error": err.Error(),
                    })
                    continue
                }

                // 証明書が変更されていれば更新
                if !certificatesEqual(cert, newCert) {
                    m.logger.Info("証明書が更新されました", nil)
                    cert = newCert
                    if reloadFunc != nil {
                        reloadFunc(newCert)
                    }
                }
            case <-m.ctx.Done():
                m.logger.Info("証明書更新チェッカーを停止します", nil)
                return
            }
        }
    }()
}
```

**解説：**
1. 別のゴルーチン（並行処理）で証明書の監視を開始
2. 初期証明書を読み込み
3. 更新チェックの間隔を計算（証明書の有効期間の1/4、最小1時間、最大1日）
4. 定期的に証明書ファイルをチェック
5. 証明書が変更されていれば新しい証明書を読み込み
6. 更新された証明書をサーバーに反映（reloadFunc）

## 8. メトリクス収集 (internal/metrics/metrics.go)

### 8.1 メトリクス定義

```go
var (
    // webhook_requests_total - リクエスト総数
    WebhookRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "webhook_requests_total",
            Help: "Webhookリクエストの総数",
        },
        []string{"method", "status", "resource_type"},
    )

    // webhook_request_duration_seconds - リクエスト処理時間
    WebhookRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "webhook_request_duration_seconds",
            Help:    "Webhookリクエストの処理時間（秒）",
            Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
        },
        []string{"method", "resource_type"},
    )

    // webhook_validation_errors_total - バリデーションエラー数
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
)
```

**解説：**
- `WebhookRequestsTotal`: リクエスト数をカウント（メソッド、ステータス、リソースタイプ別）
- `WebhookRequestDuration`: リクエスト処理時間を計測（ヒストグラム）
- `WebhookValidationErrors`: バリデーションエラー数をカウント（エラータイプ、リソースタイプ別）
- `WebhookCertificateExpiryDays`: 証明書の有効期限までの日数を記録