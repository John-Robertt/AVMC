# 发布与分发（Phase 12）

本项目通过 GitHub Actions 自动发布多平台二进制与 Docker 镜像，不再维护本地交叉编译脚本与相关说明。

## 1. GitHub Actions 自动发布
仓库已提供 GitHub Actions：
- `ci`：PR / push 时运行 `go test ./...`
- `release`：当推送 `v*` tag 时，自动发布二进制与 Docker 镜像

### 1.1 发布多平台二进制（GitHub Release）
触发方式：推送 tag（例如 `v0.1.0`）：
```bash
git tag v0.1.0
git push origin v0.1.0
```

产物：
- GitHub Release 附件（按平台打包）：
  - `avmc_<VERSION>_<GOOS>_<GOARCH>.tar.gz`（Linux/macOS）
  - `avmc_<VERSION>_<GOOS>_<GOARCH>.zip`（Windows）
  - 对应的 `avmc_<VERSION>_<GOOS>_<GOARCH>_SHA256SUMS.txt`
- `SHA256SUMS.txt`（该 release 所有附件的汇总校验和）

### 1.2 发布 Docker 镜像（GHCR，多架构）
同一条 tag 会把镜像推送到 GitHub Container Registry：
- `ghcr.io/<owner>/<repo>`

镜像标签（示例）：
- `latest`
- `v0.1.0`（原始 tag）
- `0.1.0`（semver version）
- `sha-<short>`（提交短 SHA）

> 注：GHCR 镜像名必须全小写；workflow 会自动把 `ghcr.io/${GITHUB_REPOSITORY}` 转成小写。

## 2. Docker 使用（从 GHCR 拉取运行）
把镜像名替换成你仓库对应的 `ghcr.io/<owner>/<repo>`：
```bash
docker run --rm ghcr.io/<owner>/<repo>:latest --help
docker run --rm -v /data/videos:/data/videos ghcr.io/<owner>/<repo>:latest run /data/videos
docker run --rm -v /data/videos:/data/videos ghcr.io/<owner>/<repo>:latest run /data/videos --apply
```

> 注意：容器需要对挂载目录有写权限（apply 会写 `out/` 与 `cache/` 并移动视频文件）。
