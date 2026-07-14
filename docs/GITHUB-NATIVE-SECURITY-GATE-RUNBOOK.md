# GitHub Native Security Gate Runbook

This runbook turns the repository workflows into an enforceable pull-request gate. Repository YAML cannot enable GitHub security products or alter a branch ruleset; an administrator must complete the GitHub steps below.

## Repository-managed checks

| Check | Workflow | Blocks |
| --- | --- | --- |
| Service detection | `Detect affected services` | Runs only changed services; CI/shared/unknown paths fan out to all services |
| Go formatting, vet, race tests, coverage, build | `GitHub Native Quality Gate` | The aggregate `Quality Gate` check |
| Node.js install, lint, tests with coverage, build | `GitHub Native Quality Gate` | The aggregate `Quality Gate` check |
| Java Maven `verify` or Gradle `check build` | `GitHub Native Quality Gate` | The aggregate `Quality Gate` check |
| Reachable Go vulnerabilities | `govulncheck` job, per changed Go service | The aggregate `Quality Gate` check |
| Filesystem dependency, secret, and misconfiguration scan | `trivy` job | The aggregate `Quality Gate` check |
| Newly introduced vulnerable dependencies | `dependency-review` job on pull requests | The aggregate `Quality Gate` check |
| Go semantic security and quality analysis | `CodeQL` workflow | Code-scanning rule in the GitHub ruleset |
| Dependency update pull requests | Dependabot | Review and merge through the same gate |

## Monorepo service convention

### Service detection and language contract

The quality workflow discovers services without a hard-coded service list:

| Service type | Discovery file | Required service contract |
| --- | --- | --- |
| Go | `go.mod` | `gofmt`, `go vet`, race tests, at least 70% coverage, and `go build ./...` must pass |
| Node.js / TypeScript | `package.json` | A committed `package-lock.json`, `pnpm-lock.yaml`, or `yarn.lock`, plus `lint`, `test`, and `build` scripts |
| Java / Kotlin | `pom.xml`, `build.gradle`, or `build.gradle.kts` | Maven wrapper / Maven supports `verify`, or Gradle wrapper / Gradle supports `check build` |

Place independently deployable services under `services/<service-name>/`. A root-level manifest is treated as the root application. A change inside `services/` checks only the matching service; changes to `.github/workflows/`, `scripts/ci/`, `ci/`, `libs/`, `shared/`, `go.work`, or an unrecognized path check every discovered service. This conservative fallback prevents an incorrectly classified shared change from bypassing a gate.

For Java coverage, configure JaCoCo and make its threshold part of Maven `verify` or Gradle `check`; for Node, make the `test` script enforce its own coverage threshold (for example, Vitest/Jest configuration). The central workflow intentionally does not invent per-service test or coverage tool settings.

The current Go security baseline is Go `1.25.12`; a module's `go` directive remains its source-language compatibility contract. Update the baseline through a reviewed dependency/toolchain PR rather than using a floating Go version.

When a new non-Go service is added, extend `codeql.yml` with its CodeQL language entry: `javascript-typescript` for Node.js/TypeScript and `java-kotlin` for Java/Kotlin. JavaScript/TypeScript uses CodeQL `none` build mode; compiled languages should use `manual` or a verified `autobuild` mode. Keep this as a separate, reviewed change because CodeQL build capture must match the service build layout.

## Quality-gate exception procedure

An exception is an audited, temporary release decision. It is **not** a workflow skip, `continue-on-error`, ignored scanner result, or a change that turns a failed check green.

1. The PR author opens **Issues -> New issue -> Quality gate exception** using the repository form. Link the PR, tested commit SHA, failing check URL, rule/advisory ID and expiry; never paste a secret.
2. The author creates a remediation issue or PR and records concrete compensating controls (for example feature flag off by default, network restriction, monitoring, or a tested rollback).
3. A security owner and release owner independently approve the issue. The requester cannot approve their own request. For a High/Critical finding, require the service owner as a third approver; secret-scanning findings require credential revocation/rotation and should normally be rejected rather than bypassed.
4. Only members of a dedicated `ci-exception-approvers` team may bypass the `main` ruleset. In the bypass reason, enter the exception issue URL and expiry. Do not add ordinary developers to this bypass list.
5. Use a maximum seven-day expiry for High/Critical or production-impacting exceptions. When the expiry passes, close the exception and remove any temporary bypass; the remediation PR must again pass the ordinary gates.
6. Retain the issue approvals, bypass audit event, PR URL, commit SHA, scanner evidence, remediation link, and rollback outcome. Review open exceptions weekly.

Configure the ruleset bypass list to contain only `ci-exception-approvers` (and, if required, a break-glass administrator group). This is the enforcement point; YAML alone cannot grant or revoke a GitHub ruleset bypass.

## Administrator configuration in GitHub

### 1. Enable security products

Open **Repository → Settings → Advanced Security**.

1. Enable **Code Security**. This is required before the CodeQL workflow can upload code-scanning results for a private repository.
2. Enable **Dependabot alerts** and **Dependabot security updates**.
3. Enable **Secret Protection**, then enable **Push Protection** for the repository.
4. If the organization supports it, configure delegated bypass so a developer cannot approve their own Push Protection bypass.

Create the repository labels `quality-gate-exception` and `security-review-required` so exception requests are visibly triaged.

Do not add real credentials to a test branch. Push Protection is intentionally outside workflow YAML because it blocks a push before Actions can start.

### 2. Create the `main` ruleset

Open **Repository → Settings → Rules → Rulesets → New branch ruleset**. Target `main` and configure:

1. Require a pull request before merging.
2. Require one approving review and dismiss stale approvals.
3. Require the status check named `Quality Gate`.
4. Enable **Require code scanning results** for tool `CodeQL` and select `High` as the blocking threshold. A CodeQL job that finishes successfully may still report findings, so this rule is mandatory.
5. Block force pushes and restrict ordinary maintainers from bypassing the ruleset.

Add `CODEOWNERS` only after the organization has named the responsible teams; do not add placeholder owners that would block every PR.

## End-to-end validation

Perform every negative test in a disposable branch and close the pull request without merging it.

### POC-01: Normal PR

1. Create a branch from `main` and make a harmless README change.
2. Open a pull request to `main`.
3. Confirm the PR shows successful `Quality Gate` and `CodeQL` checks.
4. Confirm Dependabot and CodeQL results appear in **Security and quality**.
5. Confirm merge becomes available only after the required review is added.

### POC-02: Dependency Review blocks an introduced vulnerable module

1. Create a disposable branch from `main`.
2. Add a dependency version that GitHub Advisory Database marks as High or Critical; use an advisory verified in the repository's Dependency Review result, never a production dependency.
3. Open a pull request and confirm `Dependency review` fails and consequently `Quality Gate` fails.
4. Remove the test dependency and close the pull request without merging it.

### POC-03: CodeQL merge protection

1. Create a disposable branch and add a minimal, non-executed Go test fixture that CodeQL identifies with a High or Critical result under `security-extended`.
2. Confirm the CodeQL workflow completes and the finding appears in **Security and quality → Code scanning**.
3. Confirm the ruleset prevents merging because of the CodeQL severity threshold.
4. Delete the fixture and close the pull request without merging it.

Do not weaken the ruleset merely to merge the test PR. Use the organization exception process if a bypass must be demonstrated.

### POC-04: Push Protection

1. Use only a provider-documented revoked test credential or test pattern that is explicitly supported by GitHub Push Protection; do not invent a token-like value because it may not match a supported pattern.
2. Attempt to push it only from a disposable branch.
3. Confirm GitHub blocks the push before a workflow run is created.
4. Remove the test file locally before retrying the push. Do not bypass and do not allow the value into repository history.

## Evidence to retain

- PR URLs and the tested commit SHA.
- Successful `Quality Gate` and CodeQL run URLs.
- A failed Dependency Review PR and its remediation/closure.
- Screenshot or audit event showing Push Protection blocked a push.
- Ruleset configuration and approval evidence.

## License boundary

For private repositories, CodeQL code scanning requires GitHub Code Security, and Secret Scanning / Push Protection require GitHub Secret Protection. If those products are not licensed, the workflows remain useful for tests, `govulncheck`, Trivy, and Dependency Review where available, but CodeQL result upload and GitHub secret controls cannot be accepted as validated.
