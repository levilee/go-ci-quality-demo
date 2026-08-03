# Open Code Review Historical PR Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a manual GitHub Actions workflow that reviews any repository PR by number and publishes a Markdown/JSON report to Actions and the selected PR.

**Architecture:** Keep the existing comment-trigger workflow unchanged. Add `open-code-review-history.yml` with `workflow_dispatch`, resolve PR metadata through GitHub API, fetch the PR Git range, run OCR CLI, render a report, upload artifacts, write the Actions Summary, and post a non-blocking comment using the explicit PR number.

**Tech Stack:** GitHub Actions, `actions/github-script@v7`, `actions/checkout@v4`, `actions/upload-artifact@v4`, Node.js, Open Code Review CLI `1.7.16`.

## Global Constraints

- Required input: numeric `pr_number`.
- Permissions: `contents: read`, `pull-requests: write`, `issues: write`.
- Review exactly the selected PR merge-base to head SHA range.
- Never execute code from the selected PR.
- OCR errors are non-blocking but must appear in the report and stderr artifact.
- Produce `ocr-report.md`, `ocr-result.json`, and `ocr-stderr.log`.

---

### Task 1: Add the manual workflow and PR context lookup

**Files:**
- Create: `.github/workflows/open-code-review-history.yml`

**Interfaces:** Consume `inputs.pr_number`; produce `pr_number`, `pr_title`, `pr_url`, `base_sha`, `base_ref`, and `head_sha` step outputs.

- [ ] Add `workflow_dispatch` with required numeric `pr_number`, permissions listed above, and a `review-history` job on `ubuntu-latest` with a 30-minute timeout and `continue-on-error: true`.
- [ ] Add `actions/github-script@v7` to validate the input, call `github.rest.pulls.get`, and set outputs using `core.setOutput('pr_number', String(pr.number))`, `core.setOutput('pr_title', pr.title)`, `core.setOutput('pr_url', pr.html_url)`, `core.setOutput('base_ref', pr.base.ref)`, `core.setOutput('base_sha', pr.base.sha)`, and `core.setOutput('head_sha', pr.head.sha)`.
- [ ] Validate with `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 .github/workflows/open-code-review-history.yml` and commit `ci: add historical PR OCR report workflow`.

### Task 2: Build the selected PR range and run OCR

**Files:**
- Modify: `.github/workflows/open-code-review-history.yml`

**Interfaces:** Consume PR context outputs; produce `merge_base`, `ocr-result.json`, `ocr-stderr.log`, and `ocr_exit_code`.

- [ ] Checkout `steps.pr-context.outputs.base_sha` with `actions/checkout@v4` and `fetch-depth: 0`.
- [ ] Fetch `pull/${PR_NUMBER}/head`, verify `${HEAD_SHA}^{commit}`, and write `merge_base=$(git merge-base "$BASE_SHA" "$HEAD_SHA")` to `$GITHUB_OUTPUT`.
- [ ] Install `@alibaba-group/open-code-review@1.7.16`, set language to Chinese, and run `ocr review --from "$MERGE_BASE" --to "$HEAD_SHA" --format json --audience agent > ocr-result.json 2> ocr-stderr.log` with the existing LLM Secrets/Variables. Capture its exit code and continue so reporting always runs.

### Task 3: Render and publish the report

**Files:**
- Create: `scripts/ci/render-ocr-history-report.js`
- Create: `scripts/ci/render-ocr-history-report.test.js`
- Modify: `.github/workflows/open-code-review-history.yml`

**Interfaces:** The renderer accepts `--result`, `--stderr`, and `--output`; it writes deterministic Markdown containing PR metadata, reviewed range, OCR status, finding count, severity/category, path/line, content, and optional suggestion. Invalid/missing JSON produces an explicit failure section and includes at most 12,000 stderr characters.

- [ ] Add Node tests for one finding, zero findings, and invalid JSON; run `node --test scripts/ci/render-ocr-history-report.test.js`.
- [ ] Run `node scripts/ci/render-ocr-history-report.js --result ocr-result.json --stderr ocr-stderr.log --output ocr-report.md` in an `if: always()` workflow step.
- [ ] Append `ocr-report.md` to `$GITHUB_STEP_SUMMARY` and upload `ocr-report.md`, `ocr-result.json`, and `ocr-stderr.log` with `actions/upload-artifact@v4`.
- [ ] Use `actions/github-script@v7` and `github.rest.issues.createComment` with `issue_number: Number(steps.pr-context.outputs.pr_number)`, a report marker, at most 50,000 report characters, and the Actions run URL. Do not use the reusable Action poster because `workflow_dispatch` has no `github.event.issue.number`.
- [ ] Run actionlint, `git diff --check`, and renderer tests; commit `ci: generate OCR reports for historical PRs` and push `fix/open-code-review-pr-context`.

Expected result: a manual run for PR #12 produces Actions Summary, the three report artifacts, and one non-blocking comment on PR #12.
