### 6.3 サーバー起動

```go
// Start starts the webhook server
func (s *Server) Start() error {
    // ルーティングを設定
    mux := http.NewServeMux()
    mux.HandleFunc("/validate-deployment", s.ServeValidateDeployment)
    mux.HandleFunc("/validate-hpa", s.ServeValidateHPA)
    mux.HandleFunc("/healthz", s.ServeHealthz)
    mux.HandleFunc("/readyz", s.ServeReadyz)
    
    // メトリクスエンドポイントを設定（有効な場合）
    if s.config.MetricsEnabled {
        go s.startMetricsServer()
    }
    
    // HTTPハンドラーを設定
    s.server.Handler = s.withLogging(mux)
    
    // 証明書を読み込み
    cert, err := s.certManager.LoadCertificate()
    if err != nil {
        return fmt.Errorf("証明書の読み込みに失敗しました: %w", err)
    }
    
    // 証明書の自動更新を設定
    go s.certManager.StartCertificateRenewal(s.reloadCertificate)
    
    // サーバー情報をログ出力
    s.logger.Info("Webhookサーバーを起動しています", map[string]interface{}{
        "port":             s.config.Port,
        "tls_cert_file":    s.config.TLSCertFile,
        "tls_key_file":     s.config.TLSKeyFile,
        "timeout":          s.config.Timeout.String(),
        "environment":      s.config.Environment,
    })
    
    // メトリクスを初期化
    metrics.SetWebhookUp(true)
    
    // サーバーを起動（TLS）
    return s.server.ListenAndServeTLS("", "")
}
```

**解説：**
1. HTTPルーティングを設定（各エンドポイントにハンドラーを割り当て）
2. メトリクスサーバーを別のゴルーチンで起動（有効な場合）
3. ロギングミドルウェアを設定
4. TLS証明書を読み込み
5. 証明書の自動更新を別のゴルーチンで開始
6. サーバー情報をログに出力
7. メトリクスを初期化（サーバーが起動したことを記録）
8. TLSサーバーを起動して待機

### 6.4 リクエスト処理

```go
// ServeValidateDeployment はDeploymentのバリデーションリクエストを処理します
func (s *Server) ServeValidateDeployment(w http.ResponseWriter, r *http.Request) {
    // リクエストのメトリクス記録を開始
    reqMetrics := metrics.NewRequestMetrics("POST", "Deployment")
    
    // リクエストのJSONをデコード
    var admissionReview admissionv1.AdmissionReview
    if err := json.NewDecoder(r.Body).Decode(&admissionReview); err != nil {
        // デコードエラー処理
        reqMetrics.RecordError("decode_error")
        s.handleError(w, err, http.StatusBadRequest)
        return
    }
    
    // レスポンス用のAdmissionReviewを準備
    responseAdmissionReview := admissionv1.AdmissionReview{
        TypeMeta: admissionReview.TypeMeta,
        Response: &admissionv1.AdmissionResponse{
            UID: admissionReview.Request.UID,
        },
    }
    
    // Deploymentオブジェクトをデコード
    var deployment appsv1.Deployment
    if err := json.Unmarshal(admissionReview.Request.Object.Raw, &deployment); err != nil {
        // デコードエラー処理
        reqMetrics.RecordError("unmarshal_error")
        responseAdmissionReview.Response.Allowed = false
        responseAdmissionReview.Response.Result = &metav1.Status{
            Message: fmt.Sprintf("Deploymentオブジェクトのデコードに失敗しました: %v", err),
        }
    } else {
        // バリデーションを実行
        err := s.validator.ValidateDeployment(r.Context(), &deployment)
        if err != nil {
            // バリデーションエラー
            reqMetrics.RecordError("validation_error")
            responseAdmissionReview.Response.Allowed = false
            responseAdmissionReview.Response.Result = &metav1.Status{
                Message: err.Error(),
            }
        } else {
            // バリデーション成功
            responseAdmissionReview.Response.Allowed = true
        }
    }
    
    // レスポンスを返す
    s.sendAdmissionResponse(w, responseAdmissionReview)
    reqMetrics.RecordSuccess()
}
```