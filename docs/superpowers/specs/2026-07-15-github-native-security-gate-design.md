# GitHub Native Security Gate Design

## Goal

Use GitHub Actions and GitHub repository security controls as the PR quality gate for this Go demo without a self-hosted SonarQube dependency.

## Scope

- Keep the existing Go quality, coverage, build, `govulncheck`, and Trivy checks.
- Add CodeQL analysis for Go.
- Add Dependabot update configuration and an official dependency-review workflow for pull requests.
- Document the GitHub-only controls that cannot be created from repository YAML: Code Security, Secret Protection, Push Protection, and a `main` ruleset.
- Provide positive and negative end-to-end validation cases.

## Design decisions

1. `Quality Gate` remains the single stable required status check. It aggregates executable CI jobs.
2. CodeQL remains a separate workflow and is enforced by the GitHub ruleset's **Require code scanning results** rule, because CodeQL analysis can complete successfully while still publishing findings.
3. Dependabot Alerts, Secret Scanning, Push Protection, and rulesets are configured in GitHub's management plane; no workflow is granted administration permissions to change them.
4. Dependabot is configured for Go modules and GitHub Actions. Dependency Review runs only on pull requests and blocks newly introduced high/critical vulnerable dependencies.
5. Production secrets and `pull_request_target` are out of scope. PR workflows use read-only repository permissions.

## Acceptance evidence

- Local Go tests and workflow static validation pass.
- A normal PR shows `Quality Gate`, `CodeQL`, and `Dependency Review` results.
- A controlled dependency-review or CodeQL negative test blocks the PR according to its respective ruleset rule.
- A controlled Push Protection test is performed only with GitHub's documented test pattern, never with a real credential.
