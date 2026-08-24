#!/usr/bin/env bash
set -e

echo "=== Tidy Go Modules ==="
go mod tidy
(cd apps/cli && go mod tidy)
(cd ../plugins/ssh && go mod tidy)
(cd sdk/go && go mod tidy)

mkdir -p bin

if [ "$1" == "--all" ]; then
    echo "=== Cross-Compiling for Windows, Linux, macOS ==="
    
    # Windows
    GOOS=windows GOARCH=amd64 go build -o bin/cliarc.exe ./apps/cli
    GOOS=windows GOARCH=amd64 go build -o bin/cliarc-ssh.exe ../plugins/ssh
    
    # Linux
    GOOS=linux GOARCH=amd64 go build -o bin/cliarc-linux-amd64 ./apps/cli
    GOOS=linux GOARCH=amd64 go build -o bin/cliarc-ssh-linux-amd64 ../plugins/ssh
    GOOS=linux GOARCH=arm64 go build -o bin/cliarc-linux-arm64 ./apps/cli
    GOOS=linux GOARCH=arm64 go build -o bin/cliarc-ssh-linux-arm64 ../plugins/ssh
    
    # macOS
    GOOS=darwin GOARCH=arm64 go build -o bin/cliarc-darwin-arm64 ./apps/cli
    GOOS=darwin GOARCH=arm64 go build -o bin/cliarc-ssh-darwin-arm64 ../plugins/ssh
    GOOS=darwin GOARCH=amd64 go build -o bin/cliarc-darwin-amd64 ./apps/cli
    GOOS=darwin GOARCH=amd64 go build -o bin/cliarc-ssh-darwin-amd64 ../plugins/ssh
    
    # Also default binary for current Unix system
    go build -o bin/cliarc ./apps/cli
    (cd ../plugins/ssh && go build -o ../../cliarc/bin/cliarc-ssh .)
else
    echo "=== Building Local Binary ==="
    go build -o bin/cliarc ./apps/cli
    (cd ../plugins/ssh && go build -o ../../cliarc/bin/cliarc-ssh .)
fi

echo "✓ Build complete in bin/"
