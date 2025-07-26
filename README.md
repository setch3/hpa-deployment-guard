# HPA Deployment Validator

Kubernetes環境において、1 replicaのDeploymentにHorizontalPodAutoscaler（HPA）が紐づくことを防ぐValidatingAdmissionWebhookです。

## 概要

HPAは最低2つのレプリカが必要ですが、1 replicaのDeploymentにHPAが設定されると正常に動作しません。このWebhookは、そのような設定ミスを事前に防ぎます。

### 主な機能

- **Deploymentバリデーション**: 1 replicaのDeploymentにHPAが既に存在する場合、Deploymentの作成/更新を拒否
- **HPAバリデーション**: 1 replicaのDeploymentを対象とするHPAの作成/更新を拒否
- **同時デプロイ対応**: ArgoCDなどでDeploymentとHPAが同時にデプロイされる場合も適切に処理
- **日本語エラーメッセージ**: 分かりやすい日本語でエラー内容と解決策を提供

## 前提条件

### 必要なツール

- **Go 1.24.2以上**
- **Docker**
- **kubectl**
- **kind** (ローカル開発・テスト用)

### インストール確認

```bash
# Goバージョン確認
go version

# Dockerの動作確認
docker info

# kubectlの確認
kubectl version --client

# kindの確認
kind version
```

## クイックスタート

### 1. リポジトリのクローン

```bash
git clone <repository-url>
cd k8s-deployment-hpa-validator
```

### 2. kind環境でのテスト実行

```bash
# 完全なE2Eテストフロー（環境セットアップ含む）
make e2e-full

# 必要に応じて自動的にイメージをビルドしてE2Eテストを実行
make e2e-full-auto

# 環境セットアップをスキップしてE2Eテストのみ実行
make e2e-quick

# または手動でステップ実行
make setup-kind      # kind環境セットアップ
make deploy-webhook  # Webhookデプロイ
make test-e2e       # E2Eテスト実行
```

**新しいE2Eテストオプション:**
- `make e2e-full-auto`: イメージが存在しない場合に自動的にビルドしてからE2Eテストを実行（要件1.1対応）
- `make e2e-quick`: 環境セットアップをスキップしてテストのみ実行
- `make build-image-only`: テストをスキップしてイメージのみビルド
- `make build-image-force`: テスト失敗時でも強制的にイメージをビルド

### 3. 本番環境へのデプロイ

```bash
# 証明書生成
make generate-certs

# Kubernetesマニフェスト適用
kubectl apply -f manifests/
```

## 詳細な実装手順

### ステップ1: 開発環境の準備

#### 1.1 依存関係の確認

```bash
# Go modulesの初期化確認
go mod tidy
go mod verify

# 依存関係の表示
go list -m all
```

#### 1.2 プロジェクト構造の確認

```
├── cmd/webhook/           # メインアプリケーション
├── internal/             # プライベートパッケージ
│   ├── cert/            # TLS証明書管理
│   ├── validator/       # バリデーションロジック
│   └── webhook/         # Webhookサーバー
├── manifests/           # Kubernetesマニフェスト
├── scripts/             # 自動化スクリプト
└── test/               # テストコード
```

### ステップ2: ローカル開発環境のセットアップ

#### 2.1 kind環境の構築

```bash
# kindクラスター作成
./scripts/setup-kind-cluster.sh

# クラスター状態確認
kubectl cluster-info
kubectl get nodes
```

**トラブルシューティング:**
- kindクラスター作成に失敗する場合は、Dockerが起動していることを確認
- ポート競合が発生する場合は、`kind-config.yaml`でポート設定を変更

#### 2.2 TLS証明書の生成

```bash
# 証明書生成スクリプト実行
./scripts/generate-certs.sh

# 生成された証明書の確認
ls -la certs/
openssl x509 -in certs/tls.crt -text -noout
```

**生成される証明書:**
- `ca.crt`, `ca.key`: CA証明書とキー
- `tls.crt`, `tls.key`: Webhook用TLS証明書とキー

### ステップ3: アプリケーションのビルドとテスト

#### 3.1 単体テストの実行

```bash
# 全ての単体テストを実行
make test-unit

# 特定のパッケージのテスト
go test -v ./internal/validator
go test -v ./internal/webhook
go test -v ./internal/cert
```

#### 3.2 統合テストの実行

```bash
# 統合テスト実行（証明書管理）
make test-integration

# または直接実行
go test -v -tags=integration ./internal/cert
```

#### 3.3 アプリケーションのビルド

```bash
# バイナリビルド
make build

# 生成されたバイナリの確認
./webhook --help
```

### ステップ4: Dockerイメージの作成

#### 4.1 イメージビルド

```bash
# 通常のイメージビルド（テスト実行後にビルド）
./scripts/build-image.sh

# テストをスキップしてイメージのみビルド
./scripts/build-image.sh --skip-tests

# テスト失敗時でも強制的にイメージをビルド
./scripts/build-image.sh --force-build

# イメージ確認
docker images | grep hpa-validator
```

**新しいビルドオプション:**
- `--skip-tests`: テストの実行をスキップしてイメージのみをビルドします（要件1.3対応）
- `--force-build`: テストが失敗してもビルドを強制的に続行します（要件1.2対応）
- `--help`: 使用方法とオプションの詳細を表示します

**環境変数による制御:**
```bash
# 環境変数でオプションを制御
SKIP_TESTS=true ./scripts/build-image.sh
FORCE_BUILD=true ./scripts/build-image.sh
DEBUG=true ./scripts/build-image.sh  # 詳細なデバッグ情報を表示
```

#### 4.2 kindクラスターへのイメージロード

```bash
# kindクラスターにイメージをロード
kind load docker-image hpa-validator:latest --name hpa-validator-cluster
```

### ステップ5: Kubernetesへのデプロイ

#### 5.1 自動デプロイ

```bash
# 完全自動デプロイ
./scripts/deploy-webhook.sh
```

#### 5.2 手動デプロイ

```bash
# 証明書Secretの作成
kubectl create secret tls k8s-deployment-hpa-validator-certs \
  --cert=certs/tls.crt \
  --key=certs/tls.key

# マニフェスト適用
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/deployment.yaml
kubectl apply -f manifests/service.yaml
kubectl apply -f manifests/webhook.yaml

# CA証明書をWebhookに設定
CA_BUNDLE=$(base64 < certs/ca.crt | tr -d '\n')
kubectl patch validatingwebhookconfiguration hpa-deployment-validator \
  --type='json' \
  -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': '${CA_BUNDLE}'}]"
```

### ステップ6: デプロイメントの検証

#### 6.1 Pod状態の確認

```bash
# Pod状態確認
kubectl get pods -l app=k8s-deployment-hpa-validator

# Pod詳細確認
kubectl describe pods -l app=k8s-deployment-hpa-validator

# ログ確認
kubectl logs -l app=k8s-deployment-hpa-validator
```

#### 6.2 Webhook設定の確認

```bash
# ValidatingWebhookConfiguration確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml

# Service確認
kubectl get service k8s-deployment-hpa-validator
```

#### 6.3 自動検証スクリプト

```bash
# Webhook接続テスト
./scripts/verify-webhook.sh

# RBAC設定確認
./scripts/verify-rbac.sh

# デプロイメント全体確認
./scripts/verify-deployment.sh
```

### ステップ7: E2Eテストの実行

#### 7.1 完全なE2Eテスト

```bash
# 環境セットアップからテスト実行まで全自動
make e2e-full

# 必要に応じて自動的にイメージをビルドしてE2Eテストを実行
make e2e-full-auto

# または直接スクリプト実行
./scripts/run-e2e-tests.sh

# 自動イメージビルド付きでスクリプト実行
./scripts/run-e2e-tests.sh --auto-build
```

#### 7.2 既存環境でのテスト実行

```bash
# 環境セットアップをスキップしてテスト実行
make e2e-quick

# または
./scripts/run-e2e-tests.sh --skip-setup
```

#### 7.3 テストカテゴリの制御

```bash
# 特定のテストカテゴリをスキップ
./scripts/run-e2e-tests.sh --skip-category=monitoring

# 複数のカテゴリをスキップ
SKIP_CATEGORIES=monitoring,performance ./scripts/run-e2e-tests.sh

# 環境変数でテスト動作をカスタマイズ
AUTO_BUILD=true ./scripts/run-e2e-tests.sh
```

#### 7.4 個別テストケース

```bash
# Go E2Eテストを直接実行
go test -v -tags=e2e ./test/e2e

# 特定のテストケースのみ実行
go test -v -tags=e2e ./test/e2e -run TestValidateDeploymentWithHPA
```

#### 7.5 テスト環境の状態確認

```bash
# テスト環境の状態を確認
./scripts/check-test-environment.sh

# 詳細な状態情報を表示
./scripts/check-test-environment.sh --verbose

# 問題を自動修復
./scripts/check-test-environment.sh --fix
```

## 設定オプション

### 環境変数

#### Webhook実行時の環境変数

| 変数名 | デフォルト値 | 説明 |
|--------|-------------|------|
| `WEBHOOK_PORT` | `8443` | Webhookサーバーのポート |
| `CERT_FILE` | `/etc/certs/tls.crt` | TLS証明書ファイルパス |
| `KEY_FILE` | `/etc/certs/tls.key` | TLS秘密鍵ファイルパス |
| `LOG_LEVEL` | `info` | ログレベル (debug, info, warn, error) |

#### ビルド・テスト時の環境変数

| 変数名 | デフォルト値 | 説明 |
|--------|-------------|------|
| `SKIP_TESTS` | `false` | テストをスキップしてイメージのみビルド |
| `FORCE_BUILD` | `false` | テスト失敗時でもビルドを強制続行 |
| `AUTO_BUILD` | `false` | イメージが存在しない場合に自動ビルド |
| `SKIP_CATEGORIES` | - | スキップするテストカテゴリ（カンマ区切り） |
| `TEST_TIMEOUT` | `30s` | テストのタイムアウト時間 |
| `DEBUG` | `false` | 詳細なデバッグ情報を表示 |

### Webhookの設定

`manifests/webhook.yaml`で以下の設定が可能です:

```yaml
# 対象リソースの設定
rules:
- operations: ["CREATE", "UPDATE"]
  apiGroups: ["apps"]
  apiVersions: ["v1"]
  resources: ["deployments"]
- operations: ["CREATE", "UPDATE"]
  apiGroups: ["autoscaling"]
  apiVersions: ["v2"]
  resources: ["horizontalpodautoscalers"]

# 失敗ポリシー
failurePolicy: Fail  # Fail または Ignore

# タイムアウト設定
timeoutSeconds: 10
```

## トラブルシューティング

### よくある問題と解決方法

#### 1. Dockerイメージのビルドエラー

**症状:**
```bash
./scripts/build-image.sh
# テストが失敗してビルドが中止される
```

**解決方法:**
```bash
# テストをスキップしてイメージのみビルド
./scripts/build-image.sh --skip-tests

# テスト失敗時でも強制的にビルド
./scripts/build-image.sh --force-build

# 詳細なエラー情報を表示
DEBUG=true ./scripts/build-image.sh

# 証明書エラーの場合
./scripts/generate-certs.sh
./scripts/build-image.sh --force-build
```

#### 2. E2Eテスト実行時のイメージ不足エラー

**症状:**
```bash
make e2e-full
# Error: image "hpa-validator:latest" not found
```

**解決方法:**
```bash
# 自動イメージビルド付きでE2Eテスト実行
make e2e-full-auto

# または手動でイメージをビルド
make build-image-only
make e2e-full

# スクリプトで自動ビルド
./scripts/run-e2e-tests.sh --auto-build
```

#### 3. テスト環境の状態確認

**症状:**
- テストが予期しない動作をする
- 環境の状態が不明

**解決方法:**
```bash
# テスト環境の状態を確認
./scripts/check-test-environment.sh

# 詳細な状態情報を表示
./scripts/check-test-environment.sh --verbose

# 問題を自動修復
./scripts/check-test-environment.sh --fix
```

#### 4. Webhook Podが起動しない

**症状:**
```bash
kubectl get pods -l app=k8s-deployment-hpa-validator
# STATUS: CrashLoopBackOff または ImagePullBackOff
```

**解決方法:**
```bash
# ログ確認
kubectl logs -l app=k8s-deployment-hpa-validator

# イメージの確認
kubectl describe pods -l app=k8s-deployment-hpa-validator

# kindクラスターにイメージが正しくロードされているか確認
docker exec -it hpa-validator-cluster-control-plane crictl images | grep hpa-validator

# イメージが見つからない場合
make build-image-only
kind load docker-image hpa-validator:latest --name hpa-validator-cluster
```

#### 5. 証明書エラー

**症状:**
```
x509: certificate signed by unknown authority
```

**解決方法:**
```bash
# 証明書の再生成
./scripts/generate-certs.sh

# CA証明書の再設定
CA_BUNDLE=$(base64 < certs/ca.crt | tr -d '\n')
kubectl patch validatingwebhookconfiguration hpa-deployment-validator \
  --type='json' \
  -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': '${CA_BUNDLE}'}]"
```

#### 6. Webhookが呼び出されない

**症状:**
- DeploymentやHPAが作成されるがWebhookが実行されない

**解決方法:**
```bash
# ValidatingWebhookConfigurationの確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml

# Serviceの確認
kubectl get service k8s-deployment-hpa-validator

# ネットワーク接続テスト
kubectl exec -it <webhook-pod> -- wget -qO- https://kubernetes.default.svc.cluster.local/api/v1/namespaces
```

#### 7. RBAC権限エラー

**症状:**
```
forbidden: User "system:serviceaccount:default:k8s-deployment-hpa-validator" cannot get resource "deployments"
```

**解決方法:**
```bash
# RBAC設定の確認
./scripts/verify-rbac.sh

# 権限の再適用
kubectl apply -f manifests/rbac.yaml
```

#### 8. E2Eテストの失敗

**症状:**
- テストが途中で失敗する
- タイムアウトエラー

**解決方法:**
```bash
# 環境の完全リセット
./scripts/cleanup-test-environment.sh --full

# 再度環境構築
make setup-kind
make deploy-webhook

# テスト実行
make test-e2e

# 特定のテストカテゴリをスキップ
SKIP_CATEGORIES=monitoring make e2e-full
```

### デバッグ用コマンド

```bash
# システム全体の状態確認
kubectl get all -A

# イベント確認
kubectl get events --sort-by='.lastTimestamp'

# Webhook詳細ログ
kubectl logs -l app=k8s-deployment-hpa-validator -f

# ネットワーク接続確認
kubectl exec -it <webhook-pod> -- netstat -tlnp

# 証明書の詳細確認
openssl x509 -in certs/tls.crt -text -noout

# テスト環境の詳細状態確認
./scripts/check-test-environment.sh --verbose

# ビルドプロセスのデバッグ
DEBUG=true ./scripts/build-image.sh

# E2Eテストのデバッグ情報
DEBUG=true ./scripts/run-e2e-tests.sh
```

### ログレベルの変更

開発時により詳細なログが必要な場合:

```bash
# Deployment環境変数を更新
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","env":[{"name":"LOG_LEVEL","value":"debug"}]}]}}}}'

# Pod再起動
kubectl rollout restart deployment k8s-deployment-hpa-validator
```

## 本番環境での運用

### 監視とアラート

#### メトリクス

Webhookは以下のメトリクスを提供します:

- `webhook_requests_total`: リクエスト総数
- `webhook_request_duration_seconds`: リクエスト処理時間
- `webhook_validation_errors_total`: バリデーションエラー数

#### ヘルスチェック

```bash
# Liveness probe
curl -k https://<webhook-service>:8443/healthz

# Readiness probe
curl -k https://<webhook-service>:8443/readyz
```

### セキュリティ考慮事項

1. **TLS証明書の定期更新**
2. **RBAC権限の最小化**
3. **ネットワークポリシーの適用**
4. **Pod Security Standardsの適用**

### パフォーマンス最適化

1. **リソース制限の適切な設定**
2. **レプリカ数の調整**
3. **キャッシュの活用**

## 開発者向け情報

### コードの構造

- `cmd/webhook/main.go`: エントリーポイント
- `internal/validator/`: バリデーションロジック
- `internal/webhook/`: HTTPサーバー実装
- `internal/cert/`: 証明書管理

### テストの追加

新しいテストケースを追加する場合:

```go
// internal/validator/validator_test.go
func TestNewValidationCase(t *testing.T) {
    // テストロジック
}
```

### ビルドとリリース

```bash
# バージョンタグ付きビルド
docker build -t hpa-validator:v1.0.0 .

# マルチアーキテクチャビルド
docker buildx build --platform linux/amd64,linux/arm64 -t hpa-validator:v1.0.0 .
```

## ライセンス

このプロジェクトはMITライセンスの下で公開されています。
