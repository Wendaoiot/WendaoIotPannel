# WendaoIotPannel 后端镜像：Go 后端 + 内嵌 MQTT broker（mochi-mqtt）
# 多阶段构建；国内网络用 goproxy.cn（可用 --build-arg GOPROXY=... 覆盖）。
#
# 构建（在 server/ 目录）： docker build -t wendao/server:latest .
# 运行：容器纯环境变量配置（WQ_*，见 internal/config/config.go applyEnv），
#       设备直连容器 1883/8883，HTTP API 仅经 nginx 反代暴露。

# ---- 构建阶段 ----
FROM golang:1.25-alpine AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# 先拷 go.mod/go.sum 充分利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/wendao-server ./cmd/server/

# ---- 运行阶段 ----
FROM alpine:3.20

# ca-certificates: 设备 TLS 校验与外呼 https；tzdata: Asia/Shanghai
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S wendao && adduser -S wendao -G wendao

COPY --from=builder /out/wendao-server /usr/local/bin/wendao-server

# broker 持久化（pebble）与 TLS 证书挂载点
RUN mkdir -p /data/broker /etc/wendao/certs && chown -R wendao:wendao /data /etc/wendao
VOLUME ["/data/broker"]

USER wendao
WORKDIR /home/wendao

# HTTP API（经 nginx 反代，不对宿主暴露）
# 1883 MQTT 明文 / 8883 MQTT over TLS
EXPOSE 8080 1883 8883

ENV TZ=Asia/Shanghai \
    WQ_SERVER_PORT=8080

ENTRYPOINT ["wendao-server"]
