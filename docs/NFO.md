# NFO 输出规范

本文档定义当前 `internal/nfo/nfo.go` 生成的 `<CODE>.nfo` 结构。

## 1. 输出文件

- 文件名固定为：`<CODE>.nfo`
- 根节点固定为：`<movie>`
- XML 头固定为：

```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
```

## 2. 字段映射

当前 `MovieMeta -> NFO` 的映射如下：

| NFO 字段 | 当前来源 |
| --- | --- |
| `title` | `meta.Title`；若为空则回退为 `CODE`；若非空且未以 `CODE` 开头，则自动加上 `CODE + 空格` 前缀 |
| `sorttitle` | `CODE` |
| `studio` | `meta.Studio`（製作商 / Maker）与 `meta.Label`（發行商 / Label）；按 Maker、Label 顺序输出为可重复 `<studio>`，去空白并去重 |
| `director` | `meta.Director` |
| `set` | `meta.Series`；使用子元素格式 `<set><name>...</name></set>` |
| `premiered` | `meta.Release` |
| `year` | `meta.Year` |
| `runtime` | `meta.RuntimeM` |
| `mpaa` | 固定 `R18+` |
| `country` | 固定 `JP` |

## 3. 图片

海报和背景图采用 Kodi 标准格式：

```xml
<thumb aspect="poster">https://...cover.jpg</thumb>
<fanart>
  <thumb>https://...fanart.jpg</thumb>
</fanart>
```

| NFO 字段 | 当前来源 |
| --- | --- |
| `thumb[aspect="poster"]` | `meta.CoverURL`；为空时省略 |
| `fanart > thumb` | `meta.FanartURL`；为空时省略 |

服务器通过文件系统命名规则（`poster.jpg` / `fanart.jpg` 与 NFO 同目录）发现图片文件，NFO 内的 URL 是补充信息。

## 4. 评分

采用 Kodi v17+ `<ratings>` 容器格式：

```xml
<ratings>
  <rating name="javdb" max="5" default="true">
    <value>4.39</value>
    <votes>1158</votes>
  </rating>
</ratings>
```

| 属性 | 当前来源 |
| --- | --- |
| `name` | 固定 `javdb` |
| `max` | 固定 `5` |
| `value` | `meta.Rating` |
| `votes` | `meta.Votes` |

当 `meta.Rating` 和 `meta.Votes` 均为零时，整个 `<ratings>` 块不输出。

## 5. 外部 ID

```xml
<uniqueid type="url">https://javdb.com/v/xxxxx</uniqueid>
```

| `type` | 当前来源 |
| --- | --- |
| `url` | `meta.Website`（详情页 URL）；为空时省略 |

## 6. actor / tag / genre

### 6.1 `actor`

- 来源：`meta.Actors`（已由 provider 按站点规则清洗；例如 JavDB 会过滤明确标记为男性的演员）
- 处理：去空白、去重、保留首次出现顺序
- 输出结构：

```xml
<actor>
  <name>演员名</name>
  <role>演员名</role>
  <order>0</order>
</actor>
```

`order` 从 0 开始递增，保持排序稳定。

### 6.2 `tag`

- 来源：`append(meta.Tags, meta.Actors...)`，其中 `meta.Actors` 使用清洗后的演员列表
- 处理：去空白、去重、保留首次出现顺序

### 6.3 `genre`

- 来源：`meta.Genres`
- 处理：去空白、去重、保留首次出现顺序
- `genre` 不追加 actors（actors 仅写入 `tag`，避免类型筛选器中出现人名噪音）

## 7. 空值处理

- 空字符串字段使用 `omitempty` 省略。
- 空数组字段不输出对应节点。
- 指针类型（`set`、`fanart`、`ratings`）为 `nil` 时不输出。
- `title` 是唯一带回退逻辑的核心字段：为空时回退到 `CODE`。

## 8. 当前固定常量

- `mpaa = R18+`
- `country = JP`
