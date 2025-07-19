# テスト手順書

HPA Deployment Validatorのテスト実行手順とテストケースの詳細説明書です。

## 目次

1. [テスト概要](#テスト概要)
2. [テスト環境の準備](#テスト環境の準備)
3. [単体テスト](#単体テスト)
4. [統合テスト](#統合テスト)
5. [E2Eテスト](#e2eテスト)
6. [テストケース詳細](#テストケース詳細)
7. [テスト結果の検証](#テスト結果の検証)
8. [トラブルシューティング](#トラブルシューティング)
9. [継続的インテグレーション](#継続的インテグレーション)

## テスト概要

### テストの目的

HPA Deployment Validatorが以下の要件を満たすことを検証します：

1. **1 replicaのDeploymentにHPAが設定されることを防ぐ**
2. **HPAが存在するDeploymentのreplicasを1に変更することを防ぐ**
3. **同時デプロイメント（ArgoCD等）での適切な処理**
4. **適切な日本語エラーメッセージの提供**

### テストレベル

| テストレベル | 目的 | 実行時間 | 自動化 |
|-------------|------|----------|--------|
| 単体テスト | 個別関数・メソッドの動作確認 | 数秒 | ✅ |
| 統合テスト | コンポーネント間の連携確認 | 数十秒 | ✅ |
| E2Eテスト | 実際のKubernetes環境での動作確認 | 数分 | ✅ |

## テスト環境の準備

### 前提条件

```bash
# 必要なツールのバージョン確認
go version          # Go 1.24.2以上
docker --version    # Docker 20.10以上
kubectl version     # kubectl 1.28以上
kind version        # kind 0.20以上
```

### 環境セットアップ

#### 1. 完全自動セットアップ

```bash
# 全自動でテスト環境を構築してE2Eテストを実行
make e2e-full
```

#### 2. 手動セットアップ

```bash
# ステップ1: kindクラスター作成
make setup-kind

# ステップ2: アプリケーションビルド
make build

# ステップ3: Dockerイメージ作成
./scripts/build-image.sh

# ステップ4: Webhookデプロイ
make deploy-webhook

# ステップ5: デプロイメント検証
make verify-deployment
```

### 環境確認

```bash
# クラスター状態確認
kubectl cluster-info
kubectl get nodes

# Webhook状態確認
kubectl get pods -l app=k8s-deployment-hpa-validator
kubectl get validatingwebhookconfigurations hpa-deployment-validator

# 接続テスト
./scripts/verify-webhook.sh
```

## 単体テスト

### 実行方法

```bash
# 全ての単体テストを実行
make test-unit

# または直接実行
go test -v ./internal/...

# 特定のパッケージのみ
go test -v ./internal/validator
go test -v ./internal/webhook
go test -v ./internal/cert
```

### カバレッジ測定

```bash
# カバレッジ付きでテスト実行
go test -v -coverprofile=coverage.out ./internal/...

# カバレッジレポート表示
go tool cover -html=coverage.out -o coverage.html
open coverage.html  # macOS
```

### 単体テストケース

#### バリデーターテスト (`internal/validator/validator_test.go`)

| テストケース | 説明 | 期待結果 |
|-------------|------|----------|
| `TestValidateDeployment_ValidCase` | 2+ replicaのDeployment | エラーなし |
| `TestValidateDeployment_InvalidCase` | 1 replicaのDeployment + HPA存在 | エラー発生 |
| `TestValidateHPA_ValidCase` | 2+ replicaのDeploymentを対象とするHPA | エラーなし |
| `TestValidateHPA_InvalidCase` | 1 replicaのDeploymentを対象とするHPA | エラー発生 |
| `TestValidateHPA_NonExistentTarget` | 存在しないDeploymentを対象とするHPA | エラーなし |

#### Webhookサーバーテスト (`internal/webhook/server_test.go`)

| テストケース | 説明 | 期待結果 |
|-------------|------|----------|
| `TestHandleAdmissionRequest_Deployment` | Deploymentリクエストの処理 | 適切なレスポンス |
| `TestHandleAdmissionRequest_HPA` | HPAリクエストの処理 | 適切なレスポンス |
| `TestHandleAdmissionRequest_InvalidJSON` | 不正なJSONリクエスト | エラーレスポンス |

#### 証明書管理テスト (`internal/cert/manager_test.go`)

| テストケース | 説明 | 期待結果 |
|-------------|------|----------|
| `TestLoadCertificates_Valid` | 有効な証明書の読み込み | 成功 |
| `TestLoadCertificates_Invalid` | 無効な証明書の読み込み | エラー |
| `TestValidateCertificate` | 証明書の検証 | 適切な検証結果 |

## 統合テスト

### 実行方法

```bash
# 統合テスト実行
make test-integration

# または直接実行
go test -v -tags=integration ./internal/cert
```

### 統合テストケース

#### 証明書統合テスト (`internal/cert/integration_test.go`)

| テストケース | 説明 | 期待結果 |
|-------------|------|----------|
| `TestCertificateGeneration` | 証明書生成の統合テスト | 有効な証明書生成 |
| `TestTLSConnection` | TLS接続テスト | 正常な接続確立 |

## E2Eテスト

### 実行方法

#### 1. 完全自動実行

```bash
# 環境セットアップからテスト実行まで全自動
make e2e-full

# または
./scripts/run-e2e-tests.sh
```

#### 2. 既存環境でのテスト実行

```bash
# 環境セットアップをスキップしてテスト実行
make e2e-quick

# または
./scripts/run-e2e-tests.sh --skip-setup
```

#### 3. 個別テスト実行

```bash
# Go E2Eテストを直接実行
go test -v -tags=e2e ./test/e2e

# 特定のテストケースのみ実行
go test -v -tags=e2e ./test/e2e -run TestValidateDeploymentWithHPA
```

### E2Eテストオプション

| オプション | 説明 |
|-----------|------|
| `--skip-setup` | 環境セットアップをスキップ |
| `--no-cleanup` | テスト後のクリーンアップをスキップ |
| `--full-cleanup` | テスト後にkind環境も削除 |

## テストケース詳細

### 1. 正常ケース（ValidDeploymentWithHPA）

#### テストケース1.1: 2 replicaのDeploymentとHPAの正常作成

**目的:** 2 replicaのDeploymentにHPAを設定できることを確認

**手順:**
1. 2 replicaのDeploymentを作成
2. 該当Deploymentを対象とするHPAを作成
3. 両方が正常に作成されることを確認

**期待結果:**
- Deploymentが正常に作成される
- HPAが正常に作成される
- Webhookによる拒否が発生しない

**検証コマンド:**
```bash
kubectl get deployment valid-deployment -n <test-namespace>
kubectl get hpa valid-hpa -n <test-namespace>
```

#### テストケース1.2: 3 replicaのDeploymentとHPAの正常作成

**目的:** 3 replicaのDeploymentにHPAを設定できることを確認

**手順:**
1. 3 replicaのDeploymentを作成
2. 該当Deploymentを対象とするHPAを作成
3. 両方が正常に作成されることを確認

**期待結果:**
- Deploymentが正常に作成される
- HPAが正常に作成される
- Webhookによる拒否が発生しない

### 2. 異常ケース（InvalidDeploymentWithHPA）

#### テストケース2.1: 1 replicaのDeploymentにHPAを追加

**目的:** 1 replicaのDeploymentにHPAを設定できないことを確認

**手順:**
1. 1 replicaのDeploymentを作成
2. 該当Deploymentを対象とするHPAを作成
3. HPA作成が拒否されることを確認

**期待結果:**
- Deploymentは正常に作成される
- HPA作成がWebhookによって拒否される
- 適切な日本語エラーメッセージが表示される

**期待エラーメッセージ:**
```
1 replicaのDeploymentを対象とするHPAは作成できません。Deploymentのreplicasを2以上に設定してください。
```

#### テストケース2.2: HPAが存在する状態で1 replicaのDeploymentを作成

**目的:** HPAが存在するDeploymentのreplicasを1に変更できないことを確認

**手順:**
1. 2 replicaのDeploymentを作成
2. 該当Deploymentを対象とするHPAを作成
3. Deploymentを削除
4. 同名で1 replicaのDeploymentを作成
5. Deployment作成が拒否されることを確認

**期待結果:**
- 初期Deploymentは正常に作成される
- HPAは正常に作成される
- 1 replicaのDeployment作成がWebhookによって拒否される

**期待エラーメッセージ:**
```
1 replicaのDeploymentにHPAが設定されています。HPAを削除するか、replicasを2以上に設定してください。
```

### 3. 同時デプロイメントシナリオ（SimultaneousDeployment）

#### テストケース3.1: 1 replicaのDeploymentとHPAの同時作成

**目的:** ArgoCDなどでの同時デプロイメント時の適切な処理を確認

**手順:**
1. 1 replicaのDeploymentとHPAを同時に作成（goroutineを使用）
2. 少なくとも一方が拒否されることを確認

**期待結果:**
- 少なくとも一方のリソース作成が拒否される
- 適切なエラーメッセージが表示される

#### テストケース3.2: 2 replicaのDeploymentとHPAの同時作成

**目的:** 正常な同時デプロイメントが成功することを確認

**手順:**
1. 2 replicaのDeploymentとHPAを同時に作成
2. 両方が正常に作成されることを確認

**期待結果:**
- 両方のリソースが正常に作成される
- Webhookによる拒否が発生しない

### 4. エッジケース（EdgeCases）

#### テストケース4.1: Deploymentの更新（2 replica → 1 replica）

**目的:** 既存DeploymentのreplicasをHPA存在下で1に変更できないことを確認

**手順:**
1. 2 replicaのDeploymentを作成
2. 該当Deploymentを対象とするHPAを作成
3. Deploymentのreplicasを1に更新
4. 更新が拒否されることを確認

**期待結果:**
- Deployment更新がWebhookによって拒否される
- 適切なエラーメッセージが表示される

#### テストケース4.2: 存在しないDeploymentを対象とするHPAの作成

**目的:** 存在しないDeploymentを対象とするHPAが作成できることを確認

**手順:**
1. 存在しないDeploymentを対象とするHPAを作成
2. HPA作成が成功することを確認

**期待結果:**
- HPAが正常に作成される（これは正常な動作）
- Webhookによる拒否が発生しない

## テスト結果の検証

### 自動検証

E2Eテスト実行後、自動的にテストレポートが生成されます：

```bash
# テストレポートの場所
ls -la test-reports/

# 最新のレポート確認
cat test-reports/e2e-test-report-*.md
open test-reports/e2e-test-report-*.html  # HTMLレポート
```

### 手動検証

#### 1. Webhook動作確認

```bash
# Webhook Podの状態確認
kubectl get pods -l app=k8s-deployment-hpa-validator
kubectl logs -l app=k8s-deployment-hpa-validator

# Webhook設定確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml
```

#### 2. テストリソースの確認

```bash
# テスト用namespaceの確認
kubectl get namespaces | grep hpa-validator-test

# テストリソースの確認
kubectl get deployments,hpa -A | grep test
```

#### 3. エラーメッセージの確認

正常なエラーメッセージの例：

```bash
# 1 replicaのDeploymentにHPAを追加しようとした場合
error validating data: ValidationError(HorizontalPodAutoscaler): 
1 replicaのDeploymentを対象とするHPAは作成できません。Deploymentのreplicasを2以上に設定してください。

# HPAが存在する状態で1 replicaのDeploymentを作成しようとした場合
error validating data: ValidationError(Deployment): 
1 replicaのDeploymentにHPAが設定されています。HPAを削除するか、replicasを2以上に設定してください。
```

### テスト成功の判定基準

| 項目 | 成功基準 |
|------|----------|
| 単体テスト | 全テストケースが成功 |
| 統合テスト | 全テストケースが成功 |
| E2Eテスト | 主要な4つのテストスイートが成功 |
| エラーメッセージ | 日本語で適切なメッセージが表示される |
| パフォーマンス | Webhook応答時間が1秒以内 |

## トラブルシューティング

### よくある問題と解決方法

#### 1. テスト環境の構築に失敗する

**症状:**
```bash
kind create cluster failed
```

**解決方法:**
```bash
# Dockerの状態確認
docker info

# 既存のkindクラスター削除
kind delete cluster --name hpa-validator-cluster

# 再度クラスター作成
./scripts/setup-kind-cluster.sh
```

#### 2. Webhook Podが起動しない

**症状:**
```bash
kubectl get pods -l app=k8s-deployment-hpa-validator
# STATUS: CrashLoopBackOff
```

**解決方法:**
```bash
# ログ確認
kubectl logs -l app=k8s-deployment-hpa-validator

# 証明書の確認
ls -la certs/
./scripts/generate-certs.sh

# 再デプロイ
./scripts/deploy-webhook.sh
```

#### 3. E2Eテストがタイムアウトする

**症状:**
```
context deadline exceeded
```

**解決方法:**
```bash
# Webhook準備状態の確認
./scripts/verify-webhook.sh

# システムリソースの確認
kubectl top nodes
kubectl top pods

# テストタイムアウトの延長
export TEST_TIMEOUT=60s
go test -v -tags=e2e ./test/e2e -timeout=5m
```

#### 4. テストが断続的に失敗する

**症状:**
- 同じテストが成功したり失敗したりする

**解決方法:**
```bash
# テスト環境の完全リセット
./scripts/cleanup-test-environment.sh --full

# 環境の再構築
make setup-kind
make deploy-webhook

# 安定化待機時間の追加
sleep 30
make test-e2e
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

# CA証明書の確認
openssl x509 -in certs/ca.crt -text -noout

# Webhook設定の更新
CA_BUNDLE=$(base64 < certs/ca.crt | tr -d '\n')
kubectl patch validatingwebhookconfiguration hpa-deployment-validator \
  --type='json' \
  -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': '${CA_BUNDLE}'}]"
```

### デバッグ用コマンド

```bash
# 詳細ログの有効化
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","env":[{"name":"LOG_LEVEL","value":"debug"}]}]}}}}'

# Webhook接続テスト
kubectl exec -it <webhook-pod> -- wget -qO- https://kubernetes.default.svc.cluster.local/api/v1/namespaces

# ネットワーク確認
kubectl exec -it <webhook-pod> -- netstat -tlnp

# DNS解決確認
kubectl exec -it <webhook-pod> -- nslookup kubernetes.default.svc.cluster.local
```

## 継続的インテグレーション

### GitHub Actions設定例

```yaml
# .github/workflows/test.yml
name: Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.24.2
    - name: Run unit tests
      run: make test-unit

  integration-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.24.2
    - name: Run integration tests
      run: make test-integration

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.24.2
    - name: Install kind
      run: |
        curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
        chmod +x ./kind
        sudo mv ./kind /usr/local/bin/kind
    - name: Run E2E tests
      run: make e2e-full
    - name: Upload test reports
      uses: actions/upload-artifact@v3
      if: always()
      with:
        name: test-reports
        path: test-reports/
```

### テスト結果の通知

```bash
# Slackへの通知例
curl -X POST -H 'Content-type: application/json' \
  --data '{"text":"E2Eテスト結果: 成功 ✅"}' \
  $SLACK_WEBHOOK_URL
```

### 定期実行

```bash
# crontabでの定期実行例
# 毎日午前2時にE2Eテストを実行
0 2 * * * cd /path/to/project && make e2e-full > /tmp/e2e-test.log 2>&1
```

## テストデータ管理

### テスト用マニフェスト

テスト用のKubernetesマニフェストは以下の場所に配置：

```
test/
├── fixtures/
│   ├── valid-deployment-2-replicas.yaml
│   ├── valid-deployment-3-replicas.yaml
│   ├── invalid-deployment-1-replica.yaml
│   ├── valid-hpa.yaml
│   └── invalid-hpa.yaml
└── e2e/
    └── e2e_test.go
```

### テストデータの更新

```bash
# テスト用マニフェストの検証
kubectl apply --dry-run=client -f test/fixtures/

# テストデータの自動生成
go run test/generate-fixtures.go
```

## パフォーマンステスト

### 負荷テスト

```bash
# 同時リクエスト数の測定
for i in {1..10}; do
  kubectl apply -f test/fixtures/valid-deployment-2-replicas.yaml &
done
wait

# レスポンス時間の測定
time kubectl apply --dry-run=server -f test/fixtures/valid-deployment-2-replicas.yaml
```

### メモリ・CPU使用量の監視

```bash
# Webhook Podのリソース使用量監視
kubectl top pods -l app=k8s-deployment-hpa-validator --containers

# 継続的な監視
watch kubectl top pods -l app=k8s-deployment-hpa-validator
```

## テスト環境の管理

### 環境の作成

```bash
# 開発環境
make setup-kind

# テスト環境（CI用）
./scripts/setup-kind-cluster.sh --ci-mode

# 本番類似環境
./scripts/setup-kind-cluster.sh --production-like
```

### 環境のクリーンアップ

```bash
# テストリソースのみクリーンアップ
./scripts/cleanup-test-environment.sh

# 完全クリーンアップ（kind環境も削除）
./scripts/cleanup-test-environment.sh --full

# 強制クリーンアップ
./scripts/cleanup-test-environment.sh --force
```

---

このテスト手順書に従って、HPA Deployment Validatorの品質を継続的に確保してください。質問や問題がある場合は、プロジェクトのIssueで報告してください。