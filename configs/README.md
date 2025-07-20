# 設定ファイル

このディレクトリには、k8s-deployment-hpa-validatorの環境別設定ファイルが含まれています。

## 設定ファイルの構造

### 環境別設定ファイル

- `development.yaml` - 開発環境用設定
- `staging.yaml` - ステージング環境用設定  
- `production.yaml` - 本番環境用設定

### 設定の優先順位

設定は以下の優先順位で読み込まれます（後のものが優先されます）：

1. デフォルト値
2. YAMLファイル
3. ConfigMap
4. 環境変数

### 設定項目

#### サーバー設定
- `port`: Webhookサーバーのポート番号（デフォルト: 8443）
- `timeout`: リクエストタイムアウト（デフォルト: 10s）
- `tls_cert_file`: TLS証明書ファイルのパス
- `tls_key_file`: TLS秘密鍵ファイルのパス

#### ログ設定
- `log_level`: ログレベル（debug, info, warn, error）
- `log_format`: ログフォーマット（json, text）

#### バリデーション設定
- `skip_namespaces`: バリデーションをスキップするnamespace一覧
- `skip_labels`: バリデーションをスキップするラベル一覧

#### 監視設定
- `metrics_enabled`: メトリクス収集の有効/無効
- `metrics_port`: メトリクスサーバーのポート番号
- `health_enabled`: ヘルスチェックの有効/無効

#### 環境情報
- `environment`: 環境名（development, staging, production）
- `cluster_name`: クラスター名
- `failure_policy`: 失敗時のポリシー（Fail, Ignore）

## 使用方法

### 環境変数による設定ファイル指定

```bash
export CONFIG_FILE=/path/to/config.yaml
export ENVIRONMENT=production
```

### プログラムでの使用

```go
// 環境別設定の読み込み
config, err := config.LoadConfigForEnvironment("production")

// ファイル指定での読み込み
config, err := config.LoadConfigWithFile("configs/custom.yaml")

// デフォルト設定の読み込み
config, err := config.LoadConfigWithDefaults()
```

### 環境変数での上書き

設定ファイルの値は環境変数で上書きできます：

```bash
export WEBHOOK_PORT=9443
export LOG_LEVEL=debug
export ENVIRONMENT=development
export SKIP_NAMESPACES="test-ns1,test-ns2"
```

## 環境別の特徴

### 開発環境（development）
- ログレベル: debug
- ログフォーマット: text（読みやすさ重視）
- 失敗ポリシー: Ignore（開発時の利便性重視）
- タイムアウト: 10s

### ステージング環境（staging）
- ログレベル: info
- ログフォーマット: json（構造化ログ）
- 失敗ポリシー: Fail（本番環境に近い設定）
- タイムアウト: 15s

### 本番環境（production）
- ログレベル: warn（重要なログのみ）
- ログフォーマット: json（構造化ログ）
- 失敗ポリシー: Fail（厳格な検証）
- タイムアウト: 30s（安定性重視）

## ConfigMapとの連携

Kubernetes環境では、ConfigMapを使用して設定を管理できます：

```bash
kubectl apply -f manifests/configmap.yaml
```

ConfigMapの設定は環境変数よりも優先度が低く、YAMLファイルの設定を補完する形で使用されます。