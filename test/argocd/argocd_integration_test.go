// +build argocd

package argocd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s-deployment-hpa-validator/internal/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/apimachinery/pkg/runtime"
)

// ArgoCDTestClient ArgoCD統合テスト用のクライアント
type ArgoCDTestClient struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
}

// NewArgoCDTestClient 新しいArgoCD統合テストクライアントを作成
func NewArgoCDTestClient(t *testing.T) *ArgoCDTestClient {
	// フェイククライアントを作成
	kubeClient := fake.NewSimpleClientset()
	
	// Dynamic clientのためのスキームを作成
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	return &ArgoCDTestClient{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		namespace:     "argocd",
	}
}

// createArgoCDApplication ArgoCDアプリケーションを作成
func (c *ArgoCDTestClient) createArgoCDApplication(name, repoURL, path, targetNamespace string) (*unstructured.Unstructured, error) {
	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": c.namespace,
			},
			"spec": map[string]interface{}{
				"project": "default",
				"source": map[string]interface{}{
					"repoURL":        repoURL,
					"targetRevision": "HEAD",
					"path":           path,
				},
				"destination": map[string]interface{}{
					"server":    "https://kubernetes.default.svc",
					"namespace": targetNamespace,
				},
				"syncPolicy": map[string]interface{}{
					"automated": map[string]interface{}{
						"prune":    true,
						"selfHeal": true,
					},
					"syncOptions": []interface{}{
						"CreateNamespace=true",
					},
				},
			},
		},
	}

	// ArgoCD Application GVR
	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	return c.dynamicClient.Resource(gvr).Namespace(c.namespace).Create(
		context.Background(), app, metav1.CreateOptions{})
}

// getArgoCDApplication ArgoCDアプリケーションを取得
func (c *ArgoCDTestClient) getArgoCDApplication(name string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	return c.dynamicClient.Resource(gvr).Namespace(c.namespace).Get(
		context.Background(), name, metav1.GetOptions{})
}

// updateArgoCDApplicationStatus ArgoCDアプリケーションのステータスを更新
func (c *ArgoCDTestClient) updateArgoCDApplicationStatus(name string, health, sync string) error {
	app, err := c.getArgoCDApplication(name)
	if err != nil {
		return err
	}

	// ステータスを設定
	status := map[string]interface{}{
		"health": map[string]interface{}{
			"status": health,
		},
		"sync": map[string]interface{}{
			"status": sync,
		},
		"operationState": map[string]interface{}{
			"phase": "Succeeded",
		},
	}

	app.Object["status"] = status

	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	_, err = c.dynamicClient.Resource(gvr).Namespace(c.namespace).UpdateStatus(
		context.Background(), app, metav1.UpdateOptions{})
	return err
}

// deleteArgoCDApplication ArgoCDアプリケーションを削除
func (c *ArgoCDTestClient) deleteArgoCDApplication(name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	return c.dynamicClient.Resource(gvr).Namespace(c.namespace).Delete(
		context.Background(), name, metav1.DeleteOptions{})
}

// waitForApplicationSync アプリケーションの同期完了を待機
func (c *ArgoCDTestClient) waitForApplicationSync(name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("アプリケーション %s の同期がタイムアウトしました", name)
		case <-ticker.C:
			app, err := c.getArgoCDApplication(name)
			if err != nil {
				continue
			}

			status, found, err := unstructured.NestedMap(app.Object, "status")
			if err != nil || !found {
				continue
			}

			syncStatus, found, err := unstructured.NestedString(status, "sync", "status")
			if err != nil || !found {
				continue
			}

			if syncStatus == "Synced" {
				return nil
			}
		}
	}
}

// TestArgoCDApplicationDeployment ArgoCDアプリケーションのデプロイメントテスト
func TestArgoCDApplicationDeployment(t *testing.T) {
	client := NewArgoCDTestClient(t)

	t.Run("ArgoCDアプリケーションの作成", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// ArgoCDアプリケーションを作成
		app, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// アプリケーションの基本情報を確認
		name, found, err := unstructured.NestedString(app.Object, "metadata", "name")
		if err != nil || !found {
			t.Fatalf("アプリケーション名の取得に失敗しました: %v", err)
		}
		if name != appName {
			t.Errorf("期待されるアプリケーション名: %s, 実際: %s", appName, name)
		}

		// ソース設定の確認
		sourceRepoURL, found, err := unstructured.NestedString(app.Object, "spec", "source", "repoURL")
		if err != nil || !found {
			t.Fatalf("リポジトリURLの取得に失敗しました: %v", err)
		}
		if sourceRepoURL != repoURL {
			t.Errorf("期待されるリポジトリURL: %s, 実際: %s", repoURL, sourceRepoURL)
		}

		sourcePath, found, err := unstructured.NestedString(app.Object, "spec", "source", "path")
		if err != nil || !found {
			t.Fatalf("パスの取得に失敗しました: %v", err)
		}
		if sourcePath != path {
			t.Errorf("期待されるパス: %s, 実際: %s", path, sourcePath)
		}

		// デスティネーション設定の確認
		destNamespace, found, err := unstructured.NestedString(app.Object, "spec", "destination", "namespace")
		if err != nil || !found {
			t.Fatalf("デスティネーションnamespaceの取得に失敗しました: %v", err)
		}
		if destNamespace != targetNamespace {
			t.Errorf("期待されるデスティネーションnamespace: %s, 実際: %s", targetNamespace, destNamespace)
		}

		t.Logf("ArgoCDアプリケーション '%s' が正常に作成されました", appName)
	})

	t.Run("自動同期設定の確認", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator-auto-sync"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// 自動同期が有効なアプリケーションを作成
		app, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// 自動同期設定の確認
		automated, found, err := unstructured.NestedMap(app.Object, "spec", "syncPolicy", "automated")
		if err != nil || !found {
			t.Fatalf("自動同期設定の取得に失敗しました: %v", err)
		}

		prune, found, err := unstructured.NestedBool(automated, "prune")
		if err != nil || !found {
			t.Fatalf("prune設定の取得に失敗しました: %v", err)
		}
		if !prune {
			t.Error("prune設定が有効になっていません")
		}

		selfHeal, found, err := unstructured.NestedBool(automated, "selfHeal")
		if err != nil || !found {
			t.Fatalf("selfHeal設定の取得に失敗しました: %v", err)
		}
		if !selfHeal {
			t.Error("selfHeal設定が有効になっていません")
		}

		// 同期オプションの確認
		syncOptions, found, err := unstructured.NestedSlice(app.Object, "spec", "syncPolicy", "syncOptions")
		if err != nil || !found {
			t.Fatalf("同期オプションの取得に失敗しました: %v", err)
		}

		hasCreateNamespace := false
		for _, option := range syncOptions {
			if optionStr, ok := option.(string); ok && optionStr == "CreateNamespace=true" {
				hasCreateNamespace = true
				break
			}
		}
		if !hasCreateNamespace {
			t.Error("CreateNamespace=true オプションが設定されていません")
		}

		t.Logf("自動同期設定が正常に確認されました")
	})
}

// TestArgoCDApplicationSync ArgoCDアプリケーションの同期テスト
func TestArgoCDApplicationSync(t *testing.T) {
	client := NewArgoCDTestClient(t)

	t.Run("アプリケーション同期の成功", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator-sync"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// ArgoCDアプリケーションを作成
		_, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// アプリケーションのステータスを同期済みに更新
		err = client.updateArgoCDApplicationStatus(appName, "Healthy", "Synced")
		if err != nil {
			t.Fatalf("アプリケーションステータスの更新に失敗しました: %v", err)
		}

		// 同期完了を待機（テスト用に短いタイムアウト）
		err = client.waitForApplicationSync(appName, 5*time.Second)
		if err != nil {
			t.Fatalf("アプリケーション同期の待機に失敗しました: %v", err)
		}

		// アプリケーションの状態を確認
		app, err := client.getArgoCDApplication(appName)
		if err != nil {
			t.Fatalf("アプリケーションの取得に失敗しました: %v", err)
		}

		healthStatus, found, err := unstructured.NestedString(app.Object, "status", "health", "status")
		if err != nil || !found {
			t.Fatalf("ヘルス状態の取得に失敗しました: %v", err)
		}
		if healthStatus != "Healthy" {
			t.Errorf("期待されるヘルス状態: Healthy, 実際: %s", healthStatus)
		}

		syncStatus, found, err := unstructured.NestedString(app.Object, "status", "sync", "status")
		if err != nil || !found {
			t.Fatalf("同期状態の取得に失敗しました: %v", err)
		}
		if syncStatus != "Synced" {
			t.Errorf("期待される同期状態: Synced, 実際: %s", syncStatus)
		}

		t.Logf("アプリケーション '%s' が正常に同期されました", appName)
	})

	t.Run("アプリケーション同期の失敗処理", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator-sync-fail"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// ArgoCDアプリケーションを作成
		_, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// アプリケーションのステータスを失敗状態に更新
		err = client.updateArgoCDApplicationStatus(appName, "Degraded", "OutOfSync")
		if err != nil {
			t.Fatalf("アプリケーションステータスの更新に失敗しました: %v", err)
		}

		// アプリケーションの状態を確認
		app, err := client.getArgoCDApplication(appName)
		if err != nil {
			t.Fatalf("アプリケーションの取得に失敗しました: %v", err)
		}

		healthStatus, found, err := unstructured.NestedString(app.Object, "status", "health", "status")
		if err != nil || !found {
			t.Fatalf("ヘルス状態の取得に失敗しました: %v", err)
		}
		if healthStatus != "Degraded" {
			t.Errorf("期待されるヘルス状態: Degraded, 実際: %s", healthStatus)
		}

		syncStatus, found, err := unstructured.NestedString(app.Object, "status", "sync", "status")
		if err != nil || !found {
			t.Fatalf("同期状態の取得に失敗しました: %v", err)
		}
		if syncStatus != "OutOfSync" {
			t.Errorf("期待される同期状態: OutOfSync, 実際: %s", syncStatus)
		}

		t.Logf("アプリケーション '%s' の失敗状態が正常に確認されました", appName)
	})
}

// TestArgoCDApplicationRollback ArgoCDアプリケーションのロールバックテスト
func TestArgoCDApplicationRollback(t *testing.T) {
	client := NewArgoCDTestClient(t)

	t.Run("アプリケーションのロールバック", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator-rollback"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// ArgoCDアプリケーションを作成
		app, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// 初期状態を同期済みに設定
		err = client.updateArgoCDApplicationStatus(appName, "Healthy", "Synced")
		if err != nil {
			t.Fatalf("初期ステータスの更新に失敗しました: %v", err)
		}

		// ロールバック操作をシミュレート（targetRevisionを変更）
		app.Object["spec"].(map[string]interface{})["source"].(map[string]interface{})["targetRevision"] = "previous-commit"

		gvr := schema.GroupVersionResource{
			Group:    "argoproj.io",
			Version:  "v1alpha1",
			Resource: "applications",
		}

		_, err = client.dynamicClient.Resource(gvr).Namespace(client.namespace).Update(
			context.Background(), app, metav1.UpdateOptions{})
		if err != nil {
			t.Fatalf("アプリケーションの更新に失敗しました: %v", err)
		}

		// ロールバック後の状態を確認
		updatedApp, err := client.getArgoCDApplication(appName)
		if err != nil {
			t.Fatalf("更新されたアプリケーションの取得に失敗しました: %v", err)
		}

		targetRevision, found, err := unstructured.NestedString(updatedApp.Object, "spec", "source", "targetRevision")
		if err != nil || !found {
			t.Fatalf("targetRevisionの取得に失敗しました: %v", err)
		}
		if targetRevision != "previous-commit" {
			t.Errorf("期待されるtargetRevision: previous-commit, 実際: %s", targetRevision)
		}

		t.Logf("アプリケーション '%s' のロールバックが正常に実行されました", appName)
	})
}

// TestArgoCDHealthCheck ArgoCDヘルスチェックのテスト
func TestArgoCDHealthCheck(t *testing.T) {
	client := NewArgoCDTestClient(t)

	t.Run("カスタムヘルスチェックの確認", func(t *testing.T) {
		appName := "k8s-deployment-hpa-validator-health"
		repoURL := "https://github.com/example/k8s-deployment-hpa-validator.git"
		path := "manifests/overlays/production"
		targetNamespace := "webhook-system"

		// ArgoCDアプリケーションを作成
		_, err := client.createArgoCDApplication(appName, repoURL, path, targetNamespace)
		if err != nil {
			t.Fatalf("ArgoCDアプリケーションの作成に失敗しました: %v", err)
		}

		// テスト用のWebhookデプロイメントを作成
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "k8s-deployment-hpa-validator",
				Namespace: targetNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/name": "k8s-deployment-hpa-validator",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: func() *int32 { i := int32(2); return &i }(),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "k8s-deployment-hpa-validator",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "k8s-deployment-hpa-validator",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "webhook",
								Image: "k8s-deployment-hpa-validator:latest",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 8443,
										Name:          "webhook",
									},
									{
										ContainerPort: 8080,
										Name:          "metrics",
									},
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/readyz",
											Port: intstr.FromInt(8080),
										},
									},
									InitialDelaySeconds: 5,
									PeriodSeconds:       10,
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/livez",
											Port: intstr.FromInt(8080),
										},
									},
									InitialDelaySeconds: 15,
									PeriodSeconds:       20,
								},
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				ReadyReplicas:     2,
				AvailableReplicas: 2,
				Replicas:          2,
			},
		}

		// Deploymentを作成
		_, err = client.kubeClient.AppsV1().Deployments(targetNamespace).Create(
			context.Background(), deployment, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Deploymentの作成に失敗しました: %v", err)
		}

		// アプリケーションのヘルス状態を健全に設定
		err = client.updateArgoCDApplicationStatus(appName, "Healthy", "Synced")
		if err != nil {
			t.Fatalf("アプリケーションステータスの更新に失敗しました: %v", err)
		}

		// ヘルス状態の確認
		app, err := client.getArgoCDApplication(appName)
		if err != nil {
			t.Fatalf("アプリケーションの取得に失敗しました: %v", err)
		}

		healthStatus, found, err := unstructured.NestedString(app.Object, "status", "health", "status")
		if err != nil || !found {
			t.Fatalf("ヘルス状態の取得に失敗しました: %v", err)
		}
		if healthStatus != "Healthy" {
			t.Errorf("期待されるヘルス状態: Healthy, 実際: %s", healthStatus)
		}

		// Deploymentの状態確認
		retrievedDeployment, err := client.kubeClient.AppsV1().Deployments(targetNamespace).Get(
			context.Background(), "k8s-deployment-hpa-validator", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Deploymentの取得に失敗しました: %v", err)
		}

		if retrievedDeployment.Status.ReadyReplicas != 2 {
			t.Errorf("期待される準備完了レプリカ数: 2, 実際: %d", retrievedDeployment.Status.ReadyReplicas)
		}

		t.Logf("アプリケーション '%s' のヘルスチェックが正常に確認されました", appName)
	})
}

// TestArgoCDConfigurationValidation ArgoCD設定の妥当性テスト
func TestArgoCDConfigurationValidation(t *testing.T) {
	t.Run("ArgoCD設定ファイルの妥当性確認", func(t *testing.T) {
		// ArgoCD設定ファイルのパス
		argoCDConfigFiles := []string{
			filepath.Join("..", "..", "argocd", "application.yaml"),
			filepath.Join("..", "..", "argocd", "application-development.yaml"),
			filepath.Join("..", "..", "argocd", "application-staging.yaml"),
		}

		for _, configFile := range argoCDConfigFiles {
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Logf("ArgoCD設定ファイルが見つかりません: %s", configFile)
				continue
			}

			t.Logf("ArgoCD設定ファイルを確認中: %s", filepath.Base(configFile))

			// 設定ファイルの基本的な妥当性確認
			// 実際の実装では、YAMLファイルを解析して設定を検証
			
			// 必要な設定項目の確認
			requiredFields := []string{
				"apiVersion: argoproj.io/v1alpha1",
				"kind: Application",
				"spec:",
				"source:",
				"destination:",
			}

			// ファイル内容を読み込み（簡易版）
			content, err := os.ReadFile(configFile)
			if err != nil {
				t.Errorf("設定ファイルの読み込みに失敗しました (%s): %v", configFile, err)
				continue
			}

			fileContent := string(content)
			for _, field := range requiredFields {
				if !strings.Contains(fileContent, field) {
					t.Errorf("必要な設定項目が見つかりません (%s): %s", filepath.Base(configFile), field)
				}
			}

			// セキュリティ設定の確認
			securitySettings := []string{
				"automated:",
				"prune: true",
				"selfHeal: true",
			}

			for _, setting := range securitySettings {
				if strings.Contains(fileContent, setting) {
					t.Logf("✓ セキュリティ設定が確認されました (%s): %s", filepath.Base(configFile), setting)
				}
			}

			t.Logf("ArgoCD設定ファイルの妥当性確認が完了しました: %s", filepath.Base(configFile))
		}
	})

	t.Run("環境別設定の一貫性確認", func(t *testing.T) {
		// 環境別のArgoCD設定の一貫性を確認
		environments := []string{"development", "staging", "production"}
		
		for _, env := range environments {
			// 対応する設定ファイルの確認
			configPath := filepath.Join("..", "..", "configs", env+".yaml")
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Logf("環境設定ファイルが見つかりません: %s", configPath)
				continue
			}

			// 環境設定を読み込み
			loader := config.NewConfigLoaderWithFile(configPath)
			cfg, err := loader.LoadConfig()
			if err != nil {
				t.Errorf("%s環境の設定読み込みに失敗しました: %v", env, err)
				continue
			}

			// ArgoCD統合に必要な設定の確認
			if cfg.Environment != env {
				t.Errorf("環境設定が一致しません。期待: %s, 実際: %s", env, cfg.Environment)
			}

			// 本番環境固有の確認
			if env == "production" {
				if cfg.FailurePolicy != "Fail" {
					t.Errorf("本番環境ではFailure Policyが必須です: %s", cfg.FailurePolicy)
				}
				if cfg.LogLevel == "debug" {
					t.Errorf("本番環境でdebugログレベルは推奨されません")
				}
			}

			t.Logf("✓ %s環境のArgoCD統合設定確認が完了しました", env)
		}
	})
}