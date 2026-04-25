# CLI 使用说明

当前 CLI 只暴露 1 个命令和 3 个运行入口：`path`、`provider`、`apply`。

## 1. 命令

```bash
avmc --help
avmc run --help
avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]
```

参数：

- `path`：扫描根目录。可省略；省略时按 [CONFIG.md](./CONFIG.md) 的规则读取 `./avmc.json`。
- `--provider`：首选 provider，允许值为 `javbus` 或 `javdb`。
- `--apply`：开启真实写入与移动；默认是 dry-run。
- `--apply=false`：显式覆盖配置文件中的 `apply=true`。

## 2. 运行模式

### 2.1 dry-run

```bash
avmc run /data/videos
```

行为：

- 做扫描、提取、分组、规划。
- 仅在 `NeedsScrape()=true` 时验证 provider 可用性。
- 不写 `out/`、不写 `cache/`、不下载图片、不移动视频。

### 2.2 apply

```bash
avmc run /data/videos --apply
```

行为：

- 写入 sidecar。
- 写入 provider cache 和 `report.json`。
- 在 sidecar 满足后移动视频到 `out/<CODE>/`。

### 2.3 指定首选 provider

```bash
avmc run /data/videos --provider javdb
```

说明：requested provider 失败时会自动降级到另一个 provider，并在 report 中记录 `provider_requested` 与 `provider_used`。

## 3. 输出通道

### 3.1 report 输出

- 若 `stdout` 是 TTY：`stdout` 输出一行人类摘要；存在 `failed` 或 `unmatched` 条目时，逐条错误详情写到 `stderr`。
- 若 `stdout` 不是 TTY：`stdout` 必须且仅输出一个 `RunReport` JSON；人类摘要写到 `stderr`。

### 3.2 进度输出

- 交互进度只在存在终端时启用。
- 进度输出优先写到 `stderr`。
- 若 `stderr` 不是 TTY 但 `stdout` 是 TTY，则退化输出到 `stdout`。
- 非 TTY 的 `stdout` 保留给单个 `RunReport` JSON。
- 交互模式完成后，当前 CLI 还会额外打印 `out:` 路径；`apply` 模式会再打印 `report:` 路径。

### 3.3 report.json

- `apply`：写入 `<path>/cache/report.json`。
- `dry-run`：不落盘。

## 4. 退出码

- `0`：执行完成，且 `failed==0 && unmatched==0`；帮助命令也返回 `0`
- `1`：运行失败、配置错误、report 写入失败，或最终存在 `failed/unmatched`
- `2`：CLI 用法错误，例如未知命令或非法参数

## 5. Docker

```bash
docker run --rm ghcr.io/john-robertt/avmc:latest --help
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos --apply
```

如果要使用配置文件，把 `avmc.json` 放到挂载目录中，例如 `/data/videos/avmc.json`。
