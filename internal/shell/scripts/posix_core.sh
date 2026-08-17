# fest shell integration - helper functions
# Add to ~/.zshrc, ~/.bashrc, or ~/.profile:
#   eval "$(fest shell-init zsh)"
#   eval "$(fest shell-init sh)"
#
# Provides: fgo (navigation), fls (listing), fest (cd-aware wrapper)
#
# Everything in this file must be valid POSIX sh: it is the whole script for
# dash, busybox ash, and the /bin/sh on minimal and embedded systems, and it is
# also the shared base for bash and zsh. Nothing here may use arrays, [[ ]],
# =~, &>, process substitution, or programmable completion. Those are bash and
# zsh only and live in bash_zsh_completions.sh.
#
# 'local' is the one exception. It is not in POSIX, but dash, busybox ash,
# mksh, and yash all implement it, and dropping it would leak every wrapper's
# working variables into the user's shell. The guard below refuses to install
# on a shell that lacks it.

# Check if fest is available
if ! command -v fest >/dev/null 2>&1; then
  return 2>/dev/null || exit 0
fi

# ksh93 has no 'local' and fails in the worst possible way: the assignment
# simply never happens and the function reads whatever the caller had in that
# variable name, with no error. Refuse up front rather than install wrappers
# that quietly cd somewhere unrelated.
#
# The probe tests the EFFECT, not just that the command ran. Under ksh93 a
# failed 'local' does not abort the function, so calling it still returns 0.
if ! (_fest_v=outer; _fest_probe() { local _fest_v=inner; [ "$_fest_v" = inner ]; }; _fest_probe) >/dev/null 2>&1; then
  echo "fest: this shell has no 'local' builtin, which fest's sh integration requires." >&2
  echo "fest: known affected: ksh93. Use bash, zsh, dash, busybox ash, or mksh." >&2
  return 2>/dev/null || exit 0
fi
unset -f _fest_probe 2>/dev/null || true

# _fest_is_status reports whether $1 is one of the status directories, which is
# how "fgo active my-festival" is distinguished from a two-part path. This
# replaces a bash =~ match; a case statement is POSIX and needs no subprocess.
_fest_is_status() {
  case "$1" in
    active|planning|ready|ritual|completed|someday|archived|dungeon) return 0 ;;
    *) return 1 ;;
  esac
}

# _fest_cd_to runs a --print form of 'fest go' and cds to what it prints.
# Returns the command's own exit code when there is nothing to cd to, so
# callers can distinguish "failed" from "printed a path that is not a
# directory". stderr is left alone by callers that need the TUI picker to
# render; this helper only ever reads stdout.
_fest_cd_to() {
  local dest exit_code
  dest=$(command fest go "$@" --print)
  exit_code=$?
  if [ "$exit_code" -eq 0 ] && [ -n "$dest" ] && [ -d "$dest" ]; then
    cd "$dest" || return 1
    return 0
  fi
  return "$exit_code"
}

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
      local dest exit_code
      dest=$(command fest go list --interactive --print 2>/dev/null)
      exit_code=$?
      if [ "$exit_code" -eq 0 ] && [ -n "$dest" ] && [ -d "$dest" ]; then
        cd "$dest" || return 1
      elif [ "$exit_code" -ne 0 ]; then
        # Fall back to non-interactive list on error (e.g., no TUI, cancelled)
        command fest go list
      fi
      ;;
    project)
      # Navigate to linked project
      local dest exit_code
      dest=$(command fest go project --print 2>&1)
      exit_code=$?
      if [ "$exit_code" -eq 0 ] && [ -n "$dest" ] && [ -d "$dest" ]; then
        cd "$dest" || return 1
      else
        echo "fgo: no project linked (use 'fest link <path>' from a festival)" >&2
        return 1
      fi
      ;;
    fest)
      # Navigate back to festival from project
      local dest exit_code
      dest=$(command fest go fest --print 2>&1)
      exit_code=$?
      if [ "$exit_code" -eq 0 ] && [ -n "$dest" ] && [ -d "$dest" ]; then
        cd "$dest" || return 1
      else
        echo "fgo: not in a linked project" >&2
        return 1
      fi
      ;;
    -*)
      # Shortcut navigation: strip leading dash and lookup
      local name dest exit_code
      name="${1#-}"
      dest=$(command fest go shortcut "$name" --print 2>&1)
      exit_code=$?
      if [ "$exit_code" -eq 0 ] && [ -n "$dest" ] && [ -d "$dest" ]; then
        cd "$dest" || return 1
      else
        echo "fgo: shortcut not found: -$name" >&2
        return 1
      fi
      ;;
    *)
      # Normal navigation (festival/phase/status directories)
      # Note: Don't redirect stderr - it must flow to the terminal for the TUI
      # picker to render.
      if [ -n "$2" ] && _fest_is_status "$1"; then
        # Status dir + festival name: combine (e.g., active my-fest → active/my-fest)
        _fest_cd_to "$1/$2"
      else
        _fest_cd_to "$@"
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

# Wrap fest binary so 'fest go' changes directory
fest() {
  case "$1" in
    go|g)
      shift
      case "$1" in
        --help|-h|help)
          command fest go --help
          ;;
        link|unlink|map|unmap|move|completions)
          command fest go "$@"
          ;;
        *)
          local dest
          if [ -n "$2" ] && _fest_is_status "$1"; then
            dest=$(command fest go "$1/$2" --print 2>/dev/null)
          else
            dest=$(command fest go "$@" --print 2>/dev/null)
          fi
          if [ -n "$dest" ] && [ -d "$dest" ]; then
            cd "$dest" || return 1
          fi
          ;;
      esac
      ;;
    *)
      command fest "$@"
      ;;
  esac
}
