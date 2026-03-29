# CLAUDE.md

## 项目概览

- AVMC 是一个 Go CLI，用于扫描视频目录、提取 `CODE`、按 `CODE` 组织条目、抓取元数据、生成 sidecar，并把视频归档到固定目录结构。
- 关键术语：
  - `CODE`：作品主键，例如 `CAWD-895`
  - `sidecar`：`<CODE>.nfo`、`poster.jpg`、`fanart.jpg`
  - `dry-run`：规划与验证模式
  - `apply`：生成 sidecar、写入 cache、输出 report、归档视频的执行模式
- 当前主链路：`cmd/avmc/main.go` -> `internal/config` -> `internal/app/run` -> `{scan, code, planner, provider, infra, nfo, domain}`
- AI 在本项目中协助完成代码变更、审查诊断、文档维护和方案讨论，以可验证的方式交付结果。

## 信息源与判断

- 项目中的信息结构分为 3 层：
  - `CLAUDE.md` 与 `AGENTS.md`：AI 工作方式、任务分流与主题路由
  - `docs/`：项目规则、接口契约与文档索引
  - 代码与测试：当前实现的直接证据
- `docs/README.md` 是权威文档入口；按主题进入对应文档。
- `docs/` 正文描述当前已实现的能力、结构、流程、接口、用法和设计意图，以“是什么、怎么用、怎么工作”组织内容。
- 工作计划与决策背景分别放在 `docs/DEVELOPMENT_PLAN.md` 与 `docs/adr/`。
- 当文档、代码和测试出现差异时，先分类举证，再确认当前行为与目标状态。

## 任务分流

- 代码变更：从 `docs/README.md` 定位主题文档，读取相关代码与测试，实施最小必要修改，按影响范围验证结果。
- 审查与诊断：先收集证据，再给出结论、风险和修复路径；输出以行为影响和稳定性问题为先。
- 提示词与协作规则维护：优先补足目标、上下文、正向动作指令、结构隔离和验证方式，保持每条描述提供可执行信息。
- 解释、问答与方案讨论：直接回应用户目标；答案依赖仓库事实时，引用对应文档或代码位置。

## 验证与收束

- 对接口、行为语义、约束条件、输出结构、文件布局或用户可见提示有影响的代码变更，使用 `go test ./...` 与 `go vet ./...` 收束结果。
- 文档、提示词或纯分析任务，采用与任务影响匹配的校验方式，例如结构一致性、术语一致性、引用可达性和镜像文件一致性。
- 验证失败时，先定位与当前任务直接相关的原因并修复，再重跑相应验证；仍有阻塞时，明确报告阻塞项、影响范围和建议的下一步。
- 对接口、行为语义、约束条件、输出结构或用户可见提示产生影响时，同步更新对应权威文档。

## 差异处理

项目中的差异按统一流程处理：分类、举证、修正。

### 分类

- **边界差异**：AI 工作方式与项目规则出现在不同容器中的位置需要调整
- **规范差异**：两份 `docs/` 文档对同一主题描述不同
- **实现差异**：`docs/` 与代码或测试反映的当前行为不同

### 举证

报告差异时使用以下信息：

- 差异类型
- 文档位置：文件与章节
- 代码或测试位置：文件、函数、类型或测试名
- 对当前任务的影响：修改范围、行为判断或验证路径

### 处理路径

- 边界差异：把内容整理到对应容器
- 规范差异：对齐权威文档
- 实现差异：结合代码与测试确认当前行为后修正文档，或在需要时请用户确认目标状态

## 任务路由

| 主题                           | 先读文档                                   | 再读代码                                                                                                          |
| ------------------------------ | ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------- |
| CLI、输出、退出码、交互进度    | `docs/CLI.md`、`docs/REPORT.md`            | `cmd/avmc/main.go`、`cmd/avmc/progress_ui.go`                                                                     |
| 配置发现、覆盖优先级、排除目录 | `docs/CONFIG.md`                           | `internal/config/config.go`、`internal/scan/scan.go`                                                              |
| CODE、分组、规划、执行流程     | `docs/DATA_MODEL.md`、`docs/ALGORITHMS.md` | `internal/code/extract.go`、`internal/app/group.go`、`internal/app/planner/planner.go`、`internal/app/run/run.go` |
| 文件布局、原子写、移动语义     | `docs/IO_CONTRACT.md`                      | `internal/app/run/run.go`、`internal/infra/fsx/fsx.go`                                                            |
| NFO 输出                       | `docs/NFO.md`                              | `internal/nfo/nfo.go`                                                                                             |
| provider、降级链、站点解析     | `docs/PROVIDERS.md`                        | `internal/provider/*.go`、`internal/provider/javbus/*.go`、`internal/provider/javdb/*.go`                         |
| HTTP client、代理、UA、缓存    | `docs/HTTP_CACHE.md`                       | `internal/infra/httpx/httpx.go`、`internal/infra/cache/cache.go`、`internal/app/run/run.go`                       |
| 架构与依赖方向                 | `docs/ARCHITECTURE.md`                     | `cmd/`、`internal/app/`、`internal/domain/`、`internal/infra/`、`internal/provider/`                              |
| 测试与发布                     | `docs/TESTING.md`、`docs/BUILD.md`         | `*_test.go`、`.github/workflows/release.yml`                                                                      |
| 提示词容器、协作规则           | `CLAUDE.md`、`AGENTS.md`、相关原则文档     | `CLAUDE.md`、`AGENTS.md`、`.agents/skills/**/*.md`                                                                |
