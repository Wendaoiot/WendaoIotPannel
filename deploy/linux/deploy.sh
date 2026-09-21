#!/usr/bin/env bash
# =====================================================================
# WendaoIotPannel Linux 裸机一键部署
#
# 用法（root）：
#   cp deploy.env.example deploy.env   # 按需修改域名/密码（可先不改，密钥自动生成）
#   ./deploy.sh                         # 全流程
#   ./deploy.sh backend                 # 只执行某一步（见下表）
#
# 步骤：
#   deps     安装 MySQL/EMQX/Nginx/Go/Node
#   database 建库建账号
#   certs    生成 8883/Nginx TLS 证书
#   emqx     配置 EMQX 认证/ACL/互踢
#   backend  编译部署 Go 后端（systemd）
#   frontend 构建前端 + Nginx
#   all      依次全部（默认）
# =====================================================================
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"

STEP="${1:-all}"

run_step() {
  case "$1" in
    deps)     bash "$HERE/scripts/00_install_deps.sh" ;;
    database) bash "$HERE/scripts/10_database.sh" ;;
    certs)    bash "$HERE/scripts/20_certs.sh" ;;
    emqx)     bash "$HERE/scripts/30_configure_emqx.sh" ;;
    backend)  bash "$HERE/scripts/40_backend.sh" ;;
    frontend) bash "$HERE/scripts/50_frontend.sh" ;;
    *) die "未知步骤：$1（可选 deps/database/certs/emqx/backend/frontend/all）" ;;
  esac
}

log "WendaoIotPannel 部署 → 步骤：$STEP"

case "$STEP" in
  all)
    for s in deps database certs emqx backend frontend; do
      log "========== 步骤：$s =========="
      run_step "$s"
    done
    ;;
  *) run_step "$STEP" ;;
esac

log "全部完成。"
if [ "$STEP" = "all" ]; then
  info "安装目录：${INSTALL_DIR:-/opt/wendao}（密钥在 secrets/，证书在 certs/）"
  info "管理后台：http://${DOMAIN:-pannel.wendaoiot.com}/"
  info "C 端 H5：http://${DOMAIN:-pannel.wendaoiot.com}/iot/"
fi
