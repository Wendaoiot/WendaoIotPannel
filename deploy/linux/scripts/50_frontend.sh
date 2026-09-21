#!/usr/bin/env bash
# 构建前端并配置 Nginx：
#  web-admin → $INSTALL_DIR/web-admin
#  web-app H5 → $INSTALL_DIR/web-app-h5
# 渲染安装 Nginx 站点。
# BUILD_FRONTEND=skip 时只装 Nginx 配置（假定产物已就位）。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"

require_root
command -v nginx >/dev/null 2>&1 || die "缺少 Nginx，请先执行 00_install_deps。"

DOMAIN="${DOMAIN:-pannel.wendaoiot.com}"
PORT="${SERVER_PORT:-8080}"
ADMIN_ROOT="$INSTALL_DIR/web-admin"
H5_ROOT="$INSTALL_DIR/web-app-h5"

if [ "${BUILD_FRONTEND:-true}" != "skip" ]; then
  command -v npm >/dev/null 2>&1 || die "缺少 Node.js/npm，请先执行 00_install_deps。"

  log "构建管理后台 web-admin ……"
  ( cd "$REPO_ROOT/web-admin" && npm install && npm run build )
  mkdir -p "$ADMIN_ROOT"
  rm -rf "$ADMIN_ROOT"/*
  cp -r "$REPO_ROOT/web-admin/dist/." "$ADMIN_ROOT/"

  log "构建 C 端 web-app H5 ……"
  ( cd "$REPO_ROOT/web-app" && npm install && npm run build:h5 )
  mkdir -p "$H5_ROOT"
  rm -rf "$H5_ROOT"/*
  # uni-app H5 产物路径 dist/build/h5
  cp -r "$REPO_ROOT/web-app/dist/build/h5/." "$H5_ROOT/"
else
  info "BUILD_FRONTEND=skip，跳过构建（使用现有产物）。"
  mkdir -p "$ADMIN_ROOT" "$H5_ROOT"
fi

# ---- 安装 Nginx 站点 ----
log "安装 Nginx 站点配置……"
sed -e "s#__DOMAIN__#$DOMAIN#g" \
    -e "s#__PORT__#$PORT#g" \
    -e "s#__ADMIN_ROOT__#$ADMIN_ROOT#g" \
    -e "s#__H5_ROOT__#$H5_ROOT#g" \
    -e "s#__CERT_DIR__#$CERT_DIR#g" \
    "$HERE/templates/nginx-wendao.conf" > /etc/nginx/conf.d/wendao.conf

# Debian/Ubuntu 默认站点可能占用 80；移除默认站点冲突
if [ -f /etc/nginx/sites-enabled/default ]; then
  rm -f /etc/nginx/sites-enabled/default
fi

nginx -t
systemctl enable --now nginx
systemctl restart nginx

chown -R "${APP_USER:-wendao}:${APP_GROUP:-wendao}" "$ADMIN_ROOT" "$H5_ROOT" 2>/dev/null || true

log "前端部署完成："
info "管理后台：http://${DOMAIN}/"
info "C 端 H5：http://${DOMAIN}/iot/"
warn "模板默认仅监听 80；如需 HTTPS，编辑 /etc/nginx/conf.d/wendao.conf 开启 443 或运行 certbot --nginx。"
