package cmd

import (
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:     "create <name>",
	Aliases: []string{"new"},
	Short:   "Alias for cliarc dev init",
	Hidden:  true,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return devInitCmd.RunE(cmd, args)
	},
}
