package commit

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
)

// Fest renders the staging guard's decisions in its own voice from camp's
// typed data; no camp error text is parsed or echoed. The facts and the ways
// out are the same ones camp names, and every printed remedy must be runnable
// on the branch that printed it. Every stage fest performs now passes its own
// --commit-large through commitkit's options forms, so one retry command is
// right everywhere: fest's own flag, on the command the user already ran.

// commitLargeRetry is the commit-it-anyway command named by every refusal and
// exclusion fest prints. Naming camp here instead would hand the user a second
// binary to rerun for a decision fest's own flag already overrides.
const commitLargeRetry = "fest commit --commit-large"

// commitNestedRetry is the embed-it-anyway command named by every nested
// repository fest reports. It is separate from commitLargeRetry because the
// two authorize unrelated things: a size override must not silently also
// grant permission to embed a foreign repository.
const commitNestedRetry = "fest commit --commit-nested"

// guardRefusalMessage is the result.Error text for a staging refusal: what was
// refused, that nothing was staged, and every way out. A refusal is the one
// moment the guard spends the user's attention, so the message has to pay for
// it rather than saying a bare "staging refused".
func guardRefusalMessage(blocked *commitkit.GuardBlockedError) string {
	var b strings.Builder
	switch blocked.Kind {
	case commitkit.Bulk:
		var worst commitkit.GuardViolation
		var files int
		var total int64
		for _, v := range blocked.Violations {
			files += v.Count
			total += v.TotalBytes
			if v.Count > worst.Count {
				worst = v
			}
		}
		fmt.Fprintf(&b, "%d untracked files (%s) under %s/ would be committed; nothing was staged\n",
			files, formatBytes(total), worst.CommonPrefix)
		fmt.Fprintf(&b, "  gitignore it        echo '%s/' >> .gitignore\n", worst.CommonPrefix)
		fmt.Fprintf(&b, "  keep and sync it    camp artifacts add %s\n", worst.CommonPrefix)
		fmt.Fprintf(&b, "  commit it anyway    %s -m \"...\"\n", commitLargeRetry)
		fmt.Fprintf(&b, "  turn the guard off  camp settings set local.commit.guards.bulk off")
	case commitkit.NestedRepo:
		fmt.Fprintf(&b, "%s would be committed as a nested git repository; nothing was staged\n",
			pluralRepos(len(blocked.Violations)))
		for _, v := range blocked.Violations {
			fmt.Fprintf(&b, "  %s\n", describeNestedRepo(v))
		}
		for _, v := range blocked.Violations {
			fmt.Fprintf(&b, "  make it a submodule  git submodule add <url> %s\n", v.Path)
		}
		fmt.Fprintf(&b, "  commit it anyway    %s -m \"...\"\n", commitNestedRetry)
		fmt.Fprintf(&b, "  turn the guard off  camp settings set local.commit.guards.nested_repos off")
	default:
		fmt.Fprintf(&b, "%s over the %s limit would be committed; nothing was staged\n",
			pluralFiles(len(blocked.Violations)), formatBytes(blocked.Limits.MaxFileSize))
		for _, v := range blocked.Violations {
			fmt.Fprintf(&b, "  %s   %s\n", v.Path, formatBytes(v.Size))
		}
		for _, v := range blocked.Violations {
			fmt.Fprintf(&b, "  keep and sync it    camp artifacts add %s\n",
				filepath.ToSlash(filepath.Dir(v.Path)))
		}
		fmt.Fprintf(&b, "  commit it anyway    %s -m \"...\"\n", commitLargeRetry)
		fmt.Fprintf(&b, "  handle it for me    camp settings set local.commit.guards.large_files auto")
	}
	return b.String()
}

// reportStageOutcome says what the guard did when staging still succeeded.
// Camp acting automatically is only safe if the user is told, so an excluded
// file, a flagged tracked file, or a guard that could not run are each one
// line here rather than a silent diff surprise. Written to stderr by callers
// so a machine-read stdout stays pure.
func reportStageOutcome(w io.Writer, outcome *commitkit.StageOutcome) {
	if outcome == nil {
		return
	}
	if outcome.Unavailable != nil {
		_, _ = fmt.Fprintf(w, "Staged without the size and bulk guard: %v\n", outcome.Unavailable)
	}
	for _, v := range outcome.Excluded {
		_, _ = fmt.Fprintf(w, "Kept out of git: %s (%s, over the %s limit); commit it anyway with '%s' or 'camp settings set local.commit.guards.large_files auto'\n",
			v.Path, formatBytes(v.Size), formatBytes(outcome.Limits.MaxFileSize), commitLargeRetry)
	}
	for _, v := range outcome.NestedRepos {
		_, _ = fmt.Fprintf(w, "Kept out of git: %s; commit it as a gitlink with '%s', declare it with 'git submodule add <url> %s', or 'camp settings set local.commit.guards.nested_repos off'\n",
			describeNestedRepo(v), commitNestedRetry, v.Path)
	}
	for _, v := range outcome.Reported {
		_, _ = fmt.Fprintf(w, "Tracked file grew past %s: %s (%s); committed as usual\n",
			formatBytes(outcome.Limits.MaxFileSize), v.Path, formatBytes(v.Size))
	}
}

// describeNestedRepo names a nested repository and, when it has one, the
// commit it sits on, so the user can tell which checkout is being discussed
// without leaving the message.
func describeNestedRepo(v commitkit.GuardViolation) string {
	if v.Head == "" {
		return fmt.Sprintf("%s (a git repository with no commits yet, not declared in .gitmodules)", v.Path)
	}
	return fmt.Sprintf("%s (a git repository at %s, not declared in .gitmodules)", v.Path, v.Head)
}

func pluralRepos(n int) string {
	if n == 1 {
		return "1 nested repository"
	}
	return fmt.Sprintf("%d nested repositories", n)
}

func pluralFiles(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// allExcludedMessage is the result.Error text for a commit whose every change
// the guard held back, so a follow-up would otherwise surface as a bare "no
// changes to commit". Silently excluding and then reporting nothing is the
// exact failure the guard exists to prevent, so this names what was kept out
// and the flag that commits it.
func allExcludedMessage(outcome *commitkit.StageOutcome) string {
	var b strings.Builder
	b.WriteString("everything changed was kept out of git by the staging guard; nothing left to commit\n")
	for _, v := range outcome.Excluded {
		fmt.Fprintf(&b, "  %s (%s, over the %s limit)\n",
			v.Path, formatBytes(v.Size), formatBytes(outcome.Limits.MaxFileSize))
	}
	for _, v := range outcome.NestedRepos {
		fmt.Fprintf(&b, "  %s\n", describeNestedRepo(v))
	}
	switch {
	case len(outcome.Excluded) > 0 && len(outcome.NestedRepos) > 0:
		fmt.Fprintf(&b, "  commit them anyway  %s --commit-nested -m \"...\"", commitLargeRetry)
	case len(outcome.NestedRepos) > 0:
		fmt.Fprintf(&b, "  commit it anyway    %s -m \"...\"", commitNestedRetry)
	default:
		fmt.Fprintf(&b, "  commit it anyway    %s -m \"...\"", commitLargeRetry)
	}
	return b.String()
}
