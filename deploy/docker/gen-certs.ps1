# 生成 EMQX 8883 TLS 自签 CA 与服务器证书（Windows PowerShell，需要 openssl）。
# 产物： certs/ca.crt server.crt server.key
# openssl 来源：Git for Windows (C:\Program Files\Git\usr\bin\openssl.exe)、
# MSYS2 (C:\msys64\usr\bin) 或其它已加入 PATH 的 openssl。
# 主机名可用 -Hosts 覆盖，默认：localhost, 127.0.0.1, pannel.wendaoiot.com
param(
    [string[]]$Hosts = @("localhost", "127.0.0.1", "pannel.wendaoiot.com"),
    [int]$CaDays = 3650,
    [int]$ServerDays = 1095
)

$ErrorActionPreference = "Stop"

# 查找 openssl
$openssl = (Get-Command openssl -ErrorAction SilentlyContinue)?.Source
if (-not $openssl) {
    $candidates = @(
        "C:\Program Files\Git\usr\bin\openssl.exe",
        "D:\Program Files\Git\usr\bin\openssl.exe",
        "C:\msys64\usr\bin\openssl.exe",
        "D:\Program Files\msys64\usr\bin\openssl.exe"
    )
    $openssl = $candidates | Where-Object { Test-Path $_ } | Select-Object -First 1
}
if (-not $openssl) { throw "未找到 openssl，请安装 Git for Windows / MSYS2 或把 openssl 加入 PATH。" }
Write-Host "using openssl: $openssl"

$certDir = Join-Path $PSScriptRoot "certs"
New-Item -ItemType Directory -Force -Path $certDir | Out-Null

$entries = foreach ($h in $Hosts) {
    if ($h -match '^\d+\.\d+\.\d+\.\d+$') { "IP:$h" } else { "DNS:$h" }
}
$extFile = Join-Path $certDir "server.ext"
"subjectAltName=" + ($entries -join ",") | Set-Content -Encoding ascii $extFile

try {
    Write-Host "=== [1/3] CA ==="
    & $openssl req -x509 -newkey rsa:2048 -nodes -days $CaDays `
        -keyout "$certDir\ca.key" -out "$certDir\ca.crt" `
        -subj "/CN=WendaoIoT Local CA/O=WendaoIoT/C=CN"

    Write-Host "=== [2/3] server key/csr ==="
    & $openssl req -newkey rsa:2048 -nodes `
        -keyout "$certDir\server.key" -out "$certDir\server.csr" `
        -subj "/CN=pannel.wendaoiot.com/O=WendaoIoT/C=CN"

    Write-Host "=== [3/3] sign server cert ==="
    & $openssl x509 -req -in "$certDir\server.csr" `
        -CA "$certDir\ca.crt" -CAkey "$certDir\ca.key" -CAcreateserial `
        -out "$certDir\server.crt" -days $ServerDays -extfile $extFile
    Remove-Item "$certDir\server.csr" -Force -ErrorAction SilentlyContinue

    & $openssl verify -CAfile "$certDir\ca.crt" "$certDir\server.crt"
    Write-Host "certificates generated in: $certDir"
} finally {
    Remove-Item $extFile -Force -ErrorAction SilentlyContinue
}
