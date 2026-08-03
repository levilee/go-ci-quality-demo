# Open Code Review GitHub Actions Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two manual GitHub Actions entry points that review the latest changes of a selected Pull Request with Open Code Review and publish comments without affecting merge protection.

**Architecture:** Use the official `alibaba/open-code-review` composite Action for authorized `/open-code-review` PR comments. Use the documented OCR CLI JSON mode plus a repository-owned Node publisher for `workflow_dispatch`, because a manually dispatched workflow has no native PR event context for comment posting. Both paths read the latest PR head SHA, review from the target branch merge-base, and use non-required permissions.

**Tech Stack:** GitHub Actions YAML, Open Code Review Action pinned to `1b193db3587e4d3f429bd8c8213479d63e3b4f21`, `@alibaba-group/open-code-review` `1.7.16`, Node.js 24 CommonJS, GitHub REST Pull Request Review API, Node built-in `node:test`.

## Global Constraints

- Only `issue_comment` with `/open-code-review` and `workflow_dispatch` with `pr_number` may trigger review.
- Do not add `pull_request`, `pull_request_target`, or `push` triggers to the Open Code Review workflow.
- Review `merge-base(base, latest_head) -> latest_head`; never use a user-supplied head SHA.
- Do not execute code from the PR head; fetch its Git object and inspect the diff only.
- Workflow permissions are `contents: read` and `pull-requests: write`.
- Open Code Review is not part of the existing `Quality Gate` job and is not a Required Check.
- LLM credentials are GitHub Secrets/Variables only; no values are committed or printed.
- Pin third-party Action references to immutable SHAs: OCR `1b193db3587e4d3f429bd8c8213479d63e3b4f21`, GitHub Script `f28e40c7f34bde8b3046d885e986cb6290c5673b`, setup-node `a0853c24544627f65ddf259abe73b1d18a591444`, and upload-artifact `ea165f8d65b6e75b540449e92b4886f43607fa02`.
- Use `continue-on-error: true` at the review jobs so an OCR/commenting failure is visible but cannot block merging.

---

### Task 1: Add and test the manual-review result publisher

**Files:**
- Create: `scripts/ci/post-open-code-review.js`
- Create: `scripts/ci/post-open-code-review.test.js`

**Interfaces:**
- Consumes: OCR JSON at `resultPath`, OCR stderr at `stderrPath`, `pullNumber`, `headSha`, and a GitHub Script Octokit client.
- Produces: `postOpenCodeReview({ github, owner, repo, pullNumber, headSha, resultPath, stderrPath, ocrExitCode })`, plus pure helper exports used by unit tests.

- [ ] **Step 1: Write the failing publisher tests**

Create `scripts/ci/post-open-code-review.test.js` with Node's built-in test runner:

```js
const assert = require('node:assert/strict');
const test = require('node:test');

const {
  SUMMARY_MARKER,
  classifyComments,
  buildInlinePayload,
  buildSummaryBody,
  commentMarker,
} = require('./post-open-code-review.js');

test('classifies line comments as inline and missing-line comments as summary', () => {
  const classified = classifyComments([
    { path: 'internal/httpapi/handler.go', content: 'Use a typed error.', start_line: 12, end_line: 12 },
    { path: 'README.md', content: 'Document the new behavior.' },
  ]);

  assert.equal(classified.inline.length, 1);
  assert.equal(classified.summary.length, 1);
  assert.equal(classified.inline[0].path, 'internal/httpapi/handler.go');
});

test('builds a GitHub right-side inline comment with a deterministic marker', () => {
  const item = {
    path: 'internal/httpapi/handler.go',
    content: 'Use a typed error.',
    startLine: 12,
    endLine: 14,
  };
  const marker = commentMarker(item);
  const payload = buildInlinePayload(item);

  assert.match(marker, /^<!-- open-code-review-inline:[0-9a-f]{16} -->$/);
  assert.deepEqual(payload, {
    path: item.path,
    start_line: 12,
    start_side: 'RIGHT',
    line: 14,
    side: 'RIGHT',
    body: `${marker}\n${item.content}`,
  });
});

test('builds a stable summary body for findings and warnings', () => {
  const body = buildSummaryBody({
    pullNumber: 7,
    headSha: 'abc123',
    inlinePosted: 1,
    skipped: 2,
    summaryItems: [{ path: 'README.md', content: 'Document the endpoint.' }],
    warnings: ['One finding had no line information.'],
    failure: '',
  });

  assert.match(body, new RegExp(SUMMARY_MARKER.replace(/[.*+?^${}()|[\\]\\\\]/g, '\\\\$&')));
  assert.match(body, /PR #7/);
  assert.match(body, /abc123/);
  assert.match(body, /README\.md/);
  assert.match(body, /One finding had no line information/);
});

- [ ] **Step 2: Run the focused tests and verify they fail**

Run:

```powershell
node --test scripts/ci/post-open-code-review.test.js
```

Expected: FAIL because `post-open-code-review.js` does not yet exist.

- [ ] **Step 3: Implement the publisher with idempotent summary and inline posting**

Create `scripts/ci/post-open-code-review.js`. It must export `SUMMARY_MARKER`, `classifyComments`, `buildInlinePayload`, `buildSummaryBody`, `commentMarker`, and `postOpenCodeReview`.

Implementation requirements:

```text
classifyComments(comments)
  - Accept result.comments or an empty array.
  - Normalize path/content/start_line/end_line.
  - Put valid positive line ranges into inline.
  - Put missing/invalid path, content, or line ranges into summary.

commentMarker(item)
  - SHA-256 hash [path, startLine, endLine, content].
  - Return <!-- open-code-review-inline:<first-16-hex-chars> -->.

buildInlinePayload(item)
  - Return path, line=endLine, side=RIGHT, and body with the marker.
  - Include start_line/start_side=RIGHT for multi-line comments.
  - Append suggestion_code as a GitHub suggestion block when present.

buildSummaryBody(input)
  - Start with <!-- open-code-review-summary -->.
  - Include PR number, head SHA, posted/skipped counts, warnings, and summary findings.
  - Report OCR failure instead of reporting a clean review.

postOpenCodeReview(input)
  - Read resultPath and stderrPath only when they exist.
  - Treat a non-zero ocrExitCode or malformed JSON as failure.
  - List existing Pull Request review comments and skip matching inline markers.
  - Use pulls.createReview with commit_id=headSha and event=COMMENT for fresh inline comments.
  - If GitHub rejects inline comments, move those findings into the summary.
  - List issue comments and update the comment containing SUMMARY_MARKER, or create it.
  - Never delete existing review comments.
```

Use only `node:fs` and `node:crypto`; the GitHub Script step supplies the Octokit client.

- [ ] **Step 4: Run the focused tests and syntax checks**

```powershell
node --test scripts/ci/post-open-code-review.test.js
node --check scripts/ci/post-open-code-review.js
```

Expected: all tests pass and `node --check` exits 0.

- [ ] **Step 5: Commit the publisher**

```powershell
git add scripts/ci/post-open-code-review.js scripts/ci/post-open-code-review.test.js
git commit -m "feat: add Open Code Review result publisher"
```

### Task 2: Add the two manual GitHub Actions triggers

**Files:**
- Create: `.github/workflows/open-code-review.yml`

**Interfaces:**
- Consumes: `OCR_LLM_URL`, `OCR_LLM_AUTH_TOKEN`, `OCR_LLM_MODEL`, `OCR_LLM_USE_ANTHROPIC`, and optional `workflow_dispatch.inputs.pr_number`.
- Produces: PR review comments, a sticky summary comment, OCR artifacts, and a visible but non-required workflow result.

- [ ] **Step 1: Add the workflow**

Create a workflow with the following exact behavior:

```yaml
name: Open Code Review

on:
  issue_comment:
    types: [created]
  workflow_dispatch:
    inputs:
      pr_number:
        description: Pull Request number to review
        required: true
        type: string

permissions:
  contents: read
  pull-requests: write

concurrency:
  group: >-
    ${{
      (
        github.event_name == 'workflow_dispatch'
        || (
          github.event_name == 'issue_comment'
          && github.event.issue.pull_request
          && github.event.comment.user.type != 'Bot'
          && (
            github.event.comment.author_association == 'MEMBER'
            || github.event.comment.author_association == 'OWNER'
            || github.event.comment.author_association == 'COLLABORATOR'
          )
          && startsWith(github.event.comment.body, '/open-code-review')
        )
      )
      && format('open-code-review-pr-{0}', github.event.inputs.pr_number || github.event.issue.number)
      || format('open-code-review-noop-{0}', github.run_id)
    }}
  cancel-in-progress: true
```

Add two jobs:

1. `comment-review`
   - Condition: `issue_comment`, `github.event.issue.pull_request`, non-Bot, author association `MEMBER`/`OWNER`/`COLLABORATOR`, and `startsWith(github.event.comment.body, '/open-code-review')`.
   - `continue-on-error: true`, `runs-on: ubuntu-latest`, timeout 30 minutes.
   - Use `alibaba/open-code-review@1b193db3587e4d3f429bd8c8213479d63e3b4f21`.
   - Pass `github_token`, the two LLM Secrets, the two LLM Variables, `language: Chinese`, `ocr_version: 1.7.16`, `review_concurrency: '4'`, `sticky_summary: 'true'`, `incremental: 'true'`, and `upload_artifacts: 'true'`.

2. `manual-review`
   - Condition: `github.event_name == 'workflow_dispatch'`.
   - `continue-on-error: true`, `runs-on: ubuntu-latest`, timeout 30 minutes.
   - Use pinned `actions/github-script` to validate `inputs.pr_number`, query the PR, require `state == 'open'`, and output `number`, `base_ref`, `head_sha`, and `title`.
   - Checkout only the trusted base branch with pinned `actions/checkout` and `fetch-depth: 0`.
   - Fetch `pull/<number>/head` without checking it out; verify the output head SHA with `git cat-file -e`.
   - Install Node.js 24 and `@alibaba-group/open-code-review@1.7.16`.
   - Run the documented command with environment variables for the LLM:

```bash
git fetch --no-tags origin "$BASE_REF"
MERGE_BASE="$(git merge-base "origin/${BASE_REF}" "${HEAD_SHA}")"
ocr review \
  --from "${MERGE_BASE}" \
  --to "${HEAD_SHA}" \
  --format json \
  --audience agent \
  --background "${PR_TITLE}" \
  >"${RUNNER_TEMP}/ocr-result.json" \
  2>"${RUNNER_TEMP}/ocr-stderr.log"
```

   - Capture the OCR exit code and output paths as step outputs without exposing Secrets.
   - Upload both output files with pinned `actions/upload-artifact`, retention 7 days.
   - Run pinned `actions/github-script` with `if: always()` and require `./scripts/ci/post-open-code-review.js`; pass `pullNumber`, `headSha`, paths, and exit code through environment variables.
   - End with a failure step when OCR exit code is non-zero; the job-level `continue-on-error` keeps it non-blocking.

- [ ] **Step 2: Validate the workflow policy**

```powershell
git diff --check
if (Get-Command actionlint -ErrorAction SilentlyContinue) { actionlint .github/workflows/open-code-review.yml }
rg -n "pull_request:|pull_request_target:|push:|workflow_dispatch|issue_comment|pull-requests: write|continue-on-error|open-code-review@" .github/workflows/open-code-review.yml
```

Expected: only the two manual triggers are present, the workflow has PR write permission, and no `Quality Gate` integration is added.

- [ ] **Step 3: Commit the workflow**

```powershell
git add .github/workflows/open-code-review.yml
git commit -m "ci: add manual Open Code Review workflow"
```

### Task 3: Add the Chinese GitHub operation runbook and README link

**Files:**
- Create: `docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md`
- Modify: `README.md`

**Interfaces:**
- Consumes: the workflow input and the four GitHub Actions Secret/Variable names.
- Produces: operator instructions for both manual trigger paths, result locations, and failure diagnosis.

- [ ] **Step 1: Add the runbook**

Create `docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md` with these required sections:

```markdown
# Open Code Review GitHub Actions 操作手册

## 功能范围

本仓库的 Open Code Review 只支持手动触发，不监听 PR 创建、PR 更新或 push。它只发表评论，不是 `Quality Gate`，也不阻断合并。

## GitHub Secrets 和 Variables

在 `Settings -> Secrets and variables -> Actions` 配置：

Secrets：`OCR_LLM_URL`、`OCR_LLM_AUTH_TOKEN`

Variables：`OCR_LLM_MODEL`、`OCR_LLM_USE_ANTHROPIC`

不要把 Token 放到 Variables、YAML、PR 评论或代码中。

## 方式一：PR 评论触发

在目标 PR 的 Conversation 中发表评论：

```text
/open-code-review
```

只有仓库 Owner、Member 或 Collaborator 的非 Bot 评论可以触发。

## 方式二：Actions 页面触发

选择 `Actions -> Open Code Review -> Run workflow`，输入打开状态的 PR 编号，例如 `123`。

命令行等价操作：

```bash
gh workflow run open-code-review.yml --ref main -f pr_number=123
```

工作流会重新读取 PR 的最新 head SHA。

## 审查范围和结果

审查范围为 `merge-base(PR base, PR latest head) -> PR latest head`。有效行号的问题发布到 Files changed；无法定位的问题进入 Conversation 摘要；原始 JSON 和 stderr 保留在 Actions Artifact 中 7 天。

## 故障排查

- 没有触发：确认评论以 `/open-code-review` 开头且触发者是协作者。
- 模型失败：下载 `open-code-review-*` Artifact，检查 `ocr-stderr.log`。
- PR 编号错误：输入当前仓库中仍为 Open 的 PR 编号。
- 工作流红色：表示 OCR 或评论发布失败，但不是 Required Check，不会改变现有合并门禁。
```

- [ ] **Step 2: Link the runbook from README**

Add under `README.md`'s `CI quality gates` list:

```markdown
- `.github/workflows/open-code-review.yml` provides manual PR review through `/open-code-review` comments or the Actions page; see [Open Code Review GitHub Actions runbook](docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md).
```

- [ ] **Step 3: Verify and commit documentation**

```powershell
git diff --check
rg -n "open-code-review|OCR_LLM_URL|OCR_LLM_AUTH_TOKEN|pr_number|Quality Gate" README.md docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md
git add README.md docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md
git commit -m "docs: add Open Code Review GitHub runbook"
```

Expected: both trigger procedures and all four configuration names are present.

### Task 4: Run final validation and prepare GitHub-side acceptance

**Files:**
- Verify: `.github/workflows/open-code-review.yml`
- Verify: `scripts/ci/post-open-code-review.js`
- Verify: `scripts/ci/post-open-code-review.test.js`
- Verify: `README.md`
- Verify: `docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md`

- [ ] **Step 1: Run existing Go checks**

```powershell
go test ./...
go vet ./...
```

Expected: PASS; this feature does not modify Go production code.

- [ ] **Step 2: Run publisher and repository checks**

```powershell
node --test scripts/ci/post-open-code-review.test.js
node --check scripts/ci/post-open-code-review.js
git diff --check
git status --short
```

Expected: tests and syntax checks pass, diff check is clean, and only intended files are changed.

- [ ] **Step 3: Review final security properties**

```powershell
git diff origin/feature/add-calculator-api...HEAD -- .github/workflows/open-code-review.yml scripts/ci/post-open-code-review.js scripts/ci/post-open-code-review.test.js README.md docs/OPEN-CODE-REVIEW-GITHUB-ACTIONS-RUNBOOK.md
```

Confirm no hardcoded credentials, no automatic trigger, no `Quality Gate` modification, no PR-head code execution, immutable Action references, and explicit non-blocking behavior.

- [ ] **Step 4: Verify on GitHub**

1. Configure the two Secrets and two Variables.
2. Use a test PR and trigger `/open-code-review`.
3. Verify line comments, summary comment, and Artifact.
4. Trigger the Actions page workflow with the same PR number.
5. Verify the latest head SHA is used and repeated comments are not duplicated.
6. Confirm Open Code Review is not selected as a Required Check and the existing `Quality Gate` is unchanged.

## Plan self-review

- Coverage: Tasks 1–2 implement both trigger paths, Task 3 documents operation, and Task 4 verifies behavior and merge-gate isolation.
- Completeness scan: no unfinished sections or unspecified versions remain; exact Action SHAs and OCR version are pinned.
- Interface consistency: the workflow passes the exact `postOpenCodeReview` parameters produced by the publisher task, and tests import the exported helper names.
