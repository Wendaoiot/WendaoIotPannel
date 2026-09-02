<p align="center">
  <b>English</b> &nbsp;|&nbsp; <a href="./README.md"><b>简体中文</b></a>
</p>

# WendaoIotPannel

WendaoIotPannel is an open-source general-purpose IoT system SaaS platform that provides complete capabilities for IoT device access, data collection, remote monitoring, and management. It solves the data link problem from device-side to cloud in IoT projects, enabling developers to quickly deploy their own IoT platforms.

This platform is developed by Wendaoiot and has been deployed in production environments across multiple industries, including smart water management, smart agriculture, smart aquaculture, and smart cities.

- Device-side uses standard MQTT protocol for access, without device model restrictions (device ID can be module IMEI, MAC, SN, etc.); Hezhou Air780EPM (LuatOS) is one of the supported models. Reference firmware is available in `hardware/`. Backend: Go + Frontend: Vue3/uni-app.
- Online Platform: http://pannel.wendaoiot.com/iot ｜ Health Check: http://pannel.wendaoiot.com/api/v1/health
- Code Repositories:
  - Gitee: https://gitee.com/wendaoiot/WendaoIotPannel
  - GitHub: https://github.com/Wendaoiot/WendaoIotPannel

**This project is licensed under the Apache 2.0 open-source license, allowing both commercial and non-commercial use.**

## Project Structure

```
wendaoiotpannel/
├── server/              # Go Backend API (Gin + GORM + MQTT)
├── web-admin/           # Vue3 + Element Plus Admin Panel (Tenant/Project/Device/OTA)
├── web-app/             # uni-app (H5 + WeChat Mini Program) Client Display
├── hardware/            # Air780EPM (LuatOS) Reference Firmware and Customer Demos (Supported Model)
│   └── 780epm_common/
│       ├── core/        # Official Hezhou Firmware .soc (Required for demo flashing, included)
│       └── project/     # 5 Customer Demos (0-5V/4-20mA/panel/rs485/ttl) + Internal Templates
├── tools/simulator/     # Device MQTT Simulator (Go)
├── dev.ps1              # Windows One-Click Startup Script
├── 提示词.md            # System Design / MQTT Protocol Specification
└── 测试清单.md          # Integration Testing Steps
```

## Prerequisites

- Go 1.22+
- Node.js 18+
- MySQL 8.0+
- EMQX or Mosquitto (MQTT Broker)

## Quick Start

### 1. Create Database

```sql
CREATE DATABASE IF NOT EXISTS wendaoiot DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. Configure and Start MQTT Broker

`server/config.yaml` defaults to connecting to the online platform `mqtt.broker: tcp://pannel.wendaoiot.com:1883` (no authentication; device identity = device ID, Air780EPM firmware uses module IMEI, other devices can use MAC/SN, etc.).

- **Local Development**: Start EMQX or Mosquitto locally and change `mqtt.broker` to `tcp://127.0.0.1:1883`;
- **Connect to Online**: Keep the default domain (requires server firewall to allow 1883/TCP).

> Air780EPM reference firmware (`mqtt_wendao.lua` / `mqtt_main.lua` in `hardware/`) and the server both use `pannel.wendaoiot.com` by default. When connecting to your own platform, modify the corresponding address constants in the demo.

### 3. Start Backend

```bash
cd server
go run ./cmd/server/
# Or: go build -o server.exe ./cmd/server/ && ./server.exe
```

Default port is 8080. Database (MySQL, default `127.0.0.1:3306`, root without password) and MQTT addresses are configured in `server/config.yaml`.

> ⚠️ `config.yaml` is the **development default configuration** (includes disclosed `jwt_secret`, empty database/MQTT passwords). For production deployment, directly modify the database password, `jwt_secret`, and other sensitive items before starting. **Do not commit production passwords back to the public repository** (add the file to local untracked changes, or overwrite the file separately during deployment).

### 4. Start Admin Panel

```bash
cd web-admin
npm install
npm run dev
```

Open http://localhost:3000

### 5. Start Client H5

```bash
cd web-app
npm install
npm run dev:h5
```

Open http://localhost:3001?project_id=1

(Online access URLs are in the "Online Platform" link at the top)

### 6. (Optional) Start Device Simulator

```bash
cd tools/simulator
# Connect to local broker
go run . -id ESP32-001
# Connect to online platform
go run . -id ESP32-001 -broker tcp://pannel.wendaoiot.com:1883
```

## Default Accounts

> ⚠️ The following are **development/demo default credentials**, only for local initialization. For production deployment, immediately change the super admin and tenant admin passwords, and replace `jwt_secret` in `server/config.yaml` (this repository is public, default values are disclosed).

| Role              | Username | Password |
| ----------------- | -------- | -------- |
| Super Administrator | admin    | admin123 |
| Tenant Administrator | tenant1  | 123456   |

## API Endpoints

| Method           | Path                          | Description                 |
| ---------------- | ----------------------------- | --------------------------- |
| POST             | `/api/v1/login`             | Login (public)              |
| GET              | `/api/v1/dashboard/stats`   | Dashboard Statistics        |
| POST/GET         | `/api/v1/tenants`           | Tenant Management           |
| POST/GET         | `/api/v1/projects`          | Project Management          |
| GET              | `/api/v1/projects/:id/data` | Project Aggregated Data     |
| POST/GET/DELETE  | `/api/v1/projects/:id/tags` | Project Tags                |
| POST/GET         | `/api/v1/devices`           | Device Management           |
| POST/GET/DELETE  | `/api/v1/devices/:id/tags`  | Device Tag Configuration    |
| GET              | `/api/v1/devices/:id/data`  | Device Historical Data      |
| POST             | `/api/v1/devices/:id/control` | Send Control Command      |

## MQTT Topics

| Topic                             | Description                             |
| --------------------------------- | --------------------------------------- |
| `wendao/{deviceId}/data`        | Device reports data (upstream)          |
| `wendao/{deviceId}/data/ack`    | Server response (downstream)            |
| `wendao/{deviceId}/control`     | Server sends control command (downstream) |
| `wendao/{deviceId}/control/ack` | Device responds with control result (upstream) |

For complete topic conventions (including status / rs485 / uart / ota, etc.), see `提示词.md`.

## Reference Device Firmware (Air780EPM / hardware)

The platform device-side uses standard MQTT protocol (topic `wendao/{deviceId}/...`). Any module/MCU supporting MQTT can access (device ID can be IMEI/MAC/SN, etc.); the `hardware/` directory contains reference firmware and customer demos for the Hezhou Air780EPM (LuatOS) model.

- Customer demos are located in `hardware/780epm_common/project/` (`demo_0_5v`, `demo_4_20ma`, `demo_panel`, `demo_rs485`, `demo_ttl`), each with an independent `readme.md`.
- Air780EPM firmware defaults to connecting to `pannel.wendaoiot.com:1883`, device identity uses module IMEI (no username/password); to connect to your own platform, modify `HOST`/`PORT`/`TOPIC_PREFIX` at the top of `mqtt_wendao.lua` in the demo.
- Flashing: Use Luatools, select the official firmware `.soc` from `core/` and download together with the project directory scripts.

## License

This project is licensed under the [Apache 2.0](LICENSE) open-source license.

**Allows both commercial and non-commercial use.**

**License**:
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

## Contact Us

Add WeChat friend to join the WeChat group (Note: WendaoIotPannel):


![WeChat QR Code](./微信加好友.png)
