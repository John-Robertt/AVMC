# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build

WORKDIR /src

# 先拉依赖，避免每次都全量重编
COPY go.mod go.sum ./
RUN go mod download

COPY . ./

# buildx 会注入 TARGETOS/TARGETARCH，用于多架构构建。
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
  go build -trimpath -ldflags "-s -w" -o /out/avmc ./cmd/avmc

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /out/avmc /usr/local/bin/avmc

ENTRYPOINT ["avmc"]
CMD ["--help"]
