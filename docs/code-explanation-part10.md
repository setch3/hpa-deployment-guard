### 8.2 リクエストメトリクス

```go
// RequestMetrics はリクエストのメトリクスを記録する構造体
type RequestMetrics struct {
    Method       string
    ResourceType string
    StartTime    time.Time
}

// NewRequestMetrics は新しいRequestMetricsインスタンスを作成
func NewRequestMetrics(method, resourceType string) *RequestMetrics {
    return &RequestMetrics{
        Method:       method,
        ResourceType: resourceType,
        StartTime:    time.Now(),
    }
}

// RecordSuccess は成功したリクエストのメトリクスを記録
func (rm *RequestMetrics) RecordSuccess() {
    duration := time.Since(rm.StartTime).Seconds()
    
    WebhookRequestsTotal.WithLabelValues(rm.Method, "success", rm.ResourceType).Inc()
    WebhookRequestDuration.WithLabelValues(rm.Method, rm.ResourceType).Observe(duration)
}

// RecordError はエラーが発生したリクエストのメトリクスを記録
func (rm *RequestMetrics) RecordError(errorType string) {
    duration := time.Since(rm.StartTime).Seconds()
    
    WebhookRequestsTotal.WithLabelValues(rm.Method, "error", rm.ResourceType).Inc()
    WebhookRequestDuration.WithLabelValues(rm.Method, rm.ResourceType).Observe(duration)
    WebhookValidationErrors.WithLabelValues(errorType, rm.ResourceType).Inc()
}
```

**解説：**
- `RequestMetrics` 構造体はリクエストのメトリクス情報を保持
- `NewRequestMetrics` 関数でメトリクス記録を開始（開始時間を記録）
- `RecordSuccess` メソッドで成功したリクエストのメトリクスを記録
  - リクエスト数をインクリメント
  - 処理時間を記録
- `RecordError` メソッドでエラーが発生したリクエストのメトリクスを記録
  - リクエスト数をインクリメント
  - 処理時間を記録
  - エラー数をインクリメント

## 9. ロギング (internal/logging/logger.go)

### 9.1 ロガー構造体

```go
// Logger はログ出力を行う構造体
type Logger struct {
    component string
    level     LogLevel
    format    string
}

// NewLogger は新しいLoggerインスタンスを作成
func NewLogger(component string) *Logger {
    level := LogLevel(strings.ToLower(os.Getenv("LOG_LEVEL")))
    if level == "" {
        level = LogLevelInfo
    }
    
    format := strings.ToLower(os.Getenv("LOG_FORMAT"))
    if format != "json" && format != "text" {
        format = "json"
    }
    
    return &Logger{
        component: component,
        level:     level,
        format:    format,
    }
}
```

**解説：**
- `Logger` 構造体はログ出力の設定を保持
- `NewLogger` 関数で新しいロガーを作成
  - 環境変数からログレベルとフォーマットを取得
  - デフォルト値：レベル=info、フォーマット=json

### 9.2 ログ出力メソッド

```go
// Debug はデバッグレベルのログを出力
func (l *Logger) Debug(message string, fields map[string]interface{}) {
    if l.shouldLog(LogLevelDebug) {
        l.log(LogLevelDebug, message, fields)
    }
}

// Info は情報レベルのログを出力
func (l *Logger) Info(message string, fields map[string]interface{}) {
    if l.shouldLog(LogLevelInfo) {
        l.log(LogLevelInfo, message, fields)
    }
}

// Warn は警告レベルのログを出力
func (l *Logger) Warn(message string, fields map[string]interface{}) {
    if l.shouldLog(LogLevelWarn) {
        l.log(LogLevelWarn, message, fields)
    }
}

// Error はエラーレベルのログを出力
func (l *Logger) Error(message string, fields map[string]interface{}) {
    if l.shouldLog(LogLevelError) {
        l.log(LogLevelError, message, fields)
    }
}
```

**解説：**
- 各ログレベル（Debug, Info, Warn, Error）に対応するメソッドを提供
- `shouldLog` メソッドで現在のログレベルに基づいて出力するかどうかを判断
- `log` メソッドで実際のログ出力を行う