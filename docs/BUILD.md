# 发布与分发

当前仓库的发布流程由 `.github/workflows/release.yml` 定义。

## 1. 当前 GitHub Actions workflow

当前发布流程由 `release` workflow 承担。

触发条件：

- 推送 tag：`v*`
- 手动触发：`workflow_dispatch`

## 2. `release` workflow 的作业

### 2.1 `test`

- 运行环境：`ubuntu-latest`
- 行为：`go test ./...`

### 2.2 `binaries`

- 依赖：`test`
- 目标平台：
  - `linux/amd64`
  - `linux/arm64`
  - `darwin/amd64`
  - `darwin/arm64`
  - `windows/amd64`
  - `windows/arm64`
- 构建命令：`CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "dist/avmc_<VERSION>_<GOOS>_<GOARCH>[.exe]" ./cmd/avmc`
- 产物：
  - 裸二进制：`avmc_<VERSION>_<GOOS>_<GOARCH>[.exe]`
  - Linux / macOS：`avmc_<VERSION>_<GOOS>_<GOARCH>.tar.gz`
  - Windows：`avmc_<VERSION>_<GOOS>_<GOARCH>.zip`
  - 对应的 `avmc_<VERSION>_<GOOS>_<GOARCH>_SHA256SUMS.txt`

### 2.3 `ghcr`

- 依赖：`test`
- 使用 buildx 推送多架构镜像到 GHCR。
- 镜像名由 `${GITHUB_REPOSITORY,,}` 动态计算；当前仓库对应 `ghcr.io/john-robertt/avmc`
- 当前平台：`linux/amd64`、`linux/arm64`
- 当前标签策略：
  - 始终推送：`sha-<short>`
  - tag 构建额外推送：`vX.Y.Z`、`X.Y.Z`、`latest`

### 2.4 `github_release`

- 依赖：`binaries`、`ghcr`
- 仅在 tag 构建时运行。
- 行为：
  - 下载二进制构建产物
  - 合并生成总校验和 `SHA256SUMS.txt`
  - 使用 `gh release create/upload` 创建或更新 GitHub Release

## 3. Dockerfile

仓库根目录的 `Dockerfile` 用于 `ghcr` 作业的多架构构建：

- 构建阶段：`golang:1.22-alpine`，`CGO_ENABLED=0`，通过 buildx 注入 `TARGETOS` / `TARGETARCH`
- 运行阶段：`alpine:3.19`，仅包含 `ca-certificates` 和编译后的 `avmc` 二进制

## 4. 使用当前镜像

```bash
docker run --rm ghcr.io/john-robertt/avmc:latest --help
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos
docker run --rm -v /data/videos:/data/videos ghcr.io/john-robertt/avmc:latest run /data/videos --apply
```

注意：容器需要对挂载目录有写权限；`apply` 模式会写 `out/` 与 `cache/`，并移动视频文件。
