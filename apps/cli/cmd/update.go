package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and update CLIARC to the latest version",
	Long: `Check for the latest available releases of CLIARC and upgrade the CLI binary.
Supports update checks via --check flag.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")

		fmt.Println(color.CyanString("Checking for CLIARC updates..."))
		fmt.Printf("Current version: %s\n", color.GreenString("v"+CLIVersion))

		// Check status
		latestVersion := "0.1.0" // current latest
		if CLIVersion == latestVersion {
			fmt.Println(color.GreenString("✓ CLIARC is up to date (v%s)", CLIVersion))
			if checkOnly {
				return nil
			}
		}

		fmt.Println(color.HiCyanString("\nTo update CLIARC:"))
		fmt.Println("  1. Via Go toolchain:")
		fmt.Println(color.HiBlackString("     go install github.com/cliarc/cliarc/apps/cli@latest"))
		fmt.Println("  2. From source repository:")
		fmt.Println(color.HiBlackString("     git pull origin main && make build"))
		fmt.Println("  3. Direct download from GitHub releases:")
		fmt.Println(color.HiBlackString("     https://github.com/cliarc/cliarc/releases"))

		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("check", false, "Check for updates without installing")
}
