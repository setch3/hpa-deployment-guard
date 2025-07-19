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

func TestManager_LoadCertificate(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := ioutil.TempDir("", "cert-test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	// テスト用の証明書を生成
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("テスト証明書の生成に失敗: %v", err)
	}

	tests := []struct {
		name        string
		certFile    string
		keyFile     string
		expectError bool
	}{
		{
			name:        "有効な証明書",
			certFile:    certFile,
			keyFile:     keyFile,
			expectError: false,
		},
		{
			name:        "存在しない証明書ファイル",
			certFile:    "/nonexistent/cert.crt",
			keyFile:     keyFile,
			expectError: true,
		},
		{
			name:        "存在しない秘密鍵ファイル",
			certFile:    certFile,
			keyFile:     "/nonexistent/key.key",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.certFile, tt.keyFile, "")
			_, err := manager.LoadCertificate()

			if tt.expectError && err == nil {
				t.Errorf("エラーが期待されましたが、エラーが発生しませんでした")
			}
			if !tt.expectError && err != nil {
				t.Errorf("エラーが期待されませんでしたが、エラーが発生しました: %v", err)
			}
		})
	}
}

func TestManager_GetCertificateInfo(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := ioutil.TempDir("", "cert-test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	// テスト用の証明書を生成
	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("テスト証明書の生成に失敗: %v", err)
	}

	manager := NewManager(certFile, keyFile, "")
	info, err := manager.GetCertificateInfo()
	if err != nil {
		t.Fatalf("証明書情報の取得に失敗: %v", err)
	}

	if info == nil {
		t.Fatal("証明書情報がnilです")
	}

	if info.IsExpired {
		t.Error("新しく生成された証明書が期限切れとして報告されました")
	}

	if len(info.DNSNames) == 0 {
		t.Error("DNS名が設定されていません")
	}

	if info.DaysUntilExpiry <= 0 {
		t.Error("有効期限までの日数が正しく計算されていません")
	}
}

func TestManager_ValidateFileExists(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir, err := ioutil.TempDir("", "cert-test")
	if err != nil {
		t.Fatalf("一時ディレクトリの作成に失敗: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// テスト用ファイルを作成
	testFile := filepath.Join(tempDir, "test.txt")
	if err := ioutil.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("テストファイルの作成に失敗: %v", err)
	}

	manager := NewManager("", "", "")

	tests := []struct {
		name        string
		filename    string
		expectError bool
	}{
		{
			name:        "存在するファイル",
			filename:    testFile,
			expectError: false,
		},
		{
			name:        "存在しないファイル",
			filename:    "/nonexistent/file.txt",
			expectError: true,
		},
		{
			name:        "ディレクトリ",
			filename:    tempDir,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateFileExists(tt.filename)

			if tt.expectError && err == nil {
				t.Errorf("エラーが期待されましたが、エラーが発生しませんでした")
			}
			if !tt.expectError && err != nil {
				t.Errorf("エラーが期待されませんでしたが、エラーが発生しました: %v", err)
			}
		})
	}
}

// generateTestCertificate generates a test certificate for testing
func generateTestCertificate(certFile, keyFile string) error {
	// 秘密鍵の生成
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// 証明書テンプレートの作成
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Organization"},
			Country:       []string{"JP"},
			Province:      []string{"Tokyo"},
			Locality:      []string{"Tokyo"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "test.example.com"},
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