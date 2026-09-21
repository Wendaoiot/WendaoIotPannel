#!/usr/bin/env bash
# 安装运行时依赖：MySQL/MariaDB、EMQX、Nginx、Go、Node.js 与构建工具。
# 支持 Debian/Ubuntu(apt) 与 RHEL/CentOS/Rocky/Alma(dnf/yum)。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"
detect_os

require_root
[ -n "$PKG_MGR" ] || die "未找到 apt/dnf/yum，无法自动安装。"

GO_VERSION="${GO_VERSION:-1.25.0}"
NODE_MAJOR="${NODE_MAJOR:-20}"

log "包管理器：$PKG_MGR"

# ---- MySQL / Nginx / 基础工具 ----
log "安装数据库、Nginx 与构建工具……"
case "$PKG_MGR" in
  apt)
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y nginx openssl curl git make ca-certificates gnupg
    # Ubuntu 提供 mysql-server；Debian 提供 default-mysql-server(MariaDB，驱动兼容)
    apt-get install -y mysql-server 2>/dev/null || apt-get install -y default-mysql-server
    ;;
  dnf|yum)
    $PKG_MGR install -y nginx openssl curl git make tar policycoreutils-python-utils
    ($PKG_MGR install -y mysql-server) || $PKG_MGR install -y mariadb-server
    ;;
esac

# ---- EMQX（官方安装脚本，自动配软件源）----
if command -v emqx >/dev/null 2>&1; then
  info "EMQX 已安装：$(emqx version 2>/dev/null | head -1)"
else
  log "通过官方脚本安装 EMQX ……"
  case "$PKG_MGR" in
    apt) curl -s https://assets.emqx.com/scripts/install-emqx-deb.sh | bash ;;
    *)   curl -s https://assets.emqx.com/scripts/install-emqx-rpm.sh | bash ;;
  esac
fi

# ---- Go ----
export PATH="$PATH:/usr/local/go/bin"
need_go=1
if command -v go >/dev/null 2>&1; then
  cur="$(go version | awk '{print $3}' | sed 's/go//')"
  if [ "$(printf '%s\n%s\n' "$GO_VERSION" "$cur" | sort -V | head -1)" = "$GO_VERSION" ]; then
    info "Go $cur 满足 >= $GO_VERSION，跳过。"; need_go=0
  fi
fi
if [ "$need_go" -eq 1 ]; then
  log "安装 Go $GO_VERSION ……"
  arch="$(uname -m)"; case "$arch" in x86_64) arch=amd64;; aarch64) arch=arm64;; *) die "不支持的架构 $arch";; esac
  tmp="$(mktemp -d)"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" -o "$tmp/go.tgz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tmp/go.tgz"
  rm -rf "$tmp"
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
fi

# ---- Node.js ----
need_node=1
if command -v node >/dev/null 2>&1; then
  cur="$(node -v | sed 's/v//' | cut -d. -f1)"
  if [ "$cur" -ge 18 ] 2>/dev/null; then info "Node $(node -v) 满足，跳过。"; need_node=0; fi
fi
if [ "$need_node" -eq 1 ]; then
  log "安装 Node.js $NODE_MAJOR ……"
  case "$PKG_MGR" in
    apt)
      mkdir -p /etc/apt/keyrings
      curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key \
        | gpg --dearmor -yes -o /etc/apt/keyrings/nodesource.gpg
      echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" \
        > /etc/apt/sources.list.d/nodesource.list
      apt-get update -y && apt-get install -y nodejs
      ;;
    dnf|yum)
      curl -fsSL "https://rpm.nodesource.com/setup_$NODE_MAJOR.x" | bash -
      $PKG_MGR install -y nodejs
      ;;
  esac
fi

log "依赖安装完成："
go version || true
node -v || true
mysql --version || true
emqx version 2>/dev/null | head -1 || true
nginx -v 2>&1 | head -1 || true
