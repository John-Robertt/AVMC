# Providers 设计（JavBus / JavDB）

目标：把“站点变化”限制在 provider 包内部；核心流程只依赖统一接口与稳定的 `MovieMeta`。

## 1. Provider 接口（建议）
Provider 只做两件事：
1) 定位页面并抓取 HTML
2) 解析 HTML 为结构化 `MovieMeta`

```go
type Provider interface {
  Name() string
  Fetch(ctx context.Context, code Code, c *http.Client) (html []byte, pageURL string, err error)
  Parse(code Code, html []byte, pageURL string) (MovieMeta, error)
}
```

约束：
- `Fetch` 不做缓存、不做重试、不做限速（这些由核心 http/cache 层统一实现）
- `Parse` 必须纯函数：相同输入 => 相同输出
- `pageURL` 必须是详情页（用于 NFO `<website>` 与 report 追溯）

## 2. Provider 自动降级（核心策略）
输入：`provider_requested`（来自 CLI/config）  
策略：优先 requested；若抓取或解析失败，自动切换另一个 provider。

要求：
- report 同时记录 `provider_requested` 与 `provider_used`
- NFO 的 `<website>` 必须写 `provider_used` 的 URL

## 3. HTML 解析测试（fixture/golden）
站点 HTML 经常改版，必须用 fixture/golden 锁住解析行为。

建议结构：
```
internal/provider/javbus/testdata/*.html
internal/provider/javbus/golden/*.json
internal/provider/javdb/testdata/*.html
internal/provider/javdb/golden/*.json
```

测试规则：
- HTML fixture 作为输入
- 解析结果序列化为 JSON 与 golden 对比
- 字段缺失容忍策略必须在测试中体现（例如空字符串/空数组）

## 4. HTTP 约束（可用性）
代理池模式要求：**每个请求新建连接**，不复用 keep-alive，以触发代理池轮换。

建议把 HTTP policy 固化在 `httpx`：
- MetaClient：provider 页面抓取（走 proxy.url，可选）
- ImageClient：图片下载（受 image_proxy 控制）

Provider 本身不感知代理池细节。

## 5. 站点进入方式（实现约束）

### 5.1 javbus
- 详情页可直接进入：`https://www.javbus.com/<CODE>`
- **成年确认（driver-verify）**：
  - 未通过验证时，`GET /<CODE>` 常见返回 `302 -> /doc/driver-verify?...`
  - 但该 `302` 的 **response body 可能仍是完整详情页 HTML**  
    因此实现上必须 **禁用自动重定向**，直接读取 302 body 并解析；只有当 body 明确是验证页时才判定被拦截
  - 图片（如 `/pics/cover/...jpg`）常见要求 `Referer=<详情页>` 且带 `Cookie: age=verified`，否则可能 `403`
- 系列：从详情页 info 区块解析「系列」文本，写入 `MovieMeta.Series`（最终进入 NFO `<set>`）
- 标签/类型：优先从 `<meta name="keywords">` 的 content 拆分得到（剔除 code/studio/series），避免从 `/genre/` 链接提取时引入噪音标签；keywords 缺失时再回退 `/genre/` 链接

### 5.2 javdb
- 不能直接拼详情页 URL；必须先搜索再匹配结果：
  - 搜索页：`https://javdb.com/search?q=<CODE>&f=all`
  - 从搜索结果中选取 `strong == <CODE>` 的条目，进入其 `href` 指向的详情页（例如 `/v/<id>`）
- 标题：JavDB 有时 `current-title` 会显示中文翻译；若页面提供隐藏的 `origin-title`，必须优先使用原标题
- 系列：从详情页 panel 中解析「系列」文本，写入 `MovieMeta.Series`（最终进入 NFO `<set>`）

## 6. 图片约定（跨 provider 一致）
- `FanartURL` 表示背景大图（优先封面原图）；apply 下载后写 `fanart.jpg`
- `poster.jpg` 不再由 provider 单独提供：统一由 `fanart.jpg` 的右半边裁切生成
