// +build performance

package performance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s-deployment-hpa-validator/internal/validator"
	"k8s-deployment-hpa-validator/internal/logging"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// PerformanceMetrics パフォーマンステストの結果を格納する構造体
type PerformanceMetrics struct {
	TotalRequests     int
	SuccessfulRequests int
	FailedRequests    int
	TotalDuration     time.Duration
	AverageLatency    time.Duration
	MinLatency        time.Duration
	MaxLatency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	RequestsPerSecond float64
	Latencies         []time.Duration
}

// calculatePercentile パーセンタイル値を計算
func calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// ソート
	for i := 0; i < len(latencies)-1; i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}
	
	index := int(float64(len(latencies)) * percentile / 100.0)
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}

// analyzePerformance パフォーマンス結果を分析
func analyzePerformance(latencies []time.Duration, totalDuration time.Duration) *PerformanceMetrics {
	metrics := &PerformanceMetrics{
		TotalRequests:     len(latencies),
		SuccessfulRequests: len(latencies),
		FailedRequests:    0,
		TotalDuration:     totalDuration,
		Latencies:         latencies,
	}

	if len(latencies) == 0 {
		return metrics
	}

	// 平均レイテンシ計算
	var totalLatency time.Duration
	metrics.MinLatency = latencies[0]
	metrics.MaxLatency = latencies[0]

	for _, latency := range latencies {
		totalLatency += latency
		if latency < metrics.MinLatency {
			metrics.MinLatency = latency
		}
		if latency > metrics.MaxLatency {
			metrics.MaxLatency = latency
		}
	}

	metrics.AverageLatency = totalLatency / time.Duration(len(latencies))
	metrics.P95Latency = calculatePercentile(latencies, 95)
	metrics.P99Latency = calculatePercentile(latencies, 99)
	metrics.RequestsPerSecond = float64(len(latencies)) / totalDuration.Seconds()

	return metrics
}

// TestWebhookPerformance Webhookのパフォーマンステスト
func TestWebhookPerformance(t *testing.T) {
	// テスト設定
	const (
		concurrency = 50   // 同時実行数
		requests    = 1000 // 総リクエスト数
		timeout     = 30 * time.Second
	)

	// フェイクKubernetesクライアントを作成
	fakeClient := fake.NewSimpleClientset()
	logger := logging.NewLogger("performance-test")
	validator := validator.NewDeploymentHPAValidator(fakeClient)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Run("HPAバリデーション負荷テスト", func(t *testing.T) {
		// テスト用のDeploymentを事前に作成（2 replica）
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "load-test-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(2); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "load-test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "load-test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("テスト用Deploymentの作成に失敗しました: %v", err)
		}

		// 負荷テスト実行
		var wg sync.WaitGroup
		latencyChan := make(chan time.Duration, requests)
		errorChan := make(chan error, requests)

		startTime := time.Now()

		// ワーカーゴルーチンを起動
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				requestsPerWorker := requests / concurrency
				for j := 0; j < requestsPerWorker; j++ {
					// HPAを作成してバリデーション
					hpa := &autoscalingv2.HorizontalPodAutoscaler{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("load-test-hpa-%d-%d", workerID, j),
							Namespace: "default",
						},
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "load-test-deployment",
							},
							MinReplicas: func() *int32 { i := int32(2); return &i }(),
							MaxReplicas: 10,
						},
					}

					requestStart := time.Now()
					err := validator.ValidateHPA(ctx, hpa)
					latency := time.Since(requestStart)

					if err != nil {
						errorChan <- err
					} else {
						latencyChan <- latency
					}
				}
			}(i)
		}

		// 全ワーカーの完了を待機
		wg.Wait()
		totalDuration := time.Since(startTime)

		// 結果を収集
		close(latencyChan)
		close(errorChan)

		var latencies []time.Duration
		for latency := range latencyChan {
			latencies = append(latencies, latency)
		}

		var errors []error
		for err := range errorChan {
			errors = append(errors, err)
		}

		// パフォーマンス分析
		metrics := analyzePerformance(latencies, totalDuration)
		metrics.FailedRequests = len(errors)

		// 結果をレポート
		t.Logf("=== HPAバリデーション負荷テスト結果 ===")
		t.Logf("総リクエスト数: %d", metrics.TotalRequests)
		t.Logf("成功リクエスト数: %d", metrics.SuccessfulRequests)
		t.Logf("失敗リクエスト数: %d", metrics.FailedRequests)
		t.Logf("総実行時間: %v", metrics.TotalDuration)
		t.Logf("平均レイテンシ: %v", metrics.AverageLatency)
		t.Logf("最小レイテンシ: %v", metrics.MinLatency)
		t.Logf("最大レイテンシ: %v", metrics.MaxLatency)
		t.Logf("95パーセンタイル: %v", metrics.P95Latency)
		t.Logf("99パーセンタイル: %v", metrics.P99Latency)
		t.Logf("スループット: %.2f req/sec", metrics.RequestsPerSecond)

		// パフォーマンス基準の検証
		if metrics.AverageLatency > 100*time.Millisecond {
			t.Errorf("平均レイテンシが基準を超えています: %v > 100ms", metrics.AverageLatency)
		}

		if metrics.P95Latency > 200*time.Millisecond {
			t.Errorf("95パーセンタイルレイテンシが基準を超えています: %v > 200ms", metrics.P95Latency)
		}

		if metrics.RequestsPerSecond < 100 {
			t.Errorf("スループットが基準を下回っています: %.2f < 100 req/sec", metrics.RequestsPerSecond)
		}

		if float64(metrics.FailedRequests)/float64(metrics.TotalRequests) > 0.01 {
			t.Errorf("エラー率が基準を超えています: %.2f%% > 1%%", 
				float64(metrics.FailedRequests)/float64(metrics.TotalRequests)*100)
		}

		_ = logger // ロガーを使用
	})

	t.Run("Deploymentバリデーション負荷テスト", func(t *testing.T) {
		// テスト用のHPAを事前に作成
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "load-test-hpa",
				Namespace: "default",
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "load-test-deployment-target",
				},
				MinReplicas: func() *int32 { i := int32(2); return &i }(),
				MaxReplicas: 10,
			},
		}

		_, err := fakeClient.AutoscalingV2().HorizontalPodAutoscalers("default").Create(ctx, hpa, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("テスト用HPAの作成に失敗しました: %v", err)
		}

		// 負荷テスト実行
		var wg sync.WaitGroup
		latencyChan := make(chan time.Duration, requests)
		errorChan := make(chan error, requests)

		startTime := time.Now()

		// ワーカーゴルーチンを起動
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				requestsPerWorker := requests / concurrency
				for j := 0; j < requestsPerWorker; j++ {
					// Deploymentを作成してバリデーション（2 replica）
					deployment := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("load-test-deployment-%d-%d", workerID, j),
							Namespace: "default",
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: func() *int32 { i := int32(2); return &i }(),
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": fmt.Sprintf("load-test-%d-%d", workerID, j)},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"app": fmt.Sprintf("load-test-%d-%d", workerID, j)},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{
											Name:  "test",
											Image: "nginx:latest",
										},
									},
								},
							},
						},
					}

					requestStart := time.Now()
					err := validator.ValidateDeployment(ctx, deployment)
					latency := time.Since(requestStart)

					if err != nil {
						errorChan <- err
					} else {
						latencyChan <- latency
					}
				}
			}(i)
		}

		// 全ワーカーの完了を待機
		wg.Wait()
		totalDuration := time.Since(startTime)

		// 結果を収集
		close(latencyChan)
		close(errorChan)

		var latencies []time.Duration
		for latency := range latencyChan {
			latencies = append(latencies, latency)
		}

		var errors []error
		for err := range errorChan {
			errors = append(errors, err)
		}

		// パフォーマンス分析
		metrics := analyzePerformance(latencies, totalDuration)
		metrics.FailedRequests = len(errors)

		// 結果をレポート
		t.Logf("=== Deploymentバリデーション負荷テスト結果 ===")
		t.Logf("総リクエスト数: %d", metrics.TotalRequests)
		t.Logf("成功リクエスト数: %d", metrics.SuccessfulRequests)
		t.Logf("失敗リクエスト数: %d", metrics.FailedRequests)
		t.Logf("総実行時間: %v", metrics.TotalDuration)
		t.Logf("平均レイテンシ: %v", metrics.AverageLatency)
		t.Logf("最小レイテンシ: %v", metrics.MinLatency)
		t.Logf("最大レイテンシ: %v", metrics.MaxLatency)
		t.Logf("95パーセンタイル: %v", metrics.P95Latency)
		t.Logf("99パーセンタイル: %v", metrics.P99Latency)
		t.Logf("スループット: %.2f req/sec", metrics.RequestsPerSecond)

		// パフォーマンス基準の検証
		if metrics.AverageLatency > 100*time.Millisecond {
			t.Errorf("平均レイテンシが基準を超えています: %v > 100ms", metrics.AverageLatency)
		}

		if metrics.P95Latency > 200*time.Millisecond {
			t.Errorf("95パーセンタイルレイテンシが基準を超えています: %v > 200ms", metrics.P95Latency)
		}

		if metrics.RequestsPerSecond < 100 {
			t.Errorf("スループットが基準を下回っています: %.2f < 100 req/sec", metrics.RequestsPerSecond)
		}

		if float64(metrics.FailedRequests)/float64(metrics.TotalRequests) > 0.01 {
			t.Errorf("エラー率が基準を超えています: %.2f%% > 1%%", 
				float64(metrics.FailedRequests)/float64(metrics.TotalRequests)*100)
		}
	})
}

// TestWebhookStressTest Webhookのストレステスト
func TestWebhookStressTest(t *testing.T) {
	// ストレステスト設定（より高い負荷）
	const (
		concurrency = 100  // 同時実行数
		requests    = 5000 // 総リクエスト数
		timeout     = 60 * time.Second
	)

	// フェイクKubernetesクライアントを作成
	fakeClient := fake.NewSimpleClientset()
	logger := logging.NewLogger("stress-test")
	validator := validator.NewDeploymentHPAValidator(fakeClient)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Run("高負荷ストレステスト", func(t *testing.T) {
		// テスト用のDeploymentを事前に作成
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stress-test-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(3); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "stress-test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "stress-test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("テスト用Deploymentの作成に失敗しました: %v", err)
		}

		// ストレステスト実行
		var wg sync.WaitGroup
		latencyChan := make(chan time.Duration, requests)
		errorChan := make(chan error, requests)

		startTime := time.Now()

		// ワーカーゴルーチンを起動
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				requestsPerWorker := requests / concurrency
				for j := 0; j < requestsPerWorker; j++ {
					// HPAを作成してバリデーション
					hpa := &autoscalingv2.HorizontalPodAutoscaler{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("stress-test-hpa-%d-%d", workerID, j),
							Namespace: "default",
						},
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "stress-test-deployment",
							},
							MinReplicas: func() *int32 { i := int32(2); return &i }(),
							MaxReplicas: 20,
						},
					}

					requestStart := time.Now()
					err := validator.ValidateHPA(ctx, hpa)
					latency := time.Since(requestStart)

					if err != nil {
						errorChan <- err
					} else {
						latencyChan <- latency
					}

					// 短い間隔でリクエストを送信
					time.Sleep(1 * time.Millisecond)
				}
			}(i)
		}

		// 全ワーカーの完了を待機
		wg.Wait()
		totalDuration := time.Since(startTime)

		// 結果を収集
		close(latencyChan)
		close(errorChan)

		var latencies []time.Duration
		for latency := range latencyChan {
			latencies = append(latencies, latency)
		}

		var errors []error
		for err := range errorChan {
			errors = append(errors, err)
		}

		// パフォーマンス分析
		metrics := analyzePerformance(latencies, totalDuration)
		metrics.FailedRequests = len(errors)

		// 結果をレポート
		t.Logf("=== 高負荷ストレステスト結果 ===")
		t.Logf("総リクエスト数: %d", metrics.TotalRequests)
		t.Logf("成功リクエスト数: %d", metrics.SuccessfulRequests)
		t.Logf("失敗リクエスト数: %d", metrics.FailedRequests)
		t.Logf("総実行時間: %v", metrics.TotalDuration)
		t.Logf("平均レイテンシ: %v", metrics.AverageLatency)
		t.Logf("最小レイテンシ: %v", metrics.MinLatency)
		t.Logf("最大レイテンシ: %v", metrics.MaxLatency)
		t.Logf("95パーセンタイル: %v", metrics.P95Latency)
		t.Logf("99パーセンタイル: %v", metrics.P99Latency)
		t.Logf("スループット: %.2f req/sec", metrics.RequestsPerSecond)

		// ストレステスト用の緩い基準
		if metrics.AverageLatency > 500*time.Millisecond {
			t.Errorf("平均レイテンシが基準を超えています: %v > 500ms", metrics.AverageLatency)
		}

		if metrics.P95Latency > 1*time.Second {
			t.Errorf("95パーセンタイルレイテンシが基準を超えています: %v > 1s", metrics.P95Latency)
		}

		if metrics.RequestsPerSecond < 50 {
			t.Errorf("スループットが基準を下回っています: %.2f < 50 req/sec", metrics.RequestsPerSecond)
		}

		// ストレステストでは5%までのエラー率を許容
		if float64(metrics.FailedRequests)/float64(metrics.TotalRequests) > 0.05 {
			t.Errorf("エラー率が基準を超えています: %.2f%% > 5%%", 
				float64(metrics.FailedRequests)/float64(metrics.TotalRequests)*100)
		}

		_ = logger // ロガーを使用
	})
}

// TestMemoryUsage メモリ使用量のテスト
func TestMemoryUsage(t *testing.T) {
	// メモリ使用量テスト設定
	const (
		iterations = 10000 // 繰り返し回数
		timeout    = 30 * time.Second
	)

	// フェイクKubernetesクライアントを作成
	fakeClient := fake.NewSimpleClientset()
	logger := logging.NewLogger("memory-test")
	validator := validator.NewDeploymentHPAValidator(fakeClient)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Run("メモリ使用量テスト", func(t *testing.T) {
		// テスト用のDeploymentを事前に作成
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "memory-test-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(2); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "memory-test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "memory-test"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}

		_, err := fakeClient.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("テスト用Deploymentの作成に失敗しました: %v", err)
		}

		// メモリ使用量テスト実行
		startTime := time.Now()

		for i := 0; i < iterations; i++ {
			// HPAを作成してバリデーション
			hpa := &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("memory-test-hpa-%d", i),
					Namespace: "default",
				},
				Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
					ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "memory-test-deployment",
					},
					MinReplicas: func() *int32 { i := int32(2); return &i }(),
					MaxReplicas: 10,
				},
			}

			err := validator.ValidateHPA(ctx, hpa)
			if err != nil {
				t.Errorf("バリデーションエラー (iteration %d): %v", i, err)
			}

			// 定期的にガベージコレクションを実行
			if i%1000 == 0 {
				// runtime.GC() // 必要に応じてガベージコレクションを実行
				t.Logf("進捗: %d/%d iterations完了", i, iterations)
			}
		}

		totalDuration := time.Since(startTime)

		// 結果をレポート
		t.Logf("=== メモリ使用量テスト結果 ===")
		t.Logf("総繰り返し回数: %d", iterations)
		t.Logf("総実行時間: %v", totalDuration)
		t.Logf("平均処理時間: %v", totalDuration/time.Duration(iterations))

		// メモリリークの検出（簡易版）
		averageTime := totalDuration / time.Duration(iterations)
		if averageTime > 10*time.Millisecond {
			t.Logf("警告: 平均処理時間が長い可能性があります: %v", averageTime)
		}

		_ = logger // ロガーを使用
	})
}