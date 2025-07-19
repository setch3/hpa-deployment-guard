package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificateIntegration(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := ioutil.TempDir("", "cert-integration-test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// CA証明書を生成
	caCertFile := filepath.Join(tempDir, "ca.crt")
	caKeyFile := filepath.Join(tempDir, "ca.key")
	if err := generateCACertificate(caCertFile, caKeyFile); err != nil {
		t.Fatalf("CA証明書の生成に失敗: %v", err)
	}

	// サーバー証明書を生成（CA署名）
	serverCertFile := filepath.Join(tempDir, "server.crt")
	serverKeyFile := filepath.Join(tempDir, "server.key")
	if err := generateServerCertificate(serverCertFile, serverKeyFile, caCertFile, caKeyFile); err != nil {
		t.Fatalf("サーバー証明書の生成に失敗: %v", err)
	}

	// 証明書マネージャーを作成
	manager := NewManager(serverCertFile, serverKeyFile, caCertFile)

	// 証明書の読み込みテスト
	t.Run("証明書読み込み", func(t *testing.T) {
		cert, err := manager.LoadCertificate()
		if err != nil {
			t.Fatalf("証明書の読み込みに失敗: %v", err)
		}

		if len(cert.Certificate) == 0 {
			t.Error("証明書が空です")
		}
	})

	// 証明書チェーン検証テスト
	t.Run("証明書チェーン検証", func(t *testing.T) {
		err := manager.ValidateCertificateChain()
		if err != nil {
			t.Fatalf("証明書チェーンの検証に失敗: %v", err)
		}
	})

	// 証明書情報取得テスト
	t.Run("証明書情報取得", func(t *testing.T) {
		info, err := manager.GetCertificateInfo()
		if err != nil {
			t.Fatalf("証明書情報の取得に失敗: %v", err)
		}

		if info.IsExpired {
			t.Error("新しく生成された証明書が期限切れとして報告されました")
		}

		if info.DaysUntilExpiry <= 0 {
			t.Error("有効期限までの日数が正しく計算されていません")
		}

		expectedDNSNames := []string{"localhost", "test.example.com", "k8s-deployment-hpa-validator.default.svc.cluster.local"}
		for _, expected := range expectedDNSNames {
			found := false
			for _, actual := range info.DNSNames {
				if actual == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("期待されるDNS名が見つかりません: %s", expected)
			}
		}
	})
}

func TestExpiredCertificate(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := ioutil.TempDir("", "cert-expired-test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certFile := filepath.Join(tempDir, "expired.crt")
	keyFile := filepath.Join(tempDir, "expired.key")

	// 期限切れの証明書を生成
	if err := generateExpiredCertificate(certFile, keyFile); err != nil {
		t.Fatalf("期限切れ証明書の生成に失敗: %v", err)
	}

	manager := NewManager(certFile, keyFile, "")

	// 期限切れ証明書の読み込みテスト
	t.Run("期限切れ証明書の検出", func(t *testing.T) {
		_, err := manager.LoadCertificate()
		if err == nil {
			t.Error("期限切れ証明書でエラーが発生しませんでした")
		}
	})

	// 証明書情報での期限切れ検出テスト
	t.Run("証明書情報での期限切れ検出", func(t *testing.T) {
		info, err := manager.GetCertificateInfo()
		if err != nil {
			t.Fatalf("証明書情報の取得に失敗: %v", err)
		}

		if !info.IsExpired {
			t.Error("期限切れ証明書が期限切れとして検出されませんでした")
		}

		if info.DaysUntilExpiry >= 0 {
			t.Error("期限切れ証明書の残り日数が正しく計算されていません")
		}
	})
}

// generateCACertificate generates a CA certificate for testing
func generateCACertificate(certFile, keyFile string) error {
	// CA秘密鍵の生成
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// CA証明書テンプレートの作成
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test CA"},
			Country:       []string{"JP"},
			Province:      []string{"Tokyo"},
			Locality:      []string{"Tokyo"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// CA証明書の生成（自己署名）
	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return err
	}

	// CA証明書をPEM形式でファイルに保存
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}); err != nil {
		return err
	}

	// CA秘密鍵をPEM形式でファイルに保存
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	caPrivateKeyDER, err := x509.MarshalPKCS8PrivateKey(caPrivateKey)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: caPrivateKeyDER}); err != nil {
		return err
	}

	return nil
}

// generateServerCertificate generates a server certificate signed by CA
func generateServerCertificate(certFile, keyFile, caCertFile, caKeyFile string) error {
	// CA証明書と秘密鍵の読み込み
	caCertPEM, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return err
	}

	caKeyPEM, err := ioutil.ReadFile(caKeyFile)
	if err != nil {
		return err
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return err
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return err
	}

	// サーバー秘密鍵の生成
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// サーバー証明書テンプレートの作成
	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:  []string{"Test Server"},
			Country:       []string{"JP"},
			Province:      []string{"Tokyo"},
			Locality:      []string{"Tokyo"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames: []string{
			"localhost",
			"test.example.com",
			"k8s-deployment-hpa-validator.default.svc.cluster.local",
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	// サーバー証明書の生成（CA署名）
	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverPrivateKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	// サーバー証明書をPEM形式でファイルに保存
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER}); err != nil {
		return err
	}

	// サーバー秘密鍵をPEM形式でファイルに保存
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	serverPrivateKeyDER, err := x509.MarshalPKCS8PrivateKey(serverPrivateKey)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: serverPrivateKeyDER}); err != nil {
		return err
	}

	return nil
}

// generateExpiredCertificate generates an expired certificate for testing
func generateExpiredCertificate(certFile, keyFile string) error {
	// 秘密鍵の生成
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// 期限切れ証明書テンプレートの作成
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Expired Test Organization"},
			Country:       []string{"JP"},
			Province:      []string{"Tokyo"},
			Locality:      []string{"Tokyo"},
		},
		NotBefore:             time.Now().Add(-48 * time.Hour), // 2日前から
		NotAfter:              time.Now().Add(-24 * time.Hour), // 1日前まで（期限切れ）
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "expired.example.com"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	// 証明書の生成（自己署名）
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// 証明書をPEM形式でファイルに保存
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// 秘密鍵をPEM形式でファイルに保存
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return err
	}

	return nil
}