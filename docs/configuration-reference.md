# 設定リファレンス

## 概要

このドキュメントは、k8s-deployment-hpa-validatorの全ての設定項目について詳細に説明します。環境別の推奨設定とベストプラクティスも含まれています。

## 設定方法

設定は以下の方法で行うことができます：

1. **環境変数** - コンテナの環境変数として設定
2. **ConfigMap** - Kubernetesの設定マップとして管理
3. **コマンドライン引数** - webhookバイナリの起動時に指定

優先順位：コマンドライン引数 > 環境変数 > ConfigMap > デフォルト値

## サーバー設定

### WEBHOOK_PORT
- **説明**: webhookサーバーがリッスンするポート番号
- **型**: 整数
- **デフォルト値**: `8443`
- **環境変数**: `WEBHOOK_PORT`
- **ConfigMap キー**: `webhook.port`
- **推奨値**:
  - 開発環境: `8443`
  - ステージング環境: `8443`
  - 本番環境: `8443`

```yaml
# ConfigMap例
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
data:
  webhook.port: "8443"
```

### TLS_CERT_FILE
- **説明**: TLS証明書ファイルのパス
- **型**: 文字列
- **デフォルト値**: `/etc/certs/tls.crt`
- **環境変数**: `TLS_CERT_FILE`
- **必須**: はい
- **推奨値**:
  - 開発環境: `/etc/certs/tls.crt`
  - ステージング環境: `/etc/certs/tls.crt`
  - 本番環境: `/etc/certs/tls.crt`

### TLS_KEY_FILE
- **説明**: TLS秘密鍵ファイルのパス
- **型**: 文字列
- **デフォルト値**: `/etc/certs/tls.key`
- **環境変数**: `TLS_KEY_FILE`
- **必須**: はい
- **推奨値**:
  - 開発環境: `/etc/certs/tls.key`
  - ステージング環境: `/etc/certs/tls.key`
  - 本番環境: `/etc/certs/tls.key`

### WEBHOOK_TIMEOUT
- **説明**: webhookリクエストのタイムアウト時間（秒）
- **型**: 整数
- **デフォルト値**: `10`
- **環境変数**: `WEBHOOK_TIMEOUT`
- **ConfigMap キー**: `webhook.timeout`
- **推奨値**:
  - 開発環境: `30` (デバッグ用に長め)
  - ステージング環境: `15`
  - 本番環境: `10`

## ログ設定

### LOG_LEVEL
- **説明**: ログ出力レベル
- **型**: 文字列
- **デフォルト値**: `info`
- **環境変数**: `LOG_LEVEL`
- **ConfigMap キー**: `log.level`
- **有効な値**: `debug`, `info`, `warn`, `error`
- **推奨値**:
  - 開発環境: `debug`
  - ステージング環境: `info`
  - 本番環境: `warn`

### LOG_FORMAT
- **説明**: ログ出力形式
- **型**: 文字列
- **デフォルト値**: `json`
- **環境変数**: `LOG_FORMAT`
- **ConfigMap キー**: `log.format`
- **有効な値**: `json`, `text`
- **推奨値**:
  - 開発環境: `text` (読みやすさ重視)
  - ステージング環境: `json`
  - 本番環境: `json` (ログ集約システム対応)

## バリデーション設定

### SKIP_NAMESPACES
- **説明**: バリデーションをスキップする名前空間のリスト（カンマ区切り）
- **型**: 文字列
- **デフォルト値**: `kube-system,kube-public,kube-node-lease`
- **環境変数**: `SKIP_NAMESPACES`
- **ConfigMap キー**: `validation.skip-namespaces`
- **推奨値**:
  - 開発環境: `kube-system,kube-public,kube-node-lease,test-system`
  - ステージング環境: `kube-system,kube-public,kube-node-lease`
  - 本番環境: `kube-system,kube-public,kube-node-lease`

### SKIP_LABELS
- **説明**: 指定されたラベルを持つリソースのバリデーションをスキップ
- **型**: 文字列
- **デフォルト値**: `k8s-deployment-hpa-validator.io/skip-validation=true`
- **環境変数**: `SKIP_LABELS`
- **ConfigMap キー**: `validation.skip-labels`

### FAILURE_POLICY
- **説明**: webhookが利用できない場合の動作
- **型**: 文字列
- **デフォルト値**: `Fail`
- **有効な値**: `Fail`, `Ignore`
- **推奨値**:
  - 開発環境: `Ignore` (開発の妨げにならないよう)
  - ステージング環境: `Fail`
  - 本番環境: `Fail`

## 監視設定

### METRICS_ENABLED
- **説明**: Prometheusメトリクスの有効/無効
- **型**: ブール値
- **デフォルト値**: `true`
- **環境変数**: `METRICS_ENABLED`
- **ConfigMap キー**: `metrics.enabled`
- **推奨値**: 全環境で `true`

### METRICS_PORT
- **説明**: メトリクスエンドポイントのポート番号
- **型**: 整数
- **デフォルト値**: `8080`
- **環境変数**: `METRICS_PORT`
- **ConfigMap キー**: `metrics.port`
- **推奨値**: 全環境で `8080`

### HEALTH_ENABLED
- **説明**: ヘルスチェックエンドポイントの有効/無効
- **型**: ブール値
- **デフォルト値**: `true`
- **環境変数**: `HEALTH_ENABLED`
- **ConfigMap キー**: `health.enabled`
- **推奨値**: 全環境で `true`

## 環境情報設定

### ENVIRONMENT
- **説明**: 実行環境の識別子
- **型**: 文字列
- **デフォルト値**: `development`
- **環境変数**: `ENVIRONMENT`
- **ConfigMap キー**: `environment`
- **有効な値**: `development`, `staging`, `production`

### CLUSTER_NAME
- **説明**: Kubernetesクラスターの名前
- **型**: 文字列
- **デフォルト値**: なし
- **環境変数**: `CLUSTER_NAME`
- **ConfigMap キー**: `cluster.name`
- **推奨値**:
  - 開発環境: `dev-cluster`
  - ステージング環境: `staging-cluster`
  - 本番環境: `prod-cluster-01`

## リソース設定

### CPU_REQUESTS
- **説明**: CPU要求量
- **推奨値**:
  - 開発環境: `50m`
  - ステージング環境: `100m`
  - 本番環境: `200m`

### CPU_LIMITS
- **説明**: CPU制限量
- **推奨値**:
  - 開発環境: `200m`
  - ステージング環境: `500m`
  - 本番環境: `1000m`

### MEMORY_REQUESTS
- **説明**: メモリ要求量
- **推奨値**:
  - 開発環境: `64Mi`
  - ステージング環境: `128Mi`
  - 本番環境: `256Mi`

### MEMORY_LIMITS
- **説明**: メモリ制限量
- **推奨値**:
  - 開発環境: `128Mi`
  - ステージング環境: `256Mi`
  - 本番環境: `512Mi`

### REPLICAS
- **説明**: レプリカ数
- **推奨値**:
  - 開発環境: `1`
  - ステージング環境: `2`
  - 本番環境: `3`

## 環境別設定例

### 開発環境設定例

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: webhook-system
data:
  # サーバー設定
  webhook.port: "8443"
  webhook.timeout: "30"
  
  # ログ設定
  log.level: "debug"
  log.format: "text"
  
  # バリデーション設定
  validation.skip-namespaces: "kube-system,kube-public,kube-node-lease,test-system"
  validation.failure-policy: "Ignore"
  
  # 監視設定
  metrics.enabled: "true"
  metrics.port: "8080"
  health.enabled: "true"
  
  # 環境情報
  environment: "development"
  cluster.name: "dev-cluster"
```

### ステージング環境設定例

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: webhook-system
data:
  # サーバー設定
  webhook.port: "8443"
  webhook.timeout: "15"
  
  # ログ設定
  log.level: "info"
  log.format: "json"
  
  # バリデーション設定
  validation.skip-namespaces: "kube-system,kube-public,kube-node-lease"
  validation.failure-policy: "Fail"
  
  # 監視設定
  metrics.enabled: "true"
  metrics.port: "8080"
  health.enabled: "true"
  
  # 環境情報
  environment: "staging"
  cluster.name: "staging-cluster"
```

### 本番環境設定例

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: webhook-system
data:
  # サーバー設定
  webhook.port: "8443"
  webhook.timeout: "10"
  
  # ログ設定
  log.level: "warn"
  log.format: "json"
  
  # バリデーション設定
  validation.skip-namespaces: "kube-system,kube-public,kube-node-lease"
  validation.failure-policy: "Fail"
  
  # 監視設定
  metrics.enabled: "true"
  metrics.port: "8080"
  health.enabled: "true"
  
  # 環境情報
  environment: "production"
  cluster.name: "prod-cluster-01"
```

## ベストプラクティス

### セキュリティ
1. **TLS証明書の管理**
   - cert-managerを使用した自動更新を推奨
   - 証明書の有効期限を定期的に監視

2. **RBAC設定**
   - 最小権限の原則を適用
   - 不要な権限は付与しない

3. **ネットワークセキュリティ**
   - NetworkPolicyを使用してトラフィックを制限
   - 必要最小限の通信のみ許可

### パフォーマンス
1. **リソース制限**
   - 適切なCPU/メモリ制限を設定
   - 環境に応じたリソース配分

2. **タイムアウト設定**
   - 適切なタイムアウト値を設定
   - 長すぎるとクラスターに影響、短すぎると失敗率が上がる

### 運用性
1. **ログ設定**
   - 本番環境では構造化ログ（JSON）を使用
   - 適切なログレベルを設定してノイズを削減

2. **監視設定**
   - メトリクスとヘルスチェックを有効化
   - アラートを適切に設定

3. **高可用性**
   - 本番環境では複数レプリカを配置
   - PodDisruptionBudgetを設定

## 設定変更時の注意事項

### 設定変更の手順
1. 設定変更前にバックアップを取得
2. ステージング環境で事前テスト
3. 段階的に本番環境に適用
4. 変更後の動作確認

### 影響の大きい設定変更
- **FAILURE_POLICY**: `Ignore`から`Fail`への変更は慎重に
- **SKIP_NAMESPACES**: 削除時は影響範囲を事前確認
- **TLS証明書**: 更新時はダウンタイムが発生する可能性

### ロールバック準備
- 変更前の設定値を記録
- 緊急時のロールバック手順を準備
- 監視とアラートで異常を早期検知

## 参考資料

- [Kubernetes Configuration Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/)
- [12-Factor App Configuration](https://12factor.net/config)
- [Prometheus Monitoring Best Practices](https://prometheus.io/docs/practices/naming/)