# AVMC 产品说明

## 1. 产品目标

AVMC 是一个 Go CLI，用来把“文件名包含番号 CODE 的本地视频目录”整理成媒体库友好的结构。当前产品由以下能力组成：

- 扫描本地目录中的视频文件（当前支持 `.mp4`、`.mkv`、`.avi`）。
- 从文件名和父目录名提取唯一 `CODE`。
- 从 `javbus` 或 `javdb` 抓取元数据。
- 生成 `NFO + poster + fanart` sidecar。
- 把同一 `CODE` 的视频移动到固定目录 `out/<CODE>/`。
- 输出稳定的 `RunReport` JSON，便于排障与重跑。

## 2. 当前用户可见能力

### 2.1 输入

- 输入数据来自本地视频目录。
- 扫描根目录由 CLI `path` 或 `avmc.json` 提供。
- 同一 `CODE` 允许对应多个视频文件。

### 2.2 输出

- 输出固定写到 `<path>/out/<CODE>/`。
- 当前 sidecar 固定为：`<CODE>.nfo`、`poster.jpg`、`fanart.jpg`。
- `apply` 模式会把视频文件移动到 `out/<CODE>/`；默认保留原文件名，冲突时追加 `__2/__3...`。
- `apply` 模式固定写 `RunReport` 到 `<path>/cache/report.json`。

### 2.3 运行模式

- `dry-run`：输出扫描、分组、规划结果，以及必要的 provider 可用性验证结果。
- `apply`：生成 sidecar、provider cache、`report.json`，并完成视频归档。

### 2.4 对外入口

- CLI 命令：`avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]`
- 配置文件：`avmc.json`
- Docker 镜像：`ghcr.io/john-robertt/avmc`

详细行为见 [CLI.md](./CLI.md) 与 [CONFIG.md](./CONFIG.md)。

## 3. 当前硬规则

- 一切围绕 `CODE` 建模：扫描 -> 提取 -> 分组 -> 规划 -> 执行。
- `CODE` 提取以唯一性为准；唯一结果进入正常流程，无法唯一化的输入进入 `unmatched` 结果集。
- 移动永远是最后一步。
- 已存在的 sidecar 不覆盖，只补齐缺失项。
- 输出结构必须稳定、可重跑。

详细规则分别见 [DATA_MODEL.md](./DATA_MODEL.md)、[ALGORITHMS.md](./ALGORITHMS.md)、[IO_CONTRACT.md](./IO_CONTRACT.md)。

## 4. 当前能力范围

- provider 列表为 `javbus` 与 `javdb`。
- 元数据抓取入口为 HTML 页面。
- 图片输出固定为 `fanart.jpg` 与由其右半边裁切得到的 `poster.jpg`。
- NFO 输出固定为 Kodi/Jellyfin/Emby 可读取的 `<movie>` 结构。

详细字段见 [PROVIDERS.md](./PROVIDERS.md) 与 [NFO.md](./NFO.md)。

## 5. 参考文档

- [CLI.md](./CLI.md)：命令、输出、退出码
- [CONFIG.md](./CONFIG.md)：配置发现与字段语义
- [IO_CONTRACT.md](./IO_CONTRACT.md)：文件布局、原子写、移动语义
- [HTTP_CACHE.md](./HTTP_CACHE.md)：HTTP client、代理、cache
- [PROVIDERS.md](./PROVIDERS.md)：provider 接口与站点解析规则
- [NFO.md](./NFO.md)：NFO 字段映射
- [REPORT.md](./REPORT.md)：运行报告 JSON 结构
