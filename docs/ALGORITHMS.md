# 端到端算法流程（可验证）

这份文档把 PRD 的对外行为落成可实现、可测试的算法步骤。核心思想：**先把数据结构定死，算法自然变成线性流水线**。

## 0. 输入/输出
- 输入：`path/` 下的视频文件树
- 输出：`<path>/out/<CODE>/...`（媒体库结构）与 `<path>/cache/...`（缓存/报告）

## 1. 配置发现与合并（确定性）
输入：CLI（仅 `path/provider/apply`）+ `avmc.json`  
输出：`EffectiveConfig`

步骤：
1) 若 CLI 提供 `path`：
   - `path = abs(clean(cli.path))`
   - 读取 `<path>/avmc.json`（可选）
2) 若 CLI 不提供 `path`：
   - 必须读取 `./avmc.json`（必选），且必须包含 `path`
   - `path = abs(clean(config.path))`
3) 覆盖优先级：
   - `provider`：CLI > config > 默认 `javbus`
   - `apply`：CLI `--apply/--apply=false` > config > 默认 `false`
   - 其他字段：仅 config 控制
4) 计算排除目录：
   - 永久排除：`<path>/out/`、`<path>/cache/`
   - 配置排除：`exclude_dirs`（相对 path）

验证点：
- `avmc run`（无参）在 cwd 有 `avmc.json` 且含 `path` 时可以运行
- `--apply=false` 能覆盖 config.apply=true

---

## 2. 扫描（O(N)）
目标：只收集“可能是视频”的文件，不读内容。

算法：
1) `WalkDir(path)`
2) 若当前目录命中排除（前缀匹配 + 路径边界）=> `SkipDir`
3) 若是文件且 ext 在白名单 => 收集 `VideoFile{AbsPath, RelPath, Ext, Size, ModTime}`

验证点：
- `out/` 与 `cache/` 永久不被扫描
- `exclude_dirs` 指定的目录整棵被排除

---

## 3. CODE 提取（严格且可解释）
输入：`VideoFile`（文件名 + 父目录名作为候选）  
输出：`Code` 或 `unmatched_code`

原则：
- 允许失败，不允许写错
- 多候选冲突 => 失败（ambiguous）

验证点：
- 大小写/分隔符变体能规范化为同一 CODE
- ambiguous/no_match 都能给出明确原因

---

## 4. 分组（扁平数据，低拷贝）
输入：`files []VideoFile`  
输出：`items []WorkItem`（每个 WorkItem 只存 file index）

算法：
- `index := map[Code]int{}`
- 逐个 file：
  - `code = Extract(file)`
  - 若 unmatched：记录到 report（`status=unmatched`），不进入 items
  - 否则：
    - 若 `index[code]` 不存在：`items = append(items, WorkItem{Code: code})`，并记录 index
    - `items[idx].FileIdx = append(...)`
- 排序（必须确定性）：
  - `items` 按 `Code` 字典序排序
  - 每个 item 内：`FileIdx` 按对应 `VideoFile.RelPath` 字典序排序

验证点：
- 同一 CODE 多文件 => 归并为一个 item
- 输出顺序稳定（`Code` 字典序；item 内按 `RelPath`），保证报告与去冲突结果可复现

---

## 5. 计划（Planner）
输入：`WorkItem` + 当前文件系统（stat）  
输出：`ItemPlan`

步骤：
1) `outDir = <path>/out/<CODE>/`
2) `OutState = stat(outDir)`：检测 nfo/poster/fanart 是否已存在；收集目录内现有文件名集合
3) `NeedScrape = NeedNFO || NeedFanart`
   - poster 由 fanart 的右半边裁切得到：当且仅当需要 NFO 或 fanart 时才必须刮削
4) `MovePlan`：
   - 默认保留原文件名
   - 若目标同名冲突（含“目录已有”和“本次规划内已占用”）=> 追加 `__2/__3...`（确定性）
   - 分配规则：从 `OutState.ExistingNames` 初始化 `used` 集合；按 item 内稳定顺序逐条分配，并把新分配的名字加入 `used`

验证点：
- 已完整条目被标记为 skipped（除非有新增文件需要归档）
- 同名冲突得到确定性的 dst 名，并写入 report 映射

---

## 6. 执行（Executor）
并发模型：按 WorkItem 并发（worker pool），item 内串行。

### 6.1 dry-run
- 仅对 `NeedScrape=true` 的 item 执行 `fetch+parse` 做可用性验证（含 provider 自动降级）；允许读取已有 `<path>/cache/`（只读）。
- **不得写入** `out/` 与 `cache/`；不下载图片；不移动任何视频文件。
- 输出契约见 [CLI.md](./CLI.md) 与 [REPORT.md](./REPORT.md)（TTY 人类摘要；非 TTY stdout 仅 JSON）。

### 6.2 apply
硬规则：移动最后一步。

步骤（每个 item）：
1) 若 `NeedScrape`：
   - `Scrape(code)`：cache->fetch->parse（requested 失败自动降级）
   - 缓存策略（必须自愈，避免“坏缓存卡死”）：
     - 优先读 `<path>/cache/providers/<p>/<CODE>.json`；存在且可解析则直接使用
     - 否则尝试 `<CODE>.html`：parse 失败时，必须绕过缓存强制 refetch 一次再 parse；仍失败才 `parse_failed`
     - 网络 fetch 失败但存在可用缓存时，允许回退使用缓存（提高可用性）
   - apply 成功后写入/更新 `<CODE>.html` 与 `<CODE>.json`；dry-run 禁止写入 cache
2) sidecar：
   - 若缺失则写入（原子写 + 不覆盖）
   - 图片按 `image_proxy` 决定是否走代理
   - fanart：下载背景图写入 `fanart.jpg`
   - poster：从 fanart 右半边裁切生成 `poster.jpg`（不再单独下载 cover）
3) move：
   - 逐文件 `rename` 到目标（同盘优先）
   - 中途失败 => 记录 `move_failed`，并尝试回滚已移动文件
4) report：
   - item 结果汇总（含 provider_used 与 src->dst）

验证点：
- `NeedScrape=true` 且刮削失败 => 该 item 禁止 move
- sidecar 写入采用临时文件 + rename；已有文件不覆盖
- EXDEV（跨盘）=> 失败并提示，不做 copy+delete
