package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LogLevel はログレベルを表す型
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogEntry は構造化ログエントリを表す構造体
type LogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       LogLevel               `json:"level"`
	Message     string                 `json:"message"`
	Component   string                 `json:"component"`
	RequestID   string                 `json:"request_id,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Namespace   string                 `json:"namespace,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Caller      string                 `json:"caller,omitempty"`
	Environment string                 `json:"environment,omitempty"`
	Version     string                 `json:"version,omitempty"`
}

// Logger は構造化ログを出力するためのロガー
type Logger struct {
	level       LogLevel
	component   string
	environment string
	version     string
	jsonFormat  bool
}

// NewLogger は新しいLoggerインスタンスを作成
func NewLogger(component string) *Logger {
	level := LogLevel(strings.ToLower(os.Getenv("LOG_LEVEL")))
	if level == "" {
		level = LogLevelInfo
	}

	jsonFormat := strings.ToLower(os.Getenv("LOG_FORMAT")) == "json"

	return &Logger{
		level:       level,
		component:   component,
		environment: os.Getenv("ENVIRONMENT"),
		version:     os.Getenv("VERSION"),
		jsonFormat:  jsonFormat,
	}
}

// shouldLog はログレベルに基づいてログ出力の可否を判定
func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		LogLevelDebug: 0,
		LogLevelInfo:  1,
		LogLevelWarn:  2,
		LogLevelError: 3,
	}

	return levels[level] >= levels[l.level]
}

// getCaller は呼び出し元の情報を取得
func (l *Logger) getCaller() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown"
	}
	
	// ファイルパスを短縮
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}
	
	return fmt.Sprintf("%s:%d", file, line)
}

// log は実際のログ出力を行う
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := LogEntry{
		Timestamp:   time.Now().UTC(),
		Level:       level,
		Message:     message,
		Component:   l.component,
		Environment: l.environment,
		Version:     l.version,
		Fields:      fields,
	}

	// 呼び出し元情報を追加（デバッグレベルの場合）
	if level == LogLevelDebug {
		entry.Caller = l.getCaller()
	}

	if l.jsonFormat {
		// JSON形式で出力
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Printf("ログのJSON変換に失敗しました: %v", err)
			return
		}
		fmt.Println(string(jsonBytes))
	} else {
		// 従来の形式で出力
		timestamp := entry.Timestamp.Format("2006-01-02T15:04:05.000Z")
		fmt.Printf("[%s] %s [%s] %s", timestamp, strings.ToUpper(string(level)), l.component, message)
		
		if len(fields) > 0 {
			fmt.Printf(" fields=%+v", fields)
		}
		fmt.Println()
	}
}

// Debug はデバッグレベルのログを出力
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelDebug, message, f)
}

// Info は情報レベルのログを出力
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelInfo, message, f)
}

// Warn は警告レベルのログを出力
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelWarn, message, f)
}

// Error はエラーレベルのログを出力
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(LogLevelError, message, f)
}

// WithRequestID はリクエストIDを含むロガーを作成
func (l *Logger) WithRequestID(requestID string) *RequestLogger {
	return &RequestLogger{
		logger:    l,
		requestID: requestID,
	}
}

// WithFields はフィールドを含むロガーを作成
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// RequestLogger はリクエストIDを含むロガー
type RequestLogger struct {
	logger    *Logger
	requestID string
}

// Debug はリクエストIDを含むデバッグログを出力
func (rl *RequestLogger) Debug(message string, fields ...map[string]interface{}) {
	f := make(map[string]interface{})
	if len(fields) > 0 {
		for k, v := range fields[0] {
			f[k] = v
		}
	}
	f["request_id"] = rl.requestID
	rl.logger.log(LogLevelDebug, message, f)
}

// Info はリクエストIDを含む情報ログを出力
func (rl *RequestLogger) Info(message string, fields ...map[string]interface{}) {
	f := make(map[string]interface{})
	if len(fields) > 0 {
		for k, v := range fields[0] {
			f[k] = v
		}
	}
	f["request_id"] = rl.requestID
	rl.logger.log(LogLevelInfo, message, f)
}

// Warn はリクエストIDを含む警告ログを出力
func (rl *RequestLogger) Warn(message string, fields ...map[string]interface{}) {
	f := make(map[string]interface{})
	if len(fields) > 0 {
		for k, v := range fields[0] {
			f[k] = v
		}
	}
	f["request_id"] = rl.requestID
	rl.logger.log(LogLevelWarn, message, f)
}

// Error はリクエストIDを含むエラーログを出力
func (rl *RequestLogger) Error(message string, fields ...map[string]interface{}) {
	f := make(map[string]interface{})
	if len(fields) > 0 {
		for k, v := range fields[0] {
			f[k] = v
		}
	}
	f["request_id"] = rl.requestID
	rl.logger.log(LogLevelError, message, f)
}

// FieldLogger はフィールドを含むロガー
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug はフィールドを含むデバッグログを出力
func (fl *FieldLogger) Debug(message string, additionalFields ...map[string]interface{}) {
	f := make(map[string]interface{})
	for k, v := range fl.fields {
		f[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			f[k] = v
		}
	}
	fl.logger.log(LogLevelDebug, message, f)
}

// Info はフィールドを含む情報ログを出力
func (fl *FieldLogger) Info(message string, additionalFields ...map[string]interface{}) {
	f := make(map[string]interface{})
	for k, v := range fl.fields {
		f[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			f[k] = v
		}
	}
	fl.logger.log(LogLevelInfo, message, f)
}

// Warn はフィールドを含む警告ログを出力
func (fl *FieldLogger) Warn(message string, additionalFields ...map[string]interface{}) {
	f := make(map[string]interface{})
	for k, v := range fl.fields {
		f[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			f[k] = v
		}
	}
	fl.logger.log(LogLevelWarn, message, f)
}

// Error はフィールドを含むエラーログを出力
func (fl *FieldLogger) Error(message string, additionalFields ...map[string]interface{}) {
	f := make(map[string]interface{})
	for k, v := range fl.fields {
		f[k] = v
	}
	if len(additionalFields) > 0 {
		for k, v := range additionalFields[0] {
			f[k] = v
		}
	}
	fl.logger.log(LogLevelError, message, f)
}

// GenerateRequestID は新しいリクエストIDを生成
func GenerateRequestID() string {
	return uuid.New().String()
}

// RequestIDFromContext はコンテキストからリクエストIDを取得
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

// ContextWithRequestID はリクエストIDをコンテキストに設定
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}