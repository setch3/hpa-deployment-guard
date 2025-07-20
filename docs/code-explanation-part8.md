## 7. 証明書管理 (internal/cert/manager.go)

### 7.1 証明書マネージャー

```go
// Manager は証明書管理を行う構造体
type Manager struct {
    certFile    string
    keyFile     string
    caFile      string
    certificate *tls.Certificate
    ctx         context.Context
    cancel      context.CancelFunc
    logger      *logging.Logger
}

// NewManager creates a new certificate manager
func NewManager(certFile, keyFile, caFile string) *Manager {
    ctx, cancel := context.WithCancel(context.Background())
    return &Manager{
        certFile: certFile,
        keyFile:  keyFile,
        caFile:   caFile,
        ctx:      ctx,
        cancel:   cancel,
        logger:   logging.NewLogger("cert-manager"),
    }
}
```

**解説：**
- `Manager` 構造体は証明書ファイルのパスと読み込んだ証明書を保持
- `NewManager` 関数で証明書マネージャーのインスタンスを作成

### 7.2 証明書の読み込み

```go
// LoadCertificate loads the TLS certificate from files
func (m *Manager) LoadCertificate() (tls.Certificate, error) {
    m.logger.Info("証明書を読み込み中", map[string]interface{}{
        "cert_file": m.certFile,
        "key_file":  m.keyFile,
    })

    // 証明書と秘密鍵を読み込み
    cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
    if err != nil {
        return tls.Certificate{}, fmt.Errorf("証明書の読み込みに失敗しました: %w", err)
    }

    // X.509証明書を解析
    x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
    if err != nil {
        return cert, fmt.Errorf("X.509証明書の解析に失敗しました: %w", err)
    }

    // 有効期限を確認
    now := time.Now()
    if now.After(x509Cert.NotAfter) {
        m.logger.Warn("証明書が期限切れです", map[string]interface{}{
            "expiry": x509Cert.NotAfter,
        })
    } else {
        daysUntilExpiry := int(x509Cert.NotAfter.Sub(now).Hours() / 24)
        m.logger.Info("証明書の検証が完了しました", map[string]interface{}{
            "expiry":            x509Cert.NotAfter,
            "days_until_expiry": daysUntilExpiry,
        })

        // メトリクスを更新
        metrics.UpdateCertificateExpiry(daysUntilExpiry)
    }

    // 証明書を保存
    m.certificate = &cert
    m.logger.Info("証明書の読み込みが完了しました")

    return cert, nil
}
```

**解説：**
1. 証明書と秘密鍵をファイルから読み込み
2. X.509証明書を解析
3. 証明書の有効期限を確認
4. 有効期限までの日数を計算してログに出力
5. メトリクスを更新（証明書の有効期限）
6. 読み込んだ証明書を保存して返す