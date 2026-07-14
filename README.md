# Go CI Quality Demo

A small Go HTTP service used to validate Jenkins and GitHub quality gates.

## Endpoints

- `GET /healthz` returns service health.
- `GET /api/greet?name=Codex` returns a greeting.
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
- `.github/workflows/ci.yml` runs the equivalent GitHub-native gate as parallel jobs and exposes a stable `Quality Gate` check for branch protection.
- Neither POC deploys the application. Deployment should begin only after the PR gate passes and the change is merged into a protected branch.
