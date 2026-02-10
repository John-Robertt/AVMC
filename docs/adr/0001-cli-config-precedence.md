# ADR 0001：CLI 与配置文件的发现规则与覆盖优先级

## 状态
已接受

## 背景
产品原则要求“少就是多”，CLI 不能参数爆炸；但运行环境差异（代理池、并发、排除目录、图片代理）又必须可控。并且希望支持“一键运行”（不传 path 也能跑）。

## 决策
1) CLI 永远只暴露 `path/provider/apply` 三个入口  
2) `avmc.json` 承担所有高级配置（并发、代理、排除、图片代理等）  
3) 配置文件发现规则固定：
   - CLI 提供 path：读取 `<path>/avmc.json`（可选）
   - CLI 不提供 path：必须读取 `./avmc.json`（必选，且必须包含 `path`）
4) 覆盖优先级固定：
   - path：CLI path > config path
   - provider：CLI > config > 默认
   - apply：CLI `--apply/--apply=false` > config > 默认 false
   - 其他字段：仅 config 控制

## 后果（Trade-offs）
- 优点：入口稳定；行为可预测；无“全盘找配置”的惊喜；Docker/脚本化更容易。
- 缺点：某些临时覆盖（如并发）不能通过 CLI 做，只能改配置文件（符合产品原则）。

