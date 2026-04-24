# 端到端算法流程

本文档只描述当前代码已经实现的执行流程。

## 1. 输入与输出

- 输入：扫描根目录 `path/` 下的视频文件树。
- 输出：
  - `<path>/out/<CODE>/...`
  - `<path>/cache/providers/<provider>/<CODE>.{html,json}`
  - `apply` 模式下的 `<path>/cache/report.json`

## 2. 配置阶段

配置发现、字段校验和覆盖优先级由 `internal/config.LoadEffective` 处理，详见 [CONFIG.md](./CONFIG.md)。执行层直接消费 `config.EffectiveConfig`。

## 3. 扫描

入口：`internal/scan.ScanVideos`

步骤：

1. `filepath.WalkDir(path)` 遍历扫描根目录。
2. 永久排除 `<path>/out/` 和 `<path>/cache/`。
3. 额外排除 `exclude_dirs` 中的路径；相对路径按 `path` 解析，绝对路径按绝对路径处理。
4. 只收集扩展名在白名单中的文件：`.mp4`、`.mkv`、`.avi`。
5. 对每个命中文件只做 `DirEntry.Info()`，生成 `VideoFile{AbsPath, RelPath, Base, Ext, Size, ModUnix}`。
6. 最终按 `RelPath` 排序，保证稳定输出。

## 4. CODE 提取

入口：`internal/code.Extract`

步骤：

1. 从 `VideoFile.Base` 提取候选。
2. 从父目录名再提取一次候选。
3. 允许的候选格式为：`[a-z]{2,6}[\s._-]+[0-9]{2,5}`，提取后规范化为 `AAAA-999` 形态。
4. 空候选集合对应 `Unmatched{Kind:"no_match"}`。
5. 若有多个不同候选，返回 `Unmatched{Kind:"ambiguous", Candidates: ...}`。
6. 若唯一候选满足 `domain.ParseCode`，返回规范化后的 `Code`。

## 5. 分组

入口：`internal/app.GroupByCode`

步骤：

1. 遍历扫描结果，按 `Code` 把文件索引聚合到 `WorkItem.FileIdx`。
2. 无法提取 `Code` 的文件不进入 `WorkItem`，而是单独记为 `Unmatched`。
3. `items` 按 `Code` 字典序排序。
4. 每个 `WorkItem.FileIdx` 再按对应 `VideoFile.RelPath` 排序。

## 6. 规划

入口：`internal/app/planner.ReadOutState` 与 `internal/app/planner.PlanItem`

步骤：

1. 读取 `out/<CODE>/` 当前状态：
   - `HasNFO`
   - `HasPoster`
   - `HasFanart`
   - `ExistingNames`
2. 为该 `CODE` 的每个输入文件分配目标文件名：
   - 默认保留原文件名
   - 若目标目录已有同名文件，或同一批规划内已占用同名文件，则追加 `__2/__3...`
3. 计算 sidecar 需求：
   - `NeedNFO = !HasNFO`
   - `NeedPoster = !HasPoster`
   - `NeedFanart = !HasFanart`
   - `NeedsScrape() = NeedNFO || NeedFanart`

当前实现的含义是：

- 缺 NFO 或缺 fanart 时，必须抓 provider。
- 仅缺 poster 时，不触发 provider 抓取；`apply` 时会尝试从已存在的 `fanart.jpg` 生成 poster。
- sidecar 名称存在但不是普通文件时，规划阶段失败为 `target_conflict`，并禁止移动视频。

## 7. 执行

入口：`internal/app/run.ExecuteWithObserver`

### 7.1 总体流程

1. 创建 `metaClient`；`apply` 模式下再创建 `imageClient`。
2. 创建 `cache.Store`，`dry-run` 时为只读。
3. 扫描、提取、分组、规划。
4. 将 `ItemPlan` 投递给 worker pool；并发度取 `EffectiveConfig.Concurrency`，最小为 1。
5. 收集每个 item 的执行结果，最后调用 `RunReport.Finalize()` 排序并统计摘要。

### 7.2 dry-run

- 若 `NeedsScrape()=true`，调用 `scrape(...)` 验证 provider 可用性。
- 不写 `out/`、不写 `cache/`、不下载图片、不移动视频。
- `NeedNFO`、`NeedPoster`、`NeedFanart` 全为 `false` 且没有待移动文件的 item 记为 `skipped`。

### 7.3 apply

硬规则：移动永远最后一步。

执行顺序：

1. 若 `NeedsScrape()=true`，先抓取元数据。
2. `ensureDir(out/<CODE>)`。
3. 若缺 NFO，调用 `nfo.Encode(meta)` 并以“不覆盖”的原子写方式生成 `<CODE>.nfo`。
4. 若缺 fanart，下载 `meta.FanartURL` 并原子写入 `fanart.jpg`。
5. 若缺 poster：
   - 优先使用本次下载到内存的 fanart 字节；
   - 否则读取本地已有的 `fanart.jpg`；
   - 调用 `imgx.PosterFromFanartRightHalfJPEG` 生成 `poster.jpg`。
6. sidecar 全部满足后，逐文件 `rename` 到目标路径。
7. 若移动中途失败，尝试把之前已移动成功的文件按倒序 rollback 回原路径。

## 8. 抓取与 cache 逻辑

入口：`internal/app/run.scrape`

当前实现步骤：

1. 只尝试读取“requested provider”的 JSON cache：`<path>/cache/providers/<requested>/<CODE>.json`。
2. 可反序列化的 JSON cache 直接形成返回结果。
3. 若 JSON cache 缺失或内容损坏，忽略它并继续走网络。
4. 调用 `provider.FetchParseTrace`，按 `requested -> fallback` 顺序抓取与解析。
5. `apply` 模式下，把最终成功 provider 的 HTML 和 JSON 写回其目录；`dry-run` 不写 cache。

更细的 HTTP、代理、重试和 cache 规则见 [HTTP_CACHE.md](./HTTP_CACHE.md)。
