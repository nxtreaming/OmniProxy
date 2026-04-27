$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root "OmniProxyBackend"
$frontend = Join-Path $root "omniproxyfrontend"
$cache = Join-Path $root ".gocache"

New-Item -ItemType Directory -Force $cache | Out-Null

$backendCommand = "`$env:GOCACHE='$cache'; Set-Location '$backend'; go run ."
$backendProcess = Start-Process -WindowStyle Hidden -FilePath "powershell.exe" -ArgumentList "-NoProfile", "-Command", $backendCommand -PassThru
$frontendProcess = Start-Process -WindowStyle Hidden -FilePath "cmd.exe" -ArgumentList "/c npm run dev -- --host 127.0.0.1" -WorkingDirectory $frontend -PassThru

Write-Host "OmniProxy backend launcher PID: $($backendProcess.Id)"
Write-Host "OmniProxy frontend launcher PID: $($frontendProcess.Id)"
Write-Host "Frontend: http://127.0.0.1:5173"
Write-Host "Control API: http://127.0.0.1:3890/api"
Write-Host "Proxy: http://127.0.0.1:3000"
