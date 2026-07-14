# GitHub Native Quality Gate POC

This branch adds a GitHub Actions implementation that can be compared with the existing Jenkins pipeline against the same Go source code.

## Pull request execution

Every pull request targeting `main` starts three independent checks:

1. **Go quality and tests**: formatting, `go vet`, unit tests, race detection, a 70% coverage threshold, and compilation.
2. **Go vulnerability scan**: reachable-vulnerability analysis with pinned `govulncheck` v1.6.0. The scanner and runner both target Go 1.25 while preserving the project's Go 1.21 language compatibility.
3. **Repository security scan**: Trivy filesystem scanning for vulnerable dependencies, secrets, and configuration errors. High and critical findings block the run.

The final **Quality Gate** job succeeds only when all three checks succeed. Configure that single, stable check as required in a GitHub ruleset after the POC is validated.

## Evidence

- Coverage and Trivy reports are retained as workflow artifacts for seven days.
- Actions are pinned to immutable commit SHAs.
- The workflow has read-only repository permissions and explicit timeouts.
- Concurrent obsolete runs for the same branch are cancelled.

## Deliberate scope

This POC validates continuous integration only. Image building, signing, test-environment deployment, smoke tests, approval, production deployment, health checks, and rollback belong to the continuous-delivery workflow after merge.

## Auditable exception gate

A failed check must not be changed to success manually. An exception is a controlled bypass of the merge rule, not a modification or rerun that hides the original result.

1. The author opens an `Exception Request` issue and links the blocked PR, failed run, finding ID, evidence, risk, scope, expiry time, rollback plan, and follow-up owner.
2. A security or release owner who did not author the PR reviews the request. The code owner still reviews the code change.
3. Only a small GitHub ruleset bypass team may merge. Use the `For pull requests only` bypass mode when it is available; never grant bypass to all administrators or developers.
4. The approver records `APPROVED`, the exception issue number, expiry, and reason in the PR before bypassing. Emergency production approval is separate from CI-risk acceptance.
5. The original failed Action run, PR discussion, reviews, ruleset bypass event, merge commit, and exception issue remain as audit evidence.
6. Exceptions expire in at most 7 days for critical/high security findings and 30 days for other findings. The follow-up issue must remove the exception or fix the cause.

Minimum separation of duties is requester, technical reviewer, and bypass approver. For this personal private-repository POC, a single owner cannot provide real separation of duties; validate the mechanics here, then use an organization team for production.

See `docs/GITHUB-NATIVE-QUALITY-GATE-PLAN.md` for the complete GitHub-native design and test matrix.

## Comparison procedure

1. Open a pull request from this branch to `main` and record the GitHub Actions duration and results.
2. Run the same commit in the Jenkins multibranch pipeline and record its duration and results.
3. Introduce separate test branches for a formatting error, failing unit test, and leaked test secret; verify that both systems block them.
4. Remove the deliberate defect after each test. Do not merge defect-injection branches.
5. After the clean workflow passes, make `Quality Gate` a required status check and disable direct pushes to `main`.
