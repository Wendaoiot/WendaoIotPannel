<p align="center">
  <a href="./README.en.md"><b>English</b></a> &nbsp;|&nbsp; <b>简体中文</b>
</p>

# WendaoIotPannel

WendaoIotPannel 是一个开源的通用物联网系统 SAAS 平台，提供完整的物联网设备接入、数据采集、远程监控和管理能力。它解决了物联网项目从设备端到云端的数据链路问题，让开发者能够快速部署属于自己的物联网平台。

这个平台由 Wendaoiot（闻道物联）研发，已被用于智慧水务、智慧农业、智慧养殖、智慧城市等多个行业的生产环境。

- 设备端走标准 MQTT 协议接入，不限定设备型号（设备ID 可为模组 IMEI、MAC、SN 等）；合宙 Air780EPM（LuatOS）是已适配型号之一，参考固件见 `hardware/`。后端 Go + 前端 Vue3/uni-app。
- 线上平台：[http://pannel.wendaoiot.com/iot](http://pannel.wendaoiot.com/iot) ｜ 健康检查：[http://pannel.wendaoiot.com/api/v1/health](http://pannel.wendaoiot.com/api/v1/health)
- 代码仓库：
  - Gitee：[https://gitee.com/wendaoiot/WendaoIotPannel](https://gitee.com/wendaoiot/WendaoIotPannel)
  - GitHub：[https://github.com/Wendaoiot/WendaoIotPannel](https://github.com/Wendaoiot/WendaoIotPannel)

**本项目采用 Apache 2.0 开源协议，允许任何商业和非商业使用。**

## 项目结构

```
wendaoiotpannel/
├── server/              # Go 后端 API（Gin + GORM + MQTT）
├── web-admin/           # Vue3 + Element Plus 管理后台（租户/项目/设备/OTA）
├── web-app/             # uni-app（H5 + 微信小程序）C 端展示
├── hardware/            # Air780EPM（LuatOS）参考固件与客户 demo（已适配型号之一）
│   └── 780epm_common/
│       ├── core/        # 合宙官方固件 .soc（烧录 demo 必需，已入库）
│       └── project/     # 5 个客户 demo（0-5V/4-20mA/panel/rs485/ttl）+ 内部模板
├── tools/simulator/     # 设备 MQTT 模拟器（Go）
├── dev.ps1              # Windows 一键启动脚本
├── 提示词.md            # 系统设计 / MQTT 协议说明
└── 测试清单.md          # 联调测试步骤
```

## 前置条件

- Go 1.22+
- Node.js 18+
- MySQL 8.0+
- EMQX 或 Mosquitto (MQTT Broker)

## 快速启动

### 1. 创建数据库

```sql
CREATE DATABASE IF NOT EXISTS wendaoiot DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 配置并启动 MQTT Broker

`server/config.yaml` 默认连接线上平台 `mqtt.broker: tcp://pannel.wendaoiot.com:1883`（无认证；设备身份 = 设备ID，Air780EPM 固件取模组 IMEI，其他设备可用 MAC/SN 等）。

- **本地联调**：在本机启动 EMQX 或 Mosquitto，并把 `mqtt.broker` 改为 `tcp://127.0.0.1:1883`；
- **连线上**：保持默认域名即可（需服务器放行 1883/TCP）。

> Air780EPM 参考固件（`hardware/` 下各 `mqtt_wendao.lua` / `mqtt_main.lua`）与服务器默认均使用域名 `pannel.wendaoiot.com`，切换自有平台时改对应地址常量即可。

### 3. 启动后端

```bash
cd server
go run ./cmd/server/
# 或者: go build -o server.exe ./cmd/server/ && ./server.exe
```

默认端口 8080。数据库（MySQL，默认 `127.0.0.1:3306`，root 无密码）与 MQTT 地址均在 `server/config.yaml` 中配置。

> ⚠️ `config.yaml` 是**开发默认配置**（含已公开的 `jwt_secret`、空数据库/ MQTT 密码）。生产部署请直接修改其中的数据库密码、`jwt_secret` 等敏感项后再启动，**不要把生产密码提交回公开仓库**（对应文件可加入本地未跟踪改动，或部署时单独覆盖文件）。

### 4. 启动管理后台

```bash
cd web-admin
npm install
npm run dev
```

打开 http://localhost:3000

### 5. 启动 C端 H5

```bash
cd web-app
npm install
npm run dev:h5
```

打开 http://localhost:3001?project_id=1

（线上访问地址见顶部「线上平台」链接）

### 6. (可选) 启动设备模拟器

```bash
cd tools/simulator
# 连本地 broker
go run . -id ESP32-001
# 连线上平台
go run . -id ESP32-001 -broker tcp://pannel.wendaoiot.com:1883
```

## 默认账号

> ⚠️ 以下为**开发/演示默认口令**，仅用于本地初始化。生产部署务必立即修改超级管理员与租户管理员密码，并更换 `server/config.yaml` 中的 `jwt_secret`（本仓库为公开仓库，默认值已公开）。

| 角色       | 用户名  | 密码     |
| ---------- | ------- | -------- |
| 超级管理员 | admin   | admin123 |
| 租户管理员 | tenant1 | 123456   |

## API 接口

| 方法            | 路由                            | 说明         |
| --------------- | ------------------------------- | ------------ |
| POST            | `/api/v1/login`               | 登录 (公开)  |
| GET             | `/api/v1/dashboard/stats`     | 仪表盘统计   |
| POST/GET        | `/api/v1/tenants`             | 租户管理     |
| POST/GET        | `/api/v1/projects`            | 项目管理     |
| GET             | `/api/v1/projects/:id/data`   | 项目聚合数据 |
| POST/GET/DELETE | `/api/v1/projects/:id/tags`   | 项目标签     |
| POST/GET        | `/api/v1/devices`             | 设备管理     |
| POST/GET/DELETE | `/api/v1/devices/:id/tags`    | 设备标签配置 |
| GET             | `/api/v1/devices/:id/data`    | 设备历史数据 |
| POST            | `/api/v1/devices/:id/control` | 下发控制指令 |

## MQTT Topic

| Topic                             | 说明                    |
| --------------------------------- | ----------------------- |
| `wendao/{deviceId}/data`        | 设备上报数据 (上行)     |
| `wendao/{deviceId}/data/ack`    | 服务端回复 (下行)       |
| `wendao/{deviceId}/control`     | 服务端下发控制 (下行)   |
| `wendao/{deviceId}/control/ack` | 设备回复控制结果 (上行) |

完整 topic 约定（含 status / rs485 / uart / ota 等）见 `提示词.md`。

## 参考设备端固件（Air780EPM / hardware）

平台设备侧为标准 MQTT 协议（主题 `wendao/{deviceId}/...`），任意支持 MQTT 的模组/MCU 均可接入（设备ID 可为 IMEI/MAC/SN 等）；`hardware/` 目录是合宙 Air780EPM（LuatOS）这一型号的参考固件与客户 demo。

- 客户 demo 位于 `hardware/780epm_common/project/`（`demo_0_5v`、`demo_4_20ma`、`demo_panel`、`demo_rs485`、`demo_ttl`），各工程内有独立 `readme.md`。
- Air780EPM 固件默认连接 `pannel.wendaoiot.com:1883`，设备身份取模组 IMEI（无用户名密码）；接自有平台时改 demo 内 `mqtt_wendao.lua` 顶部 `HOST`/`PORT`/`TOPIC_PREFIX`。
- 烧录：用 Luatools，选择 `core/` 下的官方固件 `.soc` + 工程目录脚本一起下载。

## 开源协议

本项目采用 [Apache 2.0](LICENSE) 开源协议。

**允许任何商业和非商业使用。**

## 联系我们

加好友进微信群（备注 WendaoIotPannel）：


![微信加好友](./微信加好友.png)
