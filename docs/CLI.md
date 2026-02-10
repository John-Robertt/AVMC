# CLI 使用说明

CLI 只保留三个入口：`path/provider/apply`。其余配置全部通过 `avmc.json` 控制（见 [CONFIG.md](./CONFIG.md)）。

## 1. 命令
```bash
avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]
```

参数：
- `path`：扫描根目录（可省略；用于配置文件一键运行）
- `--provider`：首选刮削源（失败会自动降级到另一个 provider）
- `--apply`：真实写入与移动；默认 dry-run；支持 `--apply=false`

## 2. 典型用法

### 2.1 dry-run（默认）
```bash
avmc run /data/videos
```
行为：
- 扫描 + 计划；若需要生成 sidecar，则会执行 `fetch+parse` 做可用性验证（含 provider 自动降级）。
- **不得写入** `out/` 与 `cache/`；不下载图片；不移动任何视频文件。

输出（对外契约，详见下文「输出与退出码」）：
- stdout 是 TTY：stdout 输出人类摘要；stderr 输出日志。
- stdout 非 TTY：stdout **仅输出一个** `RunReport` JSON；stderr 输出人类摘要/日志（JSON 结构见 [REPORT.md](./REPORT.md)）。

### 2.2 apply（落盘 + 移动）
```bash
avmc run /data/videos --apply
```
行为：
- 写入 `<path>/out/` 与 `<path>/cache/`，并移动视频到 `out/<CODE>/`（移动永远是最后一步）。
- 运行报告固定写入：`<path>/cache/report.json`（结构见 [REPORT.md](./REPORT.md)）。

### 2.3 指定 provider（首选）
```bash
avmc run /data/videos --provider javdb
```
说明：首选 javdb；若失败会自动降级到 javbus，并在 report 中标记实际来源。

### 2.4 一键运行（无参）
前提：当前目录存在 `./avmc.json` 且包含 `path` 字段。
```bash
avmc run
```

### 2.5 覆盖配置中的 apply=true（临时 dry-run）
```bash
avmc run --apply=false
```

## 3. 输出与退出码（对外契约）

### 3.1 stdout/stderr
- stdout 是 TTY：stdout 输出人类摘要（便于交互查看）。
- stdout 非 TTY：stdout 仅输出一个 `RunReport` JSON（便于脚本化）。
- 任何模式下日志/进度信息都写入 stderr（避免污染 JSON）。

### 3.2 report.json
- apply：写入 `<path>/cache/report.json`。
- dry-run：**不落盘**；当 stdout 非 TTY 时，stdout 输出的 JSON 与 report.json **同结构**。

### 3.3 退出码（最小且可解释）
- `failed==0` 且 `unmatched==0` => exit `0`
- 否则 exit `1`

## 4. Docker
```bash
docker run --rm -v /data/videos:/data/videos ghcr.io/<owner>/<repo>:latest run /data/videos
docker run --rm -v /data/videos:/data/videos ghcr.io/<owner>/<repo>:latest run /data/videos --apply
```

建议：
- 把 `avmc.json` 放在挂载的目录中（`/data/videos/avmc.json`）
- 注意容器写入权限（避免在宿主机生成 root-owned 文件）
