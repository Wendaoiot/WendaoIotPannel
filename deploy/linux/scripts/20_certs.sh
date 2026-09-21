#!/usr/bin/env bash
# 生成 TLS 证书：
#  1) MQTT 8883：自签 CA + 服务器证书（设备导入 ca.crt）
#  2) Nginx HTTPS：复用同一自签 CA + 服务器证书（生产有正规证书可替换）
# 幂等：已存在则跳过（FORCE_CERTS=1 重生成）。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"

require_root
command -v openssl >/dev/null 2>&1 || die "缺少 openssl。"

DOMAIN="${DOMAIN:-pannel.wendaoiot.com}"
SERVER_IP="${SERVER_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"

mkdir -p "$CERT_DIR"

if [ "${FORCE_CERTS:-0}" = "1" ]; then
  rm -f "$CERT_DIR"/{ca.key,ca.crt,server.key,server.crt,server.csr}
fi

if [ -s "$CERT_DIR/ca.crt" ] && [ -s "$CERT_DIR/server.crt" ]; then
  info "证书已存在，跳过（FORCE_CERTS=1 可重生成）。"
  exit 0
fi

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

cat > "$TMP/san.ext" <<EOF
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${DOMAIN}
DNS.2 = localhost
IP.1  = 127.0.0.1
EOF
if [ -n "$SERVER_IP" ] && [ "$SERVER_IP" != "127.0.0.1" ]; then
  echo "IP.2  = ${SERVER_IP}" >> "$TMP/san.ext"
fi

log "生成自签 CA ……"
openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
  -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
  -subj "/CN=WendaoIoT CA/O=WendaoIoT/C=CN"

log "生成服务器证书（域名 $DOMAIN）……"
openssl req -newkey rsa:2048 -nodes \
  -keyout "$CERT_DIR/server.key" -out "$TMP/server.csr" \
  -subj "/CN=${DOMAIN}/O=WendaoIoT/C=CN"
openssl x509 -req -in "$TMP/server.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/server.crt" -days 1095 -extfile "$TMP/san.ext"

chmod 600 "$CERT_DIR/ca.key" "$CERT_DIR/server.key"
chmod 644 "$CERT_DIR/ca.crt" "$CERT_DIR/server.crt"

openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/server.crt"
log "证书生成于 $CERT_DIR："
info "设备/MQTTX 导入 ca.crt 连 mqtts://<host>:${MQTT_TLS_PORT:-8883}"
info "有 Let's Encrypt 等正规证书时，替换 Nginx 处 server.crt/key 即可（MQTT 仍建议保留本 CA）。"
