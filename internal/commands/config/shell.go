package config

import (
	"fmt"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/shell"
	"github.com/spf13/cobra"
)

// NewShellInitCommand creates the shell-init command
func NewShellInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init <shell>",
		Short: "Output shell integration code for festival helpers",
		Annotations: map[string]string{
			"scope":         string(scope.Global),
			"agent_allowed": "false",
			"agent_reason":  "Shell config output, irrelevant to agents",
		},
		Long: `Output shell code that provides shell helper functions.

This command outputs shell-specific code that creates helper functions:
- fgo: Wraps 'fest go' to change your working directory
- fls: Wraps 'fest list' for quick festival listing

SETUP (one-time):
  # For zsh, add to ~/.zshrc:
  eval "$(fest shell-init zsh)"

  # For bash, add to ~/.bashrc:
  eval "$(fest shell-init bash)"

  # For fish, add to ~/.config/fish/config.fish:
  fest shell-init fish | source

  # For Windows PowerShell 5.1 or PowerShell 7, add to $PROFILE:
  Invoke-Expression (& fest shell-init powershell | Out-String)

  # For dash, busybox ash, or any other POSIX sh, add to ~/.profile:
  eval "$(fest shell-init sh)"

After setup, restart your shell or reload its profile (for example,
source ~/.zshrc in zsh, or . $PROFILE in PowerShell).

fgo, fls, and fest work in every supported shell. The shell-init command
installs tab completion in bash, zsh, and fish. POSIX sh and PowerShell get
the navigation helpers without tab completion.

USAGE - fgo (navigation):
  fgo              Smart navigation (linked project ↔ festival, or festivals root)
  fgo 002          Navigate to phase 002
  fgo 2/1          Navigate to phase 2, sequence 1
  fgo active       Navigate to active directory
  fgo link         Link current festival to project (or vice versa)
  fgo --help       Show fgo help

USAGE - fls (listing):
  fls              List all festivals grouped by status
  fls active       List active festivals only
  fls --json       List festivals in JSON format
  fls --help       Show fest list help`,
		Example: `  # Output zsh integration code
  fest shell-init zsh

  # Add to your shell config (zsh)
  eval "$(fest shell-init zsh)"

  # Add to $PROFILE (PowerShell)
  Invoke-Expression (& fest shell-init powershell | Out-String)

  # After setup, use the helpers:
  fgo              # Go to festivals root
  fgo 2            # Go to phase 002
  fls              # List all festivals
  fls active       # List active festivals`,
		// Derived from the supported list rather than written out, so adding a
		// shell cannot leave 'fest shell-init <TAB>' advertising a stale set.
		ValidArgs: shell.SupportedShells,
		Args:      cobra.ExactArgs(1),
		RunE:      runShellInit,
	}

	return cmd
}

func runShellInit(cmd *cobra.Command, args []string) error {
	script, err := shell.Generate(args[0])
	if err != nil {
		return err
	}
	fmt.Print(script)
	return nil
}
