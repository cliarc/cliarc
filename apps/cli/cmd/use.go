package cmd

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use [server-name]",
	Short: "Set or show the active server for CLIARC commands",
	Long: `Set or display the currently active server.
Once an active server is set, server management commands will target it automatically.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		clearFlag, _ := cmd.Flags().GetBool("clear")
		listFlag, _ := cmd.Flags().GetBool("list")

		if clearFlag {
			if err := clearActiveServerName(); err != nil {
				return err
			}
			fmt.Println(color.GreenString("✓ Active server cleared."))
			return nil
		}

		servers, err := loadServers()
		if err != nil {
			return err
		}

		active, _ := getActiveServerName()

		// If no argument provided or --list flag passed
		if len(args) == 0 || listFlag {
			if len(servers) == 0 {
				fmt.Println("No servers configured. Add one with 'cliarc server add'.")
				return nil
			}

			fmt.Println(color.CyanString("Configured Servers:"))
			for _, s := range servers {
				isActive := strings.EqualFold(s.Name, active) || strings.EqualFold(s.ID, active)
				marker := "  "
				status := ""
				if isActive {
					marker = color.GreenString("► ")
					status = color.GreenString(" (active)")
				}
				keyInfo := ""
				if s.SSHKeyID != "" {
					keyInfo = fmt.Sprintf(" [key: %s]", s.SSHKeyID)
				}
				fmt.Printf("%s%-14s %s@%s:%d%s%s\n", marker, s.Name, s.Username, s.Host, s.Port, keyInfo, status)
			}

			if active == "" {
				fmt.Println("\nNo active server set. Use 'cliarc use <server-name>' to select one.")
			}
			return nil
		}

		targetName := args[0]
		if strings.EqualFold(targetName, "clear") || strings.EqualFold(targetName, "none") {
			if err := clearActiveServerName(); err != nil {
				return err
			}
			fmt.Println(color.GreenString("✓ Active server cleared."))
			return nil
		}

		// Find the requested server
		srv, err := findServer(targetName)
		if err != nil {
			fmt.Println(color.RedString("Error: %v", err))
			if len(servers) > 0 {
				fmt.Println("\nAvailable servers:")
				for _, s := range servers {
					fmt.Printf("  - %s (%s@%s:%d)\n", s.Name, s.Username, s.Host, s.Port)
				}
			} else {
				fmt.Println("No servers found. Add one with 'cliarc server add'.")
			}
			return nil
		}

		if err := setActiveServerName(srv.Name); err != nil {
			return fmt.Errorf("failed to save active server: %w", err)
		}

		fmt.Println(color.GreenString("✓ Active server set to %q (%s@%s:%d)", srv.Name, srv.Username, srv.Host, srv.Port))
		return nil
	},
}

func init() {
	useCmd.Flags().BoolP("clear", "c", false, "Clear the currently active server")
	useCmd.Flags().BoolP("list", "l", false, "List all servers and mark the active one")
}
