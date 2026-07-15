# GitHub 原生质量门禁负向 POC 设计

## 目标

在现有 `feature/add-calculator-api` 分支中加入一个可控且容易恢复的失败测试，证明 GitHub Actions 的服务质量检查会将失败传递给汇总任务 `Quality Gate`，并由针对 `main` 的 GitHub Ruleset 阻止 PR 合并。

## 范围和约束

- 继续使用现有 `feature/add-calculator-api` 分支，不创建新分支或 worktree。
- 只修改 Go 测试代码，不修改生产处理逻辑、依赖或部署配置。
- 故障必须稳定复现，日志必须能直接指出失败原因。
- 失败状态下不得绕过 Ruleset，也不得把异常代码合入 `main`。
- POC 完成后使用 `git revert` 撤销故障提交，保留完整审计记录。
- 所有 GitHub 页面操作由用户执行，Codex 不操作浏览器。

## 方案选择

采用错误单元测试断言：请求现有计算接口执行 `7 + 5`，但故意断言结果为 `999`。实际结果稳定为 `12`，因此 `go test ./...` 会明确报告 `result = 12, want 999`。

没有选择格式错误，因为它只能验证格式门禁；没有选择漏洞或密钥样本，因为它们可能产生长期安全告警和仓库历史污染。

## 修改设计

在 `internal/httpapi/handler_test.go` 增加：

```go
func TestPOCQualityGateBlocksInvalidExpectation(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/calculate?a=7&b=5&operation=add", nil)
	NewHandler("http://unused", http.DefaultClient).ServeHTTP(recorder, request)

	var response struct {
		Result int64 `json:"result"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const deliberatelyWrongExpectedResult int64 = 999
	if response.Result != deliberatelyWrongExpectedResult {
		t.Fatalf("POC intentional failure: result = %d, want %d", response.Result, deliberatelyWrongExpectedResult)
	}
}
```

函数名、常量名和失败消息都明确说明这是 POC 故障，避免被误认为真实产品缺陷。

## 门禁数据流

1. 将故障测试提交并推送到 `feature/add-calculator-api`。
2. 针对 `main` 创建或更新 PR，触发 `.github/workflows/ci.yml`。
3. 服务发现脚本识别根目录 Go 服务发生变化。
4. `service-quality` 执行 Go 测试并因错误断言失败。
5. 汇总任务 `Quality Gate` 检查到 `service-quality` 不是 `success`，自身失败。
6. `main` Ruleset 将 `Quality Gate` 作为必需状态检查，禁止合并。

## 验收标准

- 加入故障前，`go test ./...` 退出码为 `0`。
- 加入故障后，`go test ./...` 退出码非 `0`，且只出现预期的 POC 断言失败。
- GitHub PR 中 `Quality Gate` 显示失败。
- PR 合并入口提示必需检查未通过，无法正常合并。
- 不使用管理员绕过或例外审批完成合并。

## 恢复和收尾

记录故障提交 ID 后执行 `git revert <故障提交ID>` 并推送同一分支。GitHub Actions 应重新运行并恢复为绿色。若仅验证阻断能力，可在恢复后关闭 PR；若需要验证完整红转绿流程，则等待全部必需检查成功后再按正常审批流程合并。

## 风险控制

- 故障仅存在于测试文件，不影响已部署应用。
- 分支推送前确认当前分支不是 `main`。
- 失败 PR 不允许实际合并。
- 使用 revert 而不是改写历史，保留故障注入、门禁阻断和恢复的审计链路。
