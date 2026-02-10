# 文件系统契约（out/cache/report）

这份文档把“文件系统即 API”的约定写成硬规则。实现与测试必须以此为准。

## 1. 目录与文件布局

### 1.1 输出库（媒体库扫描入口）
固定输出到：
- `<path>/out/`

对每个 `CODE`：
```
<path>/out/<CODE>/
  <CODE>.nfo
  poster.jpg
  fanart.jpg
  <video files...>          # 可多个，默认保留原文件名
```

图片规则：
- `fanart.jpg` 是背景大图
- `poster.jpg` 由 `fanart.jpg` 的右半边裁切生成（因此在 fanart 已存在时，可不依赖 provider 额外下载）

### 1.2 缓存与内部状态
固定缓存到：
- `<path>/cache/`

建议布局：
```
<path>/cache/
  report.json
  providers/
    javbus/
      <CODE>.html
      <CODE>.json
    javdb/
      <CODE>.html
      <CODE>.json
```

## 2. dry-run vs apply（写入边界）

### 2.1 dry-run（默认）
- 仅当 item 需要生成 sidecar（`NeedScrape=true`）时，才执行 `fetch+parse` 做可用性验证（含 provider 自动降级）；允许读取已有 `<path>/cache/`（只读）。
- **不得写入** `out/` 与 `cache/`；不下载图片；不移动任何视频文件。
- 输出契约见 [CLI.md](./CLI.md) 与 [REPORT.md](./REPORT.md)（TTY 人类摘要；非 TTY stdout 仅 JSON）。

### 2.2 apply
- 允许写入 `out/` 与 `cache/`
- 允许移动视频文件
- `report.json` 固定写入 `<path>/cache/report.json`

## 3. 原子写入与不覆盖

### 3.1 sidecar 不覆盖（存在即满足）
- 若目标已存在 `<CODE>.nfo` / `poster.jpg` / `fanart.jpg`：视为**已满足**，本次跳过写入（不覆盖），不算失败。
- 只有当目标路径发生类型冲突或无法写入时才算失败：
  - `target_conflict`：例如目标路径是目录/不可作为文件。
  - `io_failed`：例如权限/磁盘满/创建目录失败/原子写失败等。

> 错误码与 report 结构见 [REPORT.md](./REPORT.md)。

### 3.2 move gating（硬规则）
- 对某个 item：只有当该 item 所需的 sidecar 目标全部“已存在或本次写入成功”时，才允许 move。
- 任何抓取/解析/下载/写入失败 => 该 item 禁止 move（移动永远是最后一步）。

### 3.3 原子写入（必须）
任何写文件必须遵守：
1) 写入同目录临时文件（如 `.<name>.tmp`）
2) fsync（可选但推荐）
3) `rename` 原子替换到最终文件名

目的：防止半成品污染输出，保证幂等可重跑。

## 4. 移动语义（rename + 最后一步）

### 4.1 移动永远是最后一步
- 任何刮削/解析/下载/写入失败 => 禁止移动任何视频文件
- move 只发生在该 item 的 sidecar 目标都已满足（已存在或本次写入成功）

### 4.2 仅 rename（同盘）
- 默认使用 `rename(src, dst)` 移动
- 若遇到 EXDEV（跨盘）=> 失败并提示用户调整目录结构；**不做 copy+delete**

### 4.3 同名去冲突（确定性）
同一 `CODE` 下默认保留原文件名；若目标目录已有同名文件：
- 追加后缀 `__2`、`__3`...（只改 base，不改 ext）
- 必须把 `src -> dst` 映射写入 report

### 4.4 回滚（推荐）
若多文件移动过程中中途失败：
- 尝试把已移动的文件 rollback 回原路径
- rollback 失败也要记录到 report（不能 silent）
