# OCR Divide Bug Test Implementation Plan

> **For agentic workers:** This plan is executed inline in the current session.

**Goal:** Add a calculator divide operation with an intentional divide-by-zero defect for validating Open Code Review comments.

**Architecture:** Extend the existing `GET /api/calculate` operation switch. Add only the successful division case to the existing table-driven handler test; leave zero-divisor validation absent by design so OCR can identify it.

**Tech Stack:** Go 1.21, `net/http`, `httptest`, standard `go test`.

## Global Constraints

- Keep the change isolated to the test branch.
- Do not add divide-by-zero handling; the missing guard is the review target.
- Do not add a zero-divisor test that would force the defect to be fixed.

### Task 1: Add divide behavior and its normal-path test

**Files:**
- Modify: `internal/httpapi/handler.go`
- Modify: `internal/httpapi/handler_test.go`

- [ ] Add `case "divide": result = left / right` to the existing operation switch.
- [ ] Add a table entry for `a=10&b=2&operation=divide`, expecting HTTP 200 and result 5.
- [ ] Run `gofmt -w internal/httpapi/handler.go internal/httpapi/handler_test.go`.
- [ ] Run `go test ./internal/httpapi -run 'Test(Calculate|CalculateRejectsPost)$'` and confirm the normal calculator tests pass.
- [ ] Run `git diff --check` and inspect the diff before committing.

### Task 2: Commit the test change

**Files:**
- Commit: `internal/httpapi/handler.go`, `internal/httpapi/handler_test.go`

- [ ] Commit with `git add internal/httpapi/handler.go internal/httpapi/handler_test.go; git commit -m "feat: add divide calculation for OCR test"`.
- [ ] Confirm the branch is `test/open-code-review-bug` and show the final commit with `git status -sb` and `git log -2 --oneline`.
