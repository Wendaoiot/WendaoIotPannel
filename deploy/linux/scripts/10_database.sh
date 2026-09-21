#!/usr/bin/env bash
# 启动 MySQL/MariaDB，初始化 root 密码，建库建专用账号。
# 幂等：重复执行不会重复建库/改密（密码落 secrets/ 复用）。
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../lib/common.sh
. "$HERE/lib/common.sh"
load_config "$HERE"
detect_os

require_root

DB="${MYSQL_DATABASE:-wendaoiot}"
APP_U="${MYSQL_APP_USER:-wendaoiot}"

# 判断服务名（apt 装 mysql-server；RPM 可能是 mysqld/mariadb）
SVC=""
for c in mysql mysqld mariadb; do
  if systemctl list-unit-files "$c.service" 2>/dev/null | grep -q "$c.service"; then SVC="$c"; break; fi
done
[ -n "$SVC" ] || die "未找到数据库服务单元（mysql/mysqld/mariadb），请先执行 00_install_deps。"

log "启动数据库服务：$SVC"
systemctl enable --now "$SVC"

# 等待 socket 可用
for i in $(seq 1 30); do
  if mysqladmin ping >/dev/null 2>&1; then break; fi
  sleep 1
done

# ---- root 密码 ----
ROOT_F="$SECRET_DIR/mysql_root_password"
if [ ! -s "$ROOT_F" ]; then
  gen="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | head -c 24)"
  printf '%s' "$gen" > "$ROOT_F"; chmod 600 "$ROOT_F"
  # Debian/Ubuntu 的 mysql-server 首次 root 常无密码（auth_socket）；设置密码登录
  mysql --defaults-file=<(printf '[client]\nuser=root\n') <<SQL 2>/dev/null || true
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '${gen}';
FLUSH PRIVILEGES;
SQL
  log "已生成 MySQL root 密码（$ROOT_F）"
fi
ROOT_PW="$(cat "$ROOT_F")"

MYSQL_AUTH=(mysql -uroot -p"$ROOT_PW")

# 若旧密码不匹配（比如改过），尝试无密码 root（auth_socket）
if ! "${MYSQL_AUTH[@]}" -e 'SELECT 1' >/dev/null 2>&1; then
  if mysql -uroot <<'SQL' >/dev/null 2>&1; then
SELECT 1;
SQL
    MYSQL_AUTH=(mysql -uroot)
  else
    die "无法以 root 连接 MySQL（密码见 $ROOT_F，但不匹配）。请手工核对后重试。"
  fi
fi

# ---- 应用密码 ----
APP_F="$SECRET_DIR/mysql_app_password"
[ -n "${MYSQL_APP_PASSWORD:-}" ] && printf '%s' "$MYSQL_APP_PASSWORD" > "$APP_F"
APP_PW="$(get_secret mysql_app_password)"

log "建库 $DB 与账号 $APP_U ……"
"${MYSQL_AUTH[@]}" <<SQL
CREATE DATABASE IF NOT EXISTS \`$DB\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$APP_U'@'localhost' IDENTIFIED BY '$APP_PW';
CREATE USER IF NOT EXISTS '$APP_U'@'127.0.0.1' IDENTIFIED BY '$APP_PW';
ALTER USER '$APP_U'@'localhost' IDENTIFIED BY '$APP_PW';
ALTER USER '$APP_U'@'127.0.0.1' IDENTIFIED BY '$APP_PW';
GRANT ALL PRIVILEGES ON \`$DB\`.* TO '$APP_U'@'localhost';
GRANT ALL PRIVILEGES ON \`$DB\`.* TO '$APP_U'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL

log "数据库就绪。使用专用账号 $APP_U（密码 $APP_F），表由后端 AutoMigrate 自动创建。"
