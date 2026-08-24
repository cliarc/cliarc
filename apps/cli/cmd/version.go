package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Version information constants
const (
	CLIVersion      = "0.1.0"
	ProtocolVersion = "1"
	BuildDate       = "2026-08-24"
	CommitHash      = "release-v0.1.0"
)

type VersionInfo struct {
	CLIARC          string `json:"cliarc"`
	ProtocolVersion string `json:"protocol_version"`
	GoVersion       string `json:"go_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	BuildDate       string `json:"build_date"`
	Commit          string `json:"commit"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print CLIARC version and build information",
	Long:  `Display version, protocol version, compiler, and target architecture for CLIARC.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		shortFlag, _ := cmd.Flags().GetBool("short")
		jsonFlag, _ := cmd.Flags().GetBool("json")

		info := VersionInfo{
			CLIARC:          CLIVersion,
			ProtocolVersion: ProtocolVersion,
			GoVersion:       runtime.Version(),
			OS:              runtime.GOOS,
			Arch:            runtime.GOARCH,
			BuildDate:       BuildDate,
			Commit:          CommitHash,
		}

		if shortFlag {
			fmt.Printf("cliarc v%s\n", CLIVersion)
			return nil
		}

		if jsonFlag {
			data, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Println(color.CyanString("CLIARC Developer Platform"))
		fmt.Printf("  • Version:          %s\n", color.GreenString("v"+info.CLIARC))
		fmt.Printf("  • Protocol Version: %s\n", info.ProtocolVersion)
		fmt.Printf("  • Go Version:       %s\n", info.GoVersion)
		fmt.Printf("  • OS / Arch:        %s/%s\n", info.OS, info.Arch)
		fmt.Printf("  • Build Date:       %s\n", info.BuildDate)
		fmt.Printf("  • Commit:           %s\n", color.HiBlackString(info.Commit))
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolP("short", "s", false, "Print only version number")
	versionCmd.Flags().BoolP("json", "j", false, "Output version information in JSON")
}
