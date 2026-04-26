# 测试与验证

本文档描述当前仓库已经存在的测试矩阵和推荐验证命令。

## 1. 当前测试矩阵

| 位置 | 当前覆盖内容 |
| --- | --- |
| `internal/config/config_test.go` | 配置文件发现、缺失/非法配置错误、CLI path 下配置可选、`path/provider/apply` 覆盖顺序、`image_proxy` 与 `proxy.url` 约束、非法 provider / proxy URL |
| `internal/scan/scan_test.go` | 永久排除 `out/` 与 `cache/`、`exclude_dirs`、扩展名大小写 |
| `internal/code/extract_test.go` | CODE 规范化、`ambiguous`、`no_match` |
| `internal/app/group_test.go` | 同 CODE 合并、unmatched 输出 |
| `internal/app/planner/planner_test.go` | `ReadOutState`、sidecar 完整时不抓取、确定性去冲突 |
| `internal/infra/fsx/*_test.go` | 原子写、临时文件清理、目标类型冲突、EXDEV 标记 |
| `internal/infra/httpx/httpx_test.go` | 代理模式禁用 keep-alive、图片代理开关、非法代理 URL |
| `internal/infra/cache/cache_test.go` | provider cache 读写、只读模式拒绝写入 |
| `internal/infra/imgx/imgx_test.go` | poster 从 fanart 右半边裁切 |
| `internal/provider/scrape_test.go` | requested -> fallback 逻辑、尝试链路记录、未知 provider |
| `internal/app/run/scrape_test.go` | requested provider JSON cache 命中、坏 JSON cache 忽略后转网络抓取 |
| `internal/provider/javbus/javbus_test.go` | JavBus fixture/golden 解析、图片请求 Referer/Cookie |
| `internal/provider/javdb/javdb_test.go` | JavDB 搜索结果匹配、登录页拒绝、CODE mismatch 拒绝、N/A runtime、男演员过滤、rating 文本解析、fixture/golden 解析 |
| `internal/nfo/nfo_test.go` | NFO XML 可解析、列表去重稳定、标题回退到 CODE |
| `internal/domain/report_test.go` | `RunReport.Finalize` 的 UTC、排序、summary，以及空数组 JSON 稳定输出 |
| `internal/app/run/e2e_test.go` | dry-run 零写入；apply 生成 sidecar、写 cache、移动视频；target conflict、仅缺 poster、图片请求头、移动失败 rollback |
| `internal/app/run/observer_test.go` | `Observer` 事件顺序与 `Execute`/`ExecuteWithObserver` 结果一致性 |
| `cmd/avmc/progress_ui_test.go` | 进度 UI 的 fallback / attempt 文案格式 |
| `cmd/avmc/main_integration_test.go` | stdout 非 TTY 时只输出一个 `RunReport` JSON |

## 2. Provider fixture / golden

当前 provider 解析测试使用本地 HTML fixture 与 golden JSON，避免依赖线上站点。

fixture 与 golden 目录：

- `internal/provider/javbus/testdata/`
- `internal/provider/javbus/golden/`
- `internal/provider/javdb/testdata/`
- `internal/provider/javdb/golden/`

重新生成 golden：

- `UPDATE_GOLDEN=1 go test ./internal/provider/javbus`
- `UPDATE_GOLDEN=1 go test ./internal/provider/javdb`

## 3. 当前 E2E 与集成覆盖点

当前仓库已经覆盖的端到端/集成行为包括：

- dry-run 不创建 `out/` 与 `cache/`
- dry-run 在 `NeedsScrape()=true` 时验证 provider，并生成 `planned` 的 `files[]`
- apply 生成 `NFO + poster + fanart`
- poster 由 fanart 右半边裁切生成
- apply 写出 provider HTML / JSON cache
- apply 移动视频到 `out/<CODE>/`
- sidecar 为目录或 `out/` 根为文件时返回 `target_conflict`
- 仅缺 poster 时从已有 `fanart.jpg` 生成 poster，且不抓 provider
- provider 图片请求头策略生效
- 多文件移动后续失败时回滚已移动文件
- stdout 非 TTY 时 stdout 仅输出一个 `RunReport` JSON
- `Observer` 的阶段事件顺序为 `scan -> group -> plan -> exec`

## 4. 推荐验证命令

- `go test ./...`
- `go vet ./...`

按主题缩小范围时，可使用：

- `go test ./internal/provider/...`
- `go test ./internal/app/run -count=1`
- `go test ./cmd/avmc -count=1`

## 5. 当前测试策略说明

- 当前测试以离线 fixture、stub provider 和临时目录为主。
- 自动化验证入口为 `go test ./...` 与 `go vet ./...`。
