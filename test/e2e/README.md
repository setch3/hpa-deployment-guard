# E2Eテスト

このディレクトリには、k8s-deployment-hpa-validatorのEnd-to-End（E2E）テストが含まれています。

## 概要

E2Eテストは、実際のKubernetes環境でWebhookの動作を検証します。以下のシナリオをテストします：

### テストケース

1. **正常ケース（2+ replica + HPA）**
   - 2 replicaのDeploymentとHPAの正常作成
   - 3 replicaのDeploymentとHPAの正常作成

2. **異常ケース（1 replica + HPA）**
   - 1 replicaのDeploymentにHPAを追加（拒否されることを確認）
   - HPAが存在する状態で1 replicaのDeploymentを作成（拒否されることを確認）

3. **同時デプロイメントシナリオ**
   - 1 replicaのDeploymentとHPAの同時作成（両方拒否されることを確認）
   - 2 replicaのDeploymentとHPAの同時作成（両方成功することを確認）

4. **エッジケース**
   - Deploymentの更新（2 replica → 1 replica）でHPAが存在する場合
   - 存在しないDeploymentを対象とするHPAの作成

## 前提条件

E2Eテストを実行する前に、以下が必要です：

1. **kind環境の準備**
   ```bash
   make setup-kind
   ```

2. **Webhookのデプロイ**
   ```bash
   make deploy-webhook
   ```

3. **Webhookの動作確認**
   ```bash
   make verify-webhook
   ```

## テスト実行方法

### 個別実行

```bash
# E2Eテストのみ実行
make test-e2e
```

### 完全なフロー実行

```bash
# 環境セットアップからテスト実行まで一括実行
make e2e-full

# 環境セットアップをスキップしてテスト実行
make e2e-quick
```

### 自動化スクリプト実行

```bash
# 完全な自動化フロー
./scripts/run-e2e-tests.sh

# 環境セットアップをスキップ
./scripts/run-e2e-tests.sh --skip-setup

# クリーンアップなしで実行
./scripts/run-e2e-tests.sh --no-cleanup

# 完全クリーンアップ付きで実行
./scripts/run-e2e-tests.sh --full-cleanup
```

### 手動実行

```bash
# Go testコマンドで直接実行
go test -v -tags=e2e ./test/e2e
```

## テスト環境

- **Namespace**: `hpa-validator-test`
- **タイムアウト**: 30秒
- **クリーンアップ**: 各テスト後に自動実行

## トラブルシューティング

### Webhookが準備できていない場合

```bash
# Webhook Podの状態確認
kubectl get pods -l app=k8s-deployment-hpa-validator

# Webhook設定の確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator

# ログの確認
kubectl logs -l app=k8s-deployment-hpa-validator
```

### テストが失敗する場合

1. **Webhook Podが実行中か確認**
   ```bash
   kubectl get pods -l app=k8s-deployment-hpa-validator
   ```

2. **証明書の有効性確認**
   ```bash
   make verify-webhook
   ```

3. **RBAC権限の確認**
   ```bash
   make verify-rbac
   ```

4. **テスト用namespaceのクリーンアップ**
   ```bash
   kubectl delete namespace hpa-validator-test --ignore-not-found
   ```

## テスト結果の解釈

- **成功**: 全てのテストケースがPASSし、期待される動作が確認される
- **失敗**: Webhookが期待通りに動作しない、または環境に問題がある

## テスト自動化機能

### テストレポート生成

E2Eテスト実行後、以下のレポートが自動生成されます：

- **Markdownレポート**: テスト結果の詳細サマリー
- **HTMLレポート**: ブラウザで閲覧可能な視覚的レポート
- **JUnit XMLレポート**: CI/CD統合用のXML形式レポート
- **システム状態ログ**: Kubernetes環境の状態記録
- **パフォーマンス情報**: リソース使用量とレスポンス時間

### 自動クリーンアップ

テスト終了後、以下が自動的にクリーンアップされます：

- テスト用namespace (`hpa-validator-test`)
- 一時的なテストリソース（Deployment、HPA等）
- 一時ファイル（ログ、設定ファイル等）
- 古いテストレポート（7日以上経過したもの）

### クリーンアップオプション

```bash
# テスト用namespaceのみクリーンアップ
./scripts/cleanup-test-environment.sh --namespace-only

# 完全クリーンアップ（kind環境も削除）
./scripts/cleanup-test-environment.sh --full

# 強制クリーンアップ（確認なし）
./scripts/cleanup-test-environment.sh --force

# クリーンアップ状況の確認のみ
./scripts/cleanup-test-environment.sh --verify-only
```

## 注意事項

- E2Eテストは実際のKubernetes APIを使用するため、適切な権限が必要です
- テスト実行中は`hpa-validator-test` namespaceが使用されます
- テスト終了後、作成されたリソースは自動的にクリーンアップされます
- テストレポートは`test-reports/`ディレクトリに保存されます
- 長時間実行されるテストの場合、タイムアウト設定を調整してください