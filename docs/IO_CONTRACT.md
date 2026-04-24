# 文件系统契约

本文档定义当前代码实际使用的目录布局、写入边界和移动语义。

## 1. 目录布局

### 1.1 输出目录

固定输出到：`<path>/out/`

每个 `CODE` 的目录结构为：

```text
<path>/out/<CODE>/
  <video files...>
  <CODE>.nfo
  poster.jpg
  fanart.jpg
```

### 1.2 cache 目录

固定 cache 根目录：`<path>/cache/`

当前代码会使用：

```text
<path>/cache/
  report.json
  providers/
    <provider>/
      <CODE>.html
      <CODE>.json
```

## 2. dry-run 与 apply 的写入边界

### 2.1 dry-run

- 允许读取只读 cache。
- 不创建 `out/`。
- 不创建 `cache/`。
- 不下载图片。
- 不移动视频。

### 2.2 apply

- 允许写入 `out/`。
- 允许写入 `cache/`。
- 固定写 `report.json` 到 `<path>/cache/report.json`。
- sidecar 满足后允许移动视频。

## 3. sidecar 写入规则

### 3.1 不覆盖

- `<CODE>.nfo`、`poster.jpg`、`fanart.jpg` 若以普通文件形式存在，视为已满足，本次跳过写入。
- 若目标路径是目录或其它非普通文件，记为 `target_conflict`。

### 3.2 原子写

当前 `fsx.WriteFileAtomicNoOverwrite` / `WriteFileAtomicReplace` 的写入步骤为：

1. 在目标目录创建临时文件，文件名形如 `.<name>.tmp-*`
2. 写入内容
3. `tmp.Sync()`
4. `Rename(tmp, dst)`
5. 对目录做 best-effort `Sync()`

当前语义区分：

- sidecar 使用“不覆盖”的原子写。
- `report.json` 与 provider cache 使用“可覆盖”的原子写。

## 4. fanart 与 poster

- `fanart.jpg` 来自 `meta.FanartURL` 下载。
- `poster.jpg` 由 `fanart.jpg` 的右半边裁切生成。
- 若 `apply` 时仅缺 `poster.jpg`，会优先复用已存在的 `fanart.jpg`。

## 5. move 语义

### 5.1 移动最后一步

- 任何抓取、解析、下载、sidecar 写入失败，都会阻止当前 item 的视频移动。

### 5.2 rename only

- 当前实现只使用 `rename` 移动文件。
- 跨文件系统场景通过 EXDEV 映射为 `move_failed`。

### 5.3 同名去冲突

- 默认保留原文件名。
- 目标目录已有同名文件，或同一批规划内发生重名时，追加 `__2`、`__3` ...。
- `src -> dst` 映射必须写入 report。

### 5.4 rollback

- 多文件移动过程中若中途失败，当前实现会按倒序尝试 rollback 已移动成功的文件。
- rollback 成功的文件在 report 中标记为 `rolled_back`。
