package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"k8s-deployment-hpa-validator/internal/cert"
	"k8s-deployment-hpa-validator/internal/config"
	"k8s-deployment-hpa-validator/internal/logging"
	"k8s-deployment-hpa-validator/internal/metrics"
	"k8s-deployment-hpa-validator/internal/validator"
)

// Server represents the webhook server
type Server struct {
	server       *http.Server
	client       kubernetes.Interface
	validator    validator.Validator
	scheme       *runtime.Scheme
	codecs       serializer.CodecFactory
	certManager  *cert.Manager
	logger       *logging.Logger
	config       *config.WebhookConfig
	errorHandler *ErrorHandler
}

// createKubernetesClient creates a Kubernetes client with fallback configuration
func createKubernetesClient(logger *logging.Logger) (kubernetes.Interface, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Info("クラスター内設定の作成に失敗しました。kubeconfigを試行します", map[string]interface{}{
			"error": err.Error(),
		})
		
		// Fallback to kubeconfig
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		
		// Check if KUBECONFIG environment variable is set
		if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
			kubeconfig = kubeconfigEnv
		}
		
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create kubeconfig: %w", err)
		}
		logger.Info("kubeconfigを使用します", map[string]interface{}{
			"kubeconfig_path": kubeconfig,
		})
	} else {
		logger.Info("クラスター内設定を使用します")
	}

	// Set client configuration
	config.QPS = 50
	config.Burst = 100

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Test the client connection
	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Kubernetes API server: %w", err)
	}

	logger.Info("Kubernetes APIサーバーへの接続に成功しました")
	return client, nil
}

// NewServer creates a new webhook server instance
func NewServer(port int, certFile, keyFile string) (*Server, error) {
	return NewServerWithCA(port, certFile, keyFile, "")
}

// NewServerWithCA creates a new webhook server instance with CA certificate
func NewServerWithCA(port int, certFile, keyFile, caFile string) (*Server, error) {
	return NewServerWithConfig(port, certFile, keyFile, caFile, nil)
}

// NewServerWithConfig creates a new webhook server instance with configuration
func NewServerWithConfig(port int, certFile, keyFile, caFile string, cfg *config.WebhookConfig) (*Server, error) {
	// デフォルト設定を作成（設定が提供されていない場合）
	if cfg == nil {
		loader := config.NewConfigLoader()
		var err error
		cfg, err = loader.LoadConfig()
		if err != nil {
			// 設定読み込みに失敗した場合はデフォルト値を使用
			cfg = &config.WebhookConfig{
				Port:        port,
				TLSCertFile: certFile,
				TLSKeyFile:  keyFile,
				Environment: "development",
				LogLevel:    "info",
				LogFormat:   "json",
			}
		}
		// コマンドライン引数で指定された値を優先
		if port != 0 {
			cfg.Port = port
		}
		if certFile != "" {
			cfg.TLSCertFile = certFile
		}
		if keyFile != "" {
			cfg.TLSKeyFile = keyFile
		}
	}

	// Create structured logger
	logger := logging.NewLogger("webhook-server")

	// Create Kubernetes client with fallback configuration
	client, err := createKubernetesClient(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create validator
	v := validator.NewDeploymentHPAValidator(client)

	// Create certificate manager
	certManager := cert.NewManager(certFile, keyFile, caFile)

	// Load and validate TLS certificate
	tlsCert, err := certManager.LoadCertificate()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	// Validate certificate chain if CA is provided
	if caFile != "" {
		if err := certManager.ValidateCertificateChain(); err != nil {
			log.Printf("警告: 証明書チェーンの検証に失敗しました: %v", err)
		}
	}

	// Log certificate information and update metrics
	if info, err := certManager.GetCertificateInfo(); err == nil {
		logger.Info("証明書情報を取得しました", map[string]interface{}{
			"subject":             info.Subject,
			"not_after":           info.NotAfter.Format("2006-01-02 15:04:05"),
			"days_until_expiry":   info.DaysUntilExpiry,
			"dns_names":           info.DNSNames,
		})
		
		if info.DaysUntilExpiry <= 30 {
			logger.Warn("証明書の有効期限が近づいています", map[string]interface{}{
				"days_until_expiry": info.DaysUntilExpiry,
			})
		}
		
		// 証明書の有効期限メトリクスを更新
		metrics.UpdateCertificateExpiry(info.DaysUntilExpiry)
	}

	// Create runtime scheme and codecs for admission requests
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	// Create HTTP server with TLS
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	// Create error handler
	errorHandler := NewErrorHandler(cfg, logger)

	s := &Server{
		server:       server,
		client:       client,
		validator:    v,
		scheme:       scheme,
		codecs:       codecs,
		certManager:  certManager,
		logger:       logger,
		config:       cfg,
		errorHandler: errorHandler,
	}

	// Register handlers with middleware
	mux.HandleFunc("/validate", s.withMiddleware(s.handleValidate))
	mux.HandleFunc("/health", s.withMiddleware(s.handleHealth))
	mux.HandleFunc("/healthz", s.withMiddleware(s.handleHealthz))
	mux.HandleFunc("/readyz", s.withMiddleware(s.handleReadiness))
	mux.HandleFunc("/livez", s.withMiddleware(s.handleLiveness))
	mux.Handle("/metrics", promhttp.Handler())

	return s, nil
}

// Start starts the webhook server
func (s *Server) Start(ctx context.Context) error {
	// webhookの稼働状態を設定
	metrics.SetWebhookUp(true)
	
	// 証明書監視を開始
	s.certManager.SetReloadCallback(s.reloadCertificate)
	s.certManager.StartMonitoring(5 * time.Minute) // 5分間隔で監視
	defer s.certManager.StopMonitoring()
	
	s.logger.Info("証明書監視を開始しました", map[string]interface{}{
		"check_interval": "5m",
	})
	
	errCh := make(chan error, 1)

	go func() {
		if err := s.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("failed to start HTTPS server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		metrics.SetWebhookUp(false)
		return err
	case <-ctx.Done():
		metrics.SetWebhookUp(false)
		return s.server.Shutdown(context.Background())
	}
}

// handleValidate handles validation requests
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	// リクエストIDを生成してコンテキストに設定
	requestID := logging.GenerateRequestID()
	ctx := logging.ContextWithRequestID(r.Context(), requestID)
	requestLogger := s.logger.WithRequestID(requestID)

	requestLogger.Info("バリデーションリクエストを受信しました", map[string]interface{}{
		"remote_addr": r.RemoteAddr,
		"method":      r.Method,
		"path":        r.URL.Path,
	})

	// メトリクス記録開始
	requestMetrics := metrics.NewRequestMetrics(r.Method, "unknown")

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		requestLogger.Error("リクエストボディの読み込みに失敗しました", map[string]interface{}{
			"error": err.Error(),
		})
		requestMetrics.RecordError("request_body_read_error")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse admission request
	admissionReview := &admissionv1.AdmissionReview{}
	if err := json.Unmarshal(body, admissionReview); err != nil {
		requestLogger.Error("AdmissionReviewの解析に失敗しました", map[string]interface{}{
			"error": err.Error(),
		})
		requestMetrics.RecordError("admission_review_parse_error")
		http.Error(w, "Failed to parse admission review", http.StatusBadRequest)
		return
	}

	// リソース情報を取得
	var resourceType, resourceName, resourceNamespace string
	if admissionReview.Request != nil {
		resourceType = admissionReview.Request.Kind.Kind
		resourceName = admissionReview.Request.Name
		resourceNamespace = admissionReview.Request.Namespace
		requestMetrics.ResourceType = resourceType
	}

	requestLogger.Info("リソースのバリデーションを開始します", map[string]interface{}{
		"resource_type": resourceType,
		"resource_name": resourceName,
		"namespace":     resourceNamespace,
		"operation":     string(admissionReview.Request.Operation),
	})

	// Validate the request
	admissionResponse := s.validateAdmissionRequest(ctx, admissionReview.Request)

	// Create response
	responseReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: admissionResponse,
	}

	// Marshal and send response
	responseBytes, err := json.Marshal(responseReview)
	if err != nil {
		requestLogger.Error("レスポンスのマーシャルに失敗しました", map[string]interface{}{
			"error": err.Error(),
		})
		requestMetrics.RecordError("response_marshal_error")
		http.Error(w, "Failed to create response", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseBytes)
	
	// ログとメトリクス記録
	if admissionResponse.Allowed {
		requestLogger.Info("バリデーションが成功しました", map[string]interface{}{
			"resource_type": resourceType,
			"resource_name": resourceName,
			"namespace":     resourceNamespace,
			"allowed":       true,
		})
		requestMetrics.RecordSuccess()
	} else {
		// バリデーションエラーの詳細を記録
		errorType := "validation_failed"
		if admissionResponse.Result != nil && admissionResponse.Result.Code >= 500 {
			errorType = "system_error"
		}
		
		requestLogger.Warn("バリデーションが失敗しました", map[string]interface{}{
			"resource_type": resourceType,
			"resource_name": resourceName,
			"namespace":     resourceNamespace,
			"allowed":       false,
			"error_type":    errorType,
			"message":       admissionResponse.Result.Message,
		})
		requestMetrics.RecordError(errorType)
	}
}

// validateAdmissionRequest validates an admission request and returns an admission response
func (s *Server) validateAdmissionRequest(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	requestID := logging.RequestIDFromContext(ctx)
	requestLogger := s.logger.WithRequestID(requestID)

	if req == nil {
		err := validator.NewWebhookError(
			validator.ErrorTypeInternal,
			validator.CodeInternalUnknown,
			"AdmissionRequestがnilです",
		)
		return s.errorHandler.HandleError(ctx, err, req)
	}

	requestLogger.Debug("リソースのバリデーションを実行中", map[string]interface{}{
		"kind":      req.Kind.Kind,
		"namespace": req.Namespace,
		"name":      req.Name,
		"operation": string(req.Operation),
	})

	var err error

	switch req.Kind.Kind {
	case "Deployment":
		deployment := &appsv1.Deployment{}
		if parseErr := json.Unmarshal(req.Object.Raw, deployment); parseErr != nil {
			requestLogger.Error("Deploymentの解析に失敗しました", map[string]interface{}{
				"error": parseErr.Error(),
			})
			err = validator.NewWebhookError(
				validator.ErrorTypeInternal,
				validator.CodeInvalidResource,
				"Deploymentの解析に失敗しました",
			).WithInternalError(parseErr).WithContext(requestID, "Deployment", req.Name, req.Namespace)
		} else {
			result := s.validator.ValidateResource(ctx, "Deployment", deployment)
			if !result.Allowed {
				// ValidationResultからエラーを作成
				err = validator.NewWebhookError(
					validator.ErrorTypeValidation,
					validator.CodeDeploymentHPAConflict,
					result.Message,
				).WithContext(requestID, "Deployment", req.Name, req.Namespace)
			}
		}

	case "HorizontalPodAutoscaler":
		hpa := &autoscalingv2.HorizontalPodAutoscaler{}
		if parseErr := json.Unmarshal(req.Object.Raw, hpa); parseErr != nil {
			requestLogger.Error("HPAの解析に失敗しました", map[string]interface{}{
				"error": parseErr.Error(),
			})
			err = validator.NewWebhookError(
				validator.ErrorTypeInternal,
				validator.CodeInvalidResource,
				"HPAの解析に失敗しました",
			).WithInternalError(parseErr).WithContext(requestID, "HorizontalPodAutoscaler", req.Name, req.Namespace)
		} else {
			result := s.validator.ValidateResource(ctx, "HorizontalPodAutoscaler", hpa)
			if !result.Allowed {
				// ValidationResultからエラーを作成
				err = validator.NewWebhookError(
					validator.ErrorTypeValidation,
					validator.CodeHPASingleReplica,
					result.Message,
				).WithContext(requestID, "HorizontalPodAutoscaler", req.Name, req.Namespace)
			}
		}

	default:
		// Allow other resource types
		requestLogger.Debug("サポートされていないリソースタイプを許可します", map[string]interface{}{
			"resource_type": req.Kind.Kind,
		})
		return &admissionv1.AdmissionResponse{
			UID:     req.UID,
			Allowed: true,
		}
	}

	return s.errorHandler.HandleError(ctx, err, req)
}

// withMiddleware wraps handlers with common middleware
func (s *Server) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log request
		s.logger.Debug("HTTPリクエストを受信しました", map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
		})
		
		// Set common headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Validate request method for validation endpoint
		if r.URL.Path == "/validate" && r.Method != http.MethodPost {
			s.logger.Warn("許可されていないHTTPメソッドです", map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
			})
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Call the actual handler
		handler(w, r)
		
		// Log response
		s.logger.Debug("HTTPレスポンスを送信しました", map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
	}
}

// HealthStatus represents the health status of the webhook
type HealthStatus struct {
	Status      string                 `json:"status"`
	Message     string                 `json:"message"`
	Timestamp   string                 `json:"timestamp"`
	Components  map[string]interface{} `json:"components"`
	Version     string                 `json:"version,omitempty"`
	Environment string                 `json:"environment,omitempty"`
}

// handleHealth handles health check requests with detailed status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := s.getDetailedHealthStatus(r.Context())
	
	w.Header().Set("Content-Type", "application/json")
	
	// ヘルスチェックが失敗した場合は503を返す
	if health.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(health)
}

// handleHealthz handles Kubernetes-style health check requests
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	health := s.getDetailedHealthStatus(r.Context())
	
	w.Header().Set("Content-Type", "application/json")
	
	// Kubernetesスタイルのヘルスチェック
	if health.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"checks": health.Components,
		})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"message": health.Message,
			"checks": health.Components,
		})
	}
}

// handleReadiness handles readiness probe requests
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	readiness := s.getReadinessStatus()
	
	w.Header().Set("Content-Type", "application/json")
	
	if readiness.Status != "ready" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(readiness)
}

// handleLiveness handles liveness probe requests
func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	liveness := s.getLivenessStatus()
	
	w.Header().Set("Content-Type", "application/json")
	
	if liveness.Status != "alive" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(liveness)
}

// ReloadCertificate reloads the TLS certificate
func (s *Server) ReloadCertificate() error {
	s.logger.Info("証明書を再読み込み中...")
	
	// Load new certificate
	newCert, err := s.certManager.LoadCertificate()
	if err != nil {
		s.logger.Error("証明書の再読み込みに失敗しました", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("証明書の再読み込みに失敗しました: %w", err)
	}
	
	// Update server TLS config
	s.server.TLSConfig.Certificates = []tls.Certificate{newCert}
	
	s.logger.Info("証明書の再読み込みが完了しました")
	return nil
}

// GetCertificateStatus returns current certificate status
func (s *Server) GetCertificateStatus() (*cert.CertificateInfo, error) {
	return s.certManager.GetCertificateInfo()
}

// getDetailedHealthStatus returns detailed health status for ArgoCD
func (s *Server) getDetailedHealthStatus(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Timestamp:   metav1.Now().Format("2006-01-02T15:04:05Z"),
		Components:  make(map[string]interface{}),
		Version:     os.Getenv("VERSION"),
		Environment: os.Getenv("ENVIRONMENT"),
	}
	
	overallHealthy := true
	var messages []string
	
	// Kubernetes API接続チェック
	if _, err := s.client.Discovery().ServerVersion(); err != nil {
		overallHealthy = false
		messages = append(messages, "Kubernetes API接続に失敗")
		status.Components["kubernetes"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		status.Components["kubernetes"] = map[string]interface{}{
			"status": "healthy",
		}
	}
	
	// 証明書の状態チェック
	if s.certManager == nil {
		overallHealthy = false
		messages = append(messages, "証明書マネージャーが初期化されていません")
		status.Components["certificate"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  "certificate manager not initialized",
		}
	} else if certInfo, err := s.certManager.GetCertificateInfo(); err != nil {
		overallHealthy = false
		messages = append(messages, "証明書の取得に失敗")
		status.Components["certificate"] = map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		certStatus := "healthy"
		if certInfo.DaysUntilExpiry <= 7 {
			certStatus = "warning"
			messages = append(messages, fmt.Sprintf("証明書の有効期限が近づいています (%d日後)", certInfo.DaysUntilExpiry))
		} else if certInfo.DaysUntilExpiry <= 1 {
			certStatus = "critical"
			overallHealthy = false
			messages = append(messages, "証明書の有効期限が切れそうです")
		}
		
		status.Components["certificate"] = map[string]interface{}{
			"status":           certStatus,
			"subject":          certInfo.Subject,
			"not_after":        certInfo.NotAfter.Format("2006-01-02T15:04:05Z"),
			"days_until_expiry": certInfo.DaysUntilExpiry,
			"dns_names":        certInfo.DNSNames,
		}
	}
	
	// バリデーター機能チェック
	validatorStatus := s.checkValidatorHealth(ctx)
	status.Components["validator"] = validatorStatus
	if validatorStatus["status"] != "healthy" {
		overallHealthy = false
		if errorMsg, ok := validatorStatus["error"].(string); ok {
			messages = append(messages, fmt.Sprintf("バリデーター: %s", errorMsg))
		}
	}

	// メトリクス機能チェック
	metricsStatus := s.checkMetricsHealth()
	status.Components["metrics"] = metricsStatus
	if metricsStatus["status"] != "healthy" {
		// メトリクスは重要度が低いため、警告のみ
		if errorMsg, ok := metricsStatus["error"].(string); ok {
			messages = append(messages, fmt.Sprintf("メトリクス: %s", errorMsg))
		}
	}
	
	// 全体的なステータス設定
	if overallHealthy {
		status.Status = "healthy"
		status.Message = "すべてのコンポーネントが正常です"
	} else {
		status.Status = "unhealthy"
		if len(messages) > 0 {
			status.Message = fmt.Sprintf("問題が検出されました: %s", strings.Join(messages, ", "))
		} else {
			status.Message = "システムに問題があります"
		}
	}
	
	return status
}

// getReadinessStatus returns readiness status
func (s *Server) getReadinessStatus() HealthStatus {
	status := HealthStatus{
		Timestamp:  metav1.Now().Format("2006-01-02T15:04:05Z"),
		Components: make(map[string]interface{}),
	}
	
	ready := true
	var messages []string
	
	// Kubernetes API接続チェック
	if _, err := s.client.Discovery().ServerVersion(); err != nil {
		ready = false
		messages = append(messages, "Kubernetes APIに接続できません")
		status.Components["kubernetes"] = map[string]interface{}{
			"status": "not_ready",
			"error":  err.Error(),
		}
	} else {
		status.Components["kubernetes"] = map[string]interface{}{
			"status": "ready",
		}
	}
	
	// 証明書の読み込みチェック
	if s.certManager == nil {
		ready = false
		messages = append(messages, "証明書マネージャーが初期化されていません")
		status.Components["certificate"] = map[string]interface{}{
			"status": "not_ready",
			"error":  "certificate manager not initialized",
		}
	} else if _, err := s.certManager.GetCertificateInfo(); err != nil {
		ready = false
		messages = append(messages, "証明書を読み込めません")
		status.Components["certificate"] = map[string]interface{}{
			"status": "not_ready",
			"error":  err.Error(),
		}
	} else {
		status.Components["certificate"] = map[string]interface{}{
			"status": "ready",
		}
	}
	
	if ready {
		status.Status = "ready"
		status.Message = "webhookは受信準備完了です"
	} else {
		status.Status = "not_ready"
		if len(messages) > 0 {
			status.Message = fmt.Sprintf("準備未完了: %s", strings.Join(messages, ", "))
		} else {
			status.Message = "webhookは準備未完了です"
		}
	}
	
	return status
}

// getLivenessStatus returns liveness status
func (s *Server) getLivenessStatus() HealthStatus {
	return HealthStatus{
		Status:    "alive",
		Message:   "webhookプロセスは動作中です",
		Timestamp: metav1.Now().Format("2006-01-02T15:04:05Z"),
		Components: map[string]interface{}{
			"process": map[string]interface{}{
				"status": "alive",
				"pid":    os.Getpid(),
			},
		},
	}
}

// checkValidatorHealth はバリデーター機能の健康状態をチェック
func (s *Server) checkValidatorHealth(ctx context.Context) map[string]interface{} {
	if s.validator == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "validator not initialized",
		}
	}

	// 簡単なバリデーション機能テスト
	testDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "health-check-test",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { r := int32(2); return &r }(),
		},
	}

	result := s.validator.ValidateResource(ctx, "Deployment", testDeployment)
	if result.Code >= 500 {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  "validator internal error",
			"type":   "deployment-hpa-validator",
		}
	}

	return map[string]interface{}{
		"status": "healthy",
		"type":   "deployment-hpa-validator",
	}
}

// checkMetricsHealth はメトリクス機能の健康状態をチェック
func (s *Server) checkMetricsHealth() map[string]interface{} {
	// メトリクスエンドポイントの簡単なテスト
	req, err := http.NewRequest("GET", "http://localhost:8080/metrics", nil)
	if err != nil {
		return map[string]interface{}{
			"status": "warning",
			"error":  "failed to create metrics request",
		}
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"status": "warning",
			"error":  "metrics endpoint not accessible",
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"status": "warning",
			"error":  fmt.Sprintf("metrics endpoint returned status %d", resp.StatusCode),
		}
	}

	return map[string]interface{}{
		"status": "healthy",
		"endpoint": "/metrics",
	}
}

// reloadCertificate reloads the TLS certificate for the server
func (s *Server) reloadCertificate(newCert tls.Certificate) {
	s.logger.Info("TLS証明書を再読み込みしています", nil)
	
	// サーバーのTLS設定を更新
	if s.server.TLSConfig != nil {
		s.server.TLSConfig.Certificates = []tls.Certificate{newCert}
		s.logger.Info("TLS証明書の再読み込みが完了しました", nil)
		
		// 証明書情報を更新
		if info, err := s.certManager.GetCertificateInfo(); err == nil {
			s.logger.Info("新しい証明書情報", map[string]interface{}{
				"subject":             info.Subject,
				"not_after":           info.NotAfter.Format("2006-01-02 15:04:05"),
				"days_until_expiry":   info.DaysUntilExpiry,
				"dns_names":           info.DNSNames,
			})
			
			// メトリクスを更新
			metrics.UpdateCertificateExpiry(info.DaysUntilExpiry)
		}
	} else {
		s.logger.Error("TLS設定が見つかりません。証明書の再読み込みに失敗しました", nil)
	}
}

// GetCertificateInfo returns current certificate information
func (s *Server) GetCertificateInfo() (*cert.CertificateInfo, error) {
	return s.certManager.GetCertificateInfo()
}