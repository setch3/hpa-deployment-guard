## 2. テスト用リソース

以下のリソースは、テスト目的でのみ使用され、本番環境では使用されません。

### 2.1 テストコード

| リソース | 説明 | パス |
|---------|------|------|
| 単体テスト | 各コンポーネントの単体テスト | `internal/*/`*_test.go |
| 統合テスト | コンポーネント間の統合テスト | `internal/cert/integration_test.go` |
| E2Eテスト | エンドツーエンドテスト | `test/e2e/e2e_test.go` |
| 本番環境設定テスト | 本番環境設定のテスト | `test/production/production_config_test.go` |
| 本番環境統合テスト | 本番環境での統合テスト | `test/production/integration_test.go` |
| パフォーマンステスト | 負荷・パフォーマンステスト | `test/performance/load_test.go` |
| セキュリティテスト | セキュリティ関連のテスト | `test/security/security_test.go` |
| ArgoCD統合テスト | ArgoCD連携のテスト | `test/argocd/argocd_integration_test.go` |

### 2.2 テストスクリプト

| リソース | 説明 | パス |
|---------|------|------|
| イメージビルド | コンテナイメージのビルド | `scripts/build-image.sh` |
| 環境クリーンアップ | テスト環境のクリーンアップ | `scripts/cleanup-test-environment.sh` |
| Webhookデプロイ | テスト用デプロイスクリプト | `scripts/deploy-webhook.sh` |
| 証明書生成 | TLS証明書の生成 | `scripts/generate-certs.sh` |
| E2Eテスト実行 | E2Eテストの実行 | `scripts/run-e2e-tests.sh` |
| テスト環境セットアップ | ローカルテスト環境の構築 | `scripts/setup-kind-cluster.sh` |
| テスト結果レポート | テスト結果の出力 | `scripts/test-reporter.sh` |
| ArgoCD統合テスト | ArgoCD統合テスト実行 | `scripts/test-argocd-integration.sh` |
| デプロイ検証 | デプロイ状態の検証 | `scripts/verify-deployment.sh` |
| RBAC検証 | RBAC設定の検証 | `scripts/verify-rbac.sh` |
| Webhook検証 | Webhook動作の検証 | `scripts/verify-webhook.sh` |

### 2.3 テスト用設定

| リソース | 説明 | パス |
|---------|------|------|
| KINDクラスター設定 | ローカルテスト用K8s設定 | `kind-config.yaml` |
| テストレポート | テスト結果の保存先 | `test-reports/` |
| E2Eテスト説明 | E2Eテストの説明 | `test/e2e/README.md` |

### 2.4 CI/CD設定

| リソース | 説明 | パス |
|---------|------|------|
| GitHub Actions | CI/CDワークフロー設定 | `.github/workflows/` |
| Makefile | ビルド・テスト自動化 | `Makefile` |

## 3. リソース区分けの基準

リソースを区分けする際の基準は以下の通りです：

### 3.1 本番用リソースの特徴

- **実行時に必要**: アプリケーションの実行に必要なコンポーネント
- **クラスターにデプロイ**: Kubernetesクラスターにデプロイされるマニフェスト
- **設定情報**: アプリケーションの動作を制御する設定ファイル
- **セキュリティ資格情報**: TLS証明書など、セキュリティに関わるファイル

### 3.2 テスト用リソースの特徴

- **検証目的**: 機能やパフォーマンスを検証するためのコード
- **開発サポート**: 開発やデバッグをサポートするスクリプト
- **CI/CD**: 継続的インテグレーション/デリバリーのための設定
- **ドキュメント**: 開発者向けの説明や手順書

## 4. 環境別の考慮事項

### 4.1 開発環境

開発環境では、テスト用リソースも含めて全てのリソースが使用されることがあります。特に：

- 単体テストと統合テストを頻繁に実行
- ローカルKINDクラスターでの検証
- デバッグ用のログレベル（debug）を使用

### 4.2 ステージング環境

ステージング環境では、本番環境に近い設定で動作確認を行います：

- 本番用リソースを使用
- E2Eテストを実行して機能を検証
- 本番環境に近いログレベル（info）を使用

### 4.3 本番環境

本番環境では、テスト用リソースは一切使用せず、本番用リソースのみを使用します：

- 最小限のログレベル（warn）を使用
- 厳格な失敗ポリシー（Fail）を設定
- メトリクスとヘルスチェックを有効化