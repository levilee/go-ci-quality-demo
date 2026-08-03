# Open Code Review GitHub Actions 操作手册

## 功能范围

本仓库的 Open Code Review 只支持手动触发，不监听 PR 创建、PR 更新或 push。它只发表评论，不是 `Quality Gate`，也不阻断合并。

## 配置 GitHub Secrets 和 Variables

进入仓库：`Settings -> Secrets and variables -> Actions`。

Secrets：

| 名称 | 内容 |
| --- | --- |
| `OCR_LLM_URL` | LLM API endpoint，例如 OpenAI 兼容接口地址 |
| `OCR_LLM_AUTH_TOKEN` | LLM API token |

Variables：

| 名称 | 内容 |
| --- | --- |
| `OCR_LLM_MODEL` | 使用的模型名称 |
| `OCR_LLM_USE_ANTHROPIC` | Anthropic 使用 `true`，OpenAI 兼容接口使用 `false` |

不要把 Token 放到 Variables、YAML、PR 评论或代码中。

## 方式一：PR 评论触发

1. 打开目标 PR。
2. 确认 PR 仍处于 Open 状态。
3. 在 Conversation 中发表评论：

   ```text
   /open-code-review
   ```

4. 在 PR 的 Checks 或 Actions 中查看 `Open Code Review`。
5. 在 Files changed 查看行级评论，在 Conversation 查看摘要评论。

只有仓库 Owner、Member 或 Collaborator 的非 Bot 评论可以触发审查。

## 方式二：Actions 页面触发

1. 打开仓库的 `Actions`。
2. 选择 `Open Code Review`。
3. 点击 `Run workflow`。
4. 输入目标 PR 编号，例如 `123`。
5. 点击运行。

工作流会重新读取 PR 的最新 head SHA，不使用手工填写的 commit SHA。

命令行方式等价于：

```bash
gh workflow run open-code-review.yml --ref main -f pr_number=123
```

## 审查范围

审查范围是：

```text
merge-base(PR base, PR latest head) -> PR latest head
```

工作流只获取 Git 对象并读取差异，不 checkout 或执行 PR 分支中的脚本。

## 结果和 Artifact

- 有效行号的问题：PR `Files changed` 行级评论。
- 无法定位到 diff 行的问题：PR Conversation 摘要评论。
- 重复运行：摘要使用固定标记更新，已存在的行级结果不会重复发布。
- 原始 JSON 和 stderr：Actions run 的 Artifact，保留 7 天。
- 工作流失败：表示 OCR 或评论发布失败；它不是 Required Check，不会改变现有合并门禁。

## 故障排查

- 没有触发：确认评论以 `/open-code-review` 开头，并且触发者是仓库协作者。
- 缺少 Secret：检查 `OCR_LLM_URL` 和 `OCR_LLM_AUTH_TOKEN` 是否配置在当前仓库或可用 Environment。
- 模型请求失败：下载 `open-code-review-*` Artifact，先查看 `ocr-stderr.log`。
- PR 编号错误：使用当前仓库中仍为 Open 的 PR 编号，不要输入 URL 或 commit SHA。
- 评论未出现：确认工作流具有 `pull-requests: write`，再检查 Action 日志和 Artifact。
- Actions 页面没有 `Run workflow`：确认该工作流文件已推送到当前分支，并且仓库允许手动运行 Actions。
