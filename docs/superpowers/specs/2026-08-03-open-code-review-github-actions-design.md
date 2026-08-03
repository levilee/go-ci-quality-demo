# Open Code Review GitHub Actions 集成设计

## 背景

当前仓库已经有 GitHub Native Quality Gate、CodeQL 和 Dependabot，但没有把 Open Code Review 集成到 GitHub Pull Request 流程中。目标是在不改变现有质量门禁的前提下，为指定 PR 提供可重复的 LLM 代码审查结果。

## 目标

- 支持两种手动触发方式：
  1. 在 PR 评论中输入 `/open-code-review`。
  2. 在 GitHub Actions 页面手动运行工作流并输入 PR 编号。
- 每次运行都读取指定 PR 的最新 head SHA。
- 只审查 PR 相对于目标分支 merge-base 的变更内容。
- 将审查结果发布为 PR 行级评论和摘要评论。
- 使用 LLM Secret，不在仓库或日志中暴露凭据。
- 不自动响应 PR 创建、PR 更新或 push。
- 不将 Open Code Review 设为 Required Check，不参与现有 `Quality Gate` 的合并阻断。

## 非目标

- 不自动修改 PR 代码。
- 不自动批准、拒绝或合并 PR。
- 不把 Open Code Review 结果转换为合并门禁。
- 不执行 PR 分支中的构建脚本、测试脚本或部署脚本。

## 方案对比与选择

### 方案 A：PR 评论命令

工作流监听 `issue_comment: created`，只有评论属于 Pull Request、以 `/open-code-review` 开头、评论者为 `MEMBER`、`OWNER` 或 `COLLABORATOR` 且不是 Bot 时才执行审查。

该路径使用官方 `alibaba/open-code-review` Composite Action。Action 能够从 PR 事件上下文解析目标 PR，获取 base/head Git 对象，运行 OCR 并发布行级评论和摘要评论。

### 方案 B：Actions 页面输入 PR 编号

工作流监听 `workflow_dispatch`，要求输入正整数 `pr_number`。工作流通过 GitHub API 查询 PR 状态、base 分支和最新 head SHA，然后获取完整历史与 PR head Git 对象，计算 merge-base，并运行官方 CI 模式命令：

```text
ocr review --from <merge-base> --to <head-sha> --format json --audience agent
```

由于 `workflow_dispatch` 本身没有 Pull Request 上下文，不能直接依赖官方 Action 的 PR 评论回写上下文。因此该路径使用仓库内的小型 JSON 结果发布器，将 OCR 结果转换为 PR Review API 的行级评论和 PR 摘要评论。

### 选择结果

同时实现方案 A 和方案 B。方案 A 最大限度复用官方 Action；方案 B 保留 Actions 页面手动触发能力，并严格使用官方 CI 命令格式。

## 架构与文件职责

- `.github/workflows/open-code-review.yml`
  - 声明 `issue_comment` 和 `workflow_dispatch` 两个手动入口。
  - 设置最小 GitHub 权限、PR 级并发控制和超时。
  - 为评论触发调用官方 Action。
  - 为页面触发解析 PR、获取 Git 对象、运行 OCR、上传 Artifact，并调用结果发布器。
- `scripts/ci/post-open-code-review.js`
  - 读取 OCR JSON 和 stderr。
  - 校验评论目标、行号和基本字段。
  - 将有有效行号的问题发布为 PR Review 行级评论。
  - 将没有有效行号的问题、警告和失败信息发布到 PR 摘要评论。
  - 使用固定 HTML 标记更新摘要并避免同一结果重复发布。
- `docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md`
  - 记录 GitHub Secrets、Variables、两种触发方法、结果位置、权限和故障排查。
- `docs/superpowers/specs/2026-08-03-open-code-review-github-actions-design.md`
  - 保存本设计及验收标准。

## 触发与数据流

### 评论触发

```text
PR 评论 /open-code-review
  -> issue_comment 工作流
  -> PR、作者关系和 Bot 校验
  -> 官方 Open Code Review Action
  -> 当前 PR merge-base 到最新 head SHA
  -> LLM 审查
  -> 行级评论 + 摘要评论 + Artifact
```

### Actions 页面触发

```text
Actions -> Open Code Review -> Run workflow -> pr_number
  -> GitHub API 获取 PR
  -> 校验 PR 属于当前仓库且仍为 open
  -> checkout base 分支完整历史
  -> fetch pull/<number>/head
  -> 计算 merge-base
  -> OCR JSON 模式审查
  -> 发布行级评论 + 摘要评论 + Artifact
```

两条路径都只读取 base 分支和 PR head 的 Git 对象，不 checkout 或执行 PR 分支中的不可信代码。每次触发都从 GitHub API 重新读取最新 head SHA，避免审查旧版本。

## GitHub 权限与凭据

工作流使用：

- `contents: read`
- `pull-requests: write`

仓库需要配置：

- Secret `OCR_LLM_URL`：LLM API 地址。
- Secret `OCR_LLM_AUTH_TOKEN`：LLM API Token。
- Variable `OCR_LLM_MODEL`：模型名称。
- Variable `OCR_LLM_USE_ANTHROPIC`：使用 Anthropic 时为 `true`，OpenAI 兼容接口时为 `false`。

Token 只通过 `secrets` 注入 Action 或进程环境，不写入 YAML、脚本、Artifact 或日志。Open Code Review Action 和 OCR CLI 均使用固定版本；Action 使用不可变提交 SHA，CLI 使用明确版本号。

## 评论行为

- 有有效文件路径和行号的问题：发布到 PR 的 Files changed 行级评论。
- 无法定位到当前 diff 行的问题：发布到 PR Conversation 摘要评论。
- 没有问题：发布成功摘要，说明未生成评论。
- 重复运行：摘要使用固定标记更新；行级评论使用结果标记避免重复发布，保留历史评论，不删除人工讨论。
- 建议代码：若 OCR 返回 `suggestion_code`，转换为 GitHub suggestion block；无建议时只发布普通评论。

## 失败处理

- LLM 配置错误、网络错误、OCR 非零退出或 JSON 解析失败：保留 stderr、原始 JSON（若存在）和运行日志。
- 失败不伪造成无问题，也不修改 `Quality Gate` 状态。
- 工作流可以显示失败以便诊断，但不作为 Required Check，因此不阻断 PR 合并。
- 同一 PR 的新审查取消仍在运行的旧审查，避免过期结果覆盖最新结果。
- 不自动重试会产生额外 LLM 成本的审查；用户可在修复配置后重新手动触发。

## 验收标准

1. 新 PR 创建或更新时，Open Code Review 工作流不会自动运行。
2. PR 中输入 `/open-code-review` 后，只有授权协作者能触发审查。
3. 普通 PR 评论、Issue 评论和 Bot 评论不会消耗 LLM 配额。
4. Actions 页面输入合法 PR 编号后，能审查该 PR 的最新 head。
5. 两种触发方式都只审查目标分支 merge-base 到 PR head 的变更。
6. 行级问题出现在 Files changed，无法定位的问题出现在摘要评论。
7. 原始 OCR 结果和 stderr 可从 Artifact 获取。
8. 缺少 LLM Secret、关闭的 PR、非法 PR 编号和不存在的 PR 会得到明确错误。
9. Open Code Review 工作流未被添加到现有 `Quality Gate` 的 Required Check 或聚合逻辑。
10. `git diff --check`、工作流 YAML 静态校验和 JavaScript 语法校验通过。

## 官方依据

- Open Code Review CI 文档：<https://open-codereview.ai/docs/cicd>
- 官方 GitHub Actions 示例：<https://github.com/alibaba/open-code-review/tree/main/examples/github_actions>
- 官方 Action 输入与实现：<https://github.com/alibaba/open-code-review/blob/main/action.yml>
