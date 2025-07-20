# cert-manager統合ガイド

## 概要

このドキュメントでは、k8s-deployment-hpa-validatorとcert-managerの統合について説明します。cert-managerを使用することで、TLS証明書の自動発行・更新が可能になります。

## 前提条件

### cert-managerのインストール

```bash
# cert-managerのインストール
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# インストール確認
kubectl get pods -n cert-manager
```

## 設定方法

### 1. ClusterIssuerの設定

#### 自己署名証明書を使用する場合（開発・テスト環境）

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: webhook-ca-issuer
spec:
  selfSigned: {}
```

#### Let's Encryptを使用する場合（本番環境）

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: webhook-letsencrypt-issuer
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com  # 実際のメールアドレスに変更
    privateKeySecretRef:
      name: letsencrypt-private-key
    solvers:
    - http01:
        ingress:
          class: nginx
```

#### 内部CA証明書を使用する場合

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: webhook-internal-ca-issuer
spec:
  ca:
    secretName: internal-ca-secret
```

### 2. Certificateリソースの設定

基本的なCertificate設定は`manifests/base/certificate.yaml`に定義されています。

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: webhook-tls
  namespace: webhook-system
spec:
  secretName: webhook-tls-secret
  issuerRef:
    name: webhook-ca-issuer
    kind: ClusterIssuer
  duration: 2160h # 90日
  renewBefore: 720h # 30日前に更新
  dnsNames:
    - k8s-deployment-hpa-validator.webhook-system.svc.cluster.local
```

### 3. 環境別設定

#### 開発環境
- 自己署名証明書を使用
- 短い有効期限（90日）
- 頻繁な更新テスト

#### 本番環境
- Let's Encryptまたは内部CA証明書を使用
- 長い有効期限（1年）
- 強固な暗号化設定（RSA 4096bit）

## デプロイ手順

### 1. ClusterIssuerの作成

```bash
# 自己署名証明書の場合
kubectl apply -f manifests/base/cluster-issuer.yaml

# Let's Encryptの場合（メールアドレスを変更してから実行）
kubectl apply -f manifests/base/cluster-issuer.yaml
```

### 2. Webhookのデプロイ

```bash
# 開発環境
kubectl apply -k manifests/overlays/development

# 本番環境
kubectl apply -k manifests/overlays/production
```

### 3. 証明書の確認

```bash
# Certificate リソースの状態確認
kubectl get certificate -n webhook-system

# 証明書の詳細確認
kubectl describe certificate webhook-tls -n webhook-system

# 生成されたSecretの確認
kubectl get secret webhook-tls-secret -n webhook-system
```

## トラブルシューティング

### 証明書が発行されない場合

```bash
# CertificateRequestの確認
kubectl get certificaterequest -n webhook-system

# Orderの確認（ACME使用時）
kubectl get order -n webhook-system

# Challengeの確認（ACME使用時）
kubectl get challenge -n webhook-system

# cert-managerのログ確認
kubectl logs -n cert-manager deployment/cert-manager
```

### よくある問題と解決方法

#### 1. DNS名の不一致
- Certificateの`dnsNames`とServiceの名前が一致していることを確認
- ValidatingWebhookConfigurationの`clientConfig.service`設定を確認

#### 2. ACME Challenge失敗
- Ingressの設定を確認
- ファイアウォールの設定を確認
- DNS設定を確認

#### 3. 証明書の更新失敗
- cert-managerのRBAC権限を確認
- Secretの権限を確認

## 監視とアラート

### 証明書の有効期限監視

```yaml
# PrometheusRule例
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: cert-manager-alerts
spec:
  groups:
  - name: cert-manager
    rules:
    - alert: CertificateExpiringSoon
      expr: certmanager_certificate_expiration_timestamp_seconds - time() < 7 * 24 * 3600
      for: 1h
      labels:
        severity: warning
      annotations:
        summary: "証明書の有効期限が近づいています"
        description: "{{ $labels.name }}の証明書が7日以内に期限切れになります"
```

### Grafanaダッシュボード

cert-manager用のGrafanaダッシュボードを使用して、証明書の状態を監視できます。

```bash
# cert-manager用ダッシュボードのインポート
# Grafana Dashboard ID: 11001
```

## セキュリティ考慮事項

### 1. 秘密鍵の保護
- Secretの適切なRBAC設定
- 秘密鍵のローテーション設定

### 2. 証明書の検証
- 証明書チェーンの検証
- CRL/OCSPの確認

### 3. 監査ログ
- 証明書の発行・更新ログの記録
- 異常なアクセスの監視

## 参考資料

- [cert-manager公式ドキュメント](https://cert-manager.io/docs/)
- [Let's Encrypt公式サイト](https://letsencrypt.org/)
- [Kubernetes TLS管理ベストプラクティス](https://kubernetes.io/docs/concepts/services-networking/ingress/#tls)