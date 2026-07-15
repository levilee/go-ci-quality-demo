# GitHub 原生质量门禁负向 POC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有功能分支注入一个可控的失败 Go 单元测试，提交并推送后验证 GitHub Actions `Quality Gate` 和 `main` Ruleset 会阻止 PR 合并。

**Architecture:** 不修改生产代码，只在现有 HTTP API 测试文件中增加一个故意错误的计算结果断言。测试失败由服务检查传递到汇总门禁；POC 完成后使用 `git revert` 恢复并保留审计链路。

**Tech Stack:** Go、`net/http/httptest`、Git、GitHub Actions、GitHub Rulesets

## Global Constraints

- 使用现有 `feature/add-calculator-api` 分支，不创建新分支或 worktree。
- 不修改生产逻辑、依赖或部署配置。
- 失败提交不得合入 `main`，不得使用管理员绕过。
- GitHub 页面操作由用户执行。
- 故障恢复使用 `git revert`，不改写 Git 历史。

---

### Task 1: 验证干净基线

**Files:**
- Inspect: `internal/httpapi/handler_test.go`
- Test: `internal/httpapi/handler_test.go`

**Interfaces:**
- Consumes: 当前 `feature/add-calculator-api` 分支及已合入的 `origin/main`
- Produces: 故障注入前测试全部成功的基线证据

- [ ] **Step 1: 确认分支和工作区状态**

Run:

```powershell
git branch --show-current
git status --short
```

Expected: 当前分支为 `feature/add-calculator-api`，工作区只允许出现本计划文件。

- [ ] **Step 2: 运行基线测试**

Run:

```powershell
go test ./...
```

Expected: 命令退出码为 `0`，`internal/httpapi` 测试成功。

### Task 2: 注入可控失败

**Files:**
- Modify: `internal/httpapi/handler_test.go`
- Test: `internal/httpapi/handler_test.go`

**Interfaces:**
- Consumes: `NewHandler(upstreamURL string, client *http.Client) http.Handler` 和现有 `/api/calculate` 接口
- Produces: `TestPOCQualityGateBlocksInvalidExpectation(t *testing.T)`，稳定输出 POC 故障证据

- [ ] **Step 1: 添加故意失败的测试**

在 `internal/httpapi/handler_test.go` 中加入：

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

- [ ] **Step 2: 格式化测试文件**

Run:

```powershell
gofmt -w internal/httpapi/handler_test.go
```

Expected: 命令退出码为 `0`，避免格式门禁先于测试门禁失败。

- [ ] **Step 3: 验证测试按设计失败**

Run:

```powershell
go test ./...
```

Expected: 命令退出码非 `0`，日志包含 `POC intentional failure: result = 12, want 999`，没有其他测试失败。

### Task 3: 创建并推送审计提交

**Files:**
- Create: `docs/superpowers/plans/2026-07-15-quality-gate-negative-poc.md`
- Modify: `internal/httpapi/handler_test.go`

**Interfaces:**
- Consumes: 已验证的 POC 故障测试
- Produces: 可由 GitHub PR 和 Ruleset 审计的故障提交

- [ ] **Step 1: 检查差异和空白错误**

Run:

```powershell
git diff --check
git diff -- internal/httpapi/handler_test.go
```

Expected: `git diff --check` 退出码为 `0`；测试差异仅包含故意失败用例。

- [ ] **Step 2: 创建故障提交**

Run:

```powershell
git add docs/superpowers/plans/2026-07-15-quality-gate-negative-poc.md internal/httpapi/handler_test.go
git commit -m "test(poc): demonstrate quality gate blocking"
```

Expected: 创建一个包含实施计划和 POC 测试的提交。

- [ ] **Step 3: 推送现有分支**

Run:

```powershell
git push origin feature/add-calculator-api
```

Expected: GitHub 上的现有分支更新，`main` 保持不变。

### Task 4: GitHub 端验证和恢复

**Files:**
- Inspect: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: GitHub 上的 `feature/add-calculator-api` 故障提交
- Produces: PR 门禁失败、Ruleset 阻断和后续恢复的审计记录

- [ ] **Step 1: 创建或更新 PR**

在 GitHub 选择 base `main`、compare `feature/add-calculator-api`，创建 PR；若该分支已有打开的 PR，推送会自动更新它。

Expected: `GitHub Native Quality Gate` 工作流开始运行。

- [ ] **Step 2: 验证阻断结果**

Expected: Go 服务检查因 `TestPOCQualityGateBlocksInvalidExpectation` 失败，汇总任务 `Quality Gate` 失败，Ruleset 禁止正常合并。

- [ ] **Step 3: POC 完成后恢复**

Run:

```powershell
$faultCommit = git rev-parse HEAD
git revert $faultCommit
git push origin feature/add-calculator-api
```

Expected: `$faultCommit` 记录当前 POC 故障提交，新提交撤销故障测试，GitHub Actions 重新运行并恢复为成功。
