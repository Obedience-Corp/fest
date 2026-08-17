# Tab completion for bash and zsh.
#
# Split from posix_core.sh because everything here is bash/zsh only:
# COMPREPLY arrays, compgen, complete, compdef, compadd, [[ ]], and process
# substitution. A POSIX sh has no programmable completion to hook, so there is
# nothing in this file for it to install.

# Tab completion for fgo (position-aware: status dirs trigger filtered completions)
_fgo_completions() {
    local completions
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        completions=$(command fest go completions 2>/dev/null)
    elif [[ ${COMP_CWORD} -eq 2 ]]; then
        case "${COMP_WORDS[1]}" in
            active|planning|ready|ritual|completed|someday|archived|dungeon)
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
                active|planning|ready|ritual|completed|someday|archived|dungeon)
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
            compadd -V fgo -l -d displays -a vals
        fi
    }
    compdef _fgo_zsh fgo 2>/dev/null
fi

# Tab completion for fls - complete status names and flags
# Status vocabulary mirrors id.StatusDirectories (+ dungeon, all); keep in sync.
_fls_completions() {
    local completions="active ready planning ritual completed dungeon all --json --all --help"
    COMPREPLY=($(compgen -W "$completions" -- "${COMP_WORDS[COMP_CWORD]}"))
}

# Register completion
complete -F _fls_completions fls

# Zsh-specific: use compdef if available for fls
if [[ -n "$ZSH_VERSION" ]]; then
    _fls_zsh() {
        local -a completions
        # Status vocabulary mirrors id.StatusDirectories (+ dungeon, all); keep in sync.
        completions=(
            'active:Festivals currently in progress'
            'ready:Festivals prepared and awaiting execution'
            'planning:Festivals being designed'
            'ritual:Recurring or special festivals'
            'completed:Successfully finished festivals'
            'dungeon:All shelved festivals'
            'all:Every festival grouped by status'
            '--json:Output in JSON format'
            '--all:Include completed and dungeon festivals'
            '--help:Show help for fest list'
        )
        _describe 'fls' completions
    }
    compdef _fls_zsh fls 2>/dev/null
fi

# Tab completion for the fest binary itself, delegating to cobra's hidden
# __complete subcommand. Every cobra command on fest gets free tab completion,
# including ValidArgsFunction-driven completions like 'fest ritual run <tab>'.
_fest_completions_bash() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local -a args=("${COMP_WORDS[@]:1:$COMP_CWORD-1}" "$cur")
    local -a completions=()
    local line
    while IFS= read -r line; do
        [[ -z "$line" || "$line" == ":"* ]] && continue
        completions+=("${line%%$'\t'*}")
    done < <(command fest __complete "${args[@]}" 2>/dev/null)
    COMPREPLY=($(compgen -W "${completions[*]}" -- "$cur"))
}
complete -F _fest_completions_bash fest

if [[ -n "$ZSH_VERSION" ]]; then
    _fest_zsh() {
        # Colorized, status-ordered completion for 'fest promote <TAB>' (parity with fgo).
        # Skip when completing a flag so the generic __complete path still offers flags.
        if [[ "${words[2]}" == "promote" && $CURRENT -eq 3 && "${words[CURRENT]}" != -* ]]; then
            local -a pvals pdisplays
            local pval pdisplay
            while IFS=$'\t' read -r pval pdisplay; do
                pvals+=("$pval")
                pdisplays+=("$pdisplay")
            done < <(command fest promote completions --color 2>/dev/null)
            if (( ${#pvals} )); then
                compadd -V promote -l -d pdisplays -a pvals
                return
            fi
        fi
        # 'fest show <TAB>': empty prefix opens the festival picker TUI and inserts
        # the chosen festival; a non-empty prefix gets colorized menu completion.
        if [[ "${words[2]}" == "show" && $CURRENT -eq 3 && "${words[CURRENT]}" != -* ]]; then
            if [[ -z "${words[CURRENT]}" && -e /dev/tty ]]; then
                local picked
                picked=$(command fest show pick </dev/tty 2>/dev/tty)
                if [[ -n "$picked" ]]; then
                    compadd -U -- "$picked"
                    return
                fi
                # Cancelled or unavailable: fall through to menu completion below.
            fi
            local -a svals sdisplays
            local sval sdisplay
            while IFS=$'\t' read -r sval sdisplay; do
                svals+=("$sval")
                sdisplays+=("$sdisplay")
            done < <(command fest show completions --color 2>/dev/null)
            if (( ${#svals} )); then
                compadd -V show -l -d sdisplays -a svals
                return
            fi
        fi
        local -a vals
        local line val
        local -a args
        # words[1] is "fest"; pass words[2..CURRENT] to __complete
        args=("${(@)words[2,$CURRENT]}")
        while IFS= read -r line; do
            [[ -z "$line" || "$line" == ":"* ]] && continue
            val="${line%%$'\t'*}"
            vals+=("$val")
        done < <(command fest __complete "${args[@]}" 2>/dev/null)
        if (( ${#vals} )); then
            compadd -a vals
        fi
    }
    compdef _fest_zsh fest 2>/dev/null
fi
