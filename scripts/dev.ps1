$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root "OmniProxyBackend"
$wails = Get-Command "wails" -ErrorAction SilentlyContinue

if ($null -eq $wails) {
    $fallback = Join-Path $env:USERPROFILE "go\bin\wails.exe"
    if (-not (Test-Path $fallback)) {
        throw "Wails CLI not found. Install it or add wails.exe to PATH."
    }
    $wailsPath = $fallback
} else {
    $wailsPath = $wails.Source
}

Set-Location $backend
Write-Host "Starting OmniProxy Wails dev server..."
& $wailsPath dev
