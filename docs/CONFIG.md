# 配置文件规范（avmc.json）

目标：CLI 只保留 `path/provider/apply` 三个入口，其余高级能力全部用 `avmc.json` 控制，保持“少就是多”但仍可扩展。

## 1. 配置文件发现规则（固定）
1) 若 CLI 提供 `path`：尝试读取 `<path>/avmc.json`（可选）
2) 若 CLI 未提供 `path`：必须读取 `./avmc.json`（必选），且其中必须包含 `path`

不做任何“全盘搜索配置文件”的行为，避免惊喜与误用。

## 2. 覆盖优先级（固定）
- `path`：CLI `path` > config `path`
- `provider`：CLI `--provider` > config `provider` > 默认 `javbus`
- `apply`：CLI `--apply/--apply=false` > config `apply` > 默认 `false`
- 其他字段：仅 config 控制（CLI 不暴露）

## 3. Schema（v2）
```json
{
  "path": "/path/to/path",
  "provider": "javbus",
  "apply": false,

  "concurrency": 4,

  "javdb_base_url": "https://javdb565.com",

  "proxy": {
    "url": "http://127.0.0.1:8080"
  },

  "image_proxy": false,

  "exclude_dirs": ["temp", "downloads"]
}
```

### 3.1 字段语义
- `path`：扫描根目录。仅在“一键运行（avmc run 无参）”时强制必填。
- `provider`：默认刮削源（`javbus|javdb`）。实际运行仍允许自动降级。
- `apply`：默认是否执行落盘与移动。CLI 可用 `--apply=false` 覆盖为 dry-run。
- `concurrency`：按 CODE 并发处理的 worker 数。建议范围 `[1, 32]`（超出截断并在报告提示）。
- `javdb_base_url`：JavDB 的 base URL（可选）。当 `javdb.com` 不可达/被阻断时，可指定可用镜像域名（例如 `https://javdb565.com`）。仅影响 provider=javdb 的抓取入口（搜索与详情页）。
- `proxy.url`：HTTP 代理入口（后端可为代理池）。必须是合法 URL；启用后所有 provider 请求走代理，且必须每请求新建连接。
- `image_proxy`：图片下载是否使用 `proxy.url`。默认 `false`（图片直连下载）。若为 `true` 则必须同时配置 `proxy.url`，否则视为配置错误（`config_invalid`）。
- `exclude_dirs`：排除目录列表（相对 `path` 的路径，可多个）。

### 3.2 固定排除（无需配置）
无论 `exclude_dirs` 如何配置，扫描都必须排除：
- `<path>/out/`
- `<path>/cache/`

## 4. 常见配置示例

### 4.1 直连 + 小并发（默认）
```json
{
  "path": "/data/videos",
  "concurrency": 4
}
```

### 4.2 代理池 + 大并发（不使用代理下载图片）
```json
{
  "path": "/data/videos",
  "concurrency": 12,
  "proxy": { "url": "http://127.0.0.1:8080" },
  "image_proxy": false
}
```

### 4.3 代理池 + 图片也走代理（站点限制更严格时）
```json
{
  "path": "/data/videos",
  "concurrency": 8,
  "proxy": { "url": "http://127.0.0.1:8080" },
  "image_proxy": true,
  "exclude_dirs": ["out", "cache", "trash"]
}
```

> 注：`out` 与 `cache` 无需写入 exclude_dirs；写了也不会出错，但属于冗余。

## 5. 失败即配置错误（建议错误码）
- `config_not_found`：无参运行但 cwd 下无 `avmc.json`
- `config_invalid`：JSON 解析失败或字段非法
- `config_missing_path`：无参运行但 config.path 为空
