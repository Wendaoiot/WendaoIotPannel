# Linux 裸机部署

在**没有 Docker** 的 Linux 服务器上，用脚本一键安装并配置 WendaoIotPannel（MySQL + EMQX + Go 后端 + Nginx 前端）。
Docker 方式见 [`../docker/`](../docker)，生产部署的手工步骤与安全清单见根目录 [部署与测试.md](../../部署与测试.md)。

## 支持系统

- Debian / Ubuntu（apt）
- RHEL / CentOS / Rocky / AlmaLinux（dnf/yum）

需联网下载软件包与 Go/Node。架构 x86_64、arm64。

## 快速开始

```bash
# 以普通用户上传代码后，在项目根执行
sudo bash -c 'cd deploy/linux && cp deploy.env.example deploy.env && ./deploy.sh'
```

不想改配置可以直接跑 `sudo ./deploy.sh`——MySQL、JWT、Dashboard、互踢等**所有密钥自动生成强随机值**，
落盘 `/opt/wendao/secrets/`（权限 600）。想自定义域名/目录/密码时先编辑 `deploy.env`。

## 分步执行

```bash
sudo ./deploy.sh deps       # 安装 MySQL/MariaDB、EMQX、Nginx、Go、Node.js
sudo ./deploy.sh database   # 建库 wendaoiot、建专用账号
sudo ./deploy.sh certs      # 生成 8883 MQTT 与 Nginx 的 TLS 证书
sudo ./deploy.sh emqx       # 配置 EMQX 认证/ACL/SSL/互踢 API Key
sudo ./deploy.sh backend    # 编译后端、生成 config.yaml、装 systemd、改默认 admin 密码
sudo ./deploy.sh frontend   # 构建两个前端、配置 Nginx
sudo ./deploy.sh            # 不填步骤 = 依次执行全部
```

所有步骤**幂等**，可中断后重跑（密钥复用、已建则更新）。

## 目录结构

| 路径 | 说明 |
| --- | --- |
| `deploy.sh` | 一键入口 |
| `deploy.env.example` | 部署配置模板（`deploy.env` 不入库） |
| `lib/common.sh` | 日志、包管理、密钥生成等公共函数 |
| `scripts/00..50_*.sh` | 各阶段脚本 |
| `templates/wendao.service` | 后端 systemd 单元模板 |
| `templates/nginx-wendao.conf` | Nginx 站点模板 |

## 部署产物（服务器上）

默认安装到 `/opt/wendao/`：

```
/opt/wendao/
├── server/
│   ├── wendao-server      # 后端二进制（systemd: wendao.service）
│   └── config.yaml        # 由脚本渲染（600）
├── web-admin/             # 管理后台静态产物
├── web-app-h5/            # C 端 H5 静态产物
├── certs/                 # ca/server 证书与私钥
└── secrets/               # 全部自动生成的密码（600）
    ├── mysql_root_password
    ├── mysql_app_password
    ├── jwt_secret
    ├── mqtt_hook_secret
    ├── mqtt_server_password
    ├── emqx_dashboard_password
    ├── emqx_api.env        # 互踢 REST API Key/Secret
    └── admin_password      # 改后的超管 admin 密码
```

## 部署后

1. 打开管理后台 `http://<域名或IP>/`，用 `admin` 与 `secrets/admin_password` 登录。
2. EMQX Dashboard：`http://<server-ip>:18083`，密码见 `secrets/emqx_dashboard_password`。
3. 默认 Nginx 仅监听 80。启用 HTTPS：
   - 有域名：`sudo certbot --nginx -d <域名>`（推荐）；
   - 或编辑 `/etc/nginx/conf.d/wendao.conf`，使用 `certs/` 下证书开启 443。
4. MQTT 接入：`mqtts://<域名>:8883`（设备导入 `/opt/wendao/certs/ca.crt`）。

## 常用运维命令

```bash
sudo systemctl status wendao        # 后端状态
sudo journalctl -u wendao -f        # 后端日志
sudo systemctl restart wendao       # 重启后端
sudo systemctl restart emqx nginx   # 重启 EMQX / Nginx
curl http://127.0.0.1:8080/api/v1/health
```

## 注意事项

- 脚本需 root；EMQX 使用官方安装脚本配置软件源（联网）。
- 云服务器请在安全组放行：80（HTTP）、1883（如用明文 MQTT）、8883（mqtts）；
  **不要**把 3306、18083、8080 暴露到公网。
- 生产建议关闭 1883 明文、仅用 8883，并配好 Nginx HTTPS 与 `cors_origins`。
