package logging

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("デフォルト設定でのロガー作成", func(t *testing.T) {
		logger := NewLogger("test-component")
		
		if logger.component != "test-component" {
			t.Errorf("期待値: test-component, 実際の値: %s", logger.component)
		}
		
		if logger.level != LogLevelInfo {
			t.Errorf("期待値: %s, 実際の値: %s", LogLevelInfo, logger.level)
		}
	})

	t.Run("環境変数でのログレベル設定", func(t *testing.T) {
		os.Setenv("LOG_LEVEL", "debug")
		defer os.Unsetenv("LOG_LEVEL")
		
		logger := NewLogger("test-component")
		
		if logger.level != LogLevelDebug {
			t.Errorf("期待値: %s, 実際の値: %s", LogLevelDebug, logger.level)
		}
	})

	t.Run("JSON形式の設定", func(t *testing.T) {
		os.Setenv("LOG_FORMAT", "json")
		defer os.Unsetenv("LOG_FORMAT")
		
		logger := NewLogger("test-component")
		
		if !logger.jsonFormat {
			t.Error("JSON形式が有効になっていません")
		}
	})
}

func TestLogLevels(t *testing.T) {
	logger := NewLogger("test")
	
	testCases := []struct {
		name          string
		loggerLevel   LogLevel
		messageLevel  LogLevel
		shouldLog     bool
	}{
		{"Debug logger, Debug message", LogLevelDebug, LogLevelDebug, true},
		{"Debug logger, Info message", LogLevelDebug, LogLevelInfo, true},
		{"Info logger, Debug message", LogLevelInfo, LogLevelDebug, false},
		{"Info logger, Info message", LogLevelInfo, LogLevelInfo, true},
		{"Warn logger, Info message", LogLevelWarn, LogLevelInfo, false},
		{"Error logger, Warn message", LogLevelError, LogLevelWarn, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger.level = tc.loggerLevel
			result := logger.shouldLog(tc.messageLevel)
			
			if result != tc.shouldLog {
				t.Errorf("期待値: %v, 実際の値: %v", tc.shouldLog, result)
			}
		})
	}
}

func TestWithRequestID(t *testing.T) {
	logger := NewLogger("test")
	requestID := "test-request-id"
	
	requestLogger := logger.WithRequestID(requestID)
	
	if requestLogger.requestID != requestID {
		t.Errorf("期待値: %s, 実際の値: %s", requestID, requestLogger.requestID)
	}
}

func TestWithFields(t *testing.T) {
	logger := NewLogger("test")
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}
	
	fieldLogger := logger.WithFields(fields)
	
	if len(fieldLogger.fields) != 2 {
		t.Errorf("期待値: 2, 実際の値: %d", len(fieldLogger.fields))
	}
	
	if fieldLogger.fields["key1"] != "value1" {
		t.Errorf("期待値: value1, 実際の値: %v", fieldLogger.fields["key1"])
	}
}

func TestGenerateRequestID(t *testing.T) {
	requestID1 := GenerateRequestID()
	requestID2 := GenerateRequestID()
	
	if requestID1 == requestID2 {
		t.Error("リクエストIDが重複しています")
	}
	
	if len(requestID1) == 0 {
		t.Error("リクエストIDが空です")
	}
	
	// UUID形式の確認（簡易的）
	if !strings.Contains(requestID1, "-") {
		t.Error("リクエストIDがUUID形式ではありません")
	}
}

func TestContextWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id"
	
	ctxWithID := ContextWithRequestID(ctx, requestID)
	retrievedID := RequestIDFromContext(ctxWithID)
	
	if retrievedID != requestID {
		t.Errorf("期待値: %s, 実際の値: %s", requestID, retrievedID)
	}
}

func TestRequestIDFromContext(t *testing.T) {
	t.Run("リクエストIDが存在する場合", func(t *testing.T) {
		ctx := context.Background()
		requestID := "test-request-id"
		
		ctxWithID := ContextWithRequestID(ctx, requestID)
		retrievedID := RequestIDFromContext(ctxWithID)
		
		if retrievedID != requestID {
			t.Errorf("期待値: %s, 実際の値: %s", requestID, retrievedID)
		}
	})

	t.Run("リクエストIDが存在しない場合", func(t *testing.T) {
		ctx := context.Background()
		retrievedID := RequestIDFromContext(ctx)
		
		if retrievedID != "" {
			t.Errorf("期待値: 空文字, 実際の値: %s", retrievedID)
		}
	})
}

func TestGetCaller(t *testing.T) {
	logger := NewLogger("test")
	caller := logger.getCaller()
	
	if caller == "unknown" {
		t.Error("呼び出し元情報を取得できませんでした")
	}
	
	if !strings.Contains(caller, ":") {
		t.Error("呼び出し元情報にファイル名と行番号が含まれていません")
	}
}

// ベンチマークテスト
func BenchmarkLogInfo(b *testing.B) {
	logger := NewLogger("benchmark")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("ベンチマークテストメッセージ")
	}
}

func BenchmarkLogInfoWithFields(b *testing.B) {
	logger := NewLogger("benchmark")
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("ベンチマークテストメッセージ", fields)
	}
}

func BenchmarkRequestLogger(b *testing.B) {
	logger := NewLogger("benchmark")
	requestLogger := logger.WithRequestID("test-request-id")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestLogger.Info("ベンチマークテストメッセージ")
	}
}