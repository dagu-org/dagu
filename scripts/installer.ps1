#
# Dagu Installer Script for Windows (PowerShell)
#
# This script downloads and installs the latest version of Dagu.
# Default install location: %LOCALAPPDATA%\Programs\dagu
#
# Usage:
#   irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1 | iex
#
# Or with version:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) v1.2.3
#
# Or with version and install directory:
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) v1.2.3 "C:\tools\dagu"
#
# Examples:
#   # Install latest version to default location (%LOCALAPPDATA%\Programs\dagu)
#   irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1 | iex
#
#   # Install specific version
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) v1.2.3
#
#   # Install to custom directory
#   & ([scriptblock]::Create((irm https://raw.githubusercontent.com/dagu-org/dagu/main/scripts/installer.ps1))) latest "C:\tools\dagu"
#
#   # Run locally
#   .\installer.ps1
#   .\installer.ps1 v1.2.3
#   .\installer.ps1 v1.2.3 "C:\tools\dagu"
#

param(
    [Parameter(Position = 0)]
    [string]$Version = "",

    [Parameter(Position = 1)]
    [string]$InstallDir = ""
)

function Install-Dagu {
    param(
        [Parameter(Position = 0)]
        [string]$Version = "",

        [Parameter(Position = 1)]
        [string]$InstallDir = ""
    )

    Set-StrictMode -Version Latest
    $ErrorActionPreference = "Stop"
    $ProgressPreference = 'SilentlyContinue'

    # Ensure TLS 1.2 is used (required for GitHub API)
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    # Constants
    $RELEASES_URL = "https://github.com/dagu-org/dagu/releases"
    $API_URL = "https://api.github.com/repos/dagu-org/dagu/releases/latest"
    $FILE_BASENAME = "dagu"

    # Default installation directory
    if (-not $InstallDir) {
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\dagu"
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

    # Get latest version if not specified or if "latest" is specified
    if (-not $Version -or $Version -eq "latest") {
        Write-Output "Fetching latest version..."
        try {
            $release = Invoke-RestMethod -Uri $API_URL -ErrorAction Stop
            $Version = $release.tag_name
        }
        catch {
            Write-Error "Failed to get latest version: $_"
            return
        }
    }

    # Ensure version has 'v' prefix for the download URL
    if ($Version -and $Version -notmatch '^v') {
        $Version = "v$Version"
    }

    if (-not $Version) {
        Write-Error "Failed to determine the Dagu version to install."
        return
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
            return
        }

        # Extract archive using tar (available on Windows 10 1803+)
        Write-Output "Extracting archive..."
        try {
            $originalDir = Get-Location
            Set-Location $tempDir
            $tarOutput = & tar -xzf $tarFileName 2>&1
            Set-Location $originalDir
            if ($LASTEXITCODE -ne 0) {
                throw "tar extraction failed: $tarOutput"
            }
        }
        catch {
            Set-Location $originalDir -ErrorAction SilentlyContinue
            Write-Error "Failed to extract the archive: $_"
            return
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
            return
        }

        Write-Output "Installing to: $destPath"
        Move-Item -Path $sourcePath -Destination $destPath -Force

        Write-Output ""
        Write-Output "Dagu $Version has been installed to: $destPath"

        # Check if install directory is in PATH
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$InstallDir*") {
            # Add to PATH automatically (persistent for new terminals)
            $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
            Write-Output ""
            Write-Output "Added $InstallDir to your PATH."
        }

        # Update current session PATH
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Process")
        if ($currentPath -notlike "*$InstallDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "Process")
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
}

# Run the installer
Install-Dagu $Version $InstallDir
