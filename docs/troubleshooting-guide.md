# トラブルシューティングガイド

このガイドでは、HPA Deployment Validatorの開発・運用時によく発生する問題と、その解決方法について説明します。

## 目次

1. [ビルド関連の問題](#ビルド関連の問題)
2. [テスト実行の問題](#テスト実行の問題)
3. [E2Eテスト環境の問題](#e2eテスト環境の問題)
4. [Webhook実行時の問題](#webhook実行時の問題)
5. [証明書関連の問題](#証明書関連の問題)
6. [ネットワーク・接続の問題](#ネットワーク接続の問題)
7. [権限・RBAC関連の問題](#権限rbac関連の問題)
8. [パフォーマンス・リソースの問題](#パフォーマンスリソースの問題)
9. [エラーコード一覧](#エラーコード一覧)

---

## ビルド関連の問題

### B001: Dockerイメージのビルドが失敗する

**症状:**
```bash
./scripts/build-image.sh
ERROR: Docker build failed
```

**原因:**
- Dockerデーモンが起動していない
- Dockerfileに構文エラーがある
- ビルドコンテキストが大きすぎる
- ディスク容量不足

**解決方法:**
```bash
# 1. Dockerデーモンの確認
sudo systemctl status docker
sudo systemctl start docker

# 2. Dockerfileの構文確認
docker build --dry-run -t test .

# 3. ディスク容量の確認
docker system df
docker system prune -a

# 4. 強制ビルド（テスト失敗を無視）
./scripts/build-image.sh --force-build
```

### B002: テスト失敗によりビルドが中止される

**症状:**
```bash
./scripts/build-image.sh
FAIL: TestCertManager
Build aborted due to test failure
```

**原因:**
- 証明書ファイルが存在しない
- テスト環境の設定が不完全
- 依存関係の問題

**解決方法:**
```bash
# 1. 証明書の生成
./scripts/generate-certs.sh

# 2. テストをスキップしてビルド
./scripts/build-image.sh --skip-tests

# 3. 強制ビルド
./scripts/build-image.sh --force-build

# 4. 特定のテストカテゴリのみ実行
TEST_CATEGORIES=unit ./scripts/build-image.sh
```

### B003: プロジェクト構造の問題

**症状:**
```bash
./scripts/build-image.sh
/Users/xxxxx/hpa-deployment-guard/cmd/webhook: directory not found
```

**原因:**
- プロジェクトディレクトリ名とgo.modのモジュール名が一致しない
- 古いプロジェクト名のディレクトリで実行している
- cmd/webhookディレクトリが存在しない

**解決方法:**
```bash
# 1. 現在のディレクトリとモジュール名の確認
pwd
grep "^module" go.mod

# 2. 正しいプロジェクトディレクトリで実行しているか確認
ls -la cmd/webhook/main.go

# 3. プロジェクトディレクトリ名を修正
cd ..
mv hpa-deployment-guard k8s-deployment-hpa-validator
cd k8s-deployment-hpa-validator

# 4. ディレクトリ構造の確認
ls -la cmd/
ls -la cmd/webhook/
```

### B004: Gitリポジトリからのファイル欠落

**症状:**
```bash
ls -la cmd/webhook/main.go
ls: cmd/webhook/main.go: No such file or directory
```

**原因:**
- Gitクローン時にファイルが正しく取得されなかった
- 部分的なチェックアウトが行われた
- ファイルが誤って削除された
- .gitignoreの設定ミス

**解決方法:**
```bash
# 1. Gitリポジトリの状態確認
git status
git ls-files | grep cmd/

# 2. 欠落ファイルの復元
git checkout HEAD -- cmd/
git checkout HEAD -- cmd/webhook/main.go

# 3. 完全なリセット（注意：ローカル変更が失われます）
git reset --hard HEAD
git clean -fd

# 4. 最新の変更を取得
git pull origin main

# 5. リポジトリの再クローン（最終手段）
cd ..
rm -rf k8s-deployment-hpa-validator
git clone <repository-url> k8s-deployment-hpa-validator
cd k8s-deployment-hpa-validator
```

### B005: Go依存関係の問題

**症状:**
```bash
go build ./cmd/webhook
cannot find package "k8s.io/client-go"
```

**原因:**
- go.modファイルが破損している
- 依存関係が古い
- ネットワーク接続の問題

**解決方法:**
```bash
# 1. 依存関係の更新
go mod tidy
go mod verify

# 2. モジュールキャッシュのクリア
go clean -modcache

# 3. プロキシ設定の確認
go env GOPROXY
export GOPROXY=direct

# 4. 依存関係の再取得
go mod download
```

---

## テスト実行の問題

### T001: 単体テストが失敗する

**症状:**
```bash
go test ./internal/...
FAIL: TestValidateDeployment
```

**原因:**
- テストデータの不整合
- モックの設定ミス
- 環境依存の問題

**解決方法:**
```bash
# 1. テストキャッシュのクリア
go clean -testcache

# 2. 詳細なテスト出力
go test -v ./internal/...

# 3. 特定のテストのみ実行
go test -v ./internal/validator -run TestValidateDeployment

# 4. レースコンディションの確認
go test -race ./internal/...
```

### T002: 統合テストが失敗する

**症状:**
```bash
go test -tags=integration ./internal/cert
FAIL: TestCertManagerIntegration
```

**原因:**
- 証明書ファイルが存在しない
- ファイル権限の問題
- テスト環境の不備

**解決方法:**
```bash
# 1. 証明書の生成と権限設定
./scripts/generate-certs.sh
chmod 600 certs/tls.key
chmod 644 certs/tls.crt

# 2. テスト環境の確認
ls -la certs/
openssl x509 -in certs/tls.crt -text -noout

# 3. 統合テストのスキップ
go test ./internal/... -short
```

### T003: テストがタイムアウトする

**症状:**
```bash
go test ./internal/...
panic: test timed out after 10m0s
```

**原因:**
- 無限ループやデッドロック
- 外部サービスへの接続待機
- システムリソース不足

**解決方法:**
```bash
# 1. タイムアウト時間の延長
go test -timeout=30m ./internal/...

# 2. 並行実行数の制限
go test -p=1 ./internal/...

# 3. 詳細なプロファイリング
go test -cpuprofile=cpu.prof ./internal/...
go tool pprof cpu.prof
```

---

## E2Eテスト環境の問題

### E001: kindクラスターの作成に失敗する

**症状:**
```bash
make setup-kind
ERROR: failed to create cluster
```

**原因:**
- Dockerが起動していない
- ポート競合
- リソース不足
- 既存クラスターの残存

**解決方法:**
```bash
# 1. 既存クラスターの削除
kind delete cluster --name hpa-validator

# 2. Dockerの確認
docker info
sudo systemctl restart docker

# 3. ポート使用状況の確認
lsof -i :80 -i :443 -i :6443

# 4. システムリソースの確認
free -h
df -h

# 5. kind設定の確認
cat kind-config.yaml
```

### E002: イメージがkindクラスターで見つからない

**症状:**
```bash
make e2e-full
Error: image "hpa-validator:latest" not found
```

**原因:**
- イメージがビルドされていない
- kindクラスターにイメージがロードされていない
- イメージ名の不一致

**解決方法:**
```bash
# 1. イメージの存在確認
docker images | grep hpa-validator

# 2. イメージのビルド
make build-image-only

# 3. kindクラスターへのロード
kind load docker-image hpa-validator:latest --name hpa-validator-cluster

# 4. 自動ビルド付きテスト実行
make e2e-full-auto
```

### E003: Webhookのデプロイに失敗する

**症状:**
```bash
make deploy-webhook
Error: failed to create webhook
```

**原因:**
- 証明書の問題
- RBAC権限の不足
- リソース競合

**解決方法:**
```bash
# 1. 証明書の再生成
./scripts/generate-certs.sh

# 2. 既存リソースの削除
kubectl delete validatingwebhookconfiguration hpa-deployment-validator
kubectl delete deployment k8s-deployment-hpa-validator

# 3. RBAC権限の確認
./scripts/verify-rbac.sh

# 4. デプロイメントの再実行
./scripts/deploy-webhook.sh
```

### E004: テスト環境の状態が不明

**症状:**
- テストが予期しない結果を返す
- 環境の状態が把握できない

**解決方法:**
```bash
# 1. 環境状態の確認
./scripts/check-test-environment.sh

# 2. 詳細な状態情報
./scripts/check-test-environment.sh --verbose

# 3. 自動修復の実行
./scripts/check-test-environment.sh --fix

# 4. 完全な環境リセット
./scripts/cleanup-test-environment.sh --full
make setup-kind
make deploy-webhook
```

---

## Webhook実行時の問題

### W001: Webhook Podが起動しない

**症状:**
```bash
kubectl get pods -l app=k8s-deployment-hpa-validator
STATUS: CrashLoopBackOff
```

**原因:**
- イメージの問題
- 証明書の問題
- 設定ファイルの問題
- リソース制限

**解決方法:**
```bash
# 1. Pod詳細の確認
kubectl describe pods -l app=k8s-deployment-hpa-validator

# 2. ログの確認
kubectl logs -l app=k8s-deployment-hpa-validator

# 3. イメージの確認
kubectl get pods -l app=k8s-deployment-hpa-validator -o jsonpath='{.items[0].spec.containers[0].image}'

# 4. 証明書の確認
kubectl get secret k8s-deployment-hpa-validator-certs -o yaml

# 5. リソース制限の確認
kubectl describe deployment k8s-deployment-hpa-validator
```

### W002: Webhookが呼び出されない

**症状:**
- DeploymentやHPAが作成されるがWebhookが実行されない
- バリデーションが動作しない

**原因:**
- ValidatingWebhookConfigurationの設定ミス
- Serviceの問題
- ネットワークポリシーの制限

**解決方法:**
```bash
# 1. ValidatingWebhookConfigurationの確認
kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml

# 2. Serviceの確認
kubectl get service k8s-deployment-hpa-validator
kubectl describe service k8s-deployment-hpa-validator

# 3. エンドポイントの確認
kubectl get endpoints k8s-deployment-hpa-validator

# 4. ネットワーク接続テスト
kubectl exec -it <webhook-pod> -- wget -qO- https://kubernetes.default.svc.cluster.local/api/v1/namespaces

# 5. Webhook設定の再適用
kubectl apply -f manifests/webhook.yaml
```

### W003: Webhookのレスポンスが遅い

**症状:**
- リクエスト処理に時間がかかる
- タイムアウトエラーが発生する

**原因:**
- リソース不足
- 外部API呼び出しの遅延
- ログ出力の過多

**解決方法:**
```bash
# 1. リソース使用量の確認
kubectl top pods -l app=k8s-deployment-hpa-validator

# 2. ログレベルの調整
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","env":[{"name":"LOG_LEVEL","value":"warn"}]}]}}}}'

# 3. リソース制限の調整
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","resources":{"limits":{"cpu":"500m","memory":"256Mi"}}}]}}}}'

# 4. レプリカ数の増加
kubectl scale deployment k8s-deployment-hpa-validator --replicas=2
```

---

## 証明書関連の問題

### C001: 証明書の生成に失敗する

**症状:**
```bash
./scripts/generate-certs.sh
ERROR: Failed to generate certificates
```

**原因:**
- OpenSSLが利用できない
- ディスク容量不足
- 権限の問題

**解決方法:**
```bash
# 1. OpenSSLの確認
openssl version

# 2. ディスク容量の確認
df -h .

# 3. 権限の確認
ls -la certs/
chmod 755 certs/

# 4. 手動での証明書生成
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/tls.key -out certs/tls.crt -days 365 -nodes -subj "/CN=k8s-deployment-hpa-validator.default.svc"
```

### C002: 証明書の検証エラー

**症状:**
```bash
kubectl logs -l app=k8s-deployment-hpa-validator
x509: certificate signed by unknown authority
```

**原因:**
- CA証明書の設定ミス
- 証明書の期限切れ
- 証明書チェーンの問題

**解決方法:**
```bash
# 1. 証明書の詳細確認
openssl x509 -in certs/tls.crt -text -noout

# 2. 証明書の有効期限確認
openssl x509 -in certs/tls.crt -noout -dates

# 3. CA証明書の再設定
CA_BUNDLE=$(base64 < certs/ca.crt | tr -d '\n')
kubectl patch validatingwebhookconfiguration hpa-deployment-validator \
  --type='json' \
  -p="[{'op': 'replace', 'path': '/webhooks/0/clientConfig/caBundle', 'value': '${CA_BUNDLE}'}]"

# 4. 証明書の再生成
./scripts/generate-certs.sh
kubectl delete secret k8s-deployment-hpa-validator-certs
kubectl create secret tls k8s-deployment-hpa-validator-certs \
  --cert=certs/tls.crt --key=certs/tls.key
```

### C003: 証明書の権限エラー

**症状:**
```bash
kubectl logs -l app=k8s-deployment-hpa-validator
permission denied: certs/tls.key
```

**原因:**
- ファイル権限の設定ミス
- 所有者の問題

**解決方法:**
```bash
# 1. ファイル権限の確認
ls -la certs/

# 2. 権限の修正
chmod 600 certs/tls.key
chmod 644 certs/tls.crt

# 3. 所有者の確認
chown $(whoami):$(whoami) certs/*

# 4. Secretの再作成
kubectl delete secret k8s-deployment-hpa-validator-certs
kubectl create secret tls k8s-deployment-hpa-validator-certs \
  --cert=certs/tls.crt --key=certs/tls.key
```

---

## ネットワーク・接続の問題

### N001: ネットワーク接続エラー

**症状:**
```bash
go test ./internal/...
dial tcp: connection refused
```

**原因:**
- ファイアウォールの制限
- ネットワーク設定の問題
- サービスが起動していない

**解決方法:**
```bash
# 1. ネットワーク接続の確認
ping -c 3 8.8.8.8
curl -v https://golang.org

# 2. ポート使用状況の確認
lsof -i :8443
netstat -tlnp | grep 8443

# 3. ファイアウォール設定の確認
sudo ufw status
sudo iptables -L

# 4. プロキシ設定の確認
echo $HTTP_PROXY
echo $HTTPS_PROXY
```

### N002: DNS解決の問題

**症状:**
```bash
kubectl exec -it <webhook-pod> -- nslookup kubernetes.default.svc.cluster.local
NXDOMAIN
```

**原因:**
- DNS設定の問題
- CoreDNSの不具合
- ネットワークポリシーの制限

**解決方法:**
```bash
# 1. CoreDNSの状態確認
kubectl get pods -n kube-system -l k8s-app=kube-dns

# 2. DNS設定の確認
kubectl get configmap coredns -n kube-system -o yaml

# 3. ネットワークポリシーの確認
kubectl get networkpolicies -A

# 4. DNS解決テスト
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default.svc.cluster.local
```

### N003: TLS接続の問題

**症状:**
```bash
curl -k https://k8s-deployment-hpa-validator.default.svc:8443/healthz
curl: (35) SSL connect error
```

**原因:**
- TLS設定の問題
- 証明書の不一致
- 暗号化スイートの問題

**解決方法:**
```bash
# 1. TLS接続の詳細確認
openssl s_client -connect k8s-deployment-hpa-validator.default.svc:8443

# 2. 証明書の確認
kubectl exec -it <webhook-pod> -- openssl x509 -in /etc/certs/tls.crt -text -noout

# 3. TLS設定の確認
kubectl logs -l app=k8s-deployment-hpa-validator | grep -i tls

# 4. 暗号化スイートの確認
openssl ciphers -v 'HIGH:!aNULL:!MD5'
```

---

## 権限・RBAC関連の問題

### R001: RBAC権限エラー

**症状:**
```bash
kubectl logs -l app=k8s-deployment-hpa-validator
forbidden: User "system:serviceaccount:default:k8s-deployment-hpa-validator" cannot get resource "deployments"
```

**原因:**
- ServiceAccountの権限不足
- RoleBindingの設定ミス
- ClusterRoleの不備

**解決方法:**
```bash
# 1. RBAC設定の確認
./scripts/verify-rbac.sh

# 2. ServiceAccountの確認
kubectl get serviceaccount k8s-deployment-hpa-validator

# 3. RoleBindingの確認
kubectl get rolebinding,clusterrolebinding | grep hpa-validator

# 4. 権限の再適用
kubectl apply -f manifests/rbac.yaml

# 5. 権限の詳細確認
kubectl auth can-i get deployments --as=system:serviceaccount:default:k8s-deployment-hpa-validator
```

### R002: ファイル権限の問題

**症状:**
```bash
./scripts/build-image.sh
permission denied: ./scripts/build-image.sh
```

**原因:**
- スクリプトファイルの実行権限がない
- ディレクトリの権限問題

**解決方法:**
```bash
# 1. ファイル権限の確認
ls -la scripts/

# 2. 実行権限の付与
chmod +x scripts/*.sh

# 3. ディレクトリ権限の確認
ls -la .

# 4. 所有者の確認
chown -R $(whoami):$(whoami) .
```

### R003: Docker権限の問題

**症状:**
```bash
docker info
permission denied while trying to connect to the Docker daemon socket
```

**原因:**
- Dockerグループに所属していない
- Dockerデーモンの権限設定

**解決方法:**
```bash
# 1. 現在のグループ確認
groups

# 2. Dockerグループへの追加
sudo usermod -aG docker $USER

# 3. セッションの再開
newgrp docker

# 4. Docker権限の確認
docker info
```

---

## パフォーマンス・リソースの問題

### P001: メモリ不足エラー

**症状:**
```bash
kubectl get pods -l app=k8s-deployment-hpa-validator
STATUS: OOMKilled
```

**原因:**
- メモリ制限の設定が低すぎる
- メモリリークの発生
- 大量のリクエスト処理

**解決方法:**
```bash
# 1. メモリ使用量の確認
kubectl top pods -l app=k8s-deployment-hpa-validator

# 2. メモリ制限の調整
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","resources":{"limits":{"memory":"512Mi"}}}]}}}}'

# 3. メモリプロファイリング
kubectl exec -it <webhook-pod> -- curl http://localhost:8080/debug/pprof/heap

# 4. ログレベルの調整
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","env":[{"name":"LOG_LEVEL","value":"error"}]}]}}}}'
```

### P002: CPU使用率が高い

**症状:**
```bash
kubectl top pods -l app=k8s-deployment-hpa-validator
CPU: 950m/1000m
```

**原因:**
- 非効率なアルゴリズム
- 無限ループ
- 大量のリクエスト処理

**解決方法:**
```bash
# 1. CPU使用量の詳細確認
kubectl top pods -l app=k8s-deployment-hpa-validator --containers

# 2. CPUプロファイリング
kubectl exec -it <webhook-pod> -- curl http://localhost:8080/debug/pprof/profile

# 3. CPU制限の調整
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","resources":{"limits":{"cpu":"2000m"}}}]}}}}'

# 4. レプリカ数の増加
kubectl scale deployment k8s-deployment-hpa-validator --replicas=3
```

### P003: ディスク容量不足

**症状:**
```bash
docker build -t hpa-validator .
no space left on device
```

**原因:**
- ディスク容量の不足
- 不要なDockerイメージの蓄積
- ログファイルの肥大化

**解決方法:**
```bash
# 1. ディスク使用量の確認
df -h
docker system df

# 2. 不要なDockerリソースの削除
docker system prune -a
docker volume prune

# 3. ログファイルの削除
sudo journalctl --vacuum-time=7d

# 4. 一時ファイルの削除
sudo rm -rf /tmp/*
```

---

## エラーコード一覧

### ビルド関連エラーコード

| コード | 説明 | 解決方法 |
|--------|------|----------|
| E001 | Dockerデーモンが起動していない | `sudo systemctl start docker` |
| E002 | プロジェクトファイルが見つからない | プロジェクトルートで実行 |
| E003 | テスト失敗によるビルド中止 | `--force-build` オプション使用 |
| E003 | プロジェクト構造の問題 | プロジェクトディレクトリ名の確認と修正 |
| E004 | Go依存関係の問題 | `go mod tidy` 実行 |
| E011 | Gitリポジトリからのファイル欠落 | `git checkout HEAD -- cmd/` 実行 |
| E005 | 証明書関連のエラー | `./scripts/generate-certs.sh` 実行 |
| E006 | ディスク容量不足 | `docker system prune -a` 実行 |
| E007 | テスト環境設定の問題 | `./scripts/check-test-environment.sh` 実行 |
| E008 | 監視テストの失敗 | 証明書生成または監視テストスキップ |
| E009 | ネットワーク接続エラー | ネットワーク設定確認 |
| E010 | Goバージョンの問題 | Go 1.24.2以上にアップデート |

### 実行時エラーコード

| コード | 説明 | 解決方法 |
|--------|------|----------|
| R001 | RBAC権限不足 | `kubectl apply -f manifests/rbac.yaml` |
| R002 | 証明書検証エラー | 証明書再生成とCA設定更新 |
| R003 | ネットワーク接続失敗 | ファイアウォール・DNS設定確認 |
| R004 | リソース不足 | メモリ・CPU制限の調整 |
| R005 | 設定ファイルエラー | 設定ファイルの構文確認 |

### テスト関連エラーコード

| コード | 説明 | 解決方法 |
|--------|------|----------|
| T001 | 単体テスト失敗 | テストキャッシュクリアと再実行 |
| T002 | 統合テスト失敗 | 証明書生成と権限設定 |
| T003 | E2Eテスト環境エラー | 環境リセットと再構築 |
| T004 | テストタイムアウト | タイムアウト時間延長 |
| T005 | テストデータ不整合 | テストデータの確認と修正 |

---

## 追加のサポート情報

### ログの確認方法

```bash
# Webhookのログ
kubectl logs -l app=k8s-deployment-hpa-validator -f

# システムログ
sudo journalctl -u docker -f

# kindクラスターのログ
kind export logs /tmp/kind-logs --name hpa-validator
```

### デバッグモードの有効化

```bash
# ビルド時のデバッグ
DEBUG=true ./scripts/build-image.sh

# テスト実行時のデバッグ
DEBUG=true ./scripts/run-e2e-tests.sh

# Webhook実行時のデバッグ
kubectl patch deployment k8s-deployment-hpa-validator \
  -p='{"spec":{"template":{"spec":{"containers":[{"name":"webhook","env":[{"name":"LOG_LEVEL","value":"debug"}]}]}}}}'
```

### 問題報告時に含めるべき情報

1. **環境情報**
   ```bash
   # システム情報
   uname -a
   docker --version
   kubectl version
   go version
   
   # プロジェクト情報
   git rev-parse HEAD
   ls -la
   ```

2. **エラーログ**
   ```bash
   # 関連するログを全て含める
   kubectl logs -l app=k8s-deployment-hpa-validator --previous
   kubectl describe pods -l app=k8s-deployment-hpa-validator
   kubectl get events --sort-by='.lastTimestamp'
   ```

3. **再現手順**
   - 実行したコマンドの順序
   - 使用した設定ファイル
   - 環境変数の設定

4. **期待される動作と実際の動作**
   - 何を期待していたか
   - 実際に何が起こったか
   - エラーメッセージの全文

---

このトラブルシューティングガイドは継続的に更新されます。新しい問題や解決方法が見つかった場合は、このドキュメントに追加してください。