# CLIARC Cross-Platform Makefile

BINARY_NAME=cliarc
BIN_DIR=bin
CLI_SRC=./apps/cli
SSH_PLUGIN_SRC=../plugins/ssh

.PHONY: all build build-all clean test tidy

all: build

## Local OS Build (bin/cliarc.exe on Windows, bin/cliarc on macOS/Linux)
build: tidy
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CLI_SRC)
	cd $(SSH_PLUGIN_SRC) && go build -o ../../cliarc/$(BIN_DIR)/cliarc-ssh .

## Cross-compile for Windows, Linux, and macOS (Intel & ARM64)
build-all: tidy
	@mkdir -p $(BIN_DIR)
	# Windows (x86_64)
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CLI_SRC)
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/cliarc-ssh-windows-amd64.exe $(SSH_PLUGIN_SRC)
	
	# Linux (x86_64 & ARM64)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(CLI_SRC)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/cliarc-ssh-linux-amd64 $(SSH_PLUGIN_SRC)
	GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(CLI_SRC)
	GOOS=linux GOARCH=arm64 go build -o $(BIN_DIR)/cliarc-ssh-linux-arm64 $(SSH_PLUGIN_SRC)
	
	# macOS (Apple Silicon & Intel)
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(CLI_SRC)
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/cliarc-ssh-darwin-arm64 $(SSH_PLUGIN_SRC)
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(CLI_SRC)
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/cliarc-ssh-darwin-amd64 $(SSH_PLUGIN_SRC)

tidy:
	go mod tidy
	cd apps/cli && go mod tidy
	cd ../plugins/ssh && go mod tidy
	cd sdk/go && go mod tidy

test:
	go test -v ./tests/...

clean:
	rm -rf $(BIN_DIR)
