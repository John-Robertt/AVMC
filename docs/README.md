# AVMC 文档索引

本文档集合用于在开发前把「数据结构 + 算法 + 文件系统契约」说清楚，减少实现时的拍脑袋与返工。

## 推荐阅读顺序（从产品到实现）
1) [PRD.md](./PRD.md)（产品目标与对外契约）
2) [ARCHITECTURE.md](./ARCHITECTURE.md)（分层与模块边界）
3) [DATA_MODEL.md](./DATA_MODEL.md)（核心数据结构与不变量）
4) [ALGORITHMS.md](./ALGORITHMS.md)（端到端算法流程）
5) [IO_CONTRACT.md](./IO_CONTRACT.md)（文件系统契约：out/cache/report/原子写/移动语义）
6) [CONFIG.md](./CONFIG.md)（avmc.json：发现规则、覆盖优先级、字段语义）
7) [PROVIDERS.md](./PROVIDERS.md)（provider 插件接口、fixture/golden 测试）
8) [REPORT.md](./REPORT.md)（report.json 稳定结构与错误码）
9) [TESTING.md](./TESTING.md)（测试矩阵与验收用例）
10) [DEVELOPMENT_PLAN.md](./DEVELOPMENT_PLAN.md)（渐进式、可验证的开发计划）
11) [BUILD.md](./BUILD.md)（构建与 Docker）

（可选）架构决策记录（ADR）：见 [adr/](./adr/)

---

## 术语表（必须统一）
- `path`：用户传入（或配置文件给出）的扫描根目录；输出固定到 `<path>/out/`；缓存固定到 `<path>/cache/`。
- `CODE`：作品主键（规范化后形如 `CAWD-895`）。扫描、分组、输出目录都以 CODE 为核心。
- `dry-run`：默认模式；若需要生成 sidecar 则会执行 `fetch+parse` 验证可用性，但**不得写入** `out/` 与 `cache/`、不下载图片、不移动文件；stdout 是 TTY 时输出人类摘要，stdout 非 TTY 时 stdout 仅输出 RunReport JSON（日志走 stderr）。
- `apply`：真实执行模式；允许写入 `out/`、`cache/` 并移动视频文件；运行报告固定写入 `<path>/cache/report.json`。
- `sidecar`：与视频配套的元数据文件（`<CODE>.nfo`、`poster.jpg`、`fanart.jpg`）。
- `provider`：刮削源（`javbus`/`javdb`），负责页面定位与 HTML 解析；核心流程负责缓存/重试/限速/降级策略。

---

## 关键不变量（实现必须遵守）
1) **移动永远最后一步**：刮削/解析/侧车生成失败时，禁止移动任何视频文件。
2) **不覆盖**：已有 sidecar（nfo/poster/fanart）不得覆盖；只能补齐缺失。
3) **扫描排除固定**：`<path>/out/`、`<path>/cache/` 永远排除；`exclude_dirs` 额外排除。
4) **每请求新建连接**：启用代理池时，HTTP 请求不复用 keep-alive，以触发池化轮换。
5) **幂等可重跑**：重复运行不会把库弄乱；输出结构稳定且可预测。

---

## 文档更新规则（避免文档漂移）
- PRD：只描述对外行为与验收标准；不写实现细节。
- ARCHITECTURE / DATA_MODEL / ALGORITHMS：描述实现必须遵守的结构与流程；变更要同步测试用例。
- PROVIDERS：只写 provider 接口与解析规范；HTML 字段变化用 fixture/golden 测试锁住。
- DEVELOPMENT_PLAN：按阶段产出与验证命令维护；每完成一个阶段就更新状态与下一步。
