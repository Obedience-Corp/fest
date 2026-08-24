# fest shell integration - helper functions
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
            case active planning ready ritual completed someday archived dungeon
                command fest go completions --status $tokens[2] 2>/dev/null
        end
    end
end
complete -c fgo -f -a "(__fgo_completions)"

# Tab completion for fls
# Status vocabulary mirrors id.StatusDirectories (+ dungeon, all); keep in sync
# with the bash and zsh lists in bash_zsh_completions.sh.
complete -c fls -f -a "active ready planning parked ritual completed dungeon all"
complete -c fls -l json -d "Output in JSON format"
complete -c fls -l all -d "Include completed and dungeon festivals"
complete -c fls -l help -d "Show help for fest list"
complete -c fls -s h -d "Show help for fest list"

# Tab completion for the fest binary itself, delegating to cobra's hidden
# __complete subcommand. Picks up ValidArgsFunction completions for free
# (e.g. 'fest ritual run <tab>').
function __fest_complete
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    set -l args $tokens[2..-1] $current
    command fest __complete $args 2>/dev/null \
        | string match -rv '^:|^$' \
        | string replace -r '\t.*$' ''
end
complete -c fest -f -a "(__fest_complete)"

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
                    case active planning ready ritual completed someday archived dungeon
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

# Wrap fest binary so 'fest go' changes directory
function fest
    switch $argv[1]
        case go g
            set -e argv[1]
            switch $argv[1]
                case --help -h help
                    command fest go --help
                case link unlink map unmap move completions
                    command fest go $argv
                case '*'
                    set -l dest
                    if test (count $argv) -ge 2
                        switch $argv[1]
                            case active planning ready ritual completed someday archived dungeon
                                set dest (command fest go "$argv[1]/$argv[2]" --print 2>/dev/null)
                            case '*'
                                set dest (command fest go $argv --print 2>/dev/null)
                        end
                    else
                        set dest (command fest go $argv --print 2>/dev/null)
                    end
                    if test -n "$dest" -a -d "$dest"
                        cd $dest
                    end
            end
        case '*'
            command fest $argv
    end
end
