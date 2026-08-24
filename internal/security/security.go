package security

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

// CalculateSHA256 returns the hexadecimal SHA256 hash of a file.
func CalculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum checks if the file matches the expected SHA256 checksum.
func VerifyChecksum(filePath, expectedChecksum string) (bool, error) {
	if expectedChecksum == "" {
		return true, nil
	}
	actual, err := CalculateSHA256(filePath)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(actual, expectedChecksum), nil
}

// ConfirmPermissions prompts the user interactively to approve requested permissions.
func ConfirmPermissions(pluginName string, permissions []string, autoApprove bool) (bool, error) {
	if len(permissions) == 0 || autoApprove {
		return true, nil
	}

	fmt.Println(color.YellowString("\n⚠️  Security Alert: Plugin %q requests the following capabilities:", pluginName))
	for _, p := range permissions {
		desc := describePermission(p)
		fmt.Printf("  • %-22s %s\n", color.CyanString(p), color.HiBlackString("(%s)", desc))
	}

	fmt.Print(color.HiWhiteString("\nDo you want to grant these permissions and install this plugin? [y/N]: "))
	reader := bufio.NewReader(os.Stdin)
	ans, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	trimmed := strings.ToLower(strings.TrimSpace(ans))
	return trimmed == "y" || trimmed == "yes", nil
}

func describePermission(perm string) string {
	switch perm {
	case "filesystem:read":
		return "Read access to filesystem files and directories"
	case "filesystem:write":
		return "Modify, create, and remove files on disk"
	case "process:exec":
		return "Execute local system commands and subprocesses"
	case "network:outbound":
		return "Open outbound TCP/HTTP network connections"
	case "secrets:access":
		return "Access stored secrets, credentials, and tokens"
	case "env:read":
		return "Read system environment variables"
	default:
		return "Plugin capability"
	}
}
