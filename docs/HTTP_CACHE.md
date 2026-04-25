# HTTP 与 Cache 规范

本文档对应当前 `internal/infra/httpx/`、`internal/infra/cache/` 和 `internal/app/run.scrape` 的实现。

## 1. MetaClient

入口：`httpx.NewMetaClient(proxyURL)`

当前行为：

- 默认总超时：`20s`
- `TLSHandshakeTimeout`：`10s`
- `ResponseHeaderTimeout`：`15s`
- 默认最大重试次数：`2`（含首次请求时最多尝试 3 次）
- `GET` / `HEAD` 且 body 为空的请求进入重试路径
- transport 直接返回的网络错误进入重试路径；HTTP 3xx/4xx/5xx 响应交由上层按状态码处理
- 若请求头未设置 `User-Agent`，从内置 UA 池随机选择一个
- `proxyURL` 非空时：
  - 走该代理
  - `Transport.DisableKeepAlives = true`
  - `Request.Close = true`

## 2. ImageClient

入口：`httpx.NewImageClient(proxyURL, imageProxy)`

当前行为：

- `imageProxy=false`：图片直连，不使用 `proxyURL`
- `imageProxy=true`：图片请求走 `proxyURL`；若 `proxyURL` 为空则返回配置错误
- 代理模式下同样禁用 keep-alive

## 3. UA 池

当前内置 UA 池由 `httpx.newUAPool()` 初始化，包含 3 个浏览器 UA：

- Windows Chrome
- macOS Safari
- Linux Chrome

## 4. provider cache 路径

当前 cache 路径固定为：

```text
<path>/cache/providers/<provider>/<CODE>.html
<path>/cache/providers/<provider>/<CODE>.json
```

## 5. 当前读取策略

入口：`internal/app/run.scrape`

当前读取流程：

1. 只尝试读取 **requested provider** 的 JSON cache。
2. 若 JSON 文件存在且能反序列化为 `MovieMeta`，直接返回该结果。
3. 若 JSON 文件不存在，或内容损坏无法反序列化，则忽略该 cache，继续访问网络。

## 6. 当前写入策略

- `dry-run`：cache 以只读方式参与流程。
- `apply`：抓取成功后，把最终成功 provider 的 HTML 和 JSON 写入其目录。
- HTML 与 JSON 都通过原子覆盖写入。

- 若 requested provider 失败、fallback provider 成功，则 cache 写入 fallback provider 目录。
- 下一次运行时，仍然只会优先读取新的 requested provider 对应的 JSON cache。

## 7. provider 尝试链路

入口：`provider.FetchParseTrace`

当前行为：

- requested provider 先尝试
- 失败后再尝试另一个 provider
- 每次尝试记录 `Attempt{Provider, Stage, Err}`
- `Stage` 枚举为 `fetch`、`parse`、`ok`

## 8. 当前已实现的站点图片请求策略

- JavBus 图片下载时，下载请求会附加：
  - `Referer: <详情页>`（若详情页 URL 非空）
  - `Cookie: age=verified`

该策略由 provider 的可选图片请求接口提供；执行层只负责创建请求、应用 provider 策略并下载图片，不再内置 JavBus 域名判断。
