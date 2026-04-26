# 当前架构事实

本文档描述当前仓库的分层与依赖事实。

## 1. 包分层

### 1.1 `cmd/avmc`

- `main.go`：解析 CLI、加载配置、注册 provider、选择进度输出、调用执行入口、输出 report、返回退出码。
- `progress_ui.go`：实现交互终端进度 UI；通过 `run.Observer` 与执行层解耦。

### 1.2 `internal/config`

- 负责 `avmc.json` 发现、解析、字段校验和 CLI 覆盖合并。
- 产出 `config.EffectiveConfig`，供执行层直接消费。

### 1.3 `internal/app`

- `group.go`：把 `[]VideoFile` 按 `Code` 聚合成 `[]WorkItem`。
- `planner/`：读取 `out/<CODE>` 状态并生成 `ItemPlan`。
- `run/`：编排一次完整运行；负责调用 scan/code/planner/provider/http/cache/nfo/fs/image 等具体能力。

当前事实：`internal/app/run` 不是“只依赖抽象接口”的纯编排层，它直接依赖下列具体包：

- `internal/scan`
- `internal/app`
- `internal/app/planner`
- `internal/config`
- `internal/domain`
- `internal/provider`
- `internal/infra/cache`
- `internal/infra/httpx`
- `internal/infra/fsx`
- `internal/infra/imgx`
- `internal/nfo`

### 1.4 `internal/domain`

- 定义 `Code`、`VideoFile`、`WorkItem`、`ItemPlan`、`MovieMeta`、`RunReport` 等数据类型与错误码枚举。
- 职责聚焦于领域数据结构、枚举和值对象。

### 1.5 `internal/scan` 与 `internal/code`

- `scan`：扫描视频文件并应用排除规则。
- `code`：从文件名和父目录名提取唯一 `Code`。

### 1.6 `internal/provider`

- `provider.go`：定义统一 `Provider` 接口。
- `registry.go`：按名称注册和查找 provider。
- `scrape.go`：实现 requested -> fallback 的抓取/解析链路，并记录尝试轨迹。
- `javbus/`、`javdb/`：站点级 Fetch/Parse 实现和 fixture/golden 测试。

### 1.7 `internal/infra`

- `httpx`：构造 HTTP client，封装 UA 池、代理、keep-alive 策略、超时与有界重试。
- `cache`：读写 `<path>/cache/providers/<provider>/<CODE>.{html,json}`。
- `fsx`：原子写、rename、EXDEV 标记。
- `imgx`：从 fanart 右半边裁切 poster。

### 1.8 `internal/nfo`

- 把 `domain.MovieMeta` 编码为固定结构的 XML NFO。

## 2. 当前主执行链路

```text
cmd/avmc/main.go
  -> config.LoadEffective
  -> provider.NewRegistry
  -> run.ExecuteWithObserver
       -> httpx.NewMetaClient / NewImageClient
       -> cache.New
       -> scan.ScanVideos
       -> app.GroupByCode
       -> planner.ReadOutState
       -> planner.PlanItem
       -> execOne (worker pool)
            -> scrape
            -> nfo.Encode
            -> fsx.WriteFileAtomicNoOverwrite / WriteFileAtomicReplace
            -> imgx.PosterFromFanartRightHalfJPEG
            -> fsx.Rename
       -> RunReport.Finalize
  -> writeReportFile (apply only)
  -> emitReport
```

## 3. 当前依赖方向

当前依赖方向大体如下：

```text
cmd/avmc
  -> internal/config
  -> internal/app/run
  -> internal/domain
  -> internal/infra/fsx          // report.json 原子写
  -> internal/provider/*

internal/app/run
  -> internal/app
  -> internal/app/planner
  -> internal/config             // 消费 EffectiveConfig 类型
  -> internal/scan
  -> internal/provider
  -> internal/nfo
  -> internal/infra/{httpx,cache,fsx,imgx}
  -> internal/domain
```

`internal/domain` 处在最底层，供上层各包共享。

## 4. 当前边界说明

- 进度输出通过 `run.Observer` 解耦：`run` 只发事件，`cmd/avmc/progress_ui.go` 决定如何展示。
- provider 页面抓取与 HTML 解析留在 `internal/provider`；HTTP client、代理和重试策略留在 `internal/infra/httpx`。
- provider 可通过可选图片请求接口声明站点级图片下载请求头；执行层只应用该策略，不内置具体站点判断。

## 5. 当前架构约束

- `internal/domain` 负责承载领域数据结构与错误码枚举。
- `internal/scan` 的职责聚焦于目录遍历和 `stat`。
- `planner` 的职责聚焦于生成计划。
- `run` 负责把单条失败降级为 item 级失败，不因一个 `CODE` 的失败中断整个运行。
