# GitHub 原生 CI/CD 质量门禁实施与验证方案

版本：1.1
适用对象：Go、Java、Node.js 及多语言仓库  
POC 仓库：`levilee/go-ci-quality-demo`  
当前范围：PR 质量门禁；部署流水线作为下一阶段

## 1. 目标与结论

使用 GitHub Actions 执行检查，使用 GitHub Rulesets 阻止不合格 PR 合并，使用 Pull Request、Issue、Actions 日志和 Ruleset bypass 记录形成审计链。Jenkins 不再是必需组件；只有在需要内网构建资源、复杂共享流水线或既有 Jenkins 资产时才保留。

推荐的控制原则：

- 所有生产代码通过 PR 进入 `main`，禁止直接推送和强制推送。
- CI 在部署测试环境之前执行“快速门禁”；通过后才构建一次不可变制品并部署测试环境。
- 测试环境的集成、契约、端到端和安全验证通过后，才允许合并或进入发布审批。
- `Quality Gate` 是 Ruleset 唯一稳定的 Required Check；它汇总所有实际检查结果。
- 失败检查不能被“点成成功”。误判通过有期限、双人复核、完整证据和可追踪 bypass。
- 同一制品从测试晋级到生产，生产阶段不重新编译。

## 2. 总体架构

```text
开发分支
   │ push / PR
   ▼
GitHub Pull Request
   ├─ Go/Java/Node 格式与静态检查
   ├─ 单元测试、并发检测、覆盖率阈值
   ├─ 依赖漏洞、Secret、IaC/容器扫描
   └─ Quality Gate（聚合结果）
             │
       GitHub Ruleset
       ├─ 成功：允许进入后续流程
       └─ 失败：禁止合并
             │
             ├─ 正常修复并重新运行
             └─ 可审计例外：Issue → 独立审批 → 限权 bypass
             │
             ▼
       构建/签名/推送不可变制品
             ▼
       部署测试环境 → Smoke/集成/E2E/DAST
             ▼
       合并 main → 生产审批 → 渐进发布
             ▼
       健康检查/指标验证 → 成功或自动回滚
```

## 3. GitHub 端需要的组件

| 组件 | 用途 | 当前 POC | 生产建议 |
| --- | --- | --- | --- |
| GitHub Actions | 执行构建、测试和扫描 | 已实现 | 保留 |
| Repository Ruleset | PR、Required Check、禁止直推 | 尚未启用 | POC 通过后启用 |
| CODEOWNERS | 关键目录指定审核人 | 未实现 | 必须 |
| PR 模板 | 变更、测试、风险、回滚清单 | 未实现 | 必须 |
| Issue 模板 | 例外申请及到期治理 | 已实现：`quality-gate-exception.yml` | 必须 |
| Environments | 测试/生产部署记录与 Secrets 隔离 | 未实现 | 部署阶段启用 |
| Artifact/Container Registry | 保存覆盖率、扫描报告、镜像 | 报告已实现 | 镜像使用 digest 晋级 |
| Dependabot | Action 与依赖升级 PR | 已实现：Go Modules / GitHub Actions | 建议启用 |

当前 POC 工作流：`.github/workflows/ci.yml`。

### 3.1 多服务变更识别约定

工作流不维护静态服务清单，而是从以下文件识别服务：`go.mod`（Go）、`package.json`（Node.js/TypeScript）、`pom.xml` / `build.gradle` / `build.gradle.kts`（Java/Kotlin）。独立部署的服务统一放在 `services/<service-name>/`。

- 只改动 `services/<service-name>/` 时，只为该服务生成质量矩阵任务；Go 服务额外运行 `govulncheck`。
- 改动 `.github/workflows/`、`scripts/ci/`、`ci/`、`libs/`、`shared/`、`go.work` 或无法归属到某个服务的路径时，安全回退为全部服务检查。
- Go：格式、`vet`、`-race`、覆盖率和构建；Node.js：锁文件安装、`lint`、带覆盖率的 `test`、`build`；Java：Maven `verify` 或 Gradle `check build`。
- Node 服务必须提供锁文件以及 `lint`、`test`、`build` scripts；Java 覆盖率阈值必须由服务自己的 JaCoCo 配置纳入 Maven `verify` 或 Gradle `check`。

## 4. PR 标准流程

1. 开发者从最新 `main` 创建短生命周期分支。
2. 本地执行格式化、单元测试和基础静态检查。
3. 推送分支并创建 PR，填写变更内容、风险、验证方法和回滚方式。
4. GitHub Actions 并行执行语言质量检查、测试、漏洞扫描和仓库安全扫描。
5. 聚合任务 `Quality Gate` 只有在所有必需任务成功时才成功。
6. Ruleset 要求 `Quality Gate` 成功、至少一名审核人批准、Code Owner 批准、对话已解决。
7. 高风险变更部署临时或共享测试环境，执行 Smoke、集成、契约、E2E 和 DAST。
8. 测试证据回写 PR。满足规则后使用 squash merge。
9. `main` 生成带 commit SHA/digest 的制品；测试与生产晋级同一 digest。
10. 生产部署使用受保护环境、并发锁、健康检查和自动回滚。

## 5. 当前 Go POC 的具体门禁

### 5.1 Go quality and tests

- `gofmt -l`：存在未格式化文件立即失败。
- `go vet ./...`：基础静态分析失败即阻断。
- `go test -race -covermode=atomic`：单元测试和数据竞争检测。
- 总覆盖率不得低于 70%。
- `go build -buildvcs=false`：验证可编译性。
- 上传 `coverage.out`，保留 7 天。

### 5.2 Go vulnerability scan

- 使用固定版本 `govulncheck@v1.6.0` 分析可达漏洞；该版本要求 Go 1.25，并使用兼容 Go 1.25 的 `x/tools`。
- Runner、Docker 构建和 Jenkins Agent 使用已修复 GO-2026-5856 的 Go `1.25.12`，项目仍保持 Go 1.21 语言兼容。避免使用可能因 Runner 缓存选择旧补丁的 `1.25.x`；通过受控升级 PR 更新明确的安全补丁版本。
- POC 已在本机 Go 1.21.9 上实际发现 29 个可达标准库漏洞，证明扫描能够阻断过期运行时风险。

### 5.3 Repository security scan

- Trivy 扫描依赖漏洞、Secret 和配置错误。
- `HIGH`、`CRITICAL` 发现立即失败。
- Trivy Action 固定到不可变 commit SHA，报告上传并保留 7 天。
- POC 使用已修复 2026 年供应链事件的 Trivy Action 版本；不能退回旧 tag 或使用浮动 `latest`。

### 5.4 Quality Gate 聚合

聚合任务设置 `if: always()`，即使前置任务失败也会运行并输出每个任务的状态。只有三个前置任务全部为 `success` 才返回成功。Ruleset 只依赖该稳定名称，后续增加内部扫描任务时不需要反复修改保护规则。

## 6. 多语言仓库扩展

采用路径识别和可复用工作流，不为每个仓库复制大段 YAML：

| 技术栈 | 依赖锁定 | 静态检查 | 测试 | 安全检查 | 构建 |
| --- | --- | --- | --- | --- | --- |
| Go | `go.mod/go.sum` | gofmt、go vet、golangci-lint | `go test -race` | govulncheck、Trivy | `go build` |
| Node.js | `pnpm-lock.yaml` + `--frozen-lockfile` | ESLint、TypeScript | Vitest/Jest | `pnpm audit`、Trivy | `pnpm build` |
| Java | Maven Wrapper/Gradle Wrapper + 锁定插件 | Checkstyle、SpotBugs、PMD | JUnit/Testcontainers | OWASP Dependency-Check、Trivy | Maven/Gradle package |

组织级仓库建议在专用仓库保存 `workflow_call` 可复用工作流，并将调用方和被调用方 Action 全部固定 SHA。每个语言任务输出统一状态，最后仍由 `Quality Gate` 聚合。

## 7. 可审计的例外门禁

### 7.1 何时允许

仅适用于已确认误判、无可用修复但有补偿控制、紧急安全修复或生产事故恢复。格式错误、真实测试失败、未知原因失败、为了赶发布时间，不属于可接受例外。

以下情况禁止例外：

- Secret 已泄露但尚未轮换；
- 可利用的 Critical 漏洞且没有隔离/缓解措施；
- 数据迁移无备份或回滚；
- 审批人与提交人是同一人；
- 例外没有明确到期时间和责任人。

### 7.2 申请字段

创建 `Exception Request` Issue，至少填写：

- PR、commit SHA、失败 workflow run 链接；
- 检查名称、规则/漏洞 ID、原始报告；
- 为什么判定为误判或必须接受；
- 受影响系统、数据和环境；
- 利用可能性与最大影响；
- 补偿控制、验证证据、回滚方案；
- 负责人、审批人、到期时间、修复 Issue；
- 是否涉及生产部署以及对应变更单。

### 7.3 审批和 bypass

1. 代码作者提交申请，不得审批自己的例外。
2. Code Owner 审代码；安全负责人审安全风险；Release Manager 执行 bypass。
3. Ruleset bypass list 只加入专用小组/GitHub App，优先选择 `For pull requests only`。
4. 不授予所有管理员和普通开发者 bypass；生产仓库至少两名不同人员参与。
5. 审批人在 PR 留下结构化记录：`APPROVED EXCEPTION #编号 / 到期时间 / 风险摘要`。
6. Release Manager 通过 Ruleset bypass 合并；禁止改写检查结论、删除日志或重新创建“假成功”状态。
7. 原失败记录、Issue、PR Review、bypass 事件、merge commit 和后续修复 PR 构成完整证据链。

### 7.4 到期和复查

- Critical/High：最长 7 天；其他风险：最长 30 天。
- 每周查询未关闭且即将到期的 Exception Issue。
- 到期未修复时关闭发布权限、重新阻断或回滚变更。
- 每月统计例外数量、重复规则、平均关闭时间和责任团队；同一规则连续误判应修正规则，而不是持续 bypass。

### 7.5 私人仓库套餐限制

私有仓库能否使用 Rulesets、受保护分支及环境人工审批取决于 GitHub 套餐。GitHub 官方文档当前注明：Free/Pro/Team 的 Environment required reviewers 只适用于公有仓库；私有仓库若需要原生部署人工审批，应核对 Enterprise 能力，或使用受控 GitHub App/外部变更系统实现部署保护。不要把普通 `workflow_dispatch` 当成双人审批，它只代表有人手工触发。

## 8. Ruleset 推荐配置

先用 `Evaluate` 模式观察，再切换 `Active`：

- Target：默认分支 `main`。
- Restrict deletions：开启。
- Block force pushes：开启。
- Require a pull request before merging：开启。
- Required approvals：生产仓库至少 1–2 人。
- Dismiss stale approvals：开启。
- Require review from Code Owners：开启。
- Require conversation resolution：开启。
- Require status checks：选择 `Quality Gate`，启用 strict/up-to-date。
- Bypass：仅 `ci-exception-approvers`，使用 PR-only 模式。
- 管理员是否可绕过：生产环境默认禁止宽泛管理员绕过。

只有工作流至少成功运行一次后，`Quality Gate` 才通常会出现在可选状态检查列表中。因此正确顺序是：先创建 POC PR并跑通，再配置 Required Check。

## 9. 验证测试矩阵

每种故障创建独立临时分支和 PR，不合并故障代码：

| 编号 | 注入故障 | 预期任务 | 预期结果 |
| --- | --- | --- | --- |
| T01 | 干净代码 | 全部 | Quality Gate 成功 |
| T02 | 未执行 gofmt | Go quality | PR 禁止合并 |
| T03 | 修改断言使单测失败 | Go quality | PR 禁止合并 |
| T04 | 增加数据竞争 | Go quality | `-race` 失败 |
| T05 | 覆盖率降到 70% 以下 | Go quality | 阈值失败 |
| T06 | 引入可达漏洞依赖 | govulncheck | PR 禁止合并 |
| T07 | 提交格式正确的测试 Secret | Trivy | PR 禁止合并；立即删除并轮换 |
| T08 | 引入高危依赖或错误 IaC | Trivy | PR 禁止合并 |
| T09 | 取消/跳过一个前置任务 | Quality Gate | 聚合任务失败 |
| T10 | 未经授权尝试直推 main | Ruleset | 推送被拒绝 |
| T11 | 误判例外申请缺字段 | Exception Gate | 不批准 bypass |
| T12 | 完整例外申请且独立批准 | Exception Gate | 可 bypass，审计链完整 |
| T13 | 例外到期未修复 | Governance | 告警并重新阻断/回滚 |

记录每个系统的排队时间、执行时间、失败定位时间、误报数、维护成本和资源费用，再与 Jenkins 同 commit 的运行结果比较。

## 10. GitHub Actions 与 Jenkins 对比口径

| 维度 | GitHub 原生 | Jenkins |
| --- | --- | --- |
| PR 集成 | 原生 Checks、Rulesets、Review | 依赖 GitHub App/Webhook 回写 |
| 运维 | GitHub 托管 Runner 可零服务器 | Controller、插件、Agent、备份升级 |
| 内网访问 | 使用 self-hosted runner | Kubernetes/静态 Agent 灵活 |
| 审计 | PR、Actions、Ruleset 集中 | Jenkins 与 GitHub 证据分散 |
| 扩展 | Actions/可复用工作流 | 插件/Shared Library |
| 例外 | Ruleset bypass + Issue | Jenkins input/RBAC + GitHub 合并规则 |
| 私有部署审批 | 受套餐限制 | 自建审批逻辑更灵活 |
| 安全责任 | Action SHA、最小权限、Runner 隔离 | 插件、凭证、Controller/Agent 全栈维护 |

对当前轻量 POC，GitHub 原生方案更简单。对必须访问局域网 Kubernetes 的部署，推荐 GitHub self-hosted runner 放在集群可达网络中；若已有成熟 Jenkins 平台，也可以保留 Jenkins 负责 CD，而 GitHub Actions 负责 PR 快速门禁。

## 11. 分阶段落地

### 阶段 A：当前 POC

- 推送独立分支并创建 PR。
- 跑通三项检查和聚合门禁。
- 执行 T01–T09，保存运行链接和时间。
- 不合并、不启用强制 Ruleset，避免影响现有仓库。

### 阶段 B：治理试运行

- 增加 PR、Exception Issue 模板和 CODEOWNERS。
- Ruleset 使用 Evaluate 模式。
- 执行 T10–T13，确认 bypass 身份和审计记录。

### 阶段 C：正式启用

- Ruleset 切换 Active，`Quality Gate` 设为 Required。
- 增加 Dependabot 和 Action SHA 更新机制。
- 建立可复用多语言工作流与组织级规则。

### 阶段 D：部署闭环

- 构建、SBOM、镜像扫描、签名并推送 Registry。
- 测试环境自动部署与 Smoke/集成/E2E/DAST。
- 生产审批、渐进发布、SLO 监测和自动回滚。

## 12. 验收标准

- 干净 PR 全部通过，故障 PR 均在对应门禁被阻断。
- main 无法直接推送，Required Check 无法被普通开发者绕过。
- 例外必须由非作者批准，且能从 merge commit 追溯到 PR、Issue、失败日志和到期任务。
- CI 中不使用浮动 Action 版本，不向 PR 工作流开放写权限或生产 Secrets。
- 覆盖率、扫描报告和构建制品能够按保留策略查询。
- 部署阶段能证明使用同一制品，并能在健康检查失败时自动回滚。

## 13. 官方依据

- GitHub Rulesets 可要求 PR、状态检查和部署成功，并可配置特定角色、团队或 GitHub App 的 bypass：<https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets>
- Ruleset 的创建、Evaluate/Active 状态和 bypass list：<https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository>
- GitHub Environments 的审批、Secrets 解锁和私有仓库套餐限制：<https://docs.github.com/en/actions/reference/workflows-and-actions/deployments-and-environments>
- 部署审批与管理员 bypass 会要求说明并保留运行记录：<https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/review-deployments>
- GitHub Actions 使用 Environment、并发控制和自定义保护规则：<https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/control-deployments>
