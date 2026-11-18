@echo off
setlocal
echo ========================================================
echo Building ToothTray for Windows (GUI subsystem)
echo ========================================================

where rsrc >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Compiling Windows icon resources...
    rsrc -arch amd64 -ico ToothTray.ico -o rsrc_windows_amd64.syso
    rsrc -arch arm64 -ico ToothTray.ico -o rsrc_windows_arm64.syso
)

echo.
echo [1/2] Compiling Windows x64 (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-H windowsgui -s -w" -o ToothTray.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to build x64 binary!
    exit /b 1
)

echo.
echo [2/2] Compiling Windows ARM64...
set GOOS=windows
set GOARCH=arm64
go build -ldflags="-H windowsgui -s -w" -o ToothTray-arm64.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to build ARM64 binary!
    exit /b 1
)

echo.
echo ========================================================
echo [OK] Both binaries built successfully!
echo   - ToothTray.exe        (x64 / amd64)
echo   - ToothTray-arm64.exe  (ARM64)
echo ========================================================
