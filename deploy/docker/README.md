# deploy/docker：三容器生产栈

> EMQX 已移除，MQTT broker（mochi-mqtt，MIT）内嵌进 wendao 容器。
> 设计与运维详见 [docs/容器化部署运维手册.md](../../docs/容器化部署运维手册.md)、
> [docs/Broker内嵌改造方案.md](../../docs/Broker内嵌改造方案.md)。

## 文件

| 文件 | 说明 |
| --- | --- |
| `docker-compose.yml` | 三服务：wendao（后端+内嵌 broker）/ mysql / nginx |
| `.env.example` | 环境变量模板（复制为 `.env`；`.env` 不入库） |
| `gen-secrets.sh` | 生成强随机密钥版 `.env`（已存在则拒绝覆盖） |
| `gen-certs.sh` | 生成 8883 自签 CA 与服务器证书（`FORCE_CERTS=1` 轮换） |
| `Dockerfile.portal` | 门户镜像：构建 web-admin + nginx 服务 + wiki 静态站 |
| `nginx/pannel.conf` | pannel / wiki 双 server 站点配置 |
| `../../server/Dockerfile` | 后端镜像：golang:1.25-alpine 多阶段构建 |

## 快速开始

```bash
# 1) 生成 .env（强随机密钥）或 cp .env.example .env 手工修改
./gen-secrets.sh

# 2) 生成 8883 TLS 证书（设备侧需导入 certs/ca.crt）
./gen-certs.sh

# 3) 构建并启动
docker compose build
docker compose up -d
docker compose ps        # 全部 healthy/running
```

## 端口

| 端口 | 服务 | 说明 |
| --- | --- | --- |
| 80 | nginx | web-admin + wiki + `/api` 反代（对公网） |
| 1883 | wendao | MQTT 明文（设备接入） |
| 8883 | wendao | MQTT over TLS（设备接入，需导入 ca.crt） |
| 8080 | wendao | HTTP API，仅容器网内，经 nginx 反代访问 |

> 腾讯云安全组只需放行 `80/TCP`、`1883/TCP`、`8883/TCP`。

## 首次登录

`gen-secrets.sh` 输出管理员初始密码（也在 `.env` 的 `WQ_ADMIN_PASSWORD`）。
打开 `http://pannel.wendaoiot.com/` → 302 到 `/login` → 用 `admin / <初始密码>` 登录。

## 常用命令

```bash
docker compose ps                    # 状态
docker compose logs -f wendao        # 后端+broker 日志
docker compose restart wendao        # 重启后端（证书轮换后）
docker compose down                  # 停止（保留数据卷）
docker compose down -v               # 停止并清空 MySQL + broker 会话（慎用）
```

## 安全提醒

- `.env`、`certs/`（含 CA 与服务器私钥）不入库（根 `.gitignore` 已覆盖）。
- 管理员初始密码首次登录后尽快修改。
- 无需再维护任何 EMQX Dashboard/API Key/HTTP 回调配置——认证、ACL、互踢全部在
  wendao 进程内完成。
