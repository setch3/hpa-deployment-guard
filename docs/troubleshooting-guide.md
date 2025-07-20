# トラブルシューティングガイド

## 概要

このドキュメントは、k8s-deployment-hpa-validatorの運用中に発生する可能性のある問題と、その解決方法について説明します。

## 基本的なデバッグ手順

### 1. 現在の状態確認

```bash
# Podの状態確認
kubectl get pods -n webhook-system -l app=k8s-deployment-hpa-validator

# サービスの状態確認
kubectl get svc -n webhook-system

# ValidatingWebhookConfigurationの確認
kubectl get validatingwebhookconfiguration k8s-deployment-hpa-validator

# 証明書の状態確認
kubectl get certificate -n webhook-system
kubectl get secret -n webhook-system webhook-tls-secret
```

### 2. ログの確認

```bash
# 現在のログを確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --tail=100

# リアルタイムでログを監視
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator -f

# 複数のPodのログを同時に確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --all-containers=true
```

### 3. イベントの確認

```bash
# 名前空間のイベント確認
kubectl get events -n webhook-system --sort-by='.lastTimestamp'

# 特定のリソースのイベント確認
kubectl describe pod -n webhook-system <pod-name>
```

## よくある問題と解決方法

### 問題1: Podが起動しない

#### 症状
- Podが`Pending`、`CrashLoopBackOff`、`ImagePullBackOff`状態
- webhookが応答しない

#### 原因と解決方法

##### 1.1 イメージの問題
```bash
# 症状確認
kubectl describe pod -n webhook-system <pod-name>

# 解決方法
# - イメージ名とタグを確認
# - イメージレジストリへのアクセス権限を確認
# - ImagePullSecretが正しく設定されているか確認
```

##### 1.2 リソース不足
```bash
# 症状確認
kubectl describe nodes
kubectl top nodes
kubectl top pods -n webhook-system

# 解決方法
# - ノードのリソース使用量を確認
# - リソース要求量を調整
# - 必要に応じてノードを追加
```

##### 1.3 設定エラー
```bash
# 症状確認
kubectl logs -n webhook-system <pod-name>

# よくあるエラーメッセージ
# "証明書ファイルが見つかりません"
# "設定ファイルの読み込みに失敗しました"
# "ポートがすでに使用されています"

# 解決方法
kubectl describe configmap -n webhook-system webhook-config
kubectl describe secret -n webhook-system webhook-tls-secret
```

### 問題2: TLS証明書の問題

#### 症状
- `x509: certificate signed by unknown authority`
- `tls: bad certificate`
- webhookへの接続が失敗する

#### 原因と解決方法

##### 2.1 証明書の有効期限切れ
```bash
# 症状確認
kubectl describe certificate -n webhook-system webhook-tls
kubectl get secret -n webhook-system webhook-tls-secret -o yaml | grep -A 5 tls.crt | tail -1 | base64 -d | openssl x509 -noout -dates

# 解決方法
# cert-managerによる自動更新を確認
kubectl describe certificate -n webhook-system webhook-tls
kubectl logs -n cert-manager -l app=cert-manager
```

##### 2.2 証明書の設定ミス
```bash
# 症状確認
kubectl describe validatingwebhookconfiguration k8s-deployment-hpa-validator

# 解決方法
# CABundleが正しく設定されているか確認
# サービス名とDNS名が一致しているか確認
```

##### 2.3 cert-managerの問題
```bash
# 症状確認
kubectl get pods -n cert-manager
kubectl logs -n cert-manager -l app=cert-manager

# 解決方法
# cert-managerの再起動
kubectl rollout restart deployment -n cert-manager cert-manager
kubectl rollout restart deployment -n cert-manager cert-manager-webhook
```

### 問題3: webhookが応答しない

#### 症状
- `context deadline exceeded`
- `connection refused`
- リクエストがタイムアウトする

#### 原因と解決方法

##### 3.1 ネットワーク接続の問題
```bash
# 症状確認
kubectl get svc -n webhook-system
kubectl get endpoints -n webhook-system

# 接続テスト
kubectl port-forward -n webhook-system svc/k8s-deployment-hpa-validator 8443:443 &
curl -k https://localhost:8443/healthz

# 解決方法
# - サービスのセレクターとPodのラベルが一致しているか確認
# - NetworkPolicyが通信を阻害していないか確認
# - ファイアウォール設定を確認
```

##### 3.2 Podの健康状態の問題
```bash
# 症状確認
kubectl get pods -n webhook-system -l app=k8s-deployment-hpa-validator
kubectl describe pod -n webhook-system <pod-name>

# ヘルスチェック確認
kubectl exec -n webhook-system <pod-name> -- curl -k https://localhost:8443/healthz

# 解決方法
# - Podのリソース使用量を確認
# - アプリケーションのログを確認
# - 必要に応じてPodを再起動
kubectl delete pod -n webhook-system <pod-name>
```

##### 3.3 ValidatingWebhookConfigurationの設定問題
```bash
# 症状確認
kubectl describe validatingwebhookconfiguration k8s-deployment-hpa-validator

# よくある設定ミス
# - サービス名が間違っている
# - 名前空間が間違っている
# - パスが間違っている
# - タイムアウト設定が短すぎる

# 解決方法
# 設定を修正して再適用
kubectl apply -f manifests/base/validating-webhook-configuration.yaml
```

### 問題4: バリデーションが正しく動作しない

#### 症状
- 期待されるバリデーションエラーが発生しない
- 不適切なリソースが作成される
- バリデーションが過度に厳しい

#### 原因と解決方法

##### 4.1 スキップ設定の問題
```bash
# 症状確認
kubectl describe configmap -n webhook-system webhook-config

# 解決方法
# SKIP_NAMESPACESとSKIP_LABELSの設定を確認
# 意図しない名前空間やラベルがスキップされていないか確認
```

##### 4.2 webhookの対象リソースの問題
```bash
# 症状確認
kubectl describe validatingwebhookconfiguration k8s-deployment-hpa-validator

# 解決方法
# rules.resourcesとrules.apiVersionsが正しく設定されているか確認
# namespaceSelector、objectSelectorが適切か確認
```

##### 4.3 バリデーションロジックの問題
```bash
# 症状確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | grep -i validation

# デバッグ用のテストリソース作成
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: debug-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: debug-app
  template:
    metadata:
      labels:
        app: debug-app
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
EOF

# 解決方法
# ログを詳細に確認してバリデーションロジックをデバッグ
# 必要に応じてLOG_LEVELをdebugに変更
```

### 問題5: パフォーマンスの問題

#### 症状
- webhookの応答が遅い
- リクエストがタイムアウトする
- クラスター全体のパフォーマンスが低下

#### 原因と解決方法

##### 5.1 リソース不足
```bash
# 症状確認
kubectl top pods -n webhook-system
kubectl describe pod -n webhook-system <pod-name>

# 解決方法
# リソース制限を増加
# レプリカ数を増加
# HorizontalPodAutoscalerの設定
```

##### 5.2 ネットワークレイテンシ
```bash
# 症状確認
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | grep -i duration

# 解決方法
# webhookのタイムアウト設定を調整
# ネットワーク設定を最適化
# Podの配置を最適化（アフィニティ設定）
```

## ログ分析方法

### 構造化ログの分析

#### 基本的なログフィルタリング
```bash
# エラーログのみ表示
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq 'select(.level == "error")'

# 特定の時間範囲のログ
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --since=1h

# 特定のリソースに関するログ
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq 'select(.resource == "Deployment")'
```

#### パフォーマンス分析
```bash
# 処理時間の長いリクエストを特定
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq 'select(.duration and (.duration | tonumber) > 1000)'

# リクエスト頻度の分析
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq '.timestamp' | sort | uniq -c
```

#### エラー分析
```bash
# エラーの種類別集計
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq -r 'select(.level == "error") | .error' | sort | uniq -c

# バリデーションエラーの詳細
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator | jq 'select(.action == "validation" and .level == "error")'
```

## デバッグ用ツールとコマンド

### 1. 接続テスト用スクリプト

```bash
#!/bin/bash
# webhook-connectivity-test.sh

NAMESPACE="webhook-system"
SERVICE_NAME="k8s-deployment-hpa-validator"
PORT="443"

echo "=== Webhook接続テスト ==="

# サービスの存在確認
if ! kubectl get svc -n $NAMESPACE $SERVICE_NAME > /dev/null 2>&1; then
    echo "エラー: サービス $SERVICE_NAME が見つかりません"
    exit 1
fi

# エンドポイントの確認
ENDPOINTS=$(kubectl get endpoints -n $NAMESPACE $SERVICE_NAME -o jsonpath='{.subsets[0].addresses[*].ip}')
if [ -z "$ENDPOINTS" ]; then
    echo "エラー: エンドポイントが見つかりません"
    exit 1
fi

echo "エンドポイント: $ENDPOINTS"

# ポートフォワードでの接続テスト
kubectl port-forward -n $NAMESPACE svc/$SERVICE_NAME 8443:$PORT &
PF_PID=$!
sleep 2

# ヘルスチェック
if curl -k -s https://localhost:8443/healthz > /dev/null; then
    echo "✓ ヘルスチェック成功"
else
    echo "✗ ヘルスチェック失敗"
fi

# メトリクス確認
if curl -k -s https://localhost:8443/metrics > /dev/null; then
    echo "✓ メトリクス取得成功"
else
    echo "✗ メトリクス取得失敗"
fi

# クリーンアップ
kill $PF_PID 2>/dev/null
```

### 2. 証明書検証スクリプト

```bash
#!/bin/bash
# cert-validation.sh

NAMESPACE="webhook-system"
SECRET_NAME="webhook-tls-secret"

echo "=== TLS証明書検証 ==="

# 証明書の取得
CERT=$(kubectl get secret -n $NAMESPACE $SECRET_NAME -o jsonpath='{.data.tls\.crt}' | base64 -d)

if [ -z "$CERT" ]; then
    echo "エラー: 証明書が見つかりません"
    exit 1
fi

# 証明書の詳細表示
echo "$CERT" | openssl x509 -noout -text | grep -A 2 "Validity"
echo "$CERT" | openssl x509 -noout -text | grep -A 5 "Subject Alternative Name"

# 有効期限の確認
EXPIRY=$(echo "$CERT" | openssl x509 -noout -enddate | cut -d= -f2)
EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
CURRENT_EPOCH=$(date +%s)
DAYS_LEFT=$(( ($EXPIRY_EPOCH - $CURRENT_EPOCH) / 86400 ))

echo "証明書の有効期限: $EXPIRY"
echo "残り日数: $DAYS_LEFT 日"

if [ $DAYS_LEFT -lt 30 ]; then
    echo "警告: 証明書の有効期限が30日以内です"
fi
```

### 3. リソース使用量監視スクリプト

```bash
#!/bin/bash
# resource-monitor.sh

NAMESPACE="webhook-system"
LABEL="app=k8s-deployment-hpa-validator"

echo "=== リソース使用量監視 ==="

while true; do
    echo "$(date): リソース使用量"
    kubectl top pods -n $NAMESPACE -l $LABEL
    echo "---"
    sleep 30
done
```

## 緊急時の対応手順

### 1. webhookの無効化（緊急時）

```bash
# FailurePolicyをIgnoreに変更（一時的な回避策）
kubectl patch validatingwebhookconfiguration k8s-deployment-hpa-validator \
  --type='json' \
  -p='[{"op": "replace", "path": "/webhooks/0/failurePolicy", "value": "Ignore"}]'

# または、webhookを完全に削除
kubectl delete validatingwebhookconfiguration k8s-deployment-hpa-validator
```

### 2. サービスの復旧

```bash
# Podの再起動
kubectl rollout restart deployment -n webhook-system k8s-deployment-hpa-validator

# 設定の再適用
kubectl apply -f manifests/overlays/production/

# 状態確認
kubectl get pods -n webhook-system -l app=k8s-deployment-hpa-validator
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --tail=50
```

### 3. ロールバック

```bash
# ArgoCDでのロールバック
argocd app rollback k8s-deployment-hpa-validator <previous-revision>

# 手動でのロールバック
kubectl rollout undo deployment -n webhook-system k8s-deployment-hpa-validator
```

## 予防的メンテナンス

### 定期的な確認項目

1. **証明書の有効期限確認**（月次）
2. **リソース使用量の監視**（週次）
3. **ログの確認とローテーション**（週次）
4. **バックアップの確認**（月次）
5. **セキュリティアップデートの適用**（月次）

### 監視アラートの設定

- 証明書の有効期限（30日前にアラート）
- Podの再起動回数（閾値超過時）
- リソース使用量（80%超過時）
- エラー率（5%超過時）
- 応答時間（1秒超過時）

## サポートとエスカレーション

### 問題の分類

1. **レベル1**: 基本的な運用問題（再起動で解決）
2. **レベル2**: 設定や環境の問題（設定変更が必要）
3. **レベル3**: アプリケーションの問題（コード修正が必要）

### エスカレーション時の情報収集

```bash
# 基本情報の収集
kubectl get all -n webhook-system
kubectl describe pod -n webhook-system -l app=k8s-deployment-hpa-validator
kubectl logs -n webhook-system -l app=k8s-deployment-hpa-validator --tail=200
kubectl get events -n webhook-system --sort-by='.lastTimestamp'

# 設定情報の収集
kubectl get configmap -n webhook-system webhook-config -o yaml
kubectl get secret -n webhook-system webhook-tls-secret -o yaml
kubectl get validatingwebhookconfiguration k8s-deployment-hpa-validator -o yaml

# システム情報の収集
kubectl version
kubectl get nodes
kubectl top nodes
```

## 参考資料

- [Kubernetes Troubleshooting Guide](https://kubernetes.io/docs/tasks/debug-application-cluster/)
- [Admission Controller Troubleshooting](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#troubleshooting)
- [cert-manager Troubleshooting](https://cert-manager.io/docs/faq/troubleshooting/)