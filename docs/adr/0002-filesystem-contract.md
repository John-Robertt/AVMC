# ADR 0002：文件系统契约（out 与 cache 的职责划分）

## 状态
已接受

## 背景
媒体库需要一个“干净的输出目录结构”供扫描（Plex/Jellyfin/Emby/Kodi）。同时，为了幂等、可重跑、减少请求，需要缓存与运行状态。但如果把缓存混在 out 里，会污染媒体库结构并增加误扫描风险。

## 决策
1) 输出库固定为 `<path>/out/`，只包含媒体库相关内容（每个 CODE 一个目录 + sidecar + 视频文件）
2) 缓存与内部状态固定为 `<path>/cache/`
3) apply 报告固定为 `<path>/cache/report.json`；dry-run 只 stdout，不落盘
4) 扫描阶段永久排除 `<path>/out/` 与 `<path>/cache/`
5) 可恢复性硬规则：
   - sidecar 原子写 + 不覆盖
   - move 永远最后一步
   - 仅同盘 rename；EXDEV 失败不做 copy+delete

## 后果（Trade-offs）
- 优点：out 目录保持纯净可预测；缓存可一键清理；避免“扫到自己”；可恢复性更强。
- 缺点：多一个 cache 目录需要用户理解；但其固定位置与自动排除降低了心智负担。

