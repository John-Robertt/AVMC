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
| `num` | `CODE` |
| `studio` | `meta.Studio` |
| `set` | `meta.Series` |
| `release` | `meta.Release` |
| `premiered` | `meta.Release` |
| `year` | `meta.Year` |
| `runtime` | `meta.RuntimeM` |
| `mpaa` | 固定 `R18+` |
| `country` | 固定 `JP` |
| `poster` | 固定 `poster.jpg` |
| `thumb` | 固定 `poster.jpg` |
| `fanart` | 固定 `fanart.jpg` |
| `rating` | 固定 `0` |
| `userrating` | 固定 `0` |
| `votes` | 固定 `0` |
| `cover` | `meta.CoverURL` |
| `website` | `meta.Website` |

## 3. actor / tag / genre

### 3.1 `actor`

- 来源：`meta.Actors`
- 处理：去空白、去重、保留首次出现顺序
- 输出结构：

```xml
<actor>
  <name>演员名</name>
  <role>演员名</role>
</actor>
```

### 3.2 `tag`

- 来源：`append(meta.Tags, meta.Actors...)`
- 处理：去空白、去重、保留首次出现顺序

### 3.3 `genre`

- 来源：`append(meta.Genres, meta.Actors...)`
- 处理：去空白、去重、保留首次出现顺序

## 4. 空值处理

- 空字符串字段使用 `omitempty` 省略。
- 空数组字段不输出对应节点。
- `title` 是唯一带回退逻辑的核心字段：为空时回退到 `CODE`。

## 5. 当前固定常量

- `mpaa = R18+`
- `country = JP`
- `poster = poster.jpg`
- `thumb = poster.jpg`
- `fanart = fanart.jpg`
- `rating = 0`
- `userrating = 0`
- `votes = 0`
