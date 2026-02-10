# 测试与验收（以契约为中心）

测试的目标不是追求覆盖率，而是锁住三类易出问题的东西：
1) **数据结构不变量**（CODE、计划、冲突处理）
2) **文件系统契约**（原子写、不覆盖、移动最后、幂等）
3) **站点漂移**（fixture/golden）

## 1. 单元测试（纯函数）

### 1.1 CODE 提取与规范化
- 大小写/分隔符/括号/空格变体
- ambiguous：同一输入得到多个不同 CODE 必须失败
- no_match：必须失败并给出原因

### 1.2 exclude 语义
- 永久排除：`out/`、`cache/`
- `exclude_dirs`：相对 `path` 的路径匹配（整棵排除）

### 1.3 同名去冲突（确定性）
- 已存在 `a.mp4` => 新文件变 `a__2.mp4`，再冲突 `a__3.mp4`
- 同一批次规划内的重名也必须去冲突（不能等到执行时再撞）
- 输出稳定（按 `Code/RelPath` 排序定义），与扫描顺序无关

## 2. Provider 解析测试（fixture/golden）
- 对每个 provider 保存若干 HTML fixture
- 解析结果与 golden JSON 严格对比
- 站点改版时：先更新 fixture，再更新 golden（必须有 PR 说明）

（开发辅助）重新生成 golden：
- `UPDATE_GOLDEN=1 go test ./internal/provider/javbus`
- `UPDATE_GOLDEN=1 go test ./internal/provider/javdb`

## 3. 端到端测试（临时目录）
必须覆盖 PRD 验收点：
- dry-run：执行 `fetch+parse`（用 stub provider/fixture 驱动），但不写 out/cache、不下载图片、不移动文件
- 输出契约：stdout 非 TTY 时 stdout 仅 1 个 RunReport JSON（stderr 允许日志）；stdout 是 TTY 时输出人类摘要（stderr 允许日志）
- apply：写 sidecar + 移动视频到 out/<CODE>/
- 幂等：二次运行不重复写、不乱动
- 多版本：同 CODE 多文件只刮削一次并全部归档
- provider 降级：首选失败 -> fallback 成功，并标注 provider_used
- cache 自愈：cached html parse 失败时必须绕过缓存强制 refetch 一次；仍失败才 parse_failed
- image_proxy：false 直连下载；true 走 proxy.url
- move 失败：跨盘 EXDEV 或权限失败时不丢数据（推荐回滚）

## 4. 可验证命令（建议）
- `go test ./...`
- `go test ./... -run TestProvider -count=1`
- `go test ./... -run TestE2E -count=1`

> 注：测试应该尽量离线（fixture 驱动），避免依赖站点实时可用性。

## 5. 联网手测（可选，验证“真实可用性”）
当你需要确认站点未被拦截、图片可下载时，可以做一次联网手测（**不纳入 CI**）：

```bash
# 建议在临时目录/副本目录上跑 apply，避免污染已有 out/cache
go run ./cmd/avmc run ./test --provider javbus --apply

# JUR-566 的 NFO 样板在 demo 里（用于人工/脚本对照）
diff -u demo/JUR-566/JUR-566.nfo test/out/JUR-566/JUR-566.nfo
```
