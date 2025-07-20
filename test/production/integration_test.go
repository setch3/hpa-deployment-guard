// +build production,integration

package production

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s-deployment-hpa-validator/internal/config"
	"k8s-deployment-hpa-validator/internal/validator"
	"k8s-deployment-hpa-validator/internal/logging"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestProductionIntegration 本番環境設定での統合テスト
func TestProductionIntegration(t *testing.T) {
	// 本番環境設定を読み込み
	configPath := filepath.Join("..", "..", "configs", "production.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("本番環境設定ファイルが見つかりません: %s", configPath)
	}

	loader := config.NewConfigLoaderWithFile(configPath)
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("本番環境設定の読み込みに失敗しました: %v", err)
	}

	t.Run("本番環境設定でのコンポーネント初期化", func(t *testing.T) {
		// ロガーの初期化
		logger := logging.NewLogger("test")

		// バリデーターの初期化
		fakeClient := fake.NewSimpleClientset()
		validator := validator.NewDeploymentHPAValidator(fakeClient)
		if validator == nil {
			t.Fatal("バリデーターの初期化に失敗しました")
		}

		// 設定の検証のみ実行
		if cfg == nil {
			t.Fatal("設定が初期化されませんでした")
		}

		// 設定が正しく適用されているか確認
		summary := cfg.GetConfigSummary()
		if summary["environment"] != "production" {
			t.Errorf("サーバー設定の環境が正しくありません: %v", summary["environment"])
		}

		t.Logf("本番環境設定でのコンポーネント初期化が成功しました")
		_ = logger // ロガーを使用
	})

	t.Run("本番環境設定でのバリデーション動作", func(t *testing.T) {
		// フェイクKubernetesクライアントを作成
		fakeClient := fake.NewSimpleClientset()
		
		// ロガーを初期化
		logger := logging.NewLogger("test")

		// バリデーターを初期化
		v := validator.NewDeploymentHPAValidator(fakeClient)

		ctx := context.Background()

		// テスト用のDeploymentを作成（1 replica）
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(1); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
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

		// Deploymentを作成
		_, err = fakeClient.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("テスト用Deploymentの作成に失敗しました: %v", err)
		}

		// テスト用のHPAを作成
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-hpa",
				Namespace: "default",
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				MinReplicas: func() *int32 { i := int32(2); return &i }(),
				MaxReplicas: 10,
			},
		}

		// HPAのバリデーション（1 replicaのDeploymentを対象とするため失敗するはず）
		err = v.ValidateHPA(ctx, hpa)
		if err == nil {
			t.Error("1 replicaのDeploymentを対象とするHPAが許可されました（拒否されるべき）")
		} else {
			t.Logf("HPAバリデーションが正しく失敗しました: %v", err)
		}

		// Deploymentを2 replicaに更新
		deployment.Spec.Replicas = func() *int32 { i := int32(2); return &i }()
		_, err = fakeClient.AppsV1().Deployments("default").Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("Deploymentの更新に失敗しました: %v", err)
		}

		// 再度HPAのバリデーション（今度は成功するはず）
		err = v.ValidateHPA(ctx, hpa)
		if err != nil {
			t.Errorf("2 replicaのDeploymentを対象とするHPAが拒否されました: %v", err)
		} else {
			t.Logf("HPAバリデーションが正常に成功しました")
		}

		_ = logger // ロガーを使用
	})

	t.Run("本番環境でのスキップ設定テスト", func(t *testing.T) {
		// 本番環境でスキップされるべきnamespaceのテスト
		skipNamespaces := []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"cert-manager",
			"monitoring",
			"istio-system",
		}

		for _, ns := range skipNamespaces {
			if !cfg.ShouldSkipNamespace(ns) {
				t.Errorf("本番環境では %s namespace がスキップされる必要があります", ns)
			}
		}

		// スキップされないnamespaceのテスト
		normalNamespaces := []string{
			"default",
			"app-namespace",
			"user-workload",
		}

		for _, ns := range normalNamespaces {
			if cfg.ShouldSkipNamespace(ns) {
				t.Errorf("%s namespace が誤ってスキップされています", ns)
			}
		}

		// スキップラベルのテスト
		skipLabels := map[string]string{
			"k8s-deployment-hpa-validator.io/skip-validation": "true",
		}

		if !cfg.ShouldSkipByLabel(skipLabels) {
			t.Error("スキップラベルが正しく認識されませんでした")
		}

		// 通常のラベルのテスト
		normalLabels := map[string]string{
			"app": "test-app",
			"version": "v1.0.0",
		}

		if cfg.ShouldSkipByLabel(normalLabels) {
			t.Error("通常のラベルが誤ってスキップされました")
		}
	})

	t.Run("本番環境でのタイムアウト設定", func(t *testing.T) {
		// 本番環境では適切なタイムアウト値が設定されていることを確認
		if cfg.Timeout < 15*time.Second {
			t.Errorf("本番環境では15秒以上のタイムアウトが推奨されます。現在: %v", cfg.Timeout)
		}

		// タイムアウトが長すぎないことも確認（Kubernetesのデフォルトは30秒）
		if cfg.Timeout > 30*time.Second {
			t.Logf("警告: タイムアウトが長すぎる可能性があります: %v", cfg.Timeout)
		}
	})
}

// TestProductionConfigMapIntegration ConfigMapを使用した本番環境設定の統合テスト
func TestProductionConfigMapIntegration(t *testing.T) {
	// 本番環境用のConfigMapデータを模擬
	configMapData := map[string]string{
		"webhook.port":                "8443",
		"webhook.timeout":             "30",
		"webhook.failure-policy":      "Fail",
		"log.level":                   "warn",
		"log.format":                  "json",
		"validation.skip-namespaces":  "monitoring,istio-system",
		"validation.skip-labels":      "production.io/skip-validation=true",
		"metrics.enabled":             "true",
		"metrics.port":                "8080",
		"health.enabled":              "true",
		"environment":                 "production",
		"cluster.name":                "prod-cluster-01",
	}

	// ConfigMapデータを使用して設定ローダーを作成
	loader := config.NewConfigLoaderWithConfigMap(configMapData)
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("ConfigMapからの設定読み込みに失敗しました: %v", err)
	}

	t.Run("ConfigMapからの本番環境設定検証", func(t *testing.T) {
		if cfg.Environment != "production" {
			t.Errorf("期待される環境: production, 実際: %s", cfg.Environment)
		}

		if cfg.Port != 8443 {
			t.Errorf("期待されるポート: 8443, 実際: %d", cfg.Port)
		}

		if cfg.Timeout != 30*time.Second {
			t.Errorf("期待されるタイムアウト: 30s, 実際: %v", cfg.Timeout)
		}

		if cfg.FailurePolicy != "Fail" {
			t.Errorf("期待される失敗ポリシー: Fail, 実際: %s", cfg.FailurePolicy)
		}

		if cfg.LogLevel != "warn" {
			t.Errorf("期待されるログレベル: warn, 実際: %s", cfg.LogLevel)
		}

		if cfg.LogFormat != "json" {
			t.Errorf("期待されるログフォーマット: json, 実際: %s", cfg.LogFormat)
		}

		if !cfg.MetricsEnabled {
			t.Error("メトリクスが有効になっていません")
		}

		if !cfg.HealthEnabled {
			t.Error("ヘルスチェックが有効になっていません")
		}

		if cfg.ClusterName != "prod-cluster-01" {
			t.Errorf("期待されるクラスター名: prod-cluster-01, 実際: %s", cfg.ClusterName)
		}
	})

	t.Run("ConfigMapからの追加設定検証", func(t *testing.T) {
		// ConfigMapで追加されたスキップnamespaceの確認
		additionalSkipNamespaces := []string{"monitoring", "istio-system"}
		for _, ns := range additionalSkipNamespaces {
			if !cfg.ShouldSkipNamespace(ns) {
				t.Errorf("ConfigMapで追加された %s namespace がスキップされていません", ns)
			}
		}

		// ConfigMapで追加されたスキップラベルの確認
		additionalSkipLabels := map[string]string{
			"production.io/skip-validation": "true",
		}
		if !cfg.ShouldSkipByLabel(additionalSkipLabels) {
			t.Error("ConfigMapで追加されたスキップラベルが認識されませんでした")
		}
	})
}

// TestProductionEnvironmentVariableOverride 環境変数による本番環境設定の上書きテスト
func TestProductionEnvironmentVariableOverride(t *testing.T) {
	// 本番環境用のConfigMapデータを設定
	configMapData := map[string]string{
		"webhook.port":           "8443",
		"log.level":              "info",
		"environment":            "production",
		"webhook.failure-policy": "Ignore",
	}

	// 環境変数で上書き
	os.Setenv("WEBHOOK_PORT", "9443")
	os.Setenv("LOG_LEVEL", "warn")
	os.Setenv("FAILURE_POLICY", "Fail")
	defer func() {
		os.Unsetenv("WEBHOOK_PORT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("FAILURE_POLICY")
	}()

	// 設定を読み込み
	loader := config.NewConfigLoaderWithConfigMap(configMapData)
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("設定の読み込みに失敗しました: %v", err)
	}

	t.Run("環境変数による設定上書きの確認", func(t *testing.T) {
		// 環境変数がConfigMapを上書きしていることを確認
		if cfg.Port != 9443 {
			t.Errorf("環境変数によるポート上書きが失敗しました。期待: 9443, 実際: %d", cfg.Port)
		}

		if cfg.LogLevel != "warn" {
			t.Errorf("環境変数によるログレベル上書きが失敗しました。期待: warn, 実際: %s", cfg.LogLevel)
		}

		if cfg.FailurePolicy != "Fail" {
			t.Errorf("環境変数による失敗ポリシー上書きが失敗しました。期待: Fail, 実際: %s", cfg.FailurePolicy)
		}

		// ConfigMapの値が保持されていることを確認
		if cfg.Environment != "production" {
			t.Errorf("ConfigMapの環境設定が失われました。期待: production, 実際: %s", cfg.Environment)
		}
	})

	t.Run("本番環境での設定優先順位確認", func(t *testing.T) {
		// 設定の優先順位: 環境変数 > ConfigMap > YAMLファイル > デフォルト値
		summary := cfg.GetConfigSummary()
		
		// 環境変数で上書きされた値
		if summary["port"] != 9443 {
			t.Errorf("環境変数の優先順位が正しくありません: %v", summary["port"])
		}

		// ConfigMapから取得された値
		if summary["environment"] != "production" {
			t.Errorf("ConfigMapの値が正しく取得されていません: %v", summary["environment"])
		}
	})
}

// TestProductionResourceLimits 本番環境でのリソース制限テスト
func TestProductionResourceLimits(t *testing.T) {
	// 本番環境設定を読み込み
	configPath := filepath.Join("..", "..", "configs", "production.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("本番環境設定ファイルが見つかりません: %s", configPath)
	}

	loader := config.NewConfigLoaderWithFile(configPath)
	cfg, err := loader.LoadConfig()
	if err != nil {
		t.Fatalf("本番環境設定の読み込みに失敗しました: %v", err)
	}

	t.Run("本番環境でのポート設定", func(t *testing.T) {
		// 標準的なWebhookポートの使用確認
		if cfg.Port != 8443 {
			t.Logf("警告: 標準的なWebhookポート(8443)以外が使用されています: %d", cfg.Port)
		}

		// ポートの重複確認
		if cfg.Port == cfg.MetricsPort {
			t.Errorf("WebhookポートとMetricsポートが重複しています: %d", cfg.Port)
		}

		// 特権ポートの使用確認
		if cfg.Port < 1024 {
			t.Logf("警告: 特権ポート(%d)が使用されています", cfg.Port)
		}

		if cfg.MetricsPort < 1024 {
			t.Logf("警告: Metricsで特権ポート(%d)が使用されています", cfg.MetricsPort)
		}
	})

	t.Run("本番環境でのタイムアウト設定", func(t *testing.T) {
		// 適切なタイムアウト値の確認
		minTimeout := 15 * time.Second
		maxTimeout := 30 * time.Second

		if cfg.Timeout < minTimeout {
			t.Errorf("本番環境では%v以上のタイムアウトが推奨されます。現在: %v", minTimeout, cfg.Timeout)
		}

		if cfg.Timeout > maxTimeout {
			t.Logf("警告: タイムアウトが長すぎる可能性があります: %v", cfg.Timeout)
		}
	})

	t.Run("本番環境でのセキュリティ設定", func(t *testing.T) {
		// 必要なnamespaceがスキップリストに含まれているか確認
		requiredSkipNamespaces := []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
		}

		for _, ns := range requiredSkipNamespaces {
			if !cfg.ShouldSkipNamespace(ns) {
				t.Errorf("セキュリティ上重要な %s namespace がスキップリストに含まれていません", ns)
			}
		}

		// 本番環境固有のnamespaceがスキップリストに含まれているか確認
		productionSkipNamespaces := []string{
			"cert-manager",
			"monitoring",
			"istio-system",
		}

		for _, ns := range productionSkipNamespaces {
			if !cfg.ShouldSkipNamespace(ns) {
				t.Logf("情報: 本番環境では %s namespace もスキップリストに含めることを検討してください", ns)
			}
		}
	})
}