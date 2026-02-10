# 渐进式、可验证的开发计划

原则：每一步都必须产生一个可运行/可测试的增量，且能用明确命令验证。不要一口气写完“大工程”，也不要为了抽象而抽象。

## 当前进度（截至 2026-02-09）
- Phase 0：已完成（go.mod + CLI 骨架 + internal 基础包）
- Phase 1：已完成（配置发现与覆盖优先级 + 错误码 + 单测）
- Phase 2：已完成（扫描 + 永久排除 out/cache + exclude_dirs + 单测）
- Phase 3：已完成（CODE 提取/规范化 + ambiguous/no_match + 单测）
- Phase 4：已完成（按 CODE 分组 + 稳定排序 + 单测）
- Phase 5：已完成（OutState + Planner + 同名去冲突 + 单测）
- Phase 6：已完成（fsx 原子写 + rename/EXDEV 标记 + 单测）
- Phase 7：已完成（httpx client 策略 + cache 读写边界 + 单测）
- Phase 8：已完成（javbus/javdb：Fetch+Parse + fixture/golden：SNOS-052 / KUM-013 / JUR-566；JavDB 标题优先原标题；两站 Series 解析写入 NFO `<set>`）
- Phase 9：已完成（NFO 生成 + XML 可解析 + 单测）
- Phase 10：已完成（Executor：dry-run/apply + worker pool + 报告输出契约 + E2E 测试；poster 由 fanart 右半边裁切生成）
- Phase 11：已完成（error_msg 可操作：HTTP 状态/driver-verify/超时/TLS 提示；新增 javdb_base_url；NFO 输出结构对齐 demo 示例）
- Phase 12：已完成（GitHub Actions 发布：GitHub Release 多平台二进制 + GHCR 多架构镜像；Dockerfile 适配 buildx）

## Phase 0：工程骨架（当天可跑）
**产出**
- `go.mod` 初始化
- `cmd/avmc/main.go`：支持 `avmc run ...`（参数先解析但可不实现完整逻辑）
- `internal/` 基础目录：`domain/ config/ app/ scan/ code/ infra/{fsx,httpx,cache}/ provider/ nfo/`

**验证**
- `go test ./...`（即使只有空测试，也必须通过）
- `go run ./cmd/avmc --help`（或等价）

## Phase 1：配置发现与覆盖（把“入口”定死）
**产出**
- `internal/config`：实现 `avmc.json` 读取规则与合并优先级（CLI 仅 `path/provider/apply`）
- 错误码：`config_not_found/config_invalid/config_missing_path`

**验证**
- 单测：无参运行找不到 `./avmc.json` 必须报 `config_not_found`
- 单测：`--apply=false` 覆盖 config.apply=true

## Phase 2：扫描器（exclude + 视频白名单）
**产出**
- `internal/scan`：WalkDir + 永久排除 out/cache + 配置 exclude_dirs
- 视频扩展名白名单（先从常见 mp4/mkv/avi 开始）

**验证**
- 单测：排除目录不被扫描
- 基准手测：在临时目录造树，统计扫描到的视频文件数与期望一致

## Phase 3：CODE 提取与规范化（宁可失败不写错）
**产出**
- `internal/code`：从文件名/父目录提取 CODE；ambiguous/no_match 明确失败
- `internal/domain`：`Code` 类型与 normalize 逻辑

**验证**
- 单测：变体归一化为同一 CODE
- 单测：ambiguous 必须失败并返回候选列表（用于 error_msg）

## Phase 4：分组与排序（数据局部性）
**产出**
- 把 `[]VideoFile` 按 `Code` 分组为 `[]WorkItem`（WorkItem 存 index）
- 输出稳定排序（按 code 或首个输入路径）

**验证**
- 单测：同 CODE 多文件归并为 1 个 WorkItem
- 单测：排序稳定（重复运行输出顺序一致）

## Phase 5：OutState 与 Planner（先算清楚再动手）
**产出**
- `internal/app/planner`：读取 out/<CODE> 现状（nfo/poster/fanart 是否存在 + 文件名集合）
- 生成 `ItemPlan`：NeedScrape + MovePlan（同名去冲突）

**验证**
- 单测：已完整条目 => NeedScrape=false
- 单测：同名冲突 => dst 生成 `__2/__3`，且确定性

## Phase 6：文件系统原语（原子写 + rename + EXDEV）
**产出**
- `internal/infra/fsx`：
  - `WriteFileAtomic(dir, name, bytes)`（临时文件 + rename）
  - `Rename(src, dst)`（捕获 EXDEV）
  - 回滚辅助（可选但推荐）

**验证**
- 单测：WriteFileAtomic 写入成功后最终文件存在，临时文件不残留
- 单测/集成：模拟 EXDEV（可通过接口注入或 stub）=> move_failed

## Phase 7：HTTP 与缓存（可用性策略集中）
**产出**
- `internal/infra/httpx`：
  - MetaClient：proxy 可选、UA 随机、DisableKeepAlives、timeout、bounded retry
  - ImageClient：受 image_proxy 控制是否走代理
- `internal/infra/cache`：`<path>/cache/providers/<p>/<code>.{html,json}` 读写（apply 写、dry-run 只读）

**验证**
- 单测：启用代理时 Transport/Request 必须禁用 keep-alive（每请求新连接）
- 单测：dry-run 不写 cache；apply 写 cache（可用临时目录验证）

## Phase 8：Provider（先 parse 后 fetch，fixture 驱动）
**产出**
- `internal/provider`：Provider 接口 + registry + 自动降级策略
- `javbus/javdb`：
  - `Parse` 先实现并通过 fixture/golden
  - `Fetch` 再接入 httpx（真实抓取可用集成测试或手测）

**验证**
- `go test ./...`：fixture/golden 全通过
- 单测：requested 失败后 fallback 成功，report 标注 provider_used

## Phase 9：NFO 生成（稳定结构）
**产出**
- `internal/nfo`：`MovieMeta -> <CODE>.nfo`（最小可用集）
- 与 PRD 字段对齐，缺失字段允许为空但结构稳定

**验证**
- 单测：生成的 XML 可解析（标准库 xml.Unmarshal）
- golden：固定 meta 输入 => 固定 nfo 输出

## Phase 10：Executor（dry-run/apply + 并发）
**产出**
- `internal/app/run`：把 scan/group/plan/scrape/write/move/report 串起来
- 并发 worker pool（按 CODE）
- dry-run：执行 `fetch+parse` 验证，但不写 out/cache；stdout 非 TTY 输出 RunReport JSON（TTY 输出人类摘要，日志走 stderr）
- apply：严格遵守“移动最后”

**验证**
- E2E：临时目录 + stub provider
  - dry-run 不产生 out/cache
  - stdout 非 TTY：stdout 仅 1 个 JSON（stderr 允许日志）
  - apply 产生 out/<CODE>/ sidecar，并移动视频
  - 失败时不移动视频

## Phase 11：Report（稳定结构 + 可操作错误）
**产出**
- `internal/app/report`：结构固定（见 REPORT.md）
- `error_msg` 给出可执行提示（下一步怎么做）

**验证**
- 单测：各错误码路径 report 字段齐全
- E2E：对照验收用例检查 summary 与 item 状态

## Phase 12：打包与交付（可复现）
**产出**
- GitHub Actions：
  - `ci`：`go test ./...`
  - `release`：推送 `v*` tag 自动发布 GitHub Release（多平台二进制归档 + 校验和）与 GHCR 多架构镜像（linux/amd64, linux/arm64）
- Dockerfile：支持 buildx 的 `TARGETOS/TARGETARCH`，镜像只含二进制+CA，entrypoint=avmc

**验证**
- 触发一次 `v*` tag 发布，确认：
  - GitHub Release 附件包含 `avmc_<VERSION>_<GOOS>_<GOARCH>.tar.gz/.zip`、对应的 `*_SHA256SUMS.txt`，以及汇总 `SHA256SUMS.txt`
  - GHCR 镜像可被拉取并运行 `--help`

---

## 最终验收清单（必须全部通过）
1) 扫描排除：out/cache 永久排除；exclude_dirs 生效
2) 多版本：同 CODE 多文件只刮削一次并全部归档
3) provider 自动降级：失败切换且可追溯（provider_requested/provider_used）
4) 图片代理开关：image_proxy=false 直连；true 走代理
5) 可恢复性：移动最后、原子写、不覆盖、EXDEV 失败不丢数据、幂等可重跑
6) dry-run 零写入：不生成 out/cache/report；stdout 非 TTY 时 stdout 仅输出 JSON
