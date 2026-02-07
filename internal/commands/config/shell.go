package config

import (
	"fmt"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// NewShellInitCommand creates the shell-init command
func NewShellInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init <shell>",
		Short: "Output shell integration code for festival helpers",
		Annotations: map[string]string{
			"scope": string(scope.Global),
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

After setup, reload your shell or run: source ~/.zshrc

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

  # After setup, use the helpers:
  fgo              # Go to festivals root
  fgo 2            # Go to phase 002
  fls              # List all festivals
  fls active       # List active festivals`,
		Args: cobra.ExactArgs(1),
		RunE: runShellInit,
	}

	return cmd
}

func runShellInit(cmd *cobra.Command, args []string) error {
	shell := args[0]

	switch shell {
	case "zsh", "bash":
		fmt.Print(bashZshInit())
	case "fish":
		fmt.Print(fishInit())
	default:
		return errors.Validation("unsupported shell - supported: zsh, bash, fish").WithField("shell", shell)
	}

	return nil
}

func bashZshInit() string {
	return `# fest shell integration - helper functions
# Add to ~/.zshrc or ~/.bashrc:
#   eval "$(fest shell-init zsh)"
#
# Provides: fgo (navigation), fls (listing)

# Tab completion for fgo (position-aware: status dirs trigger filtered completions)
_fgo_completions() {
    local completions
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        completions=$(command fest go completions 2>/dev/null)
    elif [[ ${COMP_CWORD} -eq 2 ]]; then
        case "${COMP_WORDS[1]}" in
            active|planned|completed|dungeon)
                completions=$(command fest go completions --status "${COMP_WORDS[1]}" 2>/dev/null)
                ;;
        esac
    fi
    COMPREPLY=($(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}"))
}

# Register completion (works for both bash and zsh with bashcompinit)
complete -F _fgo_completions fgo

# Zsh-specific: colorized completions using compadd -d for ANSI display strings
if [[ -n "$ZSH_VERSION" ]]; then
    _fgo_zsh() {
        local -a vals displays
        local line val display cmd_args

        if (( CURRENT == 2 )); then
            # First arg: show everything
            cmd_args="--color"
        elif (( CURRENT == 3 )); then
            # Second arg: if first arg is a status dir, show its festivals
            case "${words[2]}" in
                active|planned|completed|dungeon)
                    cmd_args="--color --status ${words[2]}"
                    ;;
                *) return ;;
            esac
        else
            return
        fi

        while IFS=$'\t' read -r val display; do
            vals+=("$val")
            displays+=("$display")
        done < <(command fest go completions $cmd_args 2>/dev/null)
        if (( ${#vals} )); then
            compadd -l -d displays -a vals
        fi
    }
    compdef _fgo_zsh fgo 2>/dev/null
fi

# Tab completion for fls - complete status names and flags
_fls_completions() {
    local completions="active planned completed dungeon --json --all --help"
    COMPREPLY=($(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}"))
}

# Register completion
complete -F _fls_completions fls

# Zsh-specific: use compdef if available for fls
if [[ -n "$ZSH_VERSION" ]]; then
    _fls_zsh() {
        local -a completions
        completions=(
            'active:List active festivals'
            'planned:List planned festivals'
            'completed:List completed festivals'
            'dungeon:List dungeon festivals'
            '--json:Output in JSON format'
            '--all:Include empty status categories'
            '--help:Show help for fest list'
        )
        _describe 'fls' completions
    }
    compdef _fls_zsh fls 2>/dev/null
fi

fgo() {
    case "$1" in
        --help|-h|help)
            # Show help for fgo/fest go
            command fest go --help
            ;;
        link)
            # Context-aware linking (no cd needed, shows TUI if needed)
            command fest go "$@"
            ;;
        unlink)
            # Remove festival-project link (no cd needed)
            command fest unlink
            ;;
        map|unmap)
            # Pass through to fest go subcommands (no cd needed)
            command fest go "$@"
            ;;
        list)
            # Interactive list - select and navigate to destination
            local dest
            dest=$(command fest go list --interactive --print 2>/dev/null)
            local exit_code=$?
            if [[ $exit_code -eq 0 && -n "$dest" && -d "$dest" ]]; then
                cd "$dest"
            elif [[ $exit_code -ne 0 ]]; then
                # Fall back to non-interactive list on error (e.g., no TUI, cancelled)
                command fest go list
            fi
            ;;
        project)
            # Navigate to linked project
            local dest
            dest=$(command fest go project --print 2>&1)
            local exit_code=$?
            if [[ $exit_code -eq 0 && -n "$dest" && -d "$dest" ]]; then
                cd "$dest"
            else
                echo "fgo: no project linked (use 'fest link <path>' from a festival)" >&2
                return 1
            fi
            ;;
        fest)
            # Navigate back to festival from project
            local dest
            dest=$(command fest go fest --print 2>&1)
            local exit_code=$?
            if [[ $exit_code -eq 0 && -n "$dest" && -d "$dest" ]]; then
                cd "$dest"
            else
                echo "fgo: not in a linked project" >&2
                return 1
            fi
            ;;
        -*)
            # Shortcut navigation: strip leading dash and lookup
            local name="${1#-}"
            local dest
            dest=$(command fest go shortcut "$name" --print 2>&1)
            local exit_code=$?
            if [[ $exit_code -eq 0 && -n "$dest" && -d "$dest" ]]; then
                cd "$dest"
            else
                echo "fgo: shortcut not found: -$name" >&2
                return 1
            fi
            ;;
        *)
            # Normal navigation (festival/phase/status directories)
            # Note: Don't use 2>&1 - stderr must flow to terminal for TUI picker to render
            local dest
            if [[ -n "$2" && "$1" =~ ^(active|planned|completed|dungeon)$ ]]; then
                # Status dir + festival name: combine (e.g., active my-fest → active/my-fest)
                dest=$(command fest go "$1/$2" --print)
            else
                dest=$(command fest go "$@" --print)
            fi
            local exit_code=$?
            if [[ $exit_code -eq 0 && -n "$dest" && -d "$dest" ]]; then
                cd "$dest"
            else
                # Error messages already went to stderr from the command
                return $exit_code
            fi
            ;;
    esac
}

# fls - shorthand for 'fest list'
# Simple pass-through wrapper that calls fest list with all arguments
fls() {
    case "$1" in
        --help|-h|help)
            # Show fest list help
            command fest list --help
            ;;
        *)
            # Pass all arguments through to fest list
            command fest list "$@"
            ;;
    esac
}
`
}

func fishInit() string {
	return `# fest shell integration - helper functions
# Add to ~/.config/fish/config.fish:
#   fest shell-init fish | source
#
# Provides: fgo (navigation), fls (listing)

# Tab completion for fgo (position-aware: status dirs trigger filtered completions)
function __fgo_completions
    set -l tokens (commandline -opc)
    set -l count (count $tokens)
    if test $count -eq 1
        # First arg: show everything
        command fest go completions 2>/dev/null
    else if test $count -eq 2
        # Second arg: if first arg is a status dir, show its festivals
        switch $tokens[2]
            case active planned completed dungeon
                command fest go completions --status $tokens[2] 2>/dev/null
        end
    end
end
complete -c fgo -f -a "(__fgo_completions)"

# Tab completion for fls
complete -c fls -f -a "active planned completed dungeon"
complete -c fls -l json -d "Output in JSON format"
complete -c fls -l all -d "Include empty status categories"
complete -c fls -l help -d "Show help for fest list"
complete -c fls -s h -d "Show help for fest list"

function fgo
    switch $argv[1]
        case --help -h help
            # Show help for fgo/fest go
            command fest go --help
        case link
            # Context-aware linking (no cd needed, shows TUI if needed)
            command fest go $argv
        case unlink
            # Remove festival-project link (no cd needed)
            command fest unlink
        case map unmap
            # Pass through to fest go subcommands (no cd needed)
            command fest go $argv
        case list
            # Interactive list - select and navigate to destination
            set -l dest (command fest go list --interactive --print 2>/dev/null)
            set -l exit_code $status
            if test $exit_code -eq 0 -a -n "$dest" -a -d "$dest"
                cd $dest
            else if test $exit_code -ne 0
                # Fall back to non-interactive list on error
                command fest go list
            end
        case project
            # Navigate to linked project
            set -l dest (command fest go project --print 2>&1)
            set -l exit_code $status
            if test $exit_code -eq 0 -a -n "$dest" -a -d "$dest"
                cd $dest
            else
                echo "fgo: no project linked (use 'fest link <path>' from a festival)" >&2
                return 1
            end
        case fest
            # Navigate back to festival from project
            set -l dest (command fest go fest --print 2>&1)
            set -l exit_code $status
            if test $exit_code -eq 0 -a -n "$dest" -a -d "$dest"
                cd $dest
            else
                echo "fgo: not in a linked project" >&2
                return 1
            end
        case '-*'
            # Shortcut navigation: strip leading dash and lookup
            set -l name (string sub -s 2 $argv[1])
            set -l dest (command fest go shortcut $name --print 2>&1)
            set -l exit_code $status
            if test $exit_code -eq 0 -a -n "$dest" -a -d "$dest"
                cd $dest
            else
                echo "fgo: shortcut not found: -$name" >&2
                return 1
            end
        case '*'
            # Normal navigation (festival/phase/status directories)
            # If first arg is a status dir and second arg exists, combine them
            set -l target
            if test (count $argv) -ge 2
                switch $argv[1]
                    case active planned completed dungeon
                        set target "$argv[1]/$argv[2]"
                    case '*'
                        set target $argv
                end
            else
                set target $argv
            end
            set -l dest (command fest go $target --print 2>&1)
            set -l exit_code $status
            if test $exit_code -eq 0 -a -n "$dest" -a -d "$dest"
                cd $dest
            else
                if test -n "$dest"
                    echo $dest >&2
                end
                return 1
            end
    end
end

# fls - shorthand for 'fest list'
function fls
    switch $argv[1]
        case --help -h help
            # Show fest list help
            command fest list --help
        case '*'
            # Pass all arguments through to fest list
            command fest list $argv
    end
end
`
}
