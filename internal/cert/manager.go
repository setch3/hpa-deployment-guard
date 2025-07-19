package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// Manager handles TLS certificate operations
type Manager struct {
	certFile string
	keyFile  string
	caFile   string
}

// NewManager creates a new certificate manager
func NewManager(certFile, keyFile, caFile string) *Manager {
	return &Manager{
		certFile: certFile,
		keyFile:  keyFile,
		caFile:   caFile,
	}
}

// LoadCertificate loads and validates TLS certificate
func (m *Manager) LoadCertificate() (tls.Certificate, error) {
	log.Printf("証明書を読み込み中: %s, %s", m.certFile, m.keyFile)
	
	// 証明書ファイルの存在確認
	if err := m.validateFileExists(m.certFile); err != nil {
		return tls.Certificate{}, fmt.Errorf("証明書ファイルが見つかりません: %w", err)
	}
	
	if err := m.validateFileExists(m.keyFile); err != nil {
		return tls.Certificate{}, fmt.Errorf("秘密鍵ファイルが見つかりません: %w", err)
	}
	
	// 証明書の読み込み
	cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("証明書の読み込みに失敗しました: %w", err)
	}
	
	// 証明書の検証
	if err := m.validateCertificate(cert); err != nil {
		return tls.Certificate{}, fmt.Errorf("証明書の検証に失敗しました: %w", err)
	}
	
	log.Printf("証明書の読み込みが完了しました")
	return cert, nil
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