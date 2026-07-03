param(
    [string]$Version = "dev",
    [string]$OutputName = "OmniProxy-Dev.exe",
    [switch]$Clean,
    [switch]$Restart
)

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

$buildArgs = @(
    "build",
    "-nopackage",
    "-tags",
    "omniproxy_dev",
    "-o",
    $OutputName,
    "-ldflags",
    "-X main.appVersion=$Version"
)

if ($Clean) {
    $buildArgs = @("build", "-clean") + $buildArgs[1..($buildArgs.Count - 1)]
}

Set-Location $backend
Write-Host "Building OmniProxy Dev executable..."
Write-Host "Version: $Version"
Write-Host "Output: build\bin\$OutputName"
& $wailsPath @buildArgs
if ($LASTEXITCODE -ne 0) {
    throw "Wails dev build failed with exit code $LASTEXITCODE."
}

$outputPath = Join-Path $backend "build\bin\$OutputName"
if (-not (Test-Path $outputPath)) {
    throw "Expected output was not found: $outputPath"
}

Write-Host "Built dev executable: $outputPath"
Write-Host "Dev profile uses separate data and ports: .omniproxy-dev, 127.0.0.1:3001, control 127.0.0.1:3891"

if ($Restart) {
    $processName = [System.IO.Path]::GetFileNameWithoutExtension($OutputName)
    $running = Get-Process -Name $processName -ErrorAction SilentlyContinue | Where-Object {
        try {
            $_.Path -eq $outputPath
        } catch {
            $false
        }
    }

    foreach ($process in $running) {
        Write-Host "Stopping existing dev process $($process.Id)..."
        Stop-Process -Id $process.Id -Force
        Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
    }

    Write-Host "Starting dev executable..."
    Start-Process -FilePath $outputPath -WorkingDirectory (Split-Path -Parent $outputPath) -WindowStyle Hidden
}
