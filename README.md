# Go CI Quality Demo

A small Go HTTP service used to validate Jenkins and GitHub quality gates.

## Endpoints

- `GET /healthz` returns service health.
- `GET /api/greet?name=Codex` returns a greeting.
- `GET /api/calculate?a=7&b=5&operation=add` calculates `add`, `subtract`, or `multiply` for two integers.
- `GET /api/upstream` calls the URL configured by `UPSTREAM_URL` and returns its status and body.

## Run locally

```bash
go test ./...
go run ./cmd/server
```

Then call:

```bash
curl http://localhost:8080/healthz
curl "http://localhost:8080/api/greet?name=Codex"
curl "http://localhost:8080/api/calculate?a=7&b=5&operation=add"
curl http://localhost:8080/api/upstream
```

Configuration:

| Variable | Default |
| --- | --- |
| `PORT` | `8080` |
| `UPSTREAM_URL` | `https://api.github.com/zen` |

The repository includes parallel Jenkins and GitHub Actions quality-gate POCs, a Dockerfile, and unit tests with a mocked upstream server.

## CI quality gates

- `Jenkinsfile` validates formatting, static analysis, tests with race detection and coverage, and compilation in Jenkins.
- `.github/workflows/ci.yml` discovers affected Go, Node.js, and Java services from their manifests. It runs language-appropriate quality, test, coverage, and build checks only for affected services, but fans out to all services for shared CI/library/unknown-path changes. It also includes reachable-vulnerability, repository-security, and pull-request dependency-review checks, and exposes a stable `Quality Gate` check for branch protection.
- `.github/workflows/codeql.yml` performs scheduled and pull-request CodeQL analysis for Go. CodeQL findings are enforced by a GitHub Ruleset, not by treating a completed scan as a clean scan.
- `.github/dependabot.yml` schedules Go module and GitHub Action version updates.
- [GitHub native security-gate runbook](docs/GITHUB-NATIVE-SECURITY-GATE-RUNBOOK.md) describes the required GitHub settings, licensing boundary, and end-to-end evidence.
- `.github/ISSUE_TEMPLATE/quality-gate-exception.yml` records a time-bound exception request; a failed check stays failed and only a restricted GitHub Ruleset bypass group can merge after independent approval.
- Neither POC deploys the application. Deployment should begin only after the PR gate passes and the change is merged into a protected branch.
