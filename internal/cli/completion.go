package cli

import (
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for peppol.

To load completions:

Bash:
  $ source <(peppol completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ peppol completion bash > /etc/bash_completion.d/peppol
  # macOS:
  $ peppol completion bash > $(brew --prefix)/etc/bash_completion.d/peppol

Zsh:
  $ source <(peppol completion zsh)

  # To load completions for each session, execute once:
  $ peppol completion zsh > "${fpath[1]}/_peppol"

Fish:
  $ peppol completion fish | source

  # To load completions for each session, execute once:
  $ peppol completion fish > ~/.config/fish/completions/peppol.fish

PowerShell:
  PS> peppol completion powershell | Out-String | Invoke-Expression

  # To load completions for each session, execute once:
  PS> peppol completion powershell > peppol.ps1
  # and source this file from your PowerShell profile.
`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: "Generate bash completion script",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			},
		},
		&cobra.Command{
			Use:   "zsh",
			Short: "Generate zsh completion script",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			},
		},
		&cobra.Command{
			Use:   "fish",
			Short: "Generate fish completion script",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			},
		},
		&cobra.Command{
			Use:   "powershell",
			Short: "Generate powershell completion script",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			},
		},
	)

	return cmd
}
