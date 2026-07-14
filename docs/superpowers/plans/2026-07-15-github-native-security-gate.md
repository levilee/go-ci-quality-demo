# GitHub Native Security Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub-native PR quality and security controls to the demo, with change-scoped Go, Node.js, and Java service checks, and document the manual GitHub configuration and test evidence.

**Architecture:** `ci.yml` discovers a service from its language manifest, builds a dynamic matrix only for affected services, and retains a stable aggregate `Quality Gate` check. Shared/unknown changes safely fan out to all services. A dedicated CodeQL workflow publishes code-scanning results, and an official dependency-review workflow blocks risky dependency additions. GitHub settings enforce results, scan thresholds, reviews, secret controls, and a controlled ruleset-bypass exception process.

**Tech Stack:** GitHub Actions, GitHub CodeQL, Dependabot, Dependency Review, Go 1.25.12, govulncheck, Trivy.

## Global Constraints

- Pin all third-party Actions to a reviewed full commit SHA before merging.
- Do not add real secrets, GitHub personal access tokens, or `pull_request_target` workflows.
- Do not claim GitHub management-plane controls are enabled until they are verified in the repository UI.

---

### Task 1: Add repository-managed security workflows

**Files:**
- Create: `.github/workflows/codeql.yml`
- Create: `.github/dependabot.yml`
- Modify: `.github/workflows/ci.yml`

- [ ] Add CodeQL initialization and analysis for Go on PRs, pushes to `main`, scheduled scans, and manual dispatch. Document the reviewed extension point for Node.js/TypeScript and Java/Kotlin.
- [ ] Add pull-request dependency review with High/Critical blocking to the existing aggregate gate. License policy is intentionally deferred until the organization has an approved license allow-list.
- [ ] Add weekly Dependabot configuration for `gomod` and `github-actions` ecosystems.
- [ ] Add automatic service discovery for Go, Node.js, and Java manifests. Execute only affected services; fan out for shared/unknown changes.
- [ ] Validate workflow YAML, action pinning, and discovery-script behaviour.

### Task 2: Document GitHub management-plane controls and end-to-end evidence

**Files:**
- Create: `docs/GITHUB-NATIVE-SECURITY-GATE-RUNBOOK.md`
- Modify: `README.md`

- [ ] Describe required Code Security, Secret Protection, Push Protection, and Ruleset settings.
- [ ] Distinguish GitHub Code Security license-dependent features from repository YAML.
- [ ] Define positive and negative tests without committing a real secret or a persistent vulnerable dependency.
- [ ] Define the issue-backed, time-bound exception process and the ruleset bypass authorization model.
- [ ] Link the runbook from the project README.

### Task 3: Verify and hand off

**Files:**
- Test: Go packages and generated GitHub workflow YAML

- [ ] Run `go test ./...` using a workspace-local `GOCACHE`.
- [ ] Run the workflow linter and inspect the final diff.
- [ ] Provide manual GitHub UI steps and the expected evidence for each security control.
