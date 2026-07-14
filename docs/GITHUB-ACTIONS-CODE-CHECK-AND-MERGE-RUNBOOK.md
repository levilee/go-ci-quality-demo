# GitHub Actions 代码检查与合并操作手册

适用仓库：`levilee/go-ci-quality-demo`

目标分支：`main`

演练功能：新增 `GET /api/calculate` 接口

演练分支：`feature/add-calculator-api`

## 1. 本次演练结果目标

完整链路如下：

```text
修改代码和测试
-> 本地检查
-> commit
-> push 功能分支
-> 创建 Pull Request 到 main
-> GitHub Actions 并行执行质量与安全检查
-> Quality Gate 汇总
-> Review 和 Required Check 全部满足
-> Squash merge 到 main
-> main 再次触发 GitHub Actions
-> 保存 PR、运行日志、报告和 merge commit 作为证据
```

不得直接把功能分支 push 到 `main`，不得在检查失败时强制合并，也不得把失败检查人工改成成功。

## 2. 新接口验收说明

接口：

```http
GET /api/calculate?a=<integer>&b=<integer>&operation=<add|subtract|multiply>
```

成功示例：

```bash
curl "http://localhost:8080/api/calculate?a=7&b=5&operation=add"
```

预期响应：

```json
{"a":7,"b":5,"operation":"add","result":12}
```

验收范围：加、减、乘；负数；缺少参数；非整数参数；不支持的操作；非 GET 方法。

## 3. 开发者本地操作

### 3.1 同步并创建分支

正常项目应从最新 `main` 创建功能分支：

```powershell
git switch main
git pull --ff-only origin main
git switch -c feature/add-calculator-api
```

本次 POC 的 GitHub 原生工作流尚未合入 `main`，因此演练分支实际从 `poc/github-native-quality-gate` 创建，使 PR 自身包含并触发新门禁。

### 3.2 检查修改内容

```powershell
git status --short
git diff --check
git diff
```

重点确认：没有私钥、Token、密码、`.env`、生产地址、调试日志或无关文件。

### 3.3 执行本地质量检查

```powershell
gofmt -w cmd internal
go vet ./...
go test ./... -covermode=atomic -coverprofile coverage.out
go tool cover -func coverage.out
go build -buildvcs=false -o bin/server.exe ./cmd/server
```

Windows 没有 CGO 编译器时不能完整执行 `go test -race`；GitHub Ubuntu Runner 会执行该检查，因此本地通过不代表可以跳过远端门禁。

### 3.4 本地接口验证

终端一：

```powershell
go run ./cmd/server
```

终端二：

```powershell
Invoke-RestMethod "http://localhost:8080/healthz"
Invoke-RestMethod "http://localhost:8080/api/calculate?a=7&b=5&operation=add"
Invoke-WebRequest "http://localhost:8080/api/calculate?a=7&b=text&operation=add" -SkipHttpErrorCheck
```

## 4. 提交和推送

```powershell
git status --short
git add internal/httpapi/handler.go internal/httpapi/handler_test.go README.md docs/GITHUB-ACTIONS-CODE-CHECK-AND-MERGE-RUNBOOK.md
git diff --cached --check
git diff --cached
git commit -m "feat: add calculator API"
git push -u origin feature/add-calculator-api
```

一个 commit 应表达一个完整目的。不要使用 `git add .` 盲目提交工作区所有文件。

## 5. 创建 Pull Request

在 GitHub 仓库点击 `Compare & pull request`，配置：

- Base：`main`
- Compare：`feature/add-calculator-api`
- 标题：`feat: add calculator API`
- Reviewer：非代码作者

PR 描述建议：

```markdown
## 变更内容
- 新增 GET /api/calculate
- 支持 add、subtract、multiply
- 增加成功、参数错误和方法错误测试

## 风险范围
- 只新增路由，不修改现有接口
- 输入解析为 int64
- 当前版本不处理算术溢出，超大输入不属于接口契约

## 测试证据
- go vet ./...
- go test ./... -covermode=atomic -coverprofile coverage.out
- GitHub Native Quality Gate

## 回滚方案
- revert 本 PR 的 squash merge commit
```

## 6. GitHub Actions 执行内容

PR 创建、重新提交或重新打开后触发 `GitHub Native Quality Gate`：

| Job | 执行内容 | 阻断条件 |
| --- | --- | --- |
| Go quality and tests | gofmt、go vet、单测、race、覆盖率、build | 任一步失败或覆盖率低于 70% |
| Go vulnerability scan | govulncheck 可达漏洞扫描 | 存在可达漏洞 |
| Repository security scan | Trivy 依赖、Secret、配置扫描 | HIGH/CRITICAL 发现 |
| Quality Gate | 汇总上述结果 | 任一前置 Job 非 success |

在 PR 的 `Checks` 或 `Actions` 页面打开运行记录：

1. 确认运行的 commit SHA 等于 PR 最新 SHA。
2. 展开每个 Job，确认不是 skipped 或 cancelled。
3. 下载 `go-coverage` 与 `trivy-report` Artifact。
4. 失败时读取第一个真实失败步骤，修复后重新 commit/push。
5. 不使用重新运行掩盖可复现失败；基础设施偶发失败可以 rerun，并在 PR 留下说明。

## 7. 配置 main Ruleset

工作流至少成功运行一次后，在仓库执行：

```text
Settings
-> Rules
-> Rulesets
-> New branch ruleset
```

推荐配置：

- Target branch：`main`
- Require a pull request before merging
- Required approvals：至少 1
- Dismiss stale approvals
- Require review from Code Owners
- Require conversation resolution
- Require status checks：`Quality Gate`
- Require branches to be up to date before merging
- Block force pushes
- Restrict deletions
- Bypass 仅授予专用例外审批组

先使用 `Evaluate` 验证影响，再切换 `Active`。个人私人仓库的 Ruleset 能力取决于 GitHub 套餐；如果界面没有对应功能，不能把“管理员自觉不合并”视为等价强制控制。

## 8. 处理失败门禁

正常处理：

```powershell
# 修改代码
gofmt -w cmd internal
go test ./...
git add <明确文件列表>
git commit -m "fix: address quality gate failure"
git push
```

新 push 会产生新 SHA 并重新触发检查。旧 SHA 的成功结果不能用于新 SHA。

扫描器安装或编译失败表示检查没有执行，不能通过 `continue-on-error: true` 忽略。例如旧版 `govulncheck v1.1.3` 依赖的 `x/tools v0.23.0` 无法使用 Go 1.25 编译，应升级并锁定为兼容组合 `Go 1.25.x + govulncheck v1.6.0`。只有扫描器成功执行后报告的发现，才属于需要修复或申请例外的安全结果。

确认误判时执行可审计例外流程：保留失败运行，创建 Exception Request Issue，由非作者审批，并由 Ruleset bypass 专用角色合并。不得修改检查状态。完整规则见 `docs/GITHUB-NATIVE-QUALITY-GATE-PLAN.md`。

## 9. 合并 main

只有以下条件同时满足才点击 `Squash and merge`：

- `Quality Gate` 成功；
- PR 是最新 `main` 基线；
- Reviewer 和 Code Owner 已批准；
- 所有讨论已解决；
- PR 中已有测试和回滚证据；
- 没有未处理的安全发现或已过期例外。

合并标题使用：

```text
feat: add calculator API (#<PR-number>)
```

合并后不要立即删除证据。等待 `push: main` 触发的第二次 Actions 运行成功，再删除功能分支。

## 10. 合并后验证

```powershell
git switch main
git pull --ff-only origin main
git log --oneline -5
go test ./...
```

在 GitHub 验证：

- PR 状态为 Merged；
- merge commit/squash commit 位于 `main`；
- `main` 的 GitHub Actions 运行成功；
- Actions 运行 SHA 与 `main` 最新 SHA 相同；
- 覆盖率和 Trivy Artifact 可下载；
- 如果后续部署，部署记录使用该 SHA 构建的同一制品 digest。

接口部署后执行：

```powershell
Invoke-RestMethod "https://<test-host>/healthz"
Invoke-RestMethod "https://<test-host>/api/calculate?a=7&b=5&operation=add"
```

## 11. 本次演练证据清单

- 功能分支名称和 commit SHA；
- PR 编号和 URL；
- PR 最新 SHA 的四个检查结果；
- 覆盖率百分比与 Artifact；
- Trivy 报告；
- Reviewer/Code Owner 审批；
- squash commit SHA；
- `main` 合并后 Actions run；
- 本地或测试环境接口响应；
- 如发生失败或例外，对应 Issue、审批和修复记录。

## 12. 回滚

代码尚未部署时，在 GitHub 对已合并 PR 创建 Revert PR，并再次通过完整门禁。代码已经部署时：

1. 停止继续推广当前制品；
2. 回滚到上一个已验证镜像 digest；
3. 验证 health、readiness 和关键接口；
4. 创建 Revert PR 修正 `main`；
5. 保存事故、回滚、监控和恢复时间证据。

禁止直接强推或重写 `main` 历史进行回滚。
