# 运行报告（report.json）

报告是排障与可恢复性的关键输入：它必须**可读、可机器处理、结构稳定**。

## 1. 输出位置与输出通道（对外契约）
- apply：成功加载有效配置并进入执行阶段后，**必须**写入 `<path>/cache/report.json`；配置加载失败时尚无有效 `<path>`，只通过输出通道返回合成 `RunReport`。
- dry-run：不落盘；当 stdout 非 TTY 时通过 stdout 输出同结构 `RunReport`，当 stdout 是 TTY 时按 [CLI.md](./CLI.md) 输出人类摘要。
- 当 stdout 非 TTY 时：stdout **必须且仅**输出一个 `RunReport` JSON（dry-run/apply 同结构）。

> stdout/stderr 的分工见 [CLI.md](./CLI.md)。

## 2. 顶层结构（必须）
```json
{
  "path": "/abs/path",
  "dry_run": false,
  "started_at": "2026-02-09T10:00:00Z",
  "finished_at": "2026-02-09T10:05:00Z",
  "summary": {
    "processed": 10,
    "skipped": 3,
    "failed": 1,
    "unmatched": 2
  },
  "items": []
}
```

约束（必须满足）：
- `path` 必须是绝对路径。
  - 配置成功时，`path` 是扫描根目录。
  - 配置错误条目场景下，当前实现使用当前工作目录作为 `path`。
- `started_at`/`finished_at` 必须是 RFC3339（UTC，后缀 `Z`）。
- `summary.processed + summary.skipped + summary.failed + summary.unmatched == len(items)`。
- `items` 必须稳定排序：按 `code` 字典序；`code==""`（unmatched/config 等）排在最后。

## 3. Item 结构（必须）
每个 `CODE` 产生一个 item；无法解析 CODE（unmatched）也用 item 表达。

```json
{
  "code": "CAWD-895",
  "provider_requested": "javbus",
  "provider_used": "javdb",
  "website": "https://www.javdb.com/v/xxxxxx",
  "status": "processed",
  "error_code": "",
  "error_msg": "",
  "attempts": [
    {
      "provider": "javbus",
      "stage": "fetch",
      "error_code": "fetch_failed",
      "error_msg": "javbus 抓取失败：..."
    },
    {
      "provider": "javdb",
      "stage": "ok",
      "error_code": "",
      "error_msg": ""
    }
  ],
  "candidates": [],
  "files": [
    {
      "src": "in/CAWD-895.mp4",
      "dst": "out/CAWD-895/CAWD-895.mp4",
      "status": "moved"
    }
  ]
}
```

字段语义（必须遵守）：
- `code`：
  - 正常条目：规范化后的 CODE（如 `CAWD-895`）。
  - unmatched/config 等非 CODE 条目：必须为 `""`。
- `provider_requested`：来自 CLI/config 的首选 provider（unmatched/config 可为空字符串）。
- `provider_used`：最终成功使用的 provider；若未发生抓取/解析（例如 unmatched/config）则为空字符串。
- `website`：成功解析时必须填最终 provider 的详情页 URL；否则为空字符串。
- `attempts`：provider 尝试链路，用于解释“为何发生降级/回退”
  - 每条包含：`provider`、`stage(fetch|parse|ok)`、失败时的 `error_code/error_msg`
  - 顺序必须与实际尝试顺序一致；成功条目通常以最后一条 `stage=="ok"` 结束
  - 当前实现总是输出该字段；无尝试时输出空数组
- `candidates`：仅在 `unmatched_code(ambiguous)` 时填候选 CODE 列表；其它情况输出空数组。

### 3.1 unmatched 条目（必须形态）
- `code==""`
- `status=="unmatched"`
- `error_code=="unmatched_code"`
- `files[]` 至少包含 1 条输入文件；`dst==""`；`files[].status=="failed"`

### 3.2 配置错误条目（必须形态）
配置错误不对应任何 CODE，必须用一个“合成 item”表达：
- `code==""`
- `status=="failed"`
- `error_code in {config_not_found, config_invalid, config_missing_path}`
- `files==[]`
- `attempts==[]`
- `candidates==[]`

## 4. files[] 结构（必须）
每个输入视频文件必须有一条记录：
```json
{
  "src": "in/CAWD-895.mp4",
  "dst": "out/CAWD-895/CAWD-895__2.mp4",
  "status": "planned"
}
```

字段语义（必须遵守）：
- `src`：相对 `path` 的相对路径（用于可读与可搬运）。
- `dst`：相对 `path` 的相对路径；dry-run 为“计划目标”，apply 为“实际目标”；unmatched 时必须为 `""`。
- `status` 枚举（必须固定）：
  - `planned`：dry-run 计划移动到 `dst`
  - `moved`：apply 已移动到 `dst`
  - `rolled_back`：移动中途失败，且该文件已成功回滚
  - `failed`：该文件对应的动作失败（包括 unmatched、move_failed 等）

## 5. status 枚举（必须固定）
- `processed`：当前 item 成功完成且不是 `skipped`
  - dry-run：规划与必要的 provider 验证成功
  - apply：sidecar 生成与文件移动流程成功完成
- `skipped`：
  - dry-run/apply：无需任何变更（既不写 sidecar，也不移动文件）。
- `failed`：发生失败（`error_code` 必填）。
- `unmatched`：无法解析 CODE（`error_code=unmatched_code`，可选 `candidates`）。

## 6. error_code 枚举（必须固定）
- `unmatched_code`
- `fetch_failed`
- `parse_failed`
- `target_conflict`
- `io_failed`
- `move_failed`
- `config_not_found`
- `config_invalid`
- `config_missing_path`

含义（简述）：
- `unmatched_code`：无法从文件名/目录名提取唯一 CODE（含 ambiguous/no_match）。
- `fetch_failed`：抓取失败（网络不可达/被阻断/超时/重试耗尽）。
- `parse_failed`：HTML 可获取但解析失败（站点结构漂移或字段缺失超出容忍）。
- `target_conflict`：目标路径类型冲突（例如期望文件但实际是目录，或 out/<CODE> 不是目录）。
- `io_failed`：通用 IO 失败（创建目录/原子写/缓存读写/权限/磁盘等）。
- `move_failed`：移动失败（rename/EXDEV/权限/回滚失败等）。
- `config_*`：配置发现/解析/缺字段错误（只在无参运行或配置非法时出现）。

要求：
- `error_msg` 必须是用户可执行的提示（下一步怎么做），避免“堆栈噪音”。
