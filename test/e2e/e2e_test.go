// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	testNamespacePrefix = "hpa-validator-test"
	testTimeout         = 30 * time.Second
)

// TestClient はE2Eテスト用のKubernetesクライアント
type TestClient struct {
	clientset kubernetes.Interface
	namespace string
}

// NewTestClient は新しいテストクライアントを作成
func NewTestClient(t *testing.T) *TestClient {
	config, err := getKubeConfig()
	if err != nil {
		t.Fatalf("Kubernetesクライアントの設定取得に失敗: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Kubernetesクライアントの作成に失敗: %v", err)
	}

	// テスト名に基づいてユニークなnamespaceを生成
	testName := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	testName = strings.ReplaceAll(testName, "_", "-")
	namespace := fmt.Sprintf("%s-%s", testNamespacePrefix, testName)
	
	// namespace名が長すぎる場合は短縮
	if len(namespace) > 63 {
		namespace = namespace[:63]
	}

	return &TestClient{
		clientset: clientset,
		namespace: namespace,
	}
}

// getKubeConfig はKubernetes設定を取得
func getKubeConfig() (*rest.Config, error) {
	// まずクラスター内設定を試行
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// kubeconfigファイルから設定を読み込み
	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

// setupNamespace はテスト用のnamespaceを作成
func (tc *TestClient) setupNamespace(t *testing.T) {
	ctx := context.Background()
	
	// 既存のnamespaceを確認
	_, err := tc.clientset.CoreV1().Namespaces().Get(ctx, tc.namespace, metav1.GetOptions{})
	if err == nil {
		// namespaceが存在する場合は削除
		t.Logf("既存のnamespace '%s' を削除します", tc.namespace)
		err = tc.clientset.CoreV1().Namespaces().Delete(ctx, tc.namespace, metav1.DeleteOptions{
			PropagationPolicy: func() *metav1.DeletionPropagation {
				p := metav1.DeletePropagationForeground
				return &p
			}(),
		})
		if err != nil {
			t.Logf("namespace削除中にエラー: %v", err)
		}
		
		// 削除完了を待機
		for i := 0; i < 30; i++ {
			_, err := tc.clientset.CoreV1().Namespaces().Get(ctx, tc.namespace, metav1.GetOptions{})
			if err != nil && strings.Contains(err.Error(), "not found") {
				t.Logf("namespace '%s' の削除が完了しました", tc.namespace)
				time.Sleep(2 * time.Second) // 安全のため少し待機
				break
			}
			t.Logf("namespace '%s' の削除を待機中... (%d/30)", tc.namespace, i+1)
			time.Sleep(1 * time.Second)
		}
	}

	// 最終確認
	_, err = tc.clientset.CoreV1().Namespaces().Get(ctx, tc.namespace, metav1.GetOptions{})
	if err == nil {
		t.Fatalf("namespace '%s' がまだ存在しています。テストを続行できません", tc.namespace)
	}

	// 新しいnamespaceを作成
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tc.namespace,
		},
	}
	
	_, err = tc.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("テスト用namespace作成に失敗: %v", err)
	}
	
	t.Logf("テスト用namespace '%s' を作成しました", tc.namespace)
}

// cleanupNamespace はテスト用のnamespaceを削除
func (tc *TestClient) cleanupNamespace(t *testing.T) {
	ctx := context.Background()
	err := tc.clientset.CoreV1().Namespaces().Delete(ctx, tc.namespace, metav1.DeleteOptions{})
	if err != nil {
		t.Logf("テスト用namespace削除に失敗: %v", err)
	} else {
		t.Logf("テスト用namespace '%s' を削除しました", tc.namespace)
	}
}

// createDeployment はテスト用のDeploymentを作成
func (tc *TestClient) createDeployment(name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tc.namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
}

// createHPA はテスト用のHPAを作成
func (tc *TestClient) createHPA(name, targetDeployment string) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tc.namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       targetDeployment,
			},
			MinReplicas: func() *int32 { i := int32(2); return &i }(),
			MaxReplicas: 10,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: func() *int32 { i := int32(80); return &i }(),
						},
					},
				},
			},
		},
	}
}

// deployResource はリソースをデプロイし、エラーをチェック
func (tc *TestClient) deployResource(ctx context.Context, resource interface{}, expectError bool) error {
	switch r := resource.(type) {
	case *appsv1.Deployment:
		_, err := tc.clientset.AppsV1().Deployments(tc.namespace).Create(ctx, r, metav1.CreateOptions{})
		return err
	case *autoscalingv2.HorizontalPodAutoscaler:
		_, err := tc.clientset.AutoscalingV2().HorizontalPodAutoscalers(tc.namespace).Create(ctx, r, metav1.CreateOptions{})
		return err
	default:
		return fmt.Errorf("サポートされていないリソースタイプ: %T", resource)
	}
}

// waitForResource はリソースが作成されるまで待機
func (tc *TestClient) waitForResource(ctx context.Context, resourceType, name string) error {
	timeout := time.After(testTimeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("リソース %s/%s の作成がタイムアウトしました", resourceType, name)
		case <-ticker.C:
			switch resourceType {
			case "deployment":
				_, err := tc.clientset.AppsV1().Deployments(tc.namespace).Get(ctx, name, metav1.GetOptions{})
				if err == nil {
					return nil
				}
			case "hpa":
				_, err := tc.clientset.AutoscalingV2().HorizontalPodAutoscalers(tc.namespace).Get(ctx, name, metav1.GetOptions{})
				if err == nil {
					return nil
				}
			}
		}
	}
}

// TestValidDeploymentWithHPA は正常ケース（2+ replica + HPA）のテスト
func TestValidDeploymentWithHPA(t *testing.T) {
	tc := NewTestClient(t)
	tc.setupNamespace(t)
	defer tc.cleanupNamespace(t)

	ctx := context.Background()

	t.Run("2 replicaのDeploymentとHPAの正常作成", func(t *testing.T) {
		// 2 replicaのDeploymentを作成
		deployment := tc.createDeployment("valid-deployment", 2)
		err := tc.deployResource(ctx, deployment, false)
		if err != nil {
			t.Fatalf("2 replicaのDeployment作成に失敗: %v", err)
		}
		t.Logf("2 replicaのDeploymentが正常に作成されました")

		// Deploymentの作成を待機
		err = tc.waitForResource(ctx, "deployment", "valid-deployment")
		if err != nil {
			t.Fatalf("Deployment作成の待機に失敗: %v", err)
		}

		// HPAを作成（エラーが発生しないことを確認）
		hpa := tc.createHPA("valid-hpa", "valid-deployment")
		err = tc.deployResource(ctx, hpa, false)
		if err != nil {
			t.Fatalf("2 replicaのDeploymentに対するHPA作成に失敗: %v", err)
		}
		t.Logf("2 replicaのDeploymentに対するHPAが正常に作成されました")
	})

	t.Run("3 replicaのDeploymentとHPAの正常作成", func(t *testing.T) {
		// 3 replicaのDeploymentを作成
		deployment := tc.createDeployment("valid-deployment-3", 3)
		err := tc.deployResource(ctx, deployment, false)
		if err != nil {
			t.Fatalf("3 replicaのDeployment作成に失敗: %v", err)
		}

		// HPAを作成（エラーが発生しないことを確認）
		hpa := tc.createHPA("valid-hpa-3", "valid-deployment-3")
		err = tc.deployResource(ctx, hpa, false)
		if err != nil {
			t.Fatalf("3 replicaのDeploymentに対するHPA作成に失敗: %v", err)
		}
		t.Logf("3 replicaのDeploymentに対するHPAが正常に作成されました")
	})
}

// TestInvalidDeploymentWithHPA は異常ケース（1 replica + HPA）のテスト
func TestInvalidDeploymentWithHPA(t *testing.T) {
	tc := NewTestClient(t)
	tc.setupNamespace(t)
	defer tc.cleanupNamespace(t)

	ctx := context.Background()

	t.Run("1 replicaのDeploymentにHPAを追加（拒否されることを確認）", func(t *testing.T) {
		// 1 replicaのDeploymentを作成
		deployment := tc.createDeployment("invalid-deployment", 1)
		err := tc.deployResource(ctx, deployment, false)
		if err != nil {
			t.Fatalf("1 replicaのDeployment作成に失敗: %v", err)
		}
		t.Logf("1 replicaのDeploymentが作成されました")

		// Deploymentの作成を待機
		err = tc.waitForResource(ctx, "deployment", "invalid-deployment")
		if err != nil {
			t.Fatalf("Deployment作成の待機に失敗: %v", err)
		}

		// HPAを作成（エラーが発生することを確認）
		hpa := tc.createHPA("invalid-hpa", "invalid-deployment")
		err = tc.deployResource(ctx, hpa, true)
		if err == nil {
			t.Fatalf("1 replicaのDeploymentに対するHPA作成が許可されました（拒否されるべき）")
		}
		t.Logf("1 replicaのDeploymentに対するHPA作成が正しく拒否されました: %v", err)
	})

	t.Run("HPAが存在する状態で1 replicaのDeploymentを作成（拒否されることを確認）", func(t *testing.T) {
		// まず2 replicaのDeploymentを作成
		deployment := tc.createDeployment("temp-deployment", 2)
		err := tc.deployResource(ctx, deployment, false)
		if err != nil {
			t.Fatalf("一時的なDeployment作成に失敗: %v", err)
		}

		// HPAを作成
		hpa := tc.createHPA("existing-hpa", "temp-deployment")
		err = tc.deployResource(ctx, hpa, false)
		if err != nil {
			t.Fatalf("HPA作成に失敗: %v", err)
		}

		// Deploymentを削除
		err = tc.clientset.AppsV1().Deployments(tc.namespace).Delete(ctx, "temp-deployment", metav1.DeleteOptions{})
		if err != nil {
			t.Fatalf("一時的なDeployment削除に失敗: %v", err)
		}

		// 1 replicaのDeploymentを作成（エラーが発生することを確認）
		invalidDeployment := tc.createDeployment("temp-deployment", 1)
		err = tc.deployResource(ctx, invalidDeployment, true)
		if err == nil {
			t.Fatalf("HPAが存在する状態での1 replicaのDeployment作成が許可されました（拒否されるべき）")
		}
		t.Logf("HPAが存在する状態での1 replicaのDeployment作成が正しく拒否されました: %v", err)
	})
}

// TestSimultaneousDeployment は同時デプロイメントシナリオのテスト
func TestSimultaneousDeployment(t *testing.T) {
	tc := NewTestClient(t)
	tc.setupNamespace(t)
	defer tc.cleanupNamespace(t)

	ctx := context.Background()

	t.Run("1 replicaのDeploymentとHPAの同時作成（両方拒否されることを確認）", func(t *testing.T) {
		// 1 replicaのDeploymentとHPAを同時に作成
		deployment := tc.createDeployment("simultaneous-deployment", 1)
		hpa := tc.createHPA("simultaneous-hpa", "simultaneous-deployment")

		// 同時作成をシミュレート（goroutineを使用）
		deploymentErr := make(chan error, 1)
		hpaErr := make(chan error, 1)

		go func() {
			err := tc.deployResource(ctx, deployment, true)
			deploymentErr <- err
		}()

		go func() {
			err := tc.deployResource(ctx, hpa, true)
			hpaErr <- err
		}()

		// 両方の結果を待機
		var deployErr, hpaError error
		for i := 0; i < 2; i++ {
			select {
			case err := <-deploymentErr:
				deployErr = err
			case err := <-hpaErr:
				hpaError = err
			case <-time.After(testTimeout):
				t.Fatalf("同時デプロイメントテストがタイムアウトしました")
			}
		}

		// 少なくとも一方はエラーになることを確認
		if deployErr == nil && hpaError == nil {
			t.Fatalf("1 replicaのDeploymentとHPAの同時作成が両方とも許可されました（少なくとも一方は拒否されるべき）")
		}

		if deployErr != nil {
			t.Logf("1 replicaのDeployment作成が正しく拒否されました: %v", deployErr)
		}
		if hpaError != nil {
			t.Logf("1 replicaのDeploymentを対象とするHPA作成が正しく拒否されました: %v", hpaError)
		}
	})

	t.Run("2 replicaのDeploymentとHPAの同時作成（両方成功することを確認）", func(t *testing.T) {
		// 2 replicaのDeploymentとHPAを同時に作成
		deployment := tc.createDeployment("valid-simultaneous-deployment", 2)
		hpa := tc.createHPA("valid-simultaneous-hpa", "valid-simultaneous-deployment")

		// 同時作成をシミュレート
		deploymentErr := make(chan error, 1)
		hpaErr := make(chan error, 1)

		go func() {
			err := tc.deployResource(ctx, deployment, false)
			deploymentErr <- err
		}()

		// HPAは少し遅らせて作成（Deploymentが先に作成されるように）
		go func() {
			time.Sleep(1 * time.Second)
			err := tc.deployResource(ctx, hpa, false)
			hpaErr <- err
		}()

		// 両方の結果を待機
		var deployErr, hpaError error
		for i := 0; i < 2; i++ {
			select {
			case err := <-deploymentErr:
				deployErr = err
			case err := <-hpaErr:
				hpaError = err
			case <-time.After(testTimeout):
				t.Fatalf("有効な同時デプロイメントテストがタイムアウトしました")
			}
		}

		// 両方とも成功することを確認
		if deployErr != nil {
			t.Fatalf("2 replicaのDeployment作成に失敗: %v", deployErr)
		}
		if hpaError != nil {
			t.Fatalf("2 replicaのDeploymentを対象とするHPA作成に失敗: %v", hpaError)
		}

		t.Logf("2 replicaのDeploymentとHPAの同時作成が正常に完了しました")
	})
}

// TestEdgeCases はエッジケースのテスト
func TestEdgeCases(t *testing.T) {
	tc := NewTestClient(t)
	tc.setupNamespace(t)
	defer tc.cleanupNamespace(t)

	ctx := context.Background()

	t.Run("Deploymentの更新（2 replica → 1 replica）でHPAが存在する場合", func(t *testing.T) {
		// 2 replicaのDeploymentを作成
		deployment := tc.createDeployment("update-test-deployment", 2)
		err := tc.deployResource(ctx, deployment, false)
		if err != nil {
			t.Fatalf("初期Deployment作成に失敗: %v", err)
		}

		// HPAを作成
		hpa := tc.createHPA("update-test-hpa", "update-test-deployment")
		err = tc.deployResource(ctx, hpa, false)
		if err != nil {
			t.Fatalf("HPA作成に失敗: %v", err)
		}

		// Deploymentを1 replicaに更新（拒否されることを確認）
		updatedDeployment := tc.createDeployment("update-test-deployment", 1)
		_, err = tc.clientset.AppsV1().Deployments(tc.namespace).Update(ctx, updatedDeployment, metav1.UpdateOptions{})
		if err == nil {
			t.Fatalf("HPAが存在する状態でのDeploymentの1 replica更新が許可されました（拒否されるべき）")
		}
		t.Logf("HPAが存在する状態でのDeploymentの1 replica更新が正しく拒否されました: %v", err)
	})

	t.Run("存在しないDeploymentを対象とするHPAの作成", func(t *testing.T) {
		// 存在しないDeploymentを対象とするHPAを作成
		hpa := tc.createHPA("orphan-hpa", "non-existent-deployment")
		err := tc.deployResource(ctx, hpa, false)
		if err != nil {
			t.Fatalf("存在しないDeploymentを対象とするHPA作成に失敗: %v", err)
		}
		t.Logf("存在しないDeploymentを対象とするHPAが作成されました（これは正常な動作）")
	})
}

// TestMain はE2Eテストのメイン関数
func TestMain(m *testing.M) {
	// テスト実行前の準備
	fmt.Println("E2Eテストを開始します...")
	
	// Webhookが動作していることを確認
	if !isWebhookReady() {
		fmt.Println("警告: Webhookが準備できていない可能性があります")
		fmt.Println("以下のコマンドでWebhookをデプロイしてください:")
		fmt.Println("  ./scripts/setup-kind-cluster.sh")
		fmt.Println("  ./scripts/deploy-webhook.sh")
	}

	// テスト用namespaceを事前に削除（クリーンな状態で開始）
	cleanupTestNamespaces()

	// テスト実行
	code := m.Run()
	
	// テスト用namespaceを削除
	cleanupTestNamespaces()
	
	fmt.Println("E2Eテストが完了しました")
	
	// 終了
	os.Exit(code)
}

// cleanupTestNamespaces はテスト用namespaceを削除
func cleanupTestNamespaces() {
	config, err := getKubeConfig()
	if err != nil {
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}

	ctx := context.Background()
	
	// テスト用namespaceを一覧取得
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("警告: namespace一覧の取得に失敗: %v\n", err)
		return
	}

	// テスト用namespaceを削除
	for _, ns := range namespaces.Items {
		if strings.HasPrefix(ns.Name, testNamespacePrefix) {
			fmt.Printf("テスト用namespace '%s' を削除中...\n", ns.Name)
			err = clientset.CoreV1().Namespaces().Delete(ctx, ns.Name, metav1.DeleteOptions{
				PropagationPolicy: func() *metav1.DeletionPropagation {
					p := metav1.DeletePropagationBackground
					return &p
				}(),
			})
			if err != nil && !strings.Contains(err.Error(), "not found") {
				fmt.Printf("警告: namespace '%s' の削除に失敗: %v\n", ns.Name, err)
			}
		}
	}
	
	fmt.Println("テスト用namespaceのクリーンアップが完了しました")
}

// isWebhookReady はWebhookが準備できているかチェック
func isWebhookReady() bool {
	config, err := getKubeConfig()
	if err != nil {
		return false
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false
	}

	ctx := context.Background()
	
	// ValidatingWebhookConfigurationの存在確認
	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, "hpa-deployment-validator", metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Webhook Podの存在確認
	pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
		LabelSelector: "app=k8s-deployment-hpa-validator",
	})
	if err != nil || len(pods.Items) == 0 {
		return false
	}

	// Podが実行中かチェック
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
	}

	return true
}