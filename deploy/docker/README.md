# Docker 容器栈（MySQL + EMQX）

用 Docker Compose 在本机/单机上快速启动 WendaoIotPannel 依赖的 **MySQL 8.0** 与 **EMQX 5.8**。
生产部署的 systemd/Nginx 示例见根目录 [部署与测试.md](../../部署与测试.md)，协议设计见 [项目说明.md](../../项目说明.md)。

## 包含内容

| 文件 | 说明 |
| --- | --- |
| `docker-compose.yml` | MySQL + EMQX 服务定义，全部密码/端口由 `.env` 注入 |
| `.env.example` | 环境变量模板（复制为 `.env`；`.env` 不入库） |
| `emqx/emqx.conf` | EMQX 覆盖配置：补充 8883 SSL listener |
| `gen-certs.sh` / `gen-certs.ps1` | 生成 8883 自签 CA 与服务器证书（需要 openssl） |
| `configure-emqx.py` | 自动配置 EMQX 认证/ACL 链与互踢 API Key（仅 Python 标准库） |
| `configure-emqx.sh` / `.ps1` | 上述脚本的封装 |

## 快速开始

```bash
# 1) 准备环境变量（修改其中所有 change_me 密码）
cp .env.example .env

# 2) 生成 8883 TLS 证书（需要 openssl；Windows PowerShell 用同名 .ps1）
./gen-certs.sh

# 3) 启动容器
docker compose up -d

# 4) 等后端（:8080）起来后，配置 EMQX 认证/ACL 链
./configure-emqx.sh        # Windows PowerShell: ./configure-emqx.ps1
```

启动顺序提示：EMQX 的 HTTP 认证回调指向宿主机后端 `http://host.docker.internal:${SERVER_PORT}`，
因此应先在宿主机启动 Go 后端（`cd server && go run ./cmd/server/`），再执行 `configure-emqx`。
`docker compose` 可先于后端启动，配置成功前 broker 对业务连接会拒绝（fail-closed，属预期）。

## 端口

| 端口 | 服务 |
| --- | --- |
| 3306 | MySQL（可用 `MYSQL_PORT` 改） |
| 1883 | MQTT 明文 |
| 8883 | MQTT over TLS |
| 8083 / 8084 | MQTT over WS / WSS |
| 18083 | EMQX Dashboard |

## 常用命令

```bash
docker compose ps                 # 查看状态
docker compose logs -f emqx       # 查看 EMQX 日志
docker compose down               # 停止
docker compose down -v            # 停止并删除数据卷（清空数据库重来）
```

## configure-emqx 完成的工作

1. 创建 **内置数据库认证器**（bcrypt）并置于 HTTP 认证器之前；
2. 在内置库中创建平台超管（后端自身连接 broker，使用 `MQTT_SERVER_USER/PASSWORD`）；
3. 创建指向后端 `/mqtt/auth`、`/mqtt/acl` 的 HTTP 认证/授权（`X-Auth-Key` = `MQTT_HOOK_SECRET`）；
4. 设置 `no_match=deny`、`deny_action=disconnect`、关闭授权缓存；
5. 创建后端互踢用 REST API Key，secret 仅返回一次并写入本地 `emqx_api.env`。

`emqx_api.env` 中的 `WQ_EMQX_API_KEY/SECRET` 需同步到 `server/config.yaml` 的 `emqx` 段
（或直接用同名环境变量启动后端）。

## 安全提醒

- `.env`、`emqx_api.env`、`certs/`（含 CA 与服务器私钥）均不入库，请勿提交。
- 默认密码仅供本地开发；生产环境务必替换全部 `change_me` 值，并只放行 8883、关闭或限制 1883。
