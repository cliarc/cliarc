#!/usr/bin/env bash
# ==============================================================================
# CLIARC Universal Installer
# Compatible with: Linux, macOS, Android (Termux), Windows (WSL/Git-Bash)
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cliarc/cliarc/main/scripts/install.sh | bash
# ==============================================================================

set -e

# Color output helpers
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}"
echo "  ____ _     ___    _    ____   ____ "
echo " / ___| |   |_ _|  / \  |  _ \ / ___|"
echo "| |   | |    | |  / _ \ | |_) | |    "
echo "| |___| |___ | | / ___ \|  _ <| |___ "
echo " \____|_____|___/_/   \_\_| \_\\____|"
echo -e "${NC}"
echo -e "${BOLD}CLIARC Developer Platform — Universal Installer${NC}"
echo -e "--------------------------------------------------------"

# 1. Detect Operating System
OS_TYPE="$(uname -s)"
case "${OS_TYPE}" in
    Linux*)
        if [ -n "${PREFIX}" ] && [ -d "${PREFIX}/bin" ]; then
            TARGET_OS="android"
            OS_DISPLAY="Android (Termux)"
        else
            TARGET_OS="linux"
            OS_DISPLAY="Linux"
        fi
        ;;
    Darwin*)
        TARGET_OS="darwin"
        OS_DISPLAY="macOS"
        ;;
    CYGWIN*|MINGW*|MSYS*)
        TARGET_OS="windows"
        OS_DISPLAY="Windows (POSIX layer)"
        ;;
    *)
        TARGET_OS="linux"
        OS_DISPLAY="Generic Unix (${OS_TYPE})"
        ;;
esac

# 2. Detect Architecture
ARCH_TYPE="$(uname -m)"
case "${ARCH_TYPE}" in
    x86_64|amd64)
        TARGET_ARCH="amd64"
        ;;
    aarch64|arm64)
        TARGET_ARCH="arm64"
        ;;
    armv7l|armv8l|armhf)
        TARGET_ARCH="armv7"
        ;;
    i386|i686)
        TARGET_ARCH="386"
        ;;
    *)
        TARGET_ARCH="amd64"
        ;;
esac

echo -e "Detected Platform: ${GREEN}${OS_DISPLAY} (${TARGET_ARCH})${NC}"

# 3. Determine Installation Directory
if [ "${TARGET_OS}" = "android" ]; then
    INSTALL_DIR="${PREFIX}/bin"
elif [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
else
    INSTALL_DIR="${HOME}/.cliarc/bin"
fi

mkdir -p "${INSTALL_DIR}"
BIN_NAME="cliarc"
if [ "${TARGET_OS}" = "windows" ]; then
    BIN_NAME="cliarc.exe"
fi
TARGET_BIN="${INSTALL_DIR}/${BIN_NAME}"

echo -e "Installation Directory: ${CYAN}${INSTALL_DIR}${NC}"

# 4. Check for existing local build or download release
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo "")"
LOCAL_BIN="${SCRIPT_DIR}/../bin/${BIN_NAME}"

if [ -f "${LOCAL_BIN}" ]; then
    echo -e "Copying local binary from ${LOCAL_BIN}..."
    cp "${LOCAL_BIN}" "${TARGET_BIN}"
    chmod +x "${TARGET_BIN}"
elif command -v go >/dev/null 2>&1; then
    echo -e "Building CLIARC from source using Go..."
    TEMP_BUILD_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'cliarc')"
    git clone --depth 1 https://github.com/cliarc/cliarc.git "${TEMP_BUILD_DIR}" 2>/dev/null || true
    if [ -d "${TEMP_BUILD_DIR}/apps/cli" ]; then
        (cd "${TEMP_BUILD_DIR}/apps/cli" && go build -o "${TARGET_BIN}" .)
    else
        # Direct build if in workspace
        go build -o "${TARGET_BIN}" ./apps/cli 2>/dev/null || go install github.com/cliarc/cliarc/apps/cli@latest
    fi
    rm -rf "${TEMP_BUILD_DIR}" 2>/dev/null || true
    chmod +x "${TARGET_BIN}"
else
    # Download pre-built release binary from GitHub
    RELEASE_URL="https://github.com/cliarc/cliarc/releases/latest/download/cliarc-${TARGET_OS}-${TARGET_ARCH}"
    if [ "${TARGET_OS}" = "windows" ]; then
        RELEASE_URL="${RELEASE_URL}.exe"
    fi
    echo -e "Downloading pre-compiled binary: ${CYAN}${RELEASE_URL}${NC}"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${RELEASE_URL}" -o "${TARGET_BIN}" || true
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "${TARGET_BIN}" "${RELEASE_URL}" || true
    fi
    chmod +x "${TARGET_BIN}" 2>/dev/null || true
fi

# Fallback verification
if [ ! -f "${TARGET_BIN}" ]; then
    echo -e "${RED}Error: Failed to install cliarc binary.${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Installed executable at: ${TARGET_BIN}${NC}"

# 5. Automatically configure Shell PATH environment variables
add_to_profile() {
    local profile_file="$1"
    local export_line="$2"
    if [ -f "${profile_file}" ]; then
        if ! grep -Fq "${INSTALL_DIR}" "${profile_file}"; then
            echo "" >> "${profile_file}"
            echo "# CLIARC CLI Environment Path" >> "${profile_file}"
            echo "${export_line}" >> "${profile_file}"
            echo -e "  → Added PATH to ${CYAN}${profile_file}${NC}"
        fi
    fi
}

PATH_ENTRY="export PATH=\"${INSTALL_DIR}:\$PATH\""

# Only configure shell profiles if install dir is not standard /usr/bin or /usr/local/bin
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
    echo -e "Configuring Shell Environment Profiles..."
    
    # Bash
    [ -f "${HOME}/.bashrc" ] && add_to_profile "${HOME}/.bashrc" "${PATH_ENTRY}"
    [ -f "${HOME}/.bash_profile" ] && add_to_profile "${HOME}/.bash_profile" "${PATH_ENTRY}"
    
    # Zsh
    [ -f "${HOME}/.zshrc" ] && add_to_profile "${HOME}/.zshrc" "${PATH_ENTRY}"
    
    # Generic profile
    [ -f "${HOME}/.profile" ] && add_to_profile "${HOME}/.profile" "${PATH_ENTRY}"
    
    # Fish Shell
    if [ -d "${HOME}/.config/fish" ]; then
        mkdir -p "${HOME}/.config/fish/conf.d"
        echo "set -gx PATH ${INSTALL_DIR} \$PATH" > "${HOME}/.config/fish/conf.d/cliarc.fish"
        echo -e "  → Added Fish path configuration: ${CYAN}${HOME}/.config/fish/conf.d/cliarc.fish${NC}"
    fi
fi

# 6. Verify Installation
echo ""
echo -e "${GREEN}${BOLD}✓ CLIARC installation successful!${NC}"
if "${TARGET_BIN}" version >/dev/null 2>&1; then
    "${TARGET_BIN}" version
fi

echo ""
echo -e "To start using CLIARC immediately in current shell:"
echo -e "  ${YELLOW}export PATH=\"${INSTALL_DIR}:\$PATH\"${NC}"
echo -e "Or restart your terminal session."
