# PowerShell equivalent of 'make build' (ui + bin)
# Builds the complete Dagu application on Windows

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

$BinDir = Join-Path $RootDir ".local\bin"
$AppName = "dagu"

# Get version info
Push-Location $RootDir
try {
    $BuildVersion = git describe --tags 2>$null
    if (-not $BuildVersion) {
        $BuildVersion = "dev"
    }
} finally {
    Pop-Location
}

$Date = Get-Date -Format "yyMMddHHmmss"
$LDFlags = "-X 'main.version=$BuildVersion-$Date'"

# Step 1: Build UI
Write-Host "Building UI..." -ForegroundColor Green
& "$ScriptDir\build-ui.ps1"
if ($LASTEXITCODE -ne 0) { throw "UI build failed" }

# Step 2: Build binary
Write-Host "Building the binary..." -ForegroundColor Green
if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

Push-Location $RootDir
try {
    go build -ldflags="$LDFlags" -o "$BinDir\$AppName.exe" ./cmd
    if ($LASTEXITCODE -ne 0) { throw "Go build failed" }
} finally {
    Pop-Location
}

Write-Host "Build complete! Binary at: $BinDir\$AppName.exe" -ForegroundColor Green
