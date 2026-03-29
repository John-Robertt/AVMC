# AVMC 文档索引

本文档是 `docs/` 的注册表，只回答 3 件事：

1. 哪些文档定义当前行为。
2. 哪些文档只是计划或决策记录。
3. 每个主题应该先读哪份文档，再去看哪个代码包。

## 文档分层

- **现行规范**：定义当前代码必须满足的规则与对外契约。
- **架构事实**：描述当前仓库已经落地的包边界与依赖方向。
- **计划文档**：维护经确认的工作计划，默认不参与当前行为判断。
- **ADR**：记录决策背景，不直接替代现行规范。

## 文档注册表

| 文档 | 类型 | 权威范围 | 对应代码 |
| --- | --- | --- | --- |
| [PRD.md](./PRD.md) | 现行规范 | 产品目标、范围、用户可见行为总览 | `cmd/avmc/`、`internal/app/run/` |
| [CLI.md](./CLI.md) | 现行规范 | 命令、帮助、输出通道、退出码 | `cmd/avmc/main.go`、`cmd/avmc/progress_ui.go` |
| [CONFIG.md](./CONFIG.md) | 现行规范 | `avmc.json` 发现规则、覆盖优先级、字段语义 | `internal/config/config.go`、`internal/scan/scan.go` |
| [DATA_MODEL.md](./DATA_MODEL.md) | 现行规范 | `Code`、`VideoFile`、`WorkItem`、`ItemPlan`、`MovieMeta`、`RunReport` 等领域模型 | `internal/domain/`、`internal/code/`、`internal/app/` |
| [ALGORITHMS.md](./ALGORITHMS.md) | 现行规范 | 扫描、提取、分组、规划、执行流程 | `internal/scan/`、`internal/code/`、`internal/app/` |
| [IO_CONTRACT.md](./IO_CONTRACT.md) | 现行规范 | `out/`、`cache/`、`report.json`、原子写、移动语义 | `internal/app/run/`、`internal/infra/fsx/` |
| [HTTP_CACHE.md](./HTTP_CACHE.md) | 现行规范 | HTTP client、代理、UA、重试、provider cache 读写 | `internal/infra/httpx/`、`internal/infra/cache/`、`internal/app/run/` |
| [PROVIDERS.md](./PROVIDERS.md) | 现行规范 | provider 接口、降级链、站点解析规则、fixture/golden | `internal/provider/` |
| [NFO.md](./NFO.md) | 现行规范 | `MovieMeta -> <CODE>.nfo` 的字段映射与固定值 | `internal/nfo/nfo.go` |
| [REPORT.md](./REPORT.md) | 现行规范 | `RunReport` JSON 结构、状态枚举、错误码 | `internal/domain/report.go`、`cmd/avmc/main.go` |
| [ARCHITECTURE.md](./ARCHITECTURE.md) | 架构事实 | 当前包分层、依赖方向、例外点 | `cmd/`、`internal/` |
| [TESTING.md](./TESTING.md) | 现行规范 | 当前测试矩阵、golden 更新方式、验证命令 | `*_test.go` |
| [BUILD.md](./BUILD.md) | 现行规范 | 当前发布流程、GitHub Actions、Docker 镜像 | `.github/workflows/release.yml`、`Dockerfile` |
| [DEVELOPMENT_PLAN.md](./DEVELOPMENT_PLAN.md) | 计划文档 | 已确认的工作计划容器 | 无固定包 |
| [adr/](./adr/) | ADR | 架构决策背景与取舍 | 相关代码按主题查找 |

## 术语表

- `path`：扫描根目录。输出固定到 `<path>/out/`；缓存固定到 `<path>/cache/`。
- `CODE`：作品主键，规范化后形如 `CAWD-895`。
- `dry-run`：默认模式；允许读取只读 cache，但不写 `out/`、不写 `cache/`、不下载图片、不移动文件。
- `apply`：真实执行模式；允许写入 `out/` 与 `cache/`，并在 sidecar 满足后移动视频文件。
- `sidecar`：与视频配套的 `nfo/poster/fanart` 文件。
- `provider`：元数据来源，目前仅 `javbus` 与 `javdb`。

## 权威源索引

| 主题 | 权威文档 |
| --- | --- |
| CLI 参数、帮助、输出通道、退出码 | [CLI.md](./CLI.md) |
| 配置发现、覆盖优先级、字段语义 | [CONFIG.md](./CONFIG.md) |
| 数据结构与不变量 | [DATA_MODEL.md](./DATA_MODEL.md) |
| 扫描、提取、分组、规划、执行流程 | [ALGORITHMS.md](./ALGORITHMS.md) |
| 文件布局、原子写、不覆盖、移动最后 | [IO_CONTRACT.md](./IO_CONTRACT.md) |
| HTTP client、代理、UA、重试、provider cache | [HTTP_CACHE.md](./HTTP_CACHE.md) |
| provider 接口、降级链、站点解析 | [PROVIDERS.md](./PROVIDERS.md) |
| NFO 输出字段 | [NFO.md](./NFO.md) |
| 运行报告结构与错误码 | [REPORT.md](./REPORT.md) |
| 当前包分层与依赖方向 | [ARCHITECTURE.md](./ARCHITECTURE.md) |
| 测试矩阵与验证命令 | [TESTING.md](./TESTING.md) |
| 当前发布流程 | [BUILD.md](./BUILD.md) |

## 维护规则

- 现行规范聚焦当前行为、当前接口和当前结构。
- 工作计划与决策背景分别维护在 `DEVELOPMENT_PLAN.md` 与 `docs/adr/`。
- 同一条现行规则只在一份权威文档中定义；其它文档只引用，不重复展开。
- 当用户明确要求“文档对齐当前代码”时，以代码和测试作为当前行为证据修正文档。
