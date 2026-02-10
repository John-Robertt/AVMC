# AVMC 产品策划文档（PRD）

## 1. 背景与问题

本地视频文件（以“番号”为核心标识）在导入 Jellyfin/Emby/Kodi/Plex 等媒体库时，常见痛点是：

- 文件命名/目录结构不统一，媒体库无法稳定识别同一部作品。
- 缺少刮削得到的元数据（标题、发行日期、演员、厂牌/系列、标签等）与本地海报/背景图。
- 手工整理成本高、重复劳动多、容易出错。

**AVMC 的目标**是把“混乱的本地视频目录”转换为一个**结构稳定、可重复生成、可直接被媒体库扫描**的新库，并做到“少就是多”“科技无感”：用户只需要给出根目录，工具自动完成识别、刮削、生成侧车文件、归档视频。

> 重要约束：刮削源暂时仅支持 **JavBus** 与 **JavDB**，且均为 **HTML 抓取**。

---

## 2. 产品原则（必须坚持）

1) 少就是多  
- 命令入口尽量少；默认行为明确；不把“实现细节参数”暴露给用户。

2) 真正的科技是让你感觉不到科技的存在  
- 结果结构固定、可预测；失败时给出可操作的原因；二次运行幂等（重复执行不会把库弄乱）。

3) 数据优先  
- 一切围绕“番号 CODE”建立：扫描 -> 识别 CODE -> 刮削 -> 输出到 `out/CODE/`。
- 不做“聪明的猜测链条”，宁可失败也不写错。

4) 可靠性优先于花哨功能  
- HTML 解析用 fixture/golden 测试锁住；站点结构变化时快速发现并定位。

---

## 3. 目标用户与典型使用场景

### 3.1 目标用户
- 有本地视频收藏、文件名包含番号（例如 `CAWD-895.mp4`）的用户。
- 使用 Jellyfin/Emby/Kodi 管理媒体库（Plex 可通过读取 Kodi NFO 的插件接入）。

### 3.2 典型场景（端到端）
用户拥有一个根目录 `path/`，内部可能多层嵌套、命名不统一。用户希望运行一次命令后得到：

- `path/out/<CODE>/`：每部作品一个目录
- `<CODE>.nfo` + `poster.jpg` + `fanart.jpg`
- 视频文件被**移动**到上述目录（同一 `CODE` 支持多个文件；默认保留原文件名；同名自动去冲突）

---

## 4. 产品范围

### 4.1 目标（必须实现）
- 扫描 `path/` 下的视频文件，识别番号 CODE。
- 从指定 provider（`javbus` 或 `javdb`）进行 HTML 抓取并解析元数据。
- 在 `path/out/<CODE>/` 生成：
  - `<CODE>.nfo`
  - `poster.jpg`
  - `fanart.jpg`
  - 视频文件（可多个；把原视频移动到这里，默认不改名）
- 生成可追溯的运行报告（RunReport JSON），便于重跑与排查（apply 固定写入 `<path>/cache/report.json`；stdout 非 TTY 时输出 JSON；详见 5.4）。

### 4.2 非目标（明确不做）
- 不下载/收集视频本体（只处理本地已有视频文件）。
- 不做登录/验证码等交互式验证；不实现“验证码识别/JS 解密”等复杂反爬绕过。遇到交互式阻断仍判定 provider 不可用并失败（可通过代理池/UA/并发提升可用性，但不保证）。
- 不做转码、抽帧、去重、字幕下载等媒体处理功能。

---

## 5. 用户界面（CLI）与使用方式

### 5.1 命令（极简、稳定）
```
avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]
```

- `path`：扫描根目录（可省略；用于配置文件“一键运行”）
- `--provider`：刮削来源（仅允许 `javbus`/`javdb`）；默认 `javbus`
- `--apply`：真正写入与移动文件；默认 dry-run（做可用性验证 + 展示计划，不改动文件系统）。支持 `--apply=false` 覆盖配置文件中的 `apply=true`

> 说明：为了避免“参数爆炸”，CLI 仅暴露 `path/provider/apply` 三个入口；不提供 `--out`。输出固定写入 `<path>/out/`。

### 5.2 例子
- 预演（不改动）：  
  `avmc run /path/to/path --provider javbus`
- 执行（落盘 + 移动视频）：  
  `avmc run /path/to/path --provider javbus --apply`
- 一键运行（依赖当前目录 `./avmc.json` 中的 `path/provider/apply`）：  
  `avmc run`

### 5.3 Docker（等价）
```
docker run --rm -v /root:/root ghcr.io/<owner>/<repo>:latest run /root --provider javbus
docker run --rm -v /root:/root ghcr.io/<owner>/<repo>:latest run /root --provider javbus --apply
```

### 5.4 输出与退出码（对外契约）
- stdout 是 TTY：stdout 输出人类摘要；stderr 输出日志。
- stdout 非 TTY：stdout **仅输出一个** RunReport JSON；stderr 输出人类摘要/日志（结构见 [REPORT.md](./REPORT.md)）。
- apply：必须写入 `<path>/cache/report.json`；dry-run：不得落盘。
- 退出码：`failed==0` 且 `unmatched==0` => exit `0`；否则 exit `1`。

### 5.5 可选配置文件（不增加 CLI 参数）
为了在不增加命令行参数的前提下兼顾“可用性”（站点限制/网络环境差异），工具支持 `avmc.json` 配置文件，用于控制并发/代理/排除目录/图片代理等高级行为，并支持“一键运行”。

#### 配置文件发现规则（固定、可预测）
- 若 CLI 提供 `path`：尝试读取 `<path>/avmc.json`（可选）
- 若 CLI 未提供 `path`：必须读取 `./avmc.json`（必选），并要求其中包含 `path`

#### 覆盖优先级（固定）
- `path`：CLI `path` > config `path`
- `provider`：CLI `--provider` > config `provider` > 默认 `javbus`
- `apply`：CLI `--apply/--apply=false` > config `apply` > 默认 `false`
- 其余字段：仅由 config 控制（CLI 不暴露）

最小配置（v2）：
```json
{
  "path": "/path/to/path",
  "provider": "javbus",
  "apply": false,

  "concurrency": 4,
  "proxy": {
    "url": "http://127.0.0.1:8080"
  },
  "image_proxy": false,
  "exclude_dirs": ["temp", "downloads"]
}
```

- `path/provider/apply`：用于“一键运行”，且允许被 CLI 覆盖。
- `concurrency`：并发 worker 数；缺省时使用内置默认值。
- `proxy.url`：HTTP 代理入口（后端可为代理池）；缺省则直连。
- `image_proxy`：图片是否通过 `proxy.url` 下载；默认 `false`（不使用代理下载图片）。当 `image_proxy=true` 时要求 `proxy.url` 非空，否则视为配置错误。
- `exclude_dirs`：排除目录列表（相对 `path` 的路径，可多个）。
- 工具生成目录自动排除：无论配置如何，`out/` 与 `cache/` 永远会被排除扫描（无需在 `exclude_dirs` 重复配置）。
- UA 池：大量内置于工具内，按请求随机选择，不对外暴露配置。

---

## 6. 输出规范（文件系统即 API）

### 6.1 输出根路径
- 固定为：`<path>/out/`

### 6.2 目录结构
对每个作品 `<CODE>`：
```
<path>/out/<CODE>/
  <video files...>
  <CODE>.nfo
  poster.jpg
  fanart.jpg
```

### 6.3 必须的规则
- 扫描时必须排除 `<path>/out/` 与 `<path>/cache/`，防止“扫到自己生成的新库/缓存”导致循环处理。
- 配置排除：若 `exclude_dirs` 指定了目录，则一并排除这些路径（相对 `path`）。
- `--apply` 时对视频文件执行**移动**（优先同盘 `rename`），移动到 `out/<CODE>/`，默认**不改名**。
- 多版本：同一 `CODE` 下允许存在多个视频文件；元数据只刮削一次，侧车文件仅生成一份。
- 视频同名去冲突：若目标目录已存在同名视频文件，则按确定性规则自动改名（例如追加 `__2`、`__3`），并在报告中记录映射。
- 侧车不覆盖：若目标已存在 `<CODE>.nfo` / `poster.jpg` / `fanart.jpg`，则跳过写入（不覆盖）；只有路径冲突或写入失败才算失败。
- 幂等：再次运行时，若输出完整则跳过；不完整则只补齐缺失文件（例如缺图补图）。

---

## 7. 刮削源（Providers）

### 7.1 Provider 枚举（唯一选择）
- `javbus`
- `javdb`

### 7.2 Provider 行为约束
- 输入：规范化后的 `CODE`（例如 `CAWD-895`）
- 输出：结构化元数据 `MovieMeta` + 海报/背景图 URL（或下载结果）
- 统一的限速与缓存策略由核心实现（provider 只负责“如何定位页面 + 解析 HTML”）
- 可用性策略：优先使用 `--provider` 指定的来源；若抓取/解析失败，将自动切换到另一个 provider 重试，并在 NFO 与报告中标注最终使用的来源。

### 7.3 NFO 中标记来源（provider 作为来源标记）
- `<website>`：写入对应站点的详情页 URL（天然标记来源）

---

## 8. 元数据与 NFO 规范（最小可用集）

输出为 Kodi/Jellyfin/Emby 常见的 `<movie>` NFO 结构，字段保持“最小但够用”：

- `title`
- `sorttitle`（默认用 CODE）
- `num`（CODE）
- `studio`
- `set`（系列/集合，若无则省略或为空）
- `premiered` / `release`（ISO 日期）
- `year`
- `runtime`（分钟）
- `country`（内置常量：`JP`；不对外暴露配置）
- `mpaa`（内置常量：`R18+`；不对外暴露配置）
- `actor[]`（name/role）
- `tag[]` 与 `genre[]`
- `website`（详情页；也是来源标记）
- `poster` / `thumb` / `fanart`（本地文件名：`poster.jpg` / `fanart.jpg`）
- `cover`（封面/背景图 URL，用于追溯）
- `rating` / `userrating` / `votes`（默认 `0`，用于兼容常见 NFO 结构）
- （可选）`uniqueid`（若实现，仅作为额外来源标记；不作为最小集要求）

海报与背景图为本地文件：
- `poster.jpg`
- `fanart.jpg`

图片规则（对外契约的一部分）：
- `fanart.jpg`：背景大图（优先使用站点提供的封面原图）
- `poster.jpg`：由 `fanart.jpg` 的**右半边裁切**生成（避免额外下载，保证 poster/fanart 一致）

> 注意：本产品不追求把所有站点字段“全量搬运”，避免站点改版导致字段漂移、维护成本爆炸。

---

## 9. 失败策略与可恢复性

失败时的底线原则：**不移动视频，不写半成品**（或至少不让半成品覆盖已有结果）。

### 9.1 常见失败类型
- `unmatched_code`：无法从文件名/目录名解析出 CODE
- `fetch_failed`：网络/站点不可达/被阻断
- `parse_failed`：HTML 结构变化导致解析失败
- `target_conflict`：目标路径类型冲突（例如 `out/<CODE>` 不是目录、sidecar 目标路径被目录占用等）
- `io_failed`：通用 IO 失败（权限/磁盘/创建目录/原子写/缓存读写等）
- `move_failed`：移动/重命名失败（权限/跨盘/目标异常等）

### 9.2 运行报告（必有）
- apply：固定写入 `<path>/cache/report.json`；stdout 非 TTY 时也输出同结构 JSON。
- dry-run：不得落盘；stdout 非 TTY 时输出同结构 JSON。
- 内容包含：
  - processed/skipped/failed/unmatched 列表
  - 每条的：源路径、解析出的 CODE、`provider_requested`/`provider_used`、（apply 时）移动后的目标路径映射、错误码、简短错误信息

### 9.3 可恢复性硬规则（必须满足）
1) 移动视频永远是最后一步：刮削/解析/侧车任一步失败，都不得移动视频文件。  
2) 侧车写入原子化：写入采用临时文件 + `rename`；且遵守“不覆盖”。  
3) 同盘优先：默认仅做同盘 `rename`；跨盘（如 EXDEV）视为失败并给出可操作提示，不做隐式 copy+delete。  
4) 单条失败不影响其他：一个 `CODE` 失败不阻断其他条目处理；但只要 `failed>0` 或 `unmatched>0`，整体 exit code 必须为 1。  
5) 幂等可重跑：已完整条目跳过；不完整只补齐缺失；同 `CODE` 新增视频文件只做归档，不重复刮削。  

---

## 10. 性能与资源（内置默认；可选配置覆盖，不新增 CLI 参数）

### 10.1 可用性规则（必须满足）
1) 默认可运行：无 `avmc.json` 也能以直连方式运行（不强依赖代理）。  
2) 代理池模式正确生效：配置 `proxy.url` 后，provider 的页面抓取请求必须走该 HTTP 代理入口，且**每个请求新建连接**（不复用 Keep-Alive）。图片下载是否走代理由 `image_proxy` 决定。  
3) 内置 UA 池：每个请求随机选择 UA，不需要用户维护。  
4) provider 自动降级：首选来源失败后，自动切换另一个 provider 尝试，并标注最终来源。  
5) 有界重试与超时：网络请求必须有超时与有限重试；达到上限后以 `fetch_failed/parse_failed` 明确失败。  
6) 图片代理可控：默认图片直连下载；当 `image_proxy=true` 时，图片下载走 `proxy.url`。  

### 10.2 默认策略（内置）
为了让工具“无感”但不打爆站点：
- 内置并发（默认例如 4 worker，可由配置文件 `avmc.json` 覆盖）与 provider 级限速（例如 1 rps）
- UA 池内置：每个请求随机选择 UA，降低重复特征
- 代理池支持：当配置 `proxy.url` 时，provider 的页面抓取请求通过该 HTTP 代理入口；**每个请求必须新建连接，不复用 Keep-Alive**，以触发代理池轮换能力（图片下载是否走代理由 `image_proxy` 决定）
- 内置缓存目录：`<path>/cache/`
  - 按 `provider+CODE` 缓存原始 HTML 与解析结果（图片下载结果可选缓存，不作为对外契约）
- 二次运行优先走缓存与本地已有文件，减少重复请求

---

## 11. 交付与部署

### 11.1 多平台二进制
目标平台：
- Linux: amd64/arm64
- macOS: amd64/arm64
- Windows: amd64/arm64

产物：
- 单个可执行文件 `avmc`（或 Windows 下 `avmc.exe`）

### 11.2 Docker
- 镜像内仅包含 `avmc` 二进制与必要 CA 证书
- 默认 entrypoint 为 `avmc`

---

## 12. 测试与验收标准

### 12.1 单元测试
- CODE 解析与规范化：大小写、分隔符、括号前缀、空格等常见变体
- 输出路径与冲突判定逻辑

### 12.2 Provider 解析测试（fixture/golden）
- 为 `javbus` 与 `javdb` 各保存若干 `testdata/*.html` 样本
- 解析结果与 golden JSON 对比，锁住字段与容错行为

### 12.3 端到端测试（临时目录）
- dry-run：不生成 `out/` 内容
- apply：生成目录结构与文件齐全，视频被移动到 `out/<CODE>/`
- 幂等：二次运行应跳过已完整条目
- 多版本：同一 `CODE` 多个视频文件，只刮削一次并全部移动到 `out/<CODE>/`（侧车只生成一份）
- provider 降级：模拟首选 provider 失败，自动切换并成功写入（NFO/报告标注最终来源）
- exclude_dirs：配置的排除目录不被扫描；且 `out/` 与 `cache/` 永远自动排除
- image_proxy：`image_proxy=false` 时图片直连下载；`image_proxy=true` 时图片下载走 `proxy.url`
- 一键运行：`avmc run` 在 cwd 存在 `./avmc.json` 且包含 `path/provider/apply` 时可直接运行；CLI `path/--provider/--apply` 可覆盖配置

---

## 13. 合规与声明（产品必写）
- 本工具面向个人本地媒体库整理；使用者需自行遵守所在地法律法规与目标站点 ToS/robots。
- 默认限速与缓存以降低对站点的压力；不提供绕过反爬/验证码能力。
