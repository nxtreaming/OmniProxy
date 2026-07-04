param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [string]$AssetDirectory = "."
)

$ErrorActionPreference = "Stop"

function Resolve-AssetPath {
    param([string]$Name)

    $matches = Get-ChildItem -LiteralPath $AssetDirectory -Recurse -File -Filter $Name
    if ($matches.Count -eq 0) {
        throw "Missing release asset: $Name"
    }
    if ($matches.Count -gt 1) {
        throw "Duplicate release asset: $Name"
    }
    return $matches[0].FullName
}

$expectedAssets = @(
    "OmniProxy-Setup-$Version-windows-amd64.exe",
    "OmniProxy-Setup-$Version-windows-amd64.exe.sha256",
    "OmniProxy-$Version-darwin-universal-unsigned.dmg",
    "OmniProxy-$Version-darwin-universal-unsigned.dmg.sha256"
)

foreach ($assetName in $expectedAssets) {
    Resolve-AssetPath -Name $assetName | Out-Null
}
$releaseNotes = Get-ChildItem -LiteralPath $AssetDirectory -Recurse -File -Filter "release-notes.md"
if ($releaseNotes.Count -ne 1) {
    throw "Expected exactly one release-notes.md, found $($releaseNotes.Count)"
}

$allowedNames = @{}
foreach ($assetName in $expectedAssets) {
    $allowedNames[$assetName] = $true
}
$allowedNames["release-notes.md"] = $true

Get-ChildItem -LiteralPath $AssetDirectory -Recurse -File | ForEach-Object {
    if (-not $allowedNames.ContainsKey($_.Name)) {
        throw "Unexpected release asset: $($_.Name)"
    }
}

foreach ($checksumName in $expectedAssets | Where-Object { $_.EndsWith(".sha256") }) {
    $checksumPath = Resolve-AssetPath -Name $checksumName
    $targetName = $checksumName.Substring(0, $checksumName.Length - ".sha256".Length)
    $targetPath = Resolve-AssetPath -Name $targetName

    $content = (Get-Content -LiteralPath $checksumPath -Raw).Trim()
    if ($content -notmatch "^(?<hash>[a-fA-F0-9]{64})(\s+.+)?$") {
        throw "Invalid SHA256 file format: $checksumName"
    }

    $expectedHash = $Matches.hash.ToLowerInvariant()
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $targetPath).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        throw "Checksum mismatch for $targetName"
    }
}

Write-Host "Release assets verified for $Version"
