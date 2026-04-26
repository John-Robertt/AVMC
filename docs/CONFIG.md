# 配置文件规范（avmc.json）

## 1. 发现规则

当前实现只支持两种固定位置：

1. 若 CLI 提供 `path`：尝试读取 `<path>/avmc.json`，该文件可不存在。
2. 若 CLI 未提供 `path`：必须读取当前工作目录下的 `./avmc.json`，且其中必须包含 `path`。

## 2. 覆盖优先级

- `path`：CLI `path` > config `path`
- `provider`：CLI `--provider` > config `provider` > 默认 `javbus`
- `apply`：CLI `--apply` / `--apply=false` > config `apply` > 默认 `false`
- 其它字段：仅由配置文件控制

## 3. 当前字段集

```json
{
  "path": "/data/videos",
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

- `path`：扫描根目录；仅在 `avmc run` 无参时强制必填。
- `provider`：默认首选 provider，允许值为 `javbus` 或 `javdb`。
- `apply`：默认运行模式；CLI 可用 `--apply=false` 覆盖。
- `concurrency`：按 `CODE` 并发的 worker 数。未配置或配置为 `0` 时默认为 `4`；负数会被截断到 `1`，大于 `32` 会被截断到 `32`。
- `javdb_base_url`：JavDB 的 base URL；必须是完整的 `http://` 或 `https://` URL，仅影响 JavDB 的搜索与详情页入口。
- `proxy.url`：provider 页面抓取使用的 HTTP 代理入口；必须是带 scheme 与 host 的完整 `http://` 或 `https://` URL。
- `image_proxy`：图片下载是否走 `proxy.url`；为 `true` 时要求 `proxy.url` 非空。
- `exclude_dirs`：额外排除目录列表；相对路径按 `path` 解析，绝对路径按绝对路径处理。

### 3.2 固定排除

无论 `exclude_dirs` 是否配置，扫描都固定排除：

- `<path>/out/`
- `<path>/cache/`

## 4. 配置错误码

- `config_not_found`：无参运行且当前目录没有 `avmc.json`
- `config_invalid`：JSON 无法解析，或字段值不合法
- `config_missing_path`：无参运行时配置文件缺少 `path`
