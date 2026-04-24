# 核心数据模型

本文档对应 `internal/domain/` 以及相关构造逻辑。

## 1. `Code`

- `Code` 是作品主键，规范化后形如 `CAWD-895`。
- 当前合法形态由 `domain.ParseCode` 定义：`[A-Z]{2,6}-[0-9]{2,5}`。
- 约束：要么得到唯一 `Code`，要么失败；不允许写错。

## 2. 输入模型

### 2.1 `VideoFile`

```go
type VideoFile struct {
  AbsPath string
  RelPath string
  Base    string
  Ext     string
  Size    int64
  ModUnix int64
}
```

不变量：

- `AbsPath` 必须是 clean + absolute。
- 扫描阶段只做 `stat`，不读文件内容。

### 2.2 `Unmatched`

```go
type Unmatched struct {
  File       VideoFile
  Kind       string // "no_match" | "ambiguous"
  Candidates []Code
}
```

含义：描述无法解析出唯一 `Code` 的输入文件。

### 2.3 `WorkItem`

```go
type WorkItem struct {
  Code    Code
  FileIdx []int
}
```

不变量：

- 一个 `WorkItem` 对应一个 `Code`。
- 同一 `Code` 的多个输入文件共享一次抓取结果。

## 3. 规划模型

### 3.1 `OutState`

```go
type OutState struct {
  OutDir string
  HasNFO    bool
  HasPoster bool
  HasFanart bool
  ExistingNames map[string]struct{}
}
```

含义：描述 `out/<CODE>/` 的现状；只做 `stat` / `ReadDir`，不读文件内容。

### 3.2 `MovePlan` / `SidecarNeed` / `ItemPlan`

```go
type MovePlan struct {
  SrcAbs string
  DstAbs string
}

type SidecarNeed struct {
  NeedNFO    bool
  NeedPoster bool
  NeedFanart bool
}

func (n SidecarNeed) NeedsScrape() bool

type ItemPlan struct {
  Code              Code
  ProviderRequested string
  Moves             []MovePlan
  Need              SidecarNeed
}
```

不变量：

- `Moves` 只是计划，不代表已经执行。
- `NeedsScrape()` 当前等于 `NeedNFO || NeedFanart`。
- `Moves` 的执行前提是抓取与 sidecar 阶段顺利完成。

## 4. 元数据模型

### 4.1 `MovieMeta`

```go
type MovieMeta struct {
  Code     Code
  Title    string
  Studio   string
  Series   string
  Release  string
  Year     int
  RuntimeM int

  Actors []string
  Genres []string
  Tags   []string

  Website   string
  CoverURL  string
  FanartURL string
}
```

不变量：

- `Website` 必须是最终成功 provider 的详情页 URL。
- 字段缺失允许为空，但结构保持稳定。
- `FanartURL` 用于下载 `fanart.jpg`；`poster.jpg` 由 fanart 裁切生成。

## 5. 运行报告模型

- `RunReport`、`ItemResult`、`ProviderAttempt`、`FileResult` 的 JSON 结构见 [REPORT.md](./REPORT.md)。
- 当前状态枚举固定为：`processed`、`skipped`、`failed`、`unmatched`。
- 当前文件状态枚举固定为：`planned`、`moved`、`rolled_back`、`failed`。

## 6. 全局不变量

1. 移动永远最后一步。
2. sidecar 原子写且不覆盖已有文件。
3. sidecar 只有普通文件才算已满足；目录、符号链接或其它非普通文件视为 `target_conflict`。
4. 扫描固定排除 `<path>/out/` 与 `<path>/cache/`。
5. 重复运行应保持幂等：已完整条目跳过，不完整条目只补缺失项。
