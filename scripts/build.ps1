# CLIARC PowerShell Build Script
param(
    [switch]$All,
    [switch]$Test
)

$ErrorActionPreference = "Stop"

Write-Host "=== Tidy Go Modules ===" -ForegroundColor Cyan
go mod tidy
Push-Location apps\cli; go mod tidy; Pop-Location
Push-Location ..\plugins\ssh; go mod tidy; Pop-Location
Push-Location sdk\go; go mod tidy; Pop-Location

if (!(Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

if ($All) {
    Write-Host "=== Cross-Compiling for Windows, Linux, macOS ===" -ForegroundColor Cyan
    
    # Windows
    $env:GOOS = "windows"; $env:GOARCH = "amd64"
    go build -o bin\cliarc.exe .\apps\cli
    go build -o bin\cliarc-ssh.exe ..\plugins\ssh
    Write-Host "✓ Windows (amd64): bin\cliarc.exe, bin\cliarc-ssh.exe" -ForegroundColor Green

    # Linux
    $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -o bin\cliarc-linux-amd64 .\apps\cli
    go build -o bin\cliarc-ssh-linux-amd64 ..\plugins\ssh
    $env:GOARCH = "arm64"
    go build -o bin\cliarc-linux-arm64 .\apps\cli
    go build -o bin\cliarc-ssh-linux-arm64 ..\plugins\ssh
    Write-Host "✓ Linux (amd64/arm64): bin\cliarc-linux-*" -ForegroundColor Green

    # macOS
    $env:GOOS = "darwin"; $env:GOARCH = "arm64"
    go build -o bin\cliarc-darwin-arm64 .\apps\cli
    go build -o bin\cliarc-ssh-darwin-arm64 ..\plugins\ssh
    $env:GOARCH = "amd64"
    go build -o bin\cliarc-darwin-amd64 .\apps\cli
    go build -o bin\cliarc-ssh-darwin-amd64 ..\plugins\ssh
    Write-Host "✓ macOS (arm64/amd64): bin\cliarc-darwin-*" -ForegroundColor Green

    Remove-Item Env:GOOS; Remove-Item Env:GOARCH
} else {
    Write-Host "=== Building Local Binaries ===" -ForegroundColor Cyan
    go build -o bin\cliarc.exe .\apps\cli
    Push-Location ..\plugins\ssh
    go build -o ..\..\cliarc\bin\cliarc-ssh.exe .
    Pop-Location
    Write-Host "✓ Built bin\cliarc.exe and bin\cliarc-ssh.exe" -ForegroundColor Green
}

if ($Test) {
    Write-Host "`n=== Running Tests ===" -ForegroundColor Cyan
    go test -v ./tests/...
}
