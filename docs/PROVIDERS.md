# Providers

本文档定义当前 provider 接口、降级链路和两个站点的解析规则。

## 1. 当前接口

```go
type Provider interface {
  Name() string
  Fetch(ctx context.Context, code domain.Code, c *http.Client) (html []byte, pageURL string, err error)
  Parse(code domain.Code, html []byte, pageURL string) (domain.MovieMeta, error)
}

type ImageRequestPreparer interface {
  PrepareImageRequest(req *http.Request, meta domain.MovieMeta)
}
```

当前约束：

- `Fetch` 负责定位页面并抓取 HTML。
- `Parse` 负责把 HTML 解析成 `MovieMeta`。
- 缓存、重试和代理开关由 [HTTP_CACHE.md](./HTTP_CACHE.md) 中的上层逻辑统一处理。
- `Parse` 以相同输入得到相同输出为目标。
- `pageURL` 对应详情页 URL。

## 2. 当前 provider 注册表

当前仓库只注册 2 个 provider：

- `javbus`
- `javdb`

## 3. 自动降级链路

入口：`provider.FetchParseTrace`

当前策略：

- `provider_requested=javbus` 时：按 `javbus -> javdb` 尝试
- `provider_requested=javdb` 时：按 `javdb -> javbus` 尝试

每次尝试都会记录 `Attempt{Provider, Stage, Err}`，最终由上层转换为 report 的 `attempts[]`。

## 4. fixture / golden

当前测试目录：

```text
internal/provider/javbus/testdata/*.html
internal/provider/javbus/golden/*.json
internal/provider/javdb/testdata/*.html
internal/provider/javdb/golden/*.json
```

## 5. JavBus 当前规则

- 详情页入口：`https://www.javbus.com/<CODE>`
- `Fetch` 会禁用自动重定向，直接读取 302 响应体。
- `Location` 指向 `driver-verify` 且响应体呈现验证页时，`Fetch` 返回被拦截结果。
- `Parse` 会先校验详情页中的识别码与传入 `Code` 一致；不一致视为解析失败。
- 标题来自页面 `h3`，若前缀已包含 `CODE`，会去掉这个前缀后再写入 `MovieMeta.Title`。
- 系列来自 info 区块的“系列”字段。
- 标签优先从 `meta[name="keywords"]` 解析；keywords 缺失时退回 `/genre/` 链接。
- `CoverURL` 优先取 `a.bigImage[href]`，其次取 `div.screencap img[src]`。
- `FanartURL` 优先复用 `CoverURL`，若仍为空再尝试样品图。

## 6. JavDB 当前规则

- 不能直接拼详情页，必须先访问 `search?q=<CODE>&f=all`。
- 只接受搜索结果中 `strong == <CODE>` 的条目。
- 标题优先取 `origin-title`，不存在时退回 `current-title`。
- 系列来自详情页 panel 的“系列”字段。
- `CoverURL` 优先取 `a[data-fancybox='gallery'][href]`，其次取封面图 `img[src]`。
- 当前实现直接把 `CoverURL` 复用为 `FanartURL`。

## 7. 图片与 provider 的边界

- provider 只提供 `FanartURL` / `CoverURL` 等元数据，不直接写图片文件。
- `fanart.jpg` 下载与 `poster.jpg` 裁切由执行层处理。
- 若 provider 实现 `ImageRequestPreparer`，执行层会在下载图片前调用它补充站点级请求头。
- JavBus 当前通过该可选接口为图片下载补充 `Referer` 与 `Cookie: age=verified`。
