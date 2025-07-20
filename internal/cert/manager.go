package cert

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// 証明書の有効期限までの日数メトリクス
	certificateExpiryDays = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webhook_certificate_expiry_days",
			Help: "証明書の有効期限までの日数",
		},
		[]string{"cert_file", "subject", "issuer"},
	)

	// 証明書の有効性メトリクス
	certificateValid = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webhook_certificate_valid",
			Help: "証明書の有効性（1=有効、0=無効）",
		},
		[]string{"cert_file", "subject"},
	)

	// 証明書の更新回数メトリクス
	certificateReloads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_certificate_reloads_total",
			Help: "証明書の再読み込み回数",
		},
		[]string{"cert_file", "status"},
	)

	// 証明書監視エラーメトリクス
	certificateMonitoringErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_certificate_monitoring_errors_total",
			Help: "証明書監視エラーの回数",
		},
		[]string{"cert_file", "error_type"},
	)
)

// Manager handles TLS certificate operations
type Manager struct {
	certFile        string
	keyFile         string
	caFile          string
	currentCert     *tls.Certificate
	certMutex       sync.RWMutex
	monitoringCtx   context.Context
	monitoringStop  context.CancelFunc
	reloadCallback  func(tls.Certificate)
}

// NewManager creates a new certificate manager
func NewManager(certFile, keyFile, caFile string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		certFile:       certFile,
		keyFile:        keyFile,
		caFile:         caFile,
		monitoringCtx:  ctx,
		monitoringStop: cancel,
	}
}

// SetReloadCallback sets the callback function for certificate reload
func (m *Manager) SetReloadCallback(callback func(tls.Certificate)) {
	m.reloadCallback = callback
}

// LoadCertificate loads and validates TLS certificate
func (m *Manager) LoadCertificate() (tls.Certificate, error) {
	log.Printf("証明書を読み込み中: %s, %s", m.certFile, m.keyFile)
	
	// 証明書ファイルの存在確認
	if err := m.validateFileExists(m.certFile); err != nil {
		certificateReloads.WithLabelValues(m.certFile, "error").Inc()
		return tls.Certificate{}, fmt.Errorf("証明書ファイルが見つかりません: %w", err)
	}
	
	if err := m.validateFileExists(m.keyFile); err != nil {
		certificateReloads.WithLabelValues(m.certFile, "error").Inc()
		return tls.Certificate{}, fmt.Errorf("秘密鍵ファイルが見つかりません: %w", err)
	}
	
	// 証明書の読み込み
	cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
	if err != nil {
		certificateReloads.WithLabelValues(m.certFile, "error").Inc()
		return tls.Certificate{}, fmt.Errorf("証明書の読み込みに失敗しました: %w", err)
	}
	
	// 証明書の検証
	if err := m.validateCertificate(cert); err != nil {
		certificateReloads.WithLabelValues(m.certFile, "error").Inc()
		return tls.Certificate{}, fmt.Errorf("証明書の検証に失敗しました: %w", err)
	}
	
	// 現在の証明書を更新
	m.certMutex.Lock()
	m.currentCert = &cert
	m.certMutex.Unlock()
	
	// メトリクスを更新
	m.updateCertificateMetrics(cert)
	
	certificateReloads.WithLabelValues(m.certFile, "success").Inc()
	log.Printf("証明書の読み込みが完了しました")
	return cert, nil
}

// GetCurrentCertificate returns the current certificate safely
func (m *Manager) GetCurrentCertificate() *tls.Certificate {
	m.certMutex.RLock()
	defer m.certMutex.RUnlock()
	return m.currentCert
}

// StartMonitoring starts certificate monitoring and auto-reload
func (m *Manager) StartMonitoring(checkInterval time.Duration) {
	go m.monitorCertificate(checkInterval)
}

// StopMonitoring stops certificate monitoring
func (m *Manager) StopMonitoring() {
	if m.monitoringStop != nil {
		m.monitoringStop()
	}
}

// monitorCertificate monitors certificate files for changes and expiry
func (m *Manager) monitorCertificate(checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	
	var lastModTime time.Time
	
	// 初回のファイル更新時刻を取得
	if info, err := os.Stat(m.certFile); err == nil {
		lastModTime = info.ModTime()
	}
	
	for {
		select {
		case <-m.monitoringCtx.Done():
			log.Printf("証明書監視を停止しました")
			return
		case <-ticker.C:
			// ファイルの更新確認
			if info, err := os.Stat(m.certFile); err == nil {
				if info.ModTime().After(lastModTime) {
					log.Printf("証明書ファイルの更新を検出しました: %s", m.certFile)
					lastModTime = info.ModTime()
					
					// 証明書を再読み込み
					if cert, err := m.LoadCertificate(); err != nil {
						log.Printf("証明書の再読み込みに失敗しました: %v", err)
						certificateMonitoringErrors.WithLabelValues(m.certFile, "reload_failed").Inc()
					} else {
						log.Printf("証明書の再読み込みが完了しました")
						
						// コールバック関数を呼び出し
						if m.reloadCallback != nil {
							m.reloadCallback(cert)
						}
					}
				}
			} else {
				certificateMonitoringErrors.WithLabelValues(m.certFile, "file_access_error").Inc()
			}
			
			// 証明書の有効期限確認
			m.checkCertificateExpiry()
		}
	}
}

// checkCertificateExpiry checks certificate expiry and updates metrics
func (m *Manager) checkCertificateExpiry() {
	info, err := m.GetCertificateInfo()
	if err != nil {
		certificateMonitoringErrors.WithLabelValues(m.certFile, "expiry_check_failed").Inc()
		log.Printf("証明書の有効期限確認に失敗しました: %v", err)
		return
	}
	
	// 有効期限の警告
	if info.DaysUntilExpiry <= 30 && info.DaysUntilExpiry > 0 {
		log.Printf("警告: 証明書の有効期限まで %d 日です（%s）", info.DaysUntilExpiry, info.NotAfter.Format("2006-01-02"))
	} else if info.IsExpired {
		log.Printf("エラー: 証明書の有効期限が切れています（%s）", info.NotAfter.Format("2006-01-02"))
	}
}

// updateCertificateMetrics updates Prometheus metrics for the certificate
func (m *Manager) updateCertificateMetrics(cert tls.Certificate) {
	if len(cert.Certificate) == 0 {
		return
	}
	
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		log.Printf("メトリクス更新用の証明書パースに失敗しました: %v", err)
		return
	}
	
	subject := x509Cert.Subject.String()
	issuer := x509Cert.Issuer.String()
	daysUntilExpiry := int(time.Until(x509Cert.NotAfter).Hours() / 24)
	isValid := time.Now().Before(x509Cert.NotAfter) && time.Now().After(x509Cert.NotBefore)
	
	// メトリクスを更新
	certificateExpiryDays.WithLabelValues(m.certFile, subject, issuer).Set(float64(daysUntilExpiry))
	
	if isValid {
		certificateValid.WithLabelValues(m.certFile, subject).Set(1)
	} else {
		certificateValid.WithLabelValues(m.certFile, subject).Set(0)
	}
}

// ValidateCertificateChain validates the certificate chain with CA
func (m *Manager) ValidateCertificateChain() error {
	if m.caFile == "" {
		log.Printf("CA証明書ファイルが指定されていません。チェーン検証をスキップします")
		return nil
	}
	
	log.Printf("証明書チェーンを検証中: %s", m.caFile)
	
	// CA証明書の読み込み
	caCert, err := os.ReadFile(m.caFile)
	if err != nil {
		return fmt.Errorf("CA証明書の読み込みに失敗しました: %w", err)
	}
	
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("CA証明書のパースに失敗しました")
	}
	
	// サーバー証明書の読み込み
	serverCert, err := os.ReadFile(m.certFile)
	if err != nil {
		return fmt.Errorf("サーバー証明書の読み込みに失敗しました: %w", err)
	}
	
	block, _ := pem.Decode(serverCert)
	if block == nil {
		return fmt.Errorf("サーバー証明書のデコードに失敗しました")
	}
	
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("サーバー証明書のパースに失敗しました: %w", err)
	}
	
	// 証明書チェーンの検証
	opts := x509.VerifyOptions{
		Roots: caCertPool,
	}
	
	_, err = cert.Verify(opts)
	if err != nil {
		return fmt.Errorf("証明書チェーンの検証に失敗しました: %w", err)
	}
	
	log.Printf("証明書チェーンの検証が完了しました")
	return nil
}

// GetCertificateInfo returns certificate information
func (m *Manager) GetCertificateInfo() (*CertificateInfo, error) {
	// 証明書ファイルの読み込み
	certData, err := os.ReadFile(m.certFile)
	if err != nil {
		return nil, fmt.Errorf("証明書ファイルの読み込みに失敗しました: %w", err)
	}
	
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("証明書のデコードに失敗しました")
	}
	
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("証明書のパースに失敗しました: %w", err)
	}
	
	return &CertificateInfo{
		Subject:    cert.Subject.String(),
		Issuer:     cert.Issuer.String(),
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		DNSNames:   cert.DNSNames,
		IPAddresses: cert.IPAddresses,
		IsExpired:  time.Now().After(cert.NotAfter),
		DaysUntilExpiry: int(time.Until(cert.NotAfter).Hours() / 24),
	}, nil
}

// validateFileExists checks if file exists and is readable
func (m *Manager) validateFileExists(filename string) error {
	info, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ファイルが存在しません: %s", filename)
		}
		return fmt.Errorf("ファイルアクセスエラー: %w", err)
	}
	
	if info.IsDir() {
		return fmt.Errorf("ディレクトリが指定されました（ファイルが必要）: %s", filename)
	}
	
	// ファイルの読み取り権限確認
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("ファイルの読み取り権限がありません: %w", err)
	}
	file.Close()
	
	return nil
}

// validateCertificate validates the loaded certificate
func (m *Manager) validateCertificate(cert tls.Certificate) error {
	if len(cert.Certificate) == 0 {
		return fmt.Errorf("証明書が空です")
	}
	
	// X.509証明書としてパース
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("X.509証明書のパースに失敗しました: %w", err)
	}
	
	// 有効期限の確認
	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return fmt.Errorf("証明書はまだ有効ではありません（有効開始: %v）", x509Cert.NotBefore)
	}
	
	if now.After(x509Cert.NotAfter) {
		return fmt.Errorf("証明書の有効期限が切れています（有効期限: %v）", x509Cert.NotAfter)
	}
	
	// 有効期限が近い場合の警告
	daysUntilExpiry := int(time.Until(x509Cert.NotAfter).Hours() / 24)
	if daysUntilExpiry <= 30 {
		log.Printf("警告: 証明書の有効期限まで %d 日です", daysUntilExpiry)
	}
	
	log.Printf("証明書の検証が完了しました（有効期限: %v, 残り %d 日）", 
		x509Cert.NotAfter.Format("2006-01-02"), daysUntilExpiry)
	
	return nil
}

// CertificateInfo contains certificate information
type CertificateInfo struct {
	Subject         string
	Issuer          string
	NotBefore       time.Time
	NotAfter        time.Time
	DNSNames        []string
	IPAddresses     []net.IP
	IsExpired       bool
	DaysUntilExpiry int
}