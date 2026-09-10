# Dev 启动脚本（加固版，规避 6 类 PS 终端故障，见 memory/ps-terminal-pitfalls.md）
# 关键改动：
#   - $ErrorActionPreference = 'Stop'  → 任一失败立即退出（#6 错误被吞）
#   - Start-Process 用 -WorkingDirectory 替代 inline `cd ...;`  → 不再触发 -Command 中含分号/引号的边界问题（#2 #3）
#   - go/npm 启动走独立 .ps1 文件 → 内层 shell 不再 inline parse 任何 $ 变量（#3）
# 前置条件: MySQL 运行中，已创建 wendaoiot 数据库；EMQX/Mosquitto 运行中

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host "=== Wendao IoT 开发环境启动 ===" -ForegroundColor Cyan
Write-Host ""

$root = $PSScriptRoot

# 准备内层子脚本（绝对路径，不靠外层变量传递）
$serverScript = Join-Path $root 'tools/local/_run_server.ps1'
$adminScript  = Join-Path $root 'tools/local/_run_admin.ps1'

# 两个内层脚本各自只做一件事，文件内不使用任何外层 $ 变量
@(
    @{ path = $serverScript; body = "Set-Location -LiteralPath '$root\server'; go run ./cmd/server/" },
    @{ path = $adminScript;  body = "Set-Location -LiteralPath '$root\web-admin'; npm run dev" }
) | ForEach-Object {
    $dir = Split-Path -Parent $_.path
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    # 用 utf8NoBOM 写文件，避免 #5 BOM 问题
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($_.path, $_.body, $utf8NoBom)
}

# 启动后端
Write-Host "[1/2] 启动 Go 后端..." -ForegroundColor Green
Start-Process powershell `
    -ArgumentList @('-NoExit', '-File', $serverScript) `
    -WorkingDirectory (Join-Path $root 'server') `
    -WindowStyle Minimized

Start-Sleep -Seconds 3

# 启动管理后台
Write-Host "[2/2] 启动管理后台 (端口 3000)..." -ForegroundColor Green
Start-Process powershell `
    -ArgumentList @('-NoExit', '-File', $adminScript) `
    -WorkingDirectory (Join-Path $root 'web-admin') `
    -WindowStyle Minimized

Write-Host ""
Write-Host "=== 启动完成 ===" -ForegroundColor Cyan
Write-Host "管理后台: http://localhost:3000" -ForegroundColor Yellow
Write-Host "登录账号: admin / admin123 (超管)   tenant1 / 123456 (租户)" -ForegroundColor Yellow
Write-Host ""
Write-Host "按任意键关闭所有服务窗口..." -ForegroundColor Gray
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
