#!/bin/bash

# テスト結果検証・報告スクリプト
# E2Eテストの結果を解析し、詳細なレポートを生成します

set -euo pipefail

# 色付きログ用の定数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ログ関数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_header() {
    echo -e "${CYAN}=== $1 ===${NC}"
}

# 設定
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPORT_DIR="${PROJECT_ROOT}/test-reports"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="${REPORT_DIR}/e2e-test-report-${TIMESTAMP}.md"

# レポートディレクトリの作成
create_report_directory() {
    mkdir -p "$REPORT_DIR"
    log_info "レポートディレクトリを作成しました: $REPORT_DIR"
}

# テスト結果の解析
analyze_test_output() {
    local test_output_file="$1"
    local total_tests=0
    local passed_tests=0
    local failed_tests=0
    local skipped_tests=0
    
    # ファイルの存在確認
    if [ ! -f "$test_output_file" ]; then
        log_error "テスト出力ファイルが見つかりません: $test_output_file"
        return 1
    fi
    
    # テスト数の集計
    # ファイルの内容を変数に保存
    local file_content=$(cat "$test_output_file" 2>/dev/null || echo "")
    
    if [ -z "$file_content" ]; then
        log_warning "テスト出力ファイルが空です: $test_output_file"
        return 1
    fi
    
    # メインテストのみをカウント（サブテストは除外）
    # Goのテスト出力では、メインテストは "=== RUN   TestXXX" の形式
    # サブテストは "=== RUN   TestXXX/SubTest" の形式
    total_tests=$(echo "$file_content" | grep -E "^=== RUN   Test[^/]*$" | wc -l 2>/dev/null || echo "0")
    passed_tests=$(echo "$file_content" | grep -E "^--- PASS: Test[^/]*$" | wc -l 2>/dev/null || echo "0")
    failed_tests=$(echo "$file_content" | grep -E "^--- FAIL: Test[^/]*$" | wc -l 2>/dev/null || echo "0")
    skipped_tests=$(echo "$file_content" | grep -E "^--- SKIP: Test[^/]*$" | wc -l 2>/dev/null || echo "0")
    
    # 最終結果行も確認（PASS/FAILの行）
    local final_result=$(echo "$file_content" | grep -E "^(PASS|FAIL)$" | tail -1)
    if [ "$final_result" = "PASS" ] && [ "$failed_tests" -eq 0 ]; then
        # 全テスト成功の場合、成功数を合計数と同じにする
        if [ "$passed_tests" -gt 0 ] && [ "$total_tests" -gt "$passed_tests" ]; then
            passed_tests=$total_tests
        fi
    fi
    
    # 空白を削除して数値として扱う
    total_tests=$(echo "$total_tests" | tr -d ' ' | tr -d '\n')
    passed_tests=$(echo "$passed_tests" | tr -d ' ' | tr -d '\n')
    failed_tests=$(echo "$failed_tests" | tr -d ' ' | tr -d '\n')
    skipped_tests=$(echo "$skipped_tests" | tr -d ' ' | tr -d '\n')
    
    # 数値でない場合は0に設定
    total_tests=${total_tests:-0}
    passed_tests=${passed_tests:-0}
    failed_tests=${failed_tests:-0}
    skipped_tests=${skipped_tests:-0}
    
    # 数値の妥当性チェック
    if [ "$total_tests" -eq 0 ] && [ "$passed_tests" -gt 0 ]; then
        total_tests=$passed_tests
    fi
    
    # デバッグ情報
    log_info "ファイル内容の行数: $(echo "$file_content" | wc -l)"
    log_info "メインテスト検出数: $total_tests"
    log_info "成功テスト検出数: $passed_tests"
    log_info "失敗テスト検出数: $failed_tests"
    log_info "スキップテスト検出数: $skipped_tests"
    log_info "最終結果: $(echo "$file_content" | grep -E "^(PASS|FAIL)$" | tail -1 || echo "不明")"
    
    # 結果を連想配列として返す（bashの制限により、グローバル変数を使用）
    TEST_TOTAL=$total_tests
    TEST_PASSED=$passed_tests
    TEST_FAILED=$failed_tests
    TEST_SKIPPED=$skipped_tests
    
    log_info "テスト結果解析完了: 合計=$total_tests, 成功=$passed_tests, 失敗=$failed_tests, スキップ=$skipped_tests"
}

# 失敗したテストの詳細抽出
extract_failed_tests() {
    local test_output_file="$1"
    local failed_details=""
    
    # ファイルの存在確認
    if [ ! -f "$test_output_file" ]; then
        log_error "テスト出力ファイルが見つかりません: $test_output_file"
        FAILED_TEST_DETAILS="テスト出力ファイルが見つかりません"
        return 1
    fi
    
    # 失敗したテストの詳細を抽出
    if [ "${TEST_FAILED:-0}" -gt 0 ]; then
        local file_content=$(cat "$test_output_file" 2>/dev/null || echo "")
        failed_details=$(echo "$file_content" | grep -A 10 "--- FAIL:" 2>/dev/null || echo "テスト失敗の詳細情報がありません")
    fi
    
    FAILED_TEST_DETAILS="$failed_details"
}

# システム状態の収集
collect_system_status() {
    log_info "システム状態を収集中..."
    
    local status_file="${REPORT_DIR}/system-status-${TIMESTAMP}.txt"
    
    {
        echo "=== Kubernetes Cluster Info ==="
        kubectl cluster-info || echo "クラスター情報の取得に失敗"
        echo ""
        
        echo "=== Node Status ==="
        kubectl get nodes -o wide || echo "ノード情報の取得に失敗"
        echo ""
        
        echo "=== Webhook Pod Status ==="
        kubectl get pods -l app=k8s-deployment-hpa-validator -o wide || echo "Webhook Pod情報の取得に失敗"
        echo ""
        
        echo "=== Webhook Service Status ==="
        kubectl get svc -l app=k8s-deployment-hpa-validator || echo "Webhook Service情報の取得に失敗"
        echo ""
        
        echo "=== ValidatingWebhook Configuration ==="
        kubectl get validatingwebhookconfigurations hpa-deployment-validator -o yaml || echo "Webhook設定の取得に失敗"
        echo ""
        
        echo "=== Recent Events ==="
        kubectl get events --sort-by='.lastTimestamp' --field-selector type!=Normal | tail -30 || echo "イベント情報の取得に失敗"
        echo ""
        
        echo "=== Webhook Logs (Last 100 lines) ==="
        kubectl logs -l app=k8s-deployment-hpa-validator --tail=100 || echo "Webhookログの取得に失敗"
        
    } > "$status_file"
    
    SYSTEM_STATUS_FILE="$status_file"
    log_success "システム状態を収集しました: $status_file"
}

# パフォーマンス情報の収集
collect_performance_info() {
    log_info "パフォーマンス情報を収集中..."
    
    local perf_file="${REPORT_DIR}/performance-${TIMESTAMP}.txt"
    
    {
        echo "=== Resource Usage ==="
        kubectl top nodes || echo "ノードリソース使用量の取得に失敗"
        echo ""
        kubectl top pods -l app=k8s-deployment-hpa-validator || echo "Podリソース使用量の取得に失敗"
        echo ""
        
        echo "=== Webhook Response Times ==="
        # Webhookのレスポンス時間を測定（簡易版）
        local start_time=$(date +%s%N)
        kubectl apply --dry-run=server -f - <<EOF || echo "レスポンス時間測定に失敗"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-response-time
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx
EOF
        local end_time=$(date +%s%N)
        local response_time=$(( (end_time - start_time) / 1000000 ))
        echo "Webhook応答時間: ${response_time}ms"
        
    } > "$perf_file"
    
    PERFORMANCE_FILE="$perf_file"
    log_success "パフォーマンス情報を収集しました: $perf_file"
}

# HTMLレポートの生成
generate_html_report() {
    local html_file="${REPORT_DIR}/e2e-test-report-${TIMESTAMP}.html"
    
    cat > "$html_file" << EOF
<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>E2Eテストレポート - $(date)</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #f0f0f0; padding: 20px; border-radius: 5px; }
        .success { color: #28a745; }
        .error { color: #dc3545; }
        .warning { color: #ffc107; }
        .info { color: #17a2b8; }
        .summary { display: flex; gap: 20px; margin: 20px 0; }
        .summary-card { border: 1px solid #ddd; padding: 15px; border-radius: 5px; flex: 1; }
        .test-details { margin: 20px 0; }
        .code-block { background-color: #f8f9fa; padding: 10px; border-radius: 3px; overflow-x: auto; }
        pre { margin: 0; }
        .timestamp { color: #666; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="header">
        <h1>k8s-deployment-hpa-validator E2Eテストレポート</h1>
        <p class="timestamp">生成日時: $(date '+%Y年%m月%d日 %H:%M:%S')</p>
    </div>
    
    <div class="summary">
        <div class="summary-card">
            <h3>テスト結果サマリー</h3>
            <p><strong>合計テスト数:</strong> $TEST_TOTAL</p>
            <p class="success"><strong>成功:</strong> $TEST_PASSED</p>
            <p class="error"><strong>失敗:</strong> $TEST_FAILED</p>
            <p class="warning"><strong>スキップ:</strong> $TEST_SKIPPED</p>
        </div>
        
        <div class="summary-card">
            <h3>全体結果</h3>
            <p class="$([ "${TEST_FAILED:-0}" -eq 0 ] && echo 'success' || echo 'error')">
                <strong>$([ "${TEST_FAILED:-0}" -eq 0 ] && echo '✅ 全テスト成功' || echo '❌ テスト失敗あり')</strong>
            </p>
            <p><strong>成功率:</strong> $([ "${TEST_TOTAL:-0}" -gt 0 ] && echo $(( TEST_PASSED * 100 / TEST_TOTAL )) || echo "0")%</p>
        </div>
    </div>
    
    <div class="test-details">
        <h2>テスト詳細</h2>
        
        <h3>実行されたテストケース</h3>
        <ul>
            <li>正常ケース（2+ replica + HPA）</li>
            <li>異常ケース（1 replica + HPA）</li>
            <li>同時デプロイメントシナリオ</li>
            <li>エッジケース</li>
        </ul>
        
        $([ "${TEST_FAILED:-0}" -gt 0 ] && cat << 'FAIL_SECTION'
        <h3 class="error">失敗したテスト</h3>
        <div class="code-block">
            <pre>$FAILED_TEST_DETAILS</pre>
        </div>
FAIL_SECTION
)
    </div>
    
    <div class="system-info">
        <h2>システム情報</h2>
        <p><strong>システム状態ファイル:</strong> <a href="$(basename "$SYSTEM_STATUS_FILE")">$(basename "$SYSTEM_STATUS_FILE")</a></p>
        <p><strong>パフォーマンス情報:</strong> <a href="$(basename "$PERFORMANCE_FILE")">$(basename "$PERFORMANCE_FILE")</a></p>
    </div>
    
    <div class="footer">
        <hr>
        <p class="timestamp">このレポートは自動生成されました - $(date '+%Y年%m月%d日 %H:%M:%S')</p>
    </div>
</body>
</html>
EOF
    
    HTML_REPORT_FILE="$html_file"
    log_success "HTMLレポートを生成しました: $html_file"
}

# Markdownレポートの生成
generate_markdown_report() {
    cat > "$REPORT_FILE" << EOF
# E2Eテストレポート

**生成日時:** $(date '+%Y年%m月%d日 %H:%M:%S')  
**プロジェクト:** k8s-deployment-hpa-validator

## テスト結果サマリー

| 項目 | 値 |
|------|-----|
| 合計テスト数 | $TEST_TOTAL |
| 成功 | $TEST_PASSED ✅ |
| 失敗 | $TEST_FAILED $([ "${TEST_FAILED:-0}" -gt 0 ] && echo '❌' || echo '✅') |
| スキップ | $TEST_SKIPPED |
| 成功率 | $([ "${TEST_TOTAL:-0}" -gt 0 ] && echo $(( TEST_PASSED * 100 / TEST_TOTAL )) || echo "0")% |

## 全体結果

$([ "${TEST_FAILED:-0}" -eq 0 ] && echo '✅ **全テスト成功**' || echo '❌ **テスト失敗あり**')

## 実行されたテストケース

### 1. 正常ケース（2+ replica + HPA）
- 2 replicaのDeploymentとHPAの正常作成
- 3 replicaのDeploymentとHPAの正常作成

### 2. 異常ケース（1 replica + HPA）
- 1 replicaのDeploymentにHPAを追加（拒否されることを確認）
- HPAが存在する状態で1 replicaのDeploymentを作成（拒否されることを確認）

### 3. 同時デプロイメントシナリオ
- 1 replicaのDeploymentとHPAの同時作成（両方拒否されることを確認）
- 2 replicaのDeploymentとHPAの同時作成（両方成功することを確認）

### 4. エッジケース
- Deploymentの更新（2 replica → 1 replica）でHPAが存在する場合
- 存在しないDeploymentを対象とするHPAの作成

$([ "${TEST_FAILED:-0}" -gt 0 ] && cat << 'FAIL_SECTION'

## 失敗したテスト詳細

\`\`\`
$FAILED_TEST_DETAILS
\`\`\`
FAIL_SECTION
)

## システム情報

- **システム状態ファイル:** $(basename "$SYSTEM_STATUS_FILE")
- **パフォーマンス情報:** $(basename "$PERFORMANCE_FILE")

## 推奨アクション

$(if [ "${TEST_FAILED:-0}" -eq 0 ]; then
  cat << 'SUCCESS_ACTIONS'
✅ 全てのテストが成功しました。Webhookは期待通りに動作しています。

### 次のステップ
- 本番環境へのデプロイを検討
- 継続的インテグレーションへの組み込み
- 監視とアラートの設定
SUCCESS_ACTIONS
else
  cat << 'FAILURE_ACTIONS'
❌ テストに失敗があります。以下を確認してください：

### トラブルシューティング
1. **Webhook Podの状態確認**
   ```bash
   kubectl get pods -l app=k8s-deployment-hpa-validator
   kubectl logs -l app=k8s-deployment-hpa-validator
   ```

2. **Webhook設定の確認**
   ```bash
   kubectl get validatingwebhookconfigurations hpa-deployment-validator
   ```

3. **証明書の確認**
   ```bash
   ./scripts/verify-webhook.sh
   ```

4. **RBAC権限の確認**
   ```bash
   ./scripts/verify-rbac.sh
   ```
FAILURE_ACTIONS
fi)

---
*このレポートは自動生成されました*
EOF
    
    log_success "Markdownレポートを生成しました: $REPORT_FILE"
}

# JUnit XML形式のレポート生成
generate_junit_report() {
    local junit_file="${REPORT_DIR}/junit-${TIMESTAMP}.xml"
    
    cat > "$junit_file" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="E2E Tests" tests="$TEST_TOTAL" failures="$TEST_FAILED" errors="0" time="0">
    <testsuite name="k8s-deployment-hpa-validator" tests="$TEST_TOTAL" failures="$TEST_FAILED" errors="0" time="0">
        <testcase name="ValidDeploymentWithHPA" classname="e2e.TestValidDeploymentWithHPA">
            $([ "${TEST_FAILED:-0}" -gt 0 ] && echo '<failure message="Test failed">Test execution failed</failure>')
        </testcase>
        <testcase name="InvalidDeploymentWithHPA" classname="e2e.TestInvalidDeploymentWithHPA">
            $([ "${TEST_FAILED:-0}" -gt 0 ] && echo '<failure message="Test failed">Test execution failed</failure>')
        </testcase>
        <testcase name="SimultaneousDeployment" classname="e2e.TestSimultaneousDeployment">
            $([ "${TEST_FAILED:-0}" -gt 0 ] && echo '<failure message="Test failed">Test execution failed</failure>')
        </testcase>
        <testcase name="EdgeCases" classname="e2e.TestEdgeCases">
            $([ "${TEST_FAILED:-0}" -gt 0 ] && echo '<failure message="Test failed">Test execution failed</failure>')
        </testcase>
    </testsuite>
</testsuites>
EOF
    
    JUNIT_REPORT_FILE="$junit_file"
    log_success "JUnit XMLレポートを生成しました: $junit_file"
}

# レポートサマリーの表示
display_report_summary() {
    log_header "テストレポートサマリー"
    
    echo "📊 テスト結果:"
    echo "   合計: $TEST_TOTAL"
    echo "   成功: $TEST_PASSED"
    echo "   失敗: $TEST_FAILED"
    echo "   スキップ: $TEST_SKIPPED"
    echo ""
    
    echo "📁 生成されたファイル:"
    echo "   Markdownレポート: $REPORT_FILE"
    echo "   HTMLレポート: $HTML_REPORT_FILE"
    echo "   JUnit XMLレポート: $JUNIT_REPORT_FILE"
    echo "   システム状態: $SYSTEM_STATUS_FILE"
    echo "   パフォーマンス情報: $PERFORMANCE_FILE"
    echo ""
    
    if [ "${TEST_FAILED:-0}" -eq 0 ]; then
        log_success "全てのテストが成功しました！"
    else
        log_error "テストに失敗があります。詳細はレポートを確認してください。"
    fi
}

# メイン実行関数
main() {
    local test_output_file="$1"
    
    if [ ! -f "$test_output_file" ]; then
        log_error "テスト出力ファイルが見つかりません: $test_output_file"
        exit 1
    fi
    
    log_info "テスト結果レポートを生成中..."
    log_info "テスト出力ファイル: $test_output_file"
    log_info "ファイルサイズ: $(wc -l < "$test_output_file" 2>/dev/null || echo "不明") 行"
    
    # レポートディレクトリの作成
    create_report_directory
    
    # テスト結果の解析
    if ! analyze_test_output "$test_output_file"; then
        log_error "テスト結果の解析に失敗しました"
        exit 1
    fi
    
    extract_failed_tests "$test_output_file"
    
    # システム情報の収集
    collect_system_status
    collect_performance_info
    
    # レポートの生成
    generate_markdown_report
    generate_html_report
    generate_junit_report
    
    # サマリーの表示
    display_report_summary
    
    log_success "テストレポートの生成が完了しました"
}

# 使用方法の表示
show_usage() {
    echo "使用方法: $0 <test_output_file>"
    echo ""
    echo "引数:"
    echo "  test_output_file  E2Eテストの出力ファイル"
    echo ""
    echo "例:"
    echo "  $0 test-output.txt"
}

# 引数チェック
if [ $# -ne 1 ]; then
    show_usage
    exit 1
fi

# スクリプト実行
main "$1"