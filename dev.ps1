# Dev 启动脚本
# 前置条件: MySQL 运行中，已创建 wendaoiot 数据库；EMQX/Mosquitto 运行中

Write-Host "=== Wendao IoT 开发环境启动 ===" -ForegroundColor Cyan
Write-Host ""

# 启动后端
Write-Host "[1/3] 启动 Go 后端..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\server'; go run ./cmd/server/" -WindowStyle Minimized

Start-Sleep -Seconds 3

# 启动管理后台
Write-Host "[2/3] 启动管理后台 (端口 3000)..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\web-admin'; npm run dev" -WindowStyle Minimized

# 启动设备模拟器
Write-Host "[3/3] 启动设备模拟器..." -ForegroundColor Green
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$PSScriptRoot\tools\simulator'; go run . -id ESP32-001" -WindowStyle Minimized

Write-Host ""
Write-Host "=== 启动完成 ===" -ForegroundColor Cyan
Write-Host "管理后台: http://localhost:3000" -ForegroundColor Yellow
Write-Host "登录账号: admin / admin123 (超管)   tenant1 / 123456 (租户)" -ForegroundColor Yellow
Write-Host ""
Write-Host "按任意键关闭所有服务窗口..." -ForegroundColor Gray
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
