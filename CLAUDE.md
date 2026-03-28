# CLAUDE.md

在本仓库中完成代码变更 = 满足以下全部条件：

1. `go test ./...` 和 `go vet ./...` 全量通过
2. 变更涉及的包对应权威文档（模块地图中的"权威文档"列）已同步更新
3. 依赖方向符合分层规则：cmd→app→{domain,infra,provider}→domain；domain 仅依赖标准库；infra 与 provider 各自独立
4. Sidecar 文件通过 `fsx.WriteFileAtomicNoOverwrite` 写入，仅创建不存在的文件
5. 视频文件移动（`os.Rename`）在 NFO 写入和图片下载之后执行
6. 代码定位使用函数名/类型名，使用符号引用而非硬编码行号

## 项目定位

AVMC 是一个 Go CLI：扫描本地视频目录 → 提取番号 CODE（主键）→ 抓取元数据 → 生成 sidecar（NFO/图片）→ 按媒体库友好结构落盘。

## 项目规则

### 文档优先于实现

`docs/` 是权威规则源。每条规则只有一个权威文档定义，其他文档只引用不重复。

- **判断方法**：修改代码后，查阅模块地图中该包对应的权威文档，确认接口和行为描述一致
- **权威文档索引**：`docs/README.md`

### 幂等可重跑

重复运行产生相同结果。仅创建不存在的 sidecar 文件，失败项完整跳过。

- **判断方法**：所有文件写入使用 `fsx.WriteFileAtomicNoOverwrite`，遇到 `os.ErrExist` 视为满足并跳过
- **代码入口**：`planner.ReadOutState` 读取 `out/<CODE>/` 目录现状 → `PlanItem` 设置 `NeedNFO/NeedPoster/NeedFanart` 标志

### Move 最后一步

视频文件只在所有其他操作成功后才移动。移动中途失败时按倒序回滚已移动文件。

- **判断方法**：在 `execOne` 函数中，`os.Rename` 调用在 NFO 写入和图片下载之后；`rollbackMoves` 按倒序回滚
- **代码入口**：`internal/app/run` 包的 `execOne` 函数

### 原子写入

先写临时文件再重命名，确保任何时刻文件系统只存在完整的最终文件。

- **判断方法**：新增文件写入统一使用 `fsx.WriteFileAtomicNoOverwrite`（`fsx/fsx.go`）
- **实现**：目标目录创建 `.{name}.tmp-*` 临时文件 → 写入 → Sync → Close → Rename。临时文件以 `.` 开头对媒体库扫描不可见

### Provider 降级

首选 provider 失败时自动回退到备选 provider。

- **判断方法**：`FetchParseTrace` 通过 `fallbackOrder` 获取降级链，依次尝试，首个成功即返回
- **代码入口**：`provider/scrape.go` 的 `FetchParseTrace` 函数

## 架构全貌

### 分层与依赖方向

依赖方向严格从上到下（上层调用下层），同层独立：

```
cmd/avmc/           → CLI 入口：参数解析、配置发现与合并
       ↓
internal/app/       → 用例编排：scan → extract → group → plan → execute → report
       ↓
internal/domain/    → 纯数据结构与不变量（零 IO，仅依赖标准库）
       ↑
internal/infra/     → IO 能力实现（fsx / httpx / cache / imgx）
internal/provider/  → 站点插件（javbus / javdb），HTML 解析 → MovieMeta
```

**依赖约束**：

- `domain` 仅依赖标准库，独立于 `infra`、`provider`、`app`、`cmd`
- `infra` 与 `provider` 各自独立，通过 `domain` 类型间接协作（如 `httpx.Client` 通过参数注入）
- `provider` 使用 `infra` 能力时，通过函数参数注入而非包级导入

**核心流水线**：`cmd/avmc/main.go` → `app/run` 包驱动端到端流程。app 层只处理 `WorkItem`，文件名拼接和 HTTP 重试细节分别由 infra 和 provider 层承担。

## 模块地图

| 代码包               | 职责                                  | 权威文档           | 关键入口                            | 测试命令                             |
| -------------------- | ------------------------------------- | ------------------ | ----------------------------------- | ------------------------------------ |
| `cmd/avmc/`          | CLI 入口                              | —                  | `main.go`                           | `go test ./cmd/avmc/...`             |
| `internal/app/`      | 用例编排（流水线算法中心）            | `ALGORITHMS.md`    | `run/`, `planner/`, `group.go`      | `go test ./internal/app/...`         |
| `internal/domain/`   | 纯数据结构与不变量                    | `DATA_MODEL.md`    | `code.go`, `meta.go`, `report.go`   | `go test ./internal/domain/...`      |
| `internal/infra/`    | IO 实现（文件/HTTP/缓存/图片）        | `IO_CONTRACT.md`   | `fsx/`, `httpx/`, `cache/`, `imgx/` | `go test ./internal/infra/...`       |
| `internal/provider/` | 站点插件（Fetch + Parse → MovieMeta） | `PROVIDERS.md`     | `javbus/`, `javdb/`, `registry.go`  | `go test ./internal/provider/<name>` |
| `internal/config/`   | 配置发现与覆盖合并                    | `CONFIG.md`        | `config.go`                         | `go test ./internal/config/...`      |
| `internal/code/`     | CODE 提取与规范化                     | `DATA_MODEL.md` §1 | `extract.go`                        | `go test ./internal/code/...`        |
| `internal/nfo/`      | NFO XML 生成                          | `DATA_MODEL.md` §4 | `nfo.go`                            | `go test ./internal/nfo/...`         |
| `internal/scan/`     | 文件扫描与排除                        | `CONFIG.md` §3     | `scan.go`                           | `go test ./internal/scan/...`        |

### 扩展：新增 Provider

1. 新建包 `internal/provider/<name>/`
2. 实现 `Provider` 接口（`Fetch` + `Parse`，见 `PROVIDERS.md`）
3. 添加 `testdata/*.html` 和 golden JSON 测试
4. 注册到 `registry.go`

## 开发操作

```bash
go build -o avmc ./cmd/avmc          # 构建
docker build -t avmc:local .         # Docker 构建
go test ./...                        # 全量回归
go vet ./...                         # 静态检查
UPDATE_GOLDEN=1 go test ./internal/provider/<name>  # 更新 golden 文件
```

配置文件 `avmc.json` 放在扫描目录或 cwd，详见 `docs/CONFIG.md`。发布通过 GitHub Actions：推送 `v*` tag 触发多平台二进制 + Docker 镜像（GHCR），详见 `docs/BUILD.md`。

## 代码风格

- 注释使用中文，描述意图和失败语义
- 标准库分组导入，项目内部导入用空行分隔
- 错误处理使用 Go 惯用的 `value, ok` 双返回值模式（见 `domain.ParseCode`）
- 文件组织：每个包职责单一，文件按核心类型命名（如 `code.go`、`meta.go`）
- 测试使用 `testdata/` 目录放置 fixture 文件，golden 测试通过 `UPDATE_GOLDEN=1` 环境变量更新

## 每次变更验证流程

### 第一步：自动验证

```bash
go test ./...        # 全量测试通过
go vet ./...         # 静态检查通过
```

### 第二步：逐条检查

| #   | 检查项         | 通过标准                                          | 修复方法                                     |
| --- | -------------- | ------------------------------------------------- | -------------------------------------------- |
| 1   | 文档同步       | 模块地图中该包的权威文档描述与代码一致            | 更新文档中对应的接口/行为描述                |
| 2   | 依赖方向       | 新增 import 符合分层规则（见架构图）              | 将反向依赖重构为参数注入或接口解耦           |
| 3   | domain 纯净    | domain 包新增文件仅导入标准库，无 IO 操作         | 将 IO 逻辑移至 infra 层，domain 通过接口声明 |
| 4   | Sidecar 仅创建 | 文件写入统一调用 `fsx.WriteFileAtomicNoOverwrite` | 替换为该函数                                 |
| 5   | Move 最后      | `os.Rename` 在 NFO 写入和图片下载之后             | 调整 `execOne` 中的操作顺序                  |

### 第三步：测试失败处理

1. 阅读错误信息，定位失败的测试函数
2. 判断是测试过时还是代码逻辑错误
3. 修复后重跑 `go test ./...`，确保全量通过
