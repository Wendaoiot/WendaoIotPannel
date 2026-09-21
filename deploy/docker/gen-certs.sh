#!/usr/bin/env bash
# 生成 EMQX 8883 TLS 自签 CA 与服务器证书（需要 openssl）。
# 产物： certs/ca.crt server.crt server.key
# Windows 在 Git Bash / WSL / MSYS2 下运行；PowerShell 用户用 gen-certs.ps1。
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTDIR="$DIR/certs"
DAYS_CA=3650
DAYS_SRV=1095

# SAN 主机名/IP 可用环境变量覆盖，空格分隔
HOSTS="${CERT_HOSTS:-localhost 127.0.0.1 pannel.wendaoiot.com}"

mkdir -p "$CERTDIR"
TMP_EXT="$(mktemp)"
trap 'rm -f "$TMP_EXT"' EXIT

alt="subjectAltName="
first=1
for h in $HOSTS; do
  if [[ "$h" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    entry="IP:$h"
  else
    entry="DNS:$h"
  fi
  if [ $first -eq 1 ]; then alt+="$entry"; first=0; else alt+=",$entry"; fi
done
echo "$alt" > "$TMP_EXT"

echo "=== [1/3] CA ==="
openssl req -x509 -newkey rsa:2048 -nodes -days "$DAYS_CA" \
  -keyout "$CERTDIR/ca.key" -out "$CERTDIR/ca.crt" \
  -subj "/CN=WendaoIoT Local CA/O=WendaoIoT/C=CN"

echo "=== [2/3] server key/csr ==="
openssl req -newkey rsa:2048 -nodes \
  -keyout "$CERTDIR/server.key" -out "$CERTDIR/server.csr" \
  -subj "/CN=pannel.wendaoiot.com/O=WendaoIoT/C=CN"

echo "=== [3/3] sign server cert ==="
openssl x509 -req -in "$CERTDIR/server.csr" \
  -CA "$CERTDIR/ca.crt" -CAkey "$CERTDIR/ca.key" -CAcreateserial \
  -out "$CERTDIR/server.crt" -days "$DAYS_SRV" -extfile "$TMP_EXT"
rm -f "$CERTDIR/server.csr"

echo
openssl verify -CAfile "$CERTDIR/ca.crt" "$CERTDIR/server.crt"
echo "certificates generated in: $CERTDIR"
