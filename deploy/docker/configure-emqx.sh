#!/usr/bin/env bash
# 配置 EMQX 认证/ACL 链（封装 configure-emqx.py，需要 python3）。
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "$DIR/configure-emqx.py" "$@"
