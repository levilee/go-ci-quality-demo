# Open Code Review 历史 PR 报告设计

## 目标

新增一个独立的 GitHub Actions 手动工作流，允许操作者在 Actions 页面输入历史 PR 编号，生成 Open Code Review 报告。

## 范围

- 新增 `.github/workflows/open-code-review-history.yml`。
- 保留现有 `/open-code-review` PR 评论触发流程不变。
- 使用 `workflow_dispatch` 的 `pr_number` 输入选择目标 PR。
- 通过 GitHub API 获取目标 PR 的 base 分支、head SHA、标题和 URL。
- 只比较目标 PR 的 merge-base 到 head SHA，不扫描当前分支工作区。
- 使用 Open Code Review CLI 输出 JSON 结果。

## 报告输出

每次运行生成：

- `ocr-report.md`：面向用户的 Markdown 报告；
- `ocr-result.json`：OCR 原始 JSON 结果；
- `ocr-stderr.log`：OCR 警告和错误输出。

三类输出分别用于：

1. 写入 GitHub Actions Job Summary；
2. 上传 Actions Artifact；
3. 在目标 PR 的 Conversation 中发布一条非阻断摘要评论。

## 失败和安全策略

- OCR 失败仍上传报告和日志，并在 Summary/PR 评论中标记失败；不阻断合并。
- 权限限制为 `contents: read`、`pull-requests: write` 和 `issues: write`。
- 工作流只读取 PR 内容和 Git 对象，不 checkout 或执行 PR 中的业务代码。
- 不自动 approve、request changes、修改代码或改变合并状态。
- PR 编号输入必须对应当前仓库中的 PR；无效编号、无权限或无法获取 head 时明确失败。

## 验证标准

- Actions 页面可手动启动工作流并输入 PR 编号。
- 对包含除零缺陷的测试 PR，报告能够包含 `handler.go` 的问题描述和行号。
- Actions Summary、Artifact 和目标 PR 评论均可查看结果。
- `actionlint` 校验工作流通过。
- OCR 失败时工作流仍保留 Artifact，并明确显示失败原因。
