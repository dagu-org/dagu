# PowerShell equivalent of 'make ui'
# Builds the frontend UI for Dagu on Windows

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

$UIDir = Join-Path $RootDir "ui"
$FEAssetsDir = Join-Path $RootDir "internal\service\frontend\assets"
$FEBuildDir = Join-Path $UIDir "dist"

Write-Host "Building UI..." -ForegroundColor Green

# Step 1: Clean UI (equivalent to clean-ui target)
Write-Host "Cleaning UI build cache..." -ForegroundColor Green
Push-Location $UIDir
try {
    if (Test-Path "node_modules") {
        Remove-Item -Recurse -Force "node_modules"
    }
    if (Test-Path ".cache") {
        Remove-Item -Recurse -Force ".cache"
    }
} finally {
    Pop-Location
}

# Step 2: Build UI (equivalent to build-ui target)
Write-Host "Installing dependencies and building UI..." -ForegroundColor Green
Push-Location $UIDir
try {
    pnpm install
    if ($LASTEXITCODE -ne 0) { throw "pnpm install failed" }

    $env:NODE_OPTIONS = "--max-old-space-size=8192"
    pnpm webpack --config webpack.dev.js --progress --color
    if ($LASTEXITCODE -ne 0) { throw "webpack dev build failed" }

    pnpm webpack --config webpack.prod.js --progress --color
    if ($LASTEXITCODE -ne 0) { throw "webpack prod build failed" }
} finally {
    Pop-Location
}

Write-Host "Waiting for the build to finish..." -ForegroundColor Green
Start-Sleep -Seconds 3

# Step 3: Copy assets (equivalent to cp-assets target)
Write-Host "Copying UI assets..." -ForegroundColor Green
if (Test-Path $FEAssetsDir) {
    Get-ChildItem -Path $FEAssetsDir -File | Remove-Item -Force
}
if (-not (Test-Path $FEAssetsDir)) {
    New-Item -ItemType Directory -Path $FEAssetsDir | Out-Null
}
Copy-Item -Path (Join-Path $FEBuildDir "*") -Destination $FEAssetsDir -Force

Write-Host "UI build complete!" -ForegroundColor Green
