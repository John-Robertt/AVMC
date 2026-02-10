# 核心数据模型（Domain）

这份文档只回答三件事：
1) 系统要处理哪些数据？
2) 这些数据之间的关系是什么？
3) 哪些不变量必须被实现遵守？

## 1. 主键：CODE
- `CODE` 是作品的唯一主键（规范化后形如 `CAWD-895`）。
- 任何“智能猜测”都必须收敛到：**要么得到唯一 CODE，要么失败**。

### 1.1 CODE 规范化规则（建议）
- 字母段：大写
- 分隔符：统一为 `-`
- 结果形态：`[A-Z]{2,6}-[0-9]{2,5}`
- 若同一输入路径可得到多个不同 CODE：判定 `unmatched_code(ambiguous)`。

---

## 2. 输入数据

### 2.1 VideoFile
（建议以绝对路径作为内部主键，相对路径用于报告展示）

```go
type VideoFile struct {
  AbsPath string
  RelPath string
  Base    string // filename without ext
  Ext     string // ".mp4"
  Size    int64
  ModUnix int64
}
```

不变量：
- `AbsPath` 必须是 clean + absolute。
- 扫描阶段只做 `stat`，不读文件内容。

### 2.2 WorkItem（按 CODE 聚合）
```go
type WorkItem struct {
  Code Code
  FileIdx []int // 指向 []VideoFile
}
```

不变量：
- 1 个 WorkItem 对应 1 个 CODE。
- 同一 CODE 的多文件属于同一个 WorkItem：只刮削一次。

---

## 3. 输出状态与计划

### 3.1 OutState（仅由 stat 得到）
```go
type OutState struct {
  OutDir string
  HasNFO bool
  HasPoster bool
  HasFanart bool
  ExistingNames map[string]struct{} // 目录内现有文件名集合
}
```

不变量：
- 不读取文件内容，只做存在性/冲突判定。

### 3.2 ItemPlan（执行的最小单位）
```go
type MovePlan struct{ SrcAbs, DstAbs string }

type SidecarNeed struct {
  NeedScrape bool
  NeedNFO, NeedPoster, NeedFanart bool
}

type ItemPlan struct {
  Code Code
  ProviderRequested string
  Moves []MovePlan
  Need SidecarNeed
}
```

不变量：
- **Moves 在最后执行**（见 [IO_CONTRACT.md](./IO_CONTRACT.md)）。
- 若 `NeedScrape=true` 且刮削失败：禁止执行 Moves。

---

## 4. 元数据（刮削结果）

### 4.1 MovieMeta（最小可用集）
```go
type MovieMeta struct {
  Code Code
  Title string
  Studio string
  Series string
  Release string // ISO date, e.g. "2025-11-27"
  Year int
  RuntimeM int
  Actors []string
  Genres []string
  Tags []string
  Website string
  CoverURL string
  FanartURL string
}
```

不变量：
- `Website` 必须写入实际成功 provider 的详情页（来源标记）。
- 不追求全量字段；字段缺失允许为空，但必须结构稳定。
- 图片约定：
  - `fanart.jpg` 从 `FanartURL` 下载得到
  - `poster.jpg` 由 `fanart.jpg` 的右半边裁切生成（因此 `CoverURL` 当前不作为必须字段；必要时可与 `FanartURL` 相同）
- NFO 约定（不对外暴露配置）：
  - `mpaa` 固定为 `R18+`
  - `country` 固定为 `JP`

---

## 5. 报告（可恢复性的“事实记录”）

报告是可重跑与排障的唯一事实来源之一（另一个是文件系统）。

### 5.1 RunReport / ItemResult（建议结构，见 REPORT.md）
要点：
- `provider_requested` 与 `provider_used` 必须同时记录（自动降级可追溯）。
- 每个输入文件需要记录 `src -> dst` 映射（含同名去冲突结果）。
- 状态必须可枚举：`processed/skipped/failed/unmatched`。

---

## 6. 全局不变量（硬规则）
1) **移动最后一步**：任何失败不得移动视频文件。
2) **sidecar 原子写 + 不覆盖**：只能补齐缺失，不能覆盖已有。
3) **扫描排除固定**：`out/`、`cache/` 永远排除；`exclude_dirs` 额外排除。
4) **幂等**：重复运行不会破坏已完成输出；不完整仅补齐缺失。
