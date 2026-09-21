#!/usr/bin/env bash
# 编译部署 Go 后端：建运行用户 → 构建二进制 → 生成 config.yaml → 装 systemd →
# 启动健康检查 → 修改超管默认密码。
# 前置：10_database、30_configure_emqx 已执行。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"

require_root
export PATH="$PATH:/usr/local/go/bin"
command -v go >/dev/null 2>&1 || die "缺少 Go，请先执行 00_install_deps。"

APP_USER="${APP_USER:-wendao}"
SRV_DIR="$INSTALL_DIR/server"
PORT="${SERVER_PORT:-8080}"
DOMAIN="${DOMAIN:-pannel.wendaoiot.com}"

# ---- 运行用户 ----
if ! id "$APP_USER" >/dev/null 2>&1; then
  useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$APP_USER"
  log "已创建系统用户 $APP_USER。"
fi
mkdir -p "$SRV_DIR"

# ---- 收集密钥（均来自 secrets/，不存在则生成）----
JWT_PW="$(get_secret jwt_secret)"
HOOK_PW="$(get_secret mqtt_hook_secret)"
MYSQL_PW="$(get_secret mysql_app_password)"
SRV_MQTT_PW="$(get_secret mqtt_server_password)"

EMQX_KEY=""; EMQX_SECRET=""
if [ -f "$SECRET_DIR/emqx_api.env" ]; then
  EMQX_KEY="$(sed -n 's/^WQ_EMQX_API_KEY=//p' "$SECRET_DIR/emqx_api.env" | tr -d '\n')"
  EMQX_SECRET="$(sed -n 's/^WQ_EMQX_API_SECRET=//p' "$SECRET_DIR/emqx_api.env" | tr -d '\n')"
fi

# ---- 构建 ----
log "编译后端二进制……"
( cd "$REPO_ROOT/server" && \
  CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$SRV_DIR/wendao-server" ./cmd/server/ )

# ---- 生成 config.yaml ----
log "生成 $SRV_DIR/config.yaml ……"
cat > "$SRV_DIR/config.yaml" <<EOF
server:
  port: ${PORT}
  jwt_secret: "${JWT_PW}"
  cors_origins:
    - "https://${DOMAIN}"
  mqtt_auth_secret: "${HOOK_PW}"

mysql:
  host: 127.0.0.1
  port: ${MYSQL_PORT:-3306}
  user: "${MYSQL_APP_USER:-wendaoiot}"
  password: "${MYSQL_PW}"
  database: "${MYSQL_DATABASE:-wendaoiot}"

mqtt:
  broker: "tcp://127.0.0.1:${MQTT_PORT:-1883}"
  client_id: wendao_server
  username: "${MQTT_SERVER_USER:-wendao_server}"
  password: "${SRV_MQTT_PW}"

device:
  online_mode: connection
  offline_timeout_sec: 60
  scan_interval_sec: 60
  ping_interval_sec: 60

emqx:
  api_base: "http://127.0.0.1:${EMQX_DASHBOARD_PORT:-18083}"
  api_key: "${EMQX_KEY}"
  api_secret: "${EMQX_SECRET}"
EOF
chmod 600 "$SRV_DIR/config.yaml"

# ---- systemd ----
log "安装 systemd 服务……"
sed -e "s#__USER__#$APP_USER#g" \
    -e "s#__SRV_DIR__#$SRV_DIR#g" \
    "$HERE/templates/wendao.service" > /etc/systemd/system/wendao.service
systemctl daemon-reload

# 在启动前改属主：config.yaml 为 600，wendao 用户必须能读
chown -R "$APP_USER:$APP_USER" "$INSTALL_DIR"

systemctl enable wendao >/dev/null 2>&1
systemctl restart wendao

# ---- 健康检查 ----
log "等待后端健康检查……"
ok=0
for i in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$PORT/api/v1/health" >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done
[ "$ok" -eq 1 ] || { journalctl -u wendao -n 40 --no-pager; die "后端未通过健康检查。"; }
log "后端运行正常：http://127.0.0.1:$PORT/api/v1/health"

# ---- 修改超管默认密码（admin/admin123 → 随机强密码）----
ADMIN_F="$SECRET_DIR/admin_password"
ADMIN_PW="$(get_secret admin_password)"
RC="$(ADMIN_PW="$ADMIN_PW" PORT="$PORT" python3 - <<'PY'
import json, os, urllib.request, urllib.error

port = os.environ["PORT"]
new_pw = os.environ["ADMIN_PW"]
base = "http://127.0.0.1:%s/api/v1" % port


def req(method, path, body=None, token=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(base + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(r, timeout=10) as resp:
        raw = resp.read().decode()
        return json.loads(raw) if raw else {}


try:
    login = req("POST", "/login", {"username": "admin", "password": "admin123"})
    token = login["data"]["token"]
    users = req("GET", "/users", token=token)["data"]
    aid = next(u["id"] for u in users if u.get("username") == "admin")
    req("PUT", "/users/%s/password" % aid, {"password": new_pw}, token=token)
    print("changed")
except urllib.error.HTTPError as e:
    print("skip:" + str(e.code))
except StopIteration:
    print("skip:no-admin")
PY
)"
case "$RC" in
  changed) log "已修改超管 admin 默认密码（新密码：$ADMIN_F）。" ;;
  skip:*) warn "超管密码可能已改过（$RC），请以 $ADMIN_F 为准；如无法登录请手工重置。" ;;
  *) warn "改密脚本异常（$RC），可登录后手工修改。" ;;
esac

log "后端部署完成。"
