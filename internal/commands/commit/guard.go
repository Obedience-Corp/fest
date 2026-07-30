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
// out are the same ones camp names, so a user blocked in either tool reaches
// the same remedies, with fest commit as the retry vehicle.

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
		fmt.Fprintf(&b, "  commit it anyway    fest commit --commit-large -m \"...\"\n")
		fmt.Fprintf(&b, "  turn the guard off  camp settings set local.commit.guards.bulk off")
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
		fmt.Fprintf(&b, "  commit it anyway    fest commit --commit-large -m \"...\"\n")
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
		fmt.Fprintf(w, "Staged without the size and bulk guard: %v\n", outcome.Unavailable)
	}
	for _, v := range outcome.Excluded {
		fmt.Fprintf(w, "Kept out of git: %s (%s, over the %s limit); commit it anyway with 'fest commit --commit-large'\n",
			v.Path, formatBytes(v.Size), formatBytes(outcome.Limits.MaxFileSize))
	}
	for _, v := range outcome.Reported {
		fmt.Fprintf(w, "Tracked file grew past %s: %s (%s); committed as usual\n",
			formatBytes(outcome.Limits.MaxFileSize), v.Path, formatBytes(v.Size))
	}
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
