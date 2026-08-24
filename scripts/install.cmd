@echo off
rem ==============================================================================
rem CLIARC Windows CMD Installer
rem ==============================================================================

setlocal enabledelayedexpansion

echo   ____ _     ___    _    ____   ____ 
echo  / ___^| ^|   ^|_ _^|  / \  ^|  _ \ / ___^|
echo ^| ^|   ^| ^|    ^| ^|  / _ \ ^| ^|_) ^| ^|    
echo ^| ^|___^| ^|___ ^| ^| / ___ \^|  _ ^<^| ^|___ 
echo  \____^|_____^|___/_/   \_\_^| \_\\____^|
echo.
echo CLIARC Developer Platform — Windows Installer
echo --------------------------------------------------------

set "INSTALL_DIR=%USERPROFILE%\.cliarc\bin"
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

set "TARGET_EXE=%INSTALL_DIR%\cliarc.exe"
set "LOCAL_EXE=%~dp0..\bin\cliarc.exe"

if exist "%LOCAL_EXE%" (
    echo Copying local binary from %LOCAL_EXE%...
    copy /y "%LOCAL_EXE%" "%TARGET_EXE%" >nul
) else (
    echo Downloading CLIARC Windows binary...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri 'https://github.com/cliarc/cliarc/releases/latest/download/cliarc-windows-amd64.exe' -OutFile '%TARGET_EXE%' -UseBasicParsing"
)

if not exist "%TARGET_EXE%" (
    echo Error: Failed to install cliarc.exe
    exit /b 1
)

echo [OK] Installed at %TARGET_EXE%

rem Copy to WindowsApps for instant execution
if exist "%LOCALAPPDATA%\Microsoft\WindowsApps" (
    copy /y "%TARGET_EXE%" "%LOCALAPPDATA%\Microsoft\WindowsApps\cliarc.exe" >nul 2>&1
    echo [OK] Instant access registered in WindowsApps
)

rem Add to User PATH
powershell -NoProfile -ExecutionPolicy Bypass -Command "$u=[System.Environment]::GetEnvironmentVariable('Path','User'); if ($u -notlike '*%INSTALL_DIR%*') { [System.Environment]::SetEnvironmentVariable('Path', '%INSTALL_DIR%;' + $u, 'User') }"

echo.
echo [SUCCESS] CLIARC installed successfully!
echo Run 'cliarc version' to verify.
echo.
"%TARGET_EXE%" version
