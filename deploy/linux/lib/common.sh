#!/usr/bin/env bash
# 公共函数与变量：被 deploy.sh 及 scripts/*.sh source。
# 不单独执行。

# ---- 颜色日志 ----
if [ -t 1 ]; then
  C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YEL=$'\033[33m'; C_BLU=$'\033[36m'; C_RST=$'\033[0m'
else
  C_RED=''; C_GRN=''; C_YEL=''; C_BLU=''; C_RST=''
fi

log()  { echo "${C_GRN}[+]${C_RST} $*"; }
info() { echo "${C_BLU}[i]${C_RST} $*"; }
warn() { echo "${C_YEL}[!]${C_RST} $*" >&2; }
err()  { echo "${C_RED}[x]${C_RST} $*" >&2; }

die() { err "$*"; exit 1; }

require_root() {
  [ "$(id -u)" -eq 0 ] || die "请用 root 运行（如 sudo ./deploy.sh ...）。"
}

# ---- 发行版 / 包管理器 ----
detect_os() {
  if command -v apt-get >/dev/null 2>&1; then
    PKG_MGR="apt"
  elif command -v dnf >/dev/null 2>&1; then
    PKG_MGR="dnf"
  elif command -v yum >/dev/null 2>&1; then
    PKG_MGR="yum"
  else
    PKG_MGR=""
  fi
  export PKG_MGR
}

pkg_install() {
  case "$PKG_MGR" in
    apt)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update -y
      apt-get install -y "$@"
      ;;
    dnf) dnf install -y "$@" ;;
    yum) yum install -y "$@" ;;
    *) die "不支持的发行版（无 apt/dnf/yum），请手工安装：$*" ;;
  esac
}

# ---- 加载部署配置 ----
# DEPLOY_ENV 由 deploy.sh 指定（默认 ./deploy.env，不存在则用 deploy.env.example）。
load_config() {
  local here="$1"
  if [ -f "$here/deploy.env" ]; then
    # shellcheck disable=SC1090
    set -a; . "$here/deploy.env"; set +a
    info "已加载部署配置：$here/deploy.env"
  else
    set -a; . "$here/deploy.env.example"; set +a
    warn "未找到 deploy.env，使用内置默认值（建议 cp deploy.env.example deploy.env 后修改）"
  fi

  # 仓库根目录（deploy/linux 的上两级）
  REPO_ROOT="$(cd "$here/../.." && pwd)"
  export REPO_ROOT

  INSTALL_DIR="${INSTALL_DIR:-/opt/wendao}"
  SECRET_DIR="$INSTALL_DIR/secrets"
  CERT_DIR="${CERT_DIR:-$INSTALL_DIR/certs}"
  mkdir -p "$SECRET_DIR"
  export INSTALL_DIR SECRET_DIR CERT_DIR
}

# ---- 密钥管理：首次生成并落盘（600），后续复用 ----
# 用法： get_secret <文件名> [<生成命令>]
get_secret() {
  local name="$1"; shift
  local f="$SECRET_DIR/$name"
  if [ ! -s "$f" ]; then
    mkdir -p "$SECRET_DIR"
    if [ "$#" -gt 0 ]; then
      "$@" > "$f"
    else
      openssl rand -base64 32 | tr -d '\n' > "$f"
    fi
    chmod 600 "$f"
  fi
  cat "$f"
}

# 生成单值 token 到变量（不落多余换行）
gen_token() { openssl rand -hex "${1:-16}"; }
