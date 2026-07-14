# GitHub Native Quality Gate POC

This branch adds a GitHub Actions implementation that can be compared with the existing Jenkins pipeline against the same Go source code.

## Pull request execution

Every pull request targeting `main` starts three independent checks:

1. **Go quality and tests**: formatting, `go vet`, unit tests, race detection, a 70% coverage threshold, and compilation.
2. **Go vulnerability scan**: reachable-vulnerability analysis with pinned `govulncheck` v1.1.3. The runner uses the latest Go 1.25 patch while preserving the project's Go 1.21 language compatibility.
3. **Repository security scan**: Trivy filesystem scanning for vulnerable dependencies, secrets, and configuration errors. High and critical findings block the run.

The final **Quality Gate** job succeeds only when all three checks succeed. Configure that single, stable check as required in a GitHub ruleset after the POC is validated.

## Evidence

- Coverage and Trivy reports are retained as workflow artifacts for seven days.
- Actions are pinned to immutable commit SHAs.
- The workflow has read-only repository permissions and explicit timeouts.
- Concurrent obsolete runs for the same branch are cancelled.

## Deliberate scope

This POC validates continuous integration only. Image building, signing, test-environment deployment, smoke tests, approval, production deployment, health checks, and rollback belong to the continuous-delivery workflow after merge.

## Comparison procedure

1. Open a pull request from this branch to `main` and record the GitHub Actions duration and results.
2. Run the same commit in the Jenkins multibranch pipeline and record its duration and results.
3. Introduce separate test branches for a formatting error, failing unit test, and leaked test secret; verify that both systems block them.
4. Remove the deliberate defect after each test. Do not merge defect-injection branches.
5. After the clean workflow passes, make `Quality Gate` a required status check and disable direct pushes to `main`.
