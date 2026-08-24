# ==============================================================================
# CLIARC Windows PowerShell Universal Installer
# Usage:
#   irm https://raw.githubusercontent.com/cliarc/cliarc/main/scripts/install.ps1 | iex
# ==============================================================================

$ErrorActionPreference = "Stop"

Write-Host @"
  ____ _     ___    _    ____   ____ 
 / ___| |   |_ _|  / \  |  _ \ / ___|
| |   | |    | |  / _ \ | |_) | |    
| |___| |___ | | / ___ \|  _ <| |___ 
 \____|_____|___/_/   \_\_| \_\____|
"@ -ForegroundColor Cyan

Write-Host "CLIARC Developer Platform — Windows Installer" -ForegroundColor Green
Write-Host "--------------------------------------------------------"

$InstallDir = Join-Path $HOME ".cliarc\bin"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$TargetExe = Join-Path $InstallDir "cliarc.exe"

# 1. Determine Source
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path -ErrorAction SilentlyContinue
$LocalBin = ""
if ($ScriptDir) {
    $LocalBin = Join-Path $ScriptDir "..\bin\cliarc.exe"
}

if ($LocalBin -and (Test-Path $LocalBin)) {
    Write-Host "Installing from local binary: $LocalBin..." -ForegroundColor Cyan
    Copy-Item $LocalBin $TargetExe -Force
} elseif (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host "Building CLIARC from Go source..." -ForegroundColor Cyan
    $TempDir = Join-Path $env:TEMP ("cliarc-install-" + [System.Guid]::NewGuid().ToString().Substring(0,8))
    try {
        git clone --depth 1 https://github.com/cliarc/cliarc.git $TempDir 2>$null
        Push-Location "$TempDir\apps\cli"
        go build -o $TargetExe .
        Pop-Location
    } catch {
        # Fallback local build if in repo root
        if (Test-Path ".\apps\cli") {
            go build -o $TargetExe .\apps\cli
        }
    } finally {
        if (Test-Path $TempDir) {
            Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
        }
    }
} else {
    Write-Host "Downloading pre-compiled CLIARC release for Windows (amd64)..." -ForegroundColor Cyan
    $DownloadUrl = "https://github.com/cliarc/cliarc/releases/latest/download/cliarc-windows-amd64.exe"
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TargetExe -UseBasicParsing
}

if (!(Test-Path $TargetExe)) {
    Write-Error "Failed to install CLIARC executable."
    exit 1
}

Write-Host "✓ Installed executable at: $TargetExe" -ForegroundColor Green

# 2. Also place in instant WindowsApps/cmd_alias directory if present for immediate non-restart availability
$InstantDirs = @(
    "$env:LOCALAPPDATA\Microsoft\WindowsApps",
    "C:\cmd_alias"
)
foreach ($dir in $InstantDirs) {
    if (Test-Path $dir) {
        try {
            Copy-Item $TargetExe (Join-Path $dir "cliarc.exe") -Force -ErrorAction SilentlyContinue
            Write-Host "✓ Instant terminal access registered at: $dir\cliarc.exe" -ForegroundColor Green
        } catch {}
    }
}

# 3. Permanently configure Windows User PATH Environment Variable
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", [System.EnvironmentVariableTarget]::User)
if ($UserPath -notlike "*$InstallDir*") {
    $NewUserPath = "$InstallDir;$UserPath"
    [System.Environment]::SetEnvironmentVariable("Path", $NewUserPath, [System.EnvironmentVariableTarget]::User)
    Write-Host "✓ Added $InstallDir to Windows User PATH environment variable." -ForegroundColor Green
}

# 4. Update Current PowerShell Process PATH
if ($env:Path -notlike "*$InstallDir*") {
    $env:Path = "$InstallDir;$env:Path"
}

Write-Host "`n✓ CLIARC installation completed successfully!" -ForegroundColor Green
Write-Host "Run 'cliarc' in any terminal to get started:`n" -ForegroundColor Cyan

& "$TargetExe" version
