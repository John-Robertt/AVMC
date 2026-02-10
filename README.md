# AVMC

把“混乱的本地视频目录（以番号 CODE 为主键）”一键整理成媒体库友好的结构，并补齐元数据侧车文件：

- 自动识别番号（如 `CAWD-895`），按作品归档到固定目录：`<path>/out/<CODE>/`
- 从 `JavBus` / `JavDB` 抓取并生成：
  - `<CODE>.nfo`（Kodi/Jellyfin/Emby 可读）
  - `fanart.jpg`（背景图）
  - `poster.jpg`（由 `fanart.jpg` 右半边裁切生成）
- 可选择 **dry-run 预演**（默认，不改动文件）或 **apply 执行**（落盘+移动视频）
- 幂等可重跑：不覆盖已有 sidecar；同名视频自动去冲突；失败不会“半成品污染”

> 注意：本工具只处理你本地已有的视频文件，不下载任何视频内容；请自行确保使用场景符合当地法律法规与站点条款。

## 你会得到什么（输出长什么样）

对每个识别到的 `CODE`，输出固定为：

```text
<path>/
  out/
    <CODE>/
      <CODE>.nfo
      fanart.jpg
      poster.jpg
      <video files...>   # 可能多个；默认保留原文件名
  cache/
    report.json          # 仅 apply 会写入
    providers/           # 仅 apply 会写入（HTML/JSON 缓存，便于排障与复跑）
```

扫描时会**永久排除**：`<path>/out/` 与 `<path>/cache/`（无需配置）。

## 三分钟上手（推荐流程）

1) 准备一个目录（下文用 `/data/videos` 举例），把视频放进去（支持：`.mp4/.mkv/.avi`）。
2) 先 dry-run 预演（默认不改动文件），并把 JSON 报告导出到文件方便查看：

```bash
./avmc run /data/videos > dryrun.report.json
```

3) 确认无 `failed/unmatched` 后，再执行 apply（会写入 `out/`/`cache/` 并移动视频）：

```bash
./avmc run /data/videos --apply
```

apply 成功后，报告固定落盘在：`/data/videos/cache/report.json`。

## 命令行用法

```bash
avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]
```

- `path`：扫描根目录（可省略，用于“配置文件一键运行”，见下文）
- `--provider`：首选刮削源（失败会自动降级到另一个 provider）
- `--apply`：真正写入与移动；默认 dry-run；支持 `--apply=false` 临时覆盖配置

退出码（便于脚本化）：
- `failed==0` 且 `unmatched==0` => exit `0`
- 否则 exit `1`

## 配置文件（avmc.json）

AVMC 支持在固定位置读取 `avmc.json`，用来控制并发/代理/排除目录等高级能力，并支持“一键运行”：

- 若命令里提供了 `path`：尝试读取 `<path>/avmc.json`（可选）
- 若命令里未提供 `path`：必须读取当前目录 `./avmc.json`（必选，且其中必须包含 `path`）

最常用的一个模板（把示例值改成你自己的）：

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

## 识别规则（如何让工具认出你的番号）

AVMC 会从「文件名」和「父目录名」里提取唯一 `CODE`：

- 形式：`[字母 2~6 位] + 分隔符（空格/点/下划线/中划线等） + [数字 2~5 位]`
- 例子（都能识别）：`CAWD-895`、`cawd_895`、`cawd 895`、`CAWD.895`

常见导致 `unmatched` 的原因：
- 文件名里完全没有番号片段（例如只叫 `movie.mp4`）
- 同一个文件名/目录名里出现了多个不同号码片段（ambiguous）

## dry-run 与 apply（最重要的安全说明）

- dry-run（默认）：
  - 不写入 `out/`、不写入 `cache/`
  - 不下载图片、不移动任何视频文件
  - 仅在“确实需要生成 sidecar”时，才会做一次 `fetch+parse` 来验证 provider 可用性
- apply（`--apply`）：
  - 写入 `out/` 与 `cache/`
  - 生成/补齐 sidecar（存在即跳过，不覆盖）
  - **移动视频永远是最后一步**：只要刮削/下载/写入任一步失败，本条目就不会移动视频

## 常见报错速查（按报告里的 error_code）

- `unmatched_code`：无法从文件名/目录解析出唯一 CODE（重命名即可）
- `fetch_failed`：抓取失败（网络/超时/被限流/被引导验证页）；尝试降低并发、换 provider、配置 `proxy.url`
- `parse_failed`：页面拿到了但结构变了（provider 解析跟不上站点改版）；可先换另一个 provider 或稍后再试
- `move_failed`：移动失败（权限/被占用/跨盘 EXDEV）；确保源文件与 `<path>/out/` 在同一文件系统、且有写权限
- `target_conflict`：目标路径类型冲突（例如 `out/<CODE>` 被一个同名文件占了）；清理冲突后重跑
- `io_failed`：通用 IO（权限/磁盘/创建目录/写文件失败）；按 `error_msg` 提示处理

## 安装与运行

### 方式 A：直接使用仓库内的二进制

仓库根目录存在 `avmc` 可执行文件时，直接运行：

```bash
./avmc --help
./avmc run /data/videos
```

### 方式 B：Docker（从 GHCR 直接拉取运行，推荐）

常用命令：

```bash
docker run --rm ghcr.io/john-robertt/avmc:latest --help

# dry-run（默认不改动文件；stdout 可重定向为 JSON 报告）
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos

# apply（写入 out/cache 并移动视频）
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos --apply
```

> 提示：容器内 apply 需要对挂载目录有写权限，否则会出现 `io_failed`。

如果你希望用 `avmc.json` 配置（并发/代理/排除目录等），把它放到挂载目录里即可（例如 `/data/videos/avmc.json`），容器里用 `run /data/videos` 触发读取。

### 方式 C：Docker（本地构建）

```bash
docker build -t avmc:local .
docker run --rm -v /data/videos:/data/videos avmc:local run /data/videos
docker run --rm -v /data/videos:/data/videos avmc:local run /data/videos --apply
```

### 方式 D：从源码构建（Go 1.22+）

```bash
go build -o avmc ./cmd/avmc
./avmc run /data/videos
```

## 更详细的文档

如果你要深入了解“配置覆盖规则 / 报告 JSON 结构 / 文件系统契约 / provider 行为”，建议从这里开始：
- `docs/README.md`（文档索引）
- `docs/CLI.md`（命令行细节）
- `docs/CONFIG.md`（avmc.json）
- `docs/REPORT.md`（report.json 结构与错误码）
- `docs/IO_CONTRACT.md`（out/cache 的硬规则）
