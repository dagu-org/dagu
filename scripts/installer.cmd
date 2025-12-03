@echo off
setlocal enabledelayedexpansion

REM Dagu Installer Script for Windows (CMD)
REM
REM This script downloads and installs the latest version of Dagu.
REM For environments where PowerShell is not available.
REM Default install location: %LOCALAPPDATA%\Programs\dagu
REM
REM Usage:
REM   installer.cmd [VERSION] [INSTALL_DIR]
REM
REM Examples:
REM   installer.cmd
REM   installer.cmd v1.2.3
REM   installer.cmd 1.2.3
REM   installer.cmd latest "C:\tools\dagu"
REM   installer.cmd v1.2.3 "C:\tools\dagu"

REM Default values
set "VERSION=%~1"
set "INSTALL_DIR=%~2"

REM Set default install directory if not specified
if "!INSTALL_DIR!"=="" set "INSTALL_DIR=%LOCALAPPDATA%\Programs\dagu"

REM Handle "latest" keyword
if /i "!VERSION!"=="latest" set "VERSION="

REM Add 'v' prefix if version specified without it
if not "!VERSION!"=="" (
    if not "!VERSION:~0,1!"=="v" set "VERSION=v!VERSION!"
)

REM Determine architecture
set "ARCH=386"
if /i "%PROCESSOR_ARCHITECTURE%"=="AMD64" set "ARCH=amd64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARCH=arm64"
if /i "%PROCESSOR_ARCHITEW6432%"=="AMD64" set "ARCH=amd64"
if /i "%PROCESSOR_ARCHITEW6432%"=="ARM64" set "ARCH=arm64"

REM Check for curl availability
curl --version >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo curl is required but not available. >&2
    echo Please install curl or use the PowerShell installer instead. >&2
    exit /b 1
)

REM Create temporary directory
set "TEMP_DIR=%TEMP%\dagu-installer-%RANDOM%"
mkdir "!TEMP_DIR!" >nul 2>&1

REM Get latest version if not specified
if "!VERSION!"=="" (
    echo Fetching latest version...
    curl -fsSL "https://api.github.com/repos/dagu-org/dagu/releases/latest" -o "!TEMP_DIR!\release.json" >nul 2>&1
    if !ERRORLEVEL! neq 0 (
        echo Failed to fetch latest version. >&2
        rd /s /q "!TEMP_DIR!" >nul 2>&1
        exit /b 1
    )

    REM Extract tag_name from JSON (simple parsing)
    for /f "tokens=2 delims=:," %%a in ('findstr /c:"\"tag_name\"" "!TEMP_DIR!\release.json"') do (
        set "VERSION=%%~a"
        set "VERSION=!VERSION: =!"
        set "VERSION=!VERSION:"=!"
    )
    del "!TEMP_DIR!\release.json" >nul 2>&1
)

if "!VERSION!"=="" (
    echo Failed to determine the Dagu version to install. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

echo Installing Dagu version: !VERSION!

REM Remove 'v' prefix from version for filename
set "VERSION_NUM=!VERSION!"
if "!VERSION_NUM:~0,1!"=="v" set "VERSION_NUM=!VERSION_NUM:~1!"

REM Build download URL
set "TAR_FILE=dagu_!VERSION_NUM!_windows_!ARCH!.tar.gz"
set "DOWNLOAD_URL=https://github.com/dagu-org/dagu/releases/download/!VERSION!/!TAR_FILE!"
set "TAR_PATH=!TEMP_DIR!\!TAR_FILE!"

REM Download archive
echo Downloading: !DOWNLOAD_URL!
curl -fsSL "!DOWNLOAD_URL!" -o "!TAR_PATH!"
if !ERRORLEVEL! neq 0 (
    echo Failed to download the release archive. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

REM Check for tar availability (Windows 10 1803+)
tar --version >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo tar is required but not available. >&2
    echo Please use Windows 10 version 1803 or later, or use the PowerShell installer. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

REM Extract archive using tar
echo Extracting archive...
pushd "!TEMP_DIR!"
tar -xzf "!TAR_FILE!"
set "TAR_ERROR=!ERRORLEVEL!"
popd
if !TAR_ERROR! neq 0 (
    echo Failed to extract the archive. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

REM Ensure installation directory exists
if not exist "!INSTALL_DIR!" mkdir "!INSTALL_DIR!"

REM Check if dagu.exe exists in temp directory
if not exist "!TEMP_DIR!\dagu.exe" (
    echo dagu.exe not found in the extracted archive. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

REM Move binary to destination
set "DEST_PATH=!INSTALL_DIR!\dagu.exe"
echo Installing to: !DEST_PATH!
move /y "!TEMP_DIR!\dagu.exe" "!DEST_PATH!" >nul
if !ERRORLEVEL! neq 0 (
    echo Failed to install dagu.exe. >&2
    rd /s /q "!TEMP_DIR!" >nul 2>&1
    exit /b 1
)

REM Cleanup
rd /s /q "!TEMP_DIR!" >nul 2>&1

echo.
echo Dagu !VERSION! has been installed to: !DEST_PATH!

REM Check if install directory is in PATH
set "USER_PATH="
for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v Path 2^>nul') do set "USER_PATH=%%b"
echo !USER_PATH! | findstr /i /c:"!INSTALL_DIR!" >nul
if !ERRORLEVEL! neq 0 (
    echo.
    echo To use dagu from any terminal, add the install directory to your PATH:
    echo.
    echo   set PATH=%%PATH%%;!INSTALL_DIR!
    echo.
    echo Or add it permanently via System Properties ^> Environment Variables.
)

echo.
echo Installation complete^^! Run 'dagu --help' to get started.
exit /b 0
