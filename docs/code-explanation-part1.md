# Webhookコードの詳細解説（初学者向け）

## 1. 全体構造

Kubernetes Deployment-HPA Validatorは以下のような構造になっています：

```
cmd/webhook/main.go          # メインエントリーポイント
internal/
  ├── cert/                  # 証明書管理
  ├── config/                # 設定管理
  ├── logging/               # ログ機能
  ├── metrics/               # メトリクス収集
  ├── validator/             # バリデーションロジック
  └── webhook/               # Webhookサーバー
```

## 2. 処理の流れ

1. `main.go` でサーバーを起動
2. クライアントからのリクエストを `webhook/server.go` で受け取る
3. リクエストを解析して `validator/validator.go` でチェック
4. 結果をクライアントに返す

## 3. メインエントリーポイント (cmd/webhook/main.go)

```go
package main

import (
    "flag"
    "log"
    
    "k8s-deployment-hpa-validator/internal/config"
    "k8s-deployment-hpa-validator/internal/webhook"
)

func main() {
    // コマンドライン引数の解析
    port := flag.Int("port", 8443, "Webhookサーバーのポート")
    certFile := flag.String("cert-file", "/etc/certs/tls.crt", "TLS証明書ファイル")
    keyFile := flag.String("key-file", "/etc/certs/tls.key", "TLS秘密鍵ファイル")
    flag.Parse()
    
    // 設定の読み込み
    cfg, err := config.LoadConfigWithDefaults()
    if err != nil {
        log.Fatalf("設定の読み込みに失敗しました: %v", err)
    }
    
    // Webhookサーバーの作成と起動
    server, err := webhook.NewServerWithConfig(*port, *certFile, *keyFile, "", cfg)
    if err != nil {
        log.Fatalf("Webhookサーバーの作成に失敗しました: %v", err)
    }
    
    // サーバーを起動して待機
    if err := server.Start(); err != nil {
        log.Fatalf("サーバーの起動に失敗しました: %v", err)
    }
}
```

**解説：**
- `flag` パッケージを使ってコマンドライン引数を処理
- `config.LoadConfigWithDefaults()` で設定を読み込み
- `webhook.NewServerWithConfig()` でサーバーを作成
- `server.Start()` でサーバーを起動して待機