# AVMC

AVMC 是一个 Go CLI，用来把以 `CODE` 为主键的本地视频目录整理成固定的媒体库结构，并生成配套 sidecar 文件。

## 核心能力

- 从文件名和父目录名提取 `CODE`，例如 `CAWD-895`
- 按 `CODE` 归档视频到固定目录：`<path>/out/<CODE>/`
- 从 `JavBus` / `JavDB` 抓取元数据
- 生成：
  - `<CODE>.nfo`
  - `fanart.jpg`
  - `poster.jpg`（由 `fanart.jpg` 右半边裁切生成）
- 提供 `dry-run` 与 `apply` 两种运行模式
- 输出稳定的 `RunReport` JSON，便于查看结果与后续处理

## 输出布局

`apply` 模式下，输出目录形态如下：

```text
<path>/
  out/
    <CODE>/
      <CODE>.nfo
      fanart.jpg
      poster.jpg
      <video files...>
  cache/
    report.json
    providers/
      <provider>/
        <CODE>.html
        <CODE>.json
```

扫描时会使用固定的输入视图：`<path>/out/` 与 `<path>/cache/` 不参与扫描集合。

## 快速开始

1. 准备一个视频目录，例如 `/data/videos`
2. 先运行 `dry-run` 查看规划结果
3. 再运行 `apply` 生成 sidecar 并归档视频

```bash
./avmc run /data/videos > dryrun.report.json
./avmc run /data/videos --apply
```

`apply` 完成后，报告文件位于：`/data/videos/cache/report.json`

## 命令行

```bash
avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]
```

- `path`：扫描根目录
- `--provider`：首选 provider
- `--apply`：切换到真实执行模式
- `--apply=false`：显式使用 `dry-run`

退出码：

- `0`：执行完成，且 `failed==0 && unmatched==0`
- `1`：执行完成但存在 `failed` 或 `unmatched`，或运行阶段返回错误

## 配置文件

AVMC 使用 `avmc.json` 承载高级配置。最常用的模板如下：

```json
{
  "path": "/data/videos",
  "provider": "javbus",
  "apply": false,
  "concurrency": 4,
  "proxy": {
    "url": "http://127.0.0.1:8080"
  },
  "image_proxy": false,
  "exclude_dirs": ["temp", "downloads"],
  "javdb_base_url": "https://javdb565.com"
}
```

配置文件发现规则：

- 命令提供 `path` 时，读取 `<path>/avmc.json`
- 命令省略 `path` 时，读取当前目录的 `./avmc.json`

## CODE 识别

AVMC 从文件名和父目录名提取 `CODE`，规范化后的形态为：`[A-Z]{2,6}-[0-9]{2,5}`。

示例：

- `CAWD-895.mp4` -> `CAWD-895`
- `cawd_895.mkv` -> `CAWD-895`
- `CAWD.895/part1.avi` -> `CAWD-895`

提取结果会体现在 `RunReport` 中：

- 唯一 `CODE` 进入正常流程
- 无法唯一化的输入以 `unmatched_code` 记录

## 运行模式

### dry-run

- 输出扫描、分组、规划结果
- 对需要抓取的条目执行 provider 可用性验证
- 通过 stdout 输出 `RunReport`

### apply

- 生成 sidecar
- 写入 provider cache
- 写入 `report.json`
- 把视频归档到 `out/<CODE>/`

## report 与错误码

`RunReport` 的 `error_code` 当前包括：

- `unmatched_code`
- `fetch_failed`
- `parse_failed`
- `target_conflict`
- `io_failed`
- `move_failed`
- `config_not_found`
- `config_invalid`
- `config_missing_path`

完整结构见 `docs/REPORT.md`。

## 获取与运行

### 仓库内二进制

```bash
./avmc --help
./avmc run /data/videos
```

### Docker（GHCR）

```bash
docker run --rm ghcr.io/john-robertt/avmc:latest --help
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos --apply
```

### 本地构建

```bash
go build -o avmc ./cmd/avmc
./avmc run /data/videos
```

## 文档入口

- `docs/README.md`：文档注册表
- `docs/CLI.md`：CLI 细节
- `docs/CONFIG.md`：配置规则
- `docs/ALGORITHMS.md`：执行流程
- `docs/IO_CONTRACT.md`：文件系统契约
- `docs/HTTP_CACHE.md`：HTTP 与 cache
- `docs/PROVIDERS.md`：provider 规则
- `docs/NFO.md`：NFO 字段映射
- `docs/REPORT.md`：运行报告结构
