#
# Dagu Installer Script for Windows (PowerShell)
#
# This script downloads and installs the latest version of Dagu.
#
# Usage:
#   irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1 | iex
#
# Or with parameters:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) -Version v1.2.3
#
# Options:
#   -Version <version>      Install a specific version (e.g., -Version v1.2.3)
#   -InstallDir <path>      Install to a custom directory (default: $env:LOCALAPPDATA\dagu)
#
# Examples:
#   # Install latest version to default location
#   irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1 | iex
#
#   # Install specific version
#   .\installer.ps1 -Version v1.2.3
#
#   # Install to custom directory
#   .\installer.ps1 -InstallDir "C:\tools\dagu"
#

param(
    [Parameter(Position = 0)]
    [string]$Version = "",

    [Parameter()]
    [string]$InstallDir = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = 'SilentlyContinue'

# Constants
$RELEASES_URL = "https://github.com/dagu-org/dagu/releases"
$API_URL = "https://api.github.com/repos/dagu-org/dagu/releases/latest"
$FILE_BASENAME = "dagu"

# Default installation directory
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "dagu"
}

# Determine architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ([Environment]::Is64BitProcess) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    } else {
        # Running in 32-bit process on 64-bit OS
        if ($env:PROCESSOR_ARCHITEW6432 -eq "ARM64") { "arm64" } else { "amd64" }
    }
} else {
    # 32-bit OS
    "386"
}

# Get latest version if not specified
if (-not $Version) {
    Write-Output "Fetching latest version..."
    try {
        $release = Invoke-RestMethod -Uri $API_URL -ErrorAction Stop
        $Version = $release.tag_name
    }
    catch {
        Write-Error "Failed to get latest version: $_"
        exit 1
    }
}

if (-not $Version) {
    Write-Error "Failed to determine the Dagu version to install."
    exit 1
}

Write-Output "Installing Dagu version: $Version"

# Create temporary directory
$tempDir = Join-Path $env:TEMP "dagu-installer-$([guid]::NewGuid().ToString('N').Substring(0, 8))"
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

try {
    # Build download URL (remove 'v' prefix from version for filename)
    $versionNumber = $Version -replace '^v', ''
    $tarFileName = "${FILE_BASENAME}_${versionNumber}_windows_${arch}.tar.gz"
    $downloadUrl = "$RELEASES_URL/download/$Version/$tarFileName"
    $tarPath = Join-Path $tempDir $tarFileName

    # Download archive
    Write-Output "Downloading: $downloadUrl"
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tarPath -ErrorAction Stop
    }
    catch {
        Write-Error "Failed to download the release archive: $_"
        exit 1
    }

    # Extract archive using tar (available on Windows 10 1803+)
    Write-Output "Extracting archive..."
    try {
        Push-Location $tempDir
        tar -xzf $tarPath 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "tar extraction failed"
        }
        Pop-Location
    }
    catch {
        Pop-Location -ErrorAction SilentlyContinue
        Write-Error "Failed to extract the archive: $_"
        exit 1
    }

    # Ensure installation directory exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    # Move binary to destination
    $sourcePath = Join-Path $tempDir "dagu.exe"
    $destPath = Join-Path $InstallDir "dagu.exe"

    if (-not (Test-Path $sourcePath)) {
        Write-Error "dagu.exe not found in the extracted archive."
        exit 1
    }

    Write-Output "Installing to: $destPath"
    Move-Item -Path $sourcePath -Destination $destPath -Force

    Write-Output ""
    Write-Output "Dagu $Version has been installed to: $destPath"

    # Check if install directory is in PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-Output ""
        Write-Output "Warning: $InstallDir is not in your PATH."
        Write-Output ""
        Write-Output "To add it to your PATH, run the following command:"
        Write-Output ""
        Write-Output "  `$env:Path += `";$InstallDir`""
        Write-Output "  [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$InstallDir', 'User')"
        Write-Output ""
        Write-Output "Or add it manually via System Properties > Environment Variables."
        Write-Output ""

        # Offer to add to PATH automatically
        $response = Read-Host "Would you like to add Dagu to your PATH automatically? (y/N)"
        if ($response -eq 'y' -or $response -eq 'Y') {
            $newPath = $userPath + ";" + $InstallDir
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            $env:Path = $env:Path + ";" + $InstallDir
            Write-Output "Added $InstallDir to your PATH."
            Write-Output "Please restart your terminal for the changes to take effect."
        }
    }

    Write-Output ""
    Write-Output "Installation complete! Run 'dagu --help' to get started."
}
finally {
    # Cleanup
    if (Test-Path $tempDir) {
        Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
    }
}
