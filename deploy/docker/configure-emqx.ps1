# 配置 EMQX 认证/ACL 链（封装 configure-emqx.py，需要 python）。
$ErrorActionPreference = "Stop"
& python (Join-Path $PSScriptRoot "configure-emqx.py") @args
