# Makefile for k8s-deployment-hpa-validator

.PHONY: test test-unit test-integration test-e2e build clean setup-kind deploy-webhook

# Go設定
GO_VERSION := 1.21
BINARY_NAME := webhook
DOCKER_IMAGE := k8s-deployment-hpa-validator:latest

# テスト関連
test: test-unit test-integration
	@echo "全てのテストが完了しました"

test-unit:
	@echo "単体テストを実行中..."
	go test -v ./internal/...

test-integration:
	@echo "統合テストを実行中..."
	go test -v -tags=integration ./internal/cert

test-production:
	@echo "本番環境設定テストを実行中..."
	go test -v -tags=production ./test/production

test-production-integration:
	@echo "本番環境統合テストを実行中..."
	go test -v -tags=production,integration ./test/production

test-all-environments:
	@echo "全環境設定テストを実行中..."
	@echo "開発環境設定テスト..."
	ENVIRONMENT=development go test -v ./internal/config
	@echo "ステージング環境設定テスト..."
	ENVIRONMENT=staging go test -v ./internal/config
	@echo "本番環境設定テスト..."
	ENVIRONMENT=production go test -v ./internal/config
	@echo "本番環境専用テスト..."
	go test -v -tags=production ./test/production

test-performance:
	@echo "パフォーマンステストを実行中..."
	go test -v -tags=performance ./test/performance

test-load:
	@echo "負荷テストを実行中..."
	go test -v -tags=performance -run TestWebhookPerformance ./test/performance

test-stress:
	@echo "ストレステストを実行中..."
	go test -v -tags=performance -run TestWebhookStressTest ./test/performance

test-memory:
	@echo "メモリ使用量テストを実行中..."
	go test -v -tags=performance -run TestMemoryUsage ./test/performance

test-security:
	@echo "セキュリティテストを実行中..."
	go test -v -tags=security ./test/security

test-tls:
	@echo "TLS設定テストを実行中..."
	go test -v -tags=security -run TestTLSConfiguration ./test/security

test-rbac:
	@echo "RBAC設定テストを実行中..."
	go test -v -tags=security -run TestRBACConfiguration ./test/security

test-pod-security:
	@echo "Pod Securityテストを実行中..."
	go test -v -tags=security -run TestPodSecurityStandards ./test/security

test-network-security:
	@echo "ネットワークセキュリティテストを実行中..."
	go test -v -tags=security -run TestNetworkSecurity ./test/security

test-vulnerability:
	@echo "脆弱性スキャンテストを実行中..."
	go test -v -tags=security -run TestVulnerabilityScanning ./test/security

test-compliance:
	@echo "セキュリティコンプライアンステストを実行中..."
	go test -v -tags=security -run TestSecurityCompliance ./test/security

test-argocd:
	@echo "ArgoCD統合テストを実行中..."
	go test -v -tags=argocd ./test/argocd

test-argocd-integration:
	@echo "ArgoCD統合テスト（実環境）を実行中..."
	./scripts/test-argocd-integration.sh test

test-argocd-cleanup:
	@echo "ArgoCD統合テスト環境をクリーンアップ中..."
	./scripts/test-argocd-integration.sh cleanup

test-e2e:
	@echo "E2Eテストを実行中..."
	@echo "Webhookが動作していることを確認してください"
	go test -v -tags=e2e ./test/e2e

# ビルド関連
build:
	@echo "バイナリをビルド中..."
	go build -o $(BINARY_NAME) ./cmd/webhook

clean:
	@echo "クリーンアップ中..."
	rm -f $(BINARY_NAME)
	docker rmi $(DOCKER_IMAGE) 2>/dev/null || true

# 開発環境セットアップ
setup-kind:
	@echo "kind環境をセットアップ中..."
	./scripts/setup-kind-cluster.sh

deploy-webhook:
	@echo "Webhookをデプロイ中..."
	./scripts/deploy-webhook.sh

# 完全なE2Eテストフロー
e2e-full:
	@echo "完全なE2Eテストフローを実行中..."
	./scripts/run-e2e-tests.sh

# E2Eテストフロー（環境セットアップをスキップ）
e2e-quick:
	@echo "E2Eテストを実行中（環境セットアップをスキップ）..."
	./scripts/run-e2e-tests.sh --skip-setup

# 証明書生成
generate-certs:
	@echo "TLS証明書を生成中..."
	./scripts/generate-certs.sh

# 検証スクリプト
verify-deployment:
	@echo "デプロイメントを検証中..."
	./scripts/verify-deployment.sh

verify-webhook:
	@echo "Webhookを検証中..."
	./scripts/verify-webhook.sh

verify-rbac:
	@echo "RBAC設定を検証中..."
	./scripts/verify-rbac.sh

# クリーンアップ
cleanup-test:
	@echo "テスト環境をクリーンアップ中..."
	./scripts/cleanup-test-environment.sh

cleanup-full:
	@echo "完全なクリーンアップを実行中..."
	./scripts/cleanup-test-environment.sh --full

# ヘルプ
help:
	@echo "利用可能なターゲット:"
	@echo ""
	@echo "テスト関連:"
	@echo "  test          - 単体テストと統合テストを実行"
	@echo "  test-unit     - 単体テストのみ実行"
	@echo "  test-integration - 統合テストのみ実行"
	@echo "  test-production - 本番環境設定テストを実行"
	@echo "  test-production-integration - 本番環境統合テストを実行"
	@echo "  test-all-environments - 全環境設定テストを実行"
	@echo "  test-performance - パフォーマンステストを実行"
	@echo "  test-load     - 負荷テストを実行"
	@echo "  test-stress   - ストレステストを実行"
	@echo "  test-memory   - メモリ使用量テストを実行"
	@echo "  test-security - セキュリティテストを実行"
	@echo "  test-tls      - TLS設定テストを実行"
	@echo "  test-rbac     - RBAC設定テストを実行"
	@echo "  test-pod-security - Pod Securityテストを実行"
	@echo "  test-network-security - ネットワークセキュリティテストを実行"
	@echo "  test-vulnerability - 脆弱性スキャンテストを実行"
	@echo "  test-compliance - セキュリティコンプライアンステストを実行"
	@echo "  test-argocd   - ArgoCD統合テストを実行"
	@echo "  test-argocd-integration - ArgoCD統合テスト（実環境）を実行"
	@echo "  test-argocd-cleanup - ArgoCD統合テスト環境をクリーンアップ"
	@echo "  test-e2e      - E2Eテストのみ実行"
	@echo "  e2e-full      - 完全なE2Eテストフロー（環境セットアップ含む）"
	@echo "  e2e-quick     - E2Eテスト実行（環境セットアップをスキップ）"
	@echo ""
	@echo "ビルド関連:"
	@echo "  build         - バイナリをビルド"
	@echo "  clean         - ビルド成果物をクリーンアップ"
	@echo ""
	@echo "環境セットアップ:"
	@echo "  setup-kind    - kind環境をセットアップ"
	@echo "  deploy-webhook - Webhookをデプロイ"
	@echo "  generate-certs - TLS証明書を生成"
	@echo ""
	@echo "検証:"
	@echo "  verify-deployment - デプロイメントを検証"
	@echo "  verify-webhook - Webhookを検証"
	@echo "  verify-rbac   - RBAC設定を検証"
	@echo ""
	@echo "クリーンアップ:"
	@echo "  cleanup-test  - テスト環境をクリーンアップ"
	@echo "  cleanup-full  - 完全なクリーンアップ（kind環境も削除）"