package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell autocompletion script",
	Long: `Generate shell autocompletion scripts for CLIARC.

Installation Instructions:

  Bash:
    $ source <(cliarc completion bash)
    # To load completions for each session:
    $ cliarc completion bash > /etc/bash_completion.d/cliarc

  Zsh:
    # If shell completion is not already enabled in your environment:
    $ echo "autoload -U compinit; compinit" >> ~/.zshrc
    # To load completions for each session:
    $ cliarc completion zsh > "${fpath[1]}/_cliarc"

  Fish:
    $ cliarc completion fish | source
    # To load completions for each session:
    $ cliarc completion fish > ~/.config/fish/completions/cliarc.fish

  PowerShell:
    PS> cliarc completion powershell | Out-String | Invoke-Expression
    # To load completions for every PowerShell session, add the output to your profile:
    PS> cliarc completion powershell > $PROFILE`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish, powershell)", args[0])
		}
	},
}
