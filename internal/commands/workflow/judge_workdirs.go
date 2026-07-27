package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// judgeWorkingDir is one place a phase's work actually landed.
//
// It exists because a judge that cannot see the working directory is being
// asked whether an implementation satisfied its goal while structurally unable
// to look at the implementation. Implementation phases produce code in
// projects/*, and the festival directory holds only the plan and whatever the
// executor wrote about the work. Without this the judge can only evaluate the
// executor's own account of what it did.
type judgeWorkingDir struct {
	// Sequence is the sequence directory that declared this working dir, so a
	// judge can tell which deliverable belongs where when a phase spans repos.
	Sequence string `json:"sequence"`
	// Path is relative to the campaign root, exactly as fest_working_dir
	// declares it (for example "projects/camp").
	//
	// Deliberately relative. An absolute path carries the operator's home
	// directory and username, and this value is rendered into a judge prompt
	// that reaches a model provider and is recorded in ledgers and transcripts.
	// The judge resolves against campaign_root, which appears once.
	Path string `json:"path"`
}

// workingDirSkip is a fest_working_dir declaration that was present but could
// not be used.
//
// Reporting these matters because the failure is otherwise invisible: a
// sequence with a typo'd or escaping working dir looks exactly like a design
// phase that declared none, and if every sequence fails that way the judge
// silently receives no working dirs at all, which is the incomplete-request
// failure this whole path exists to fix.
type workingDirSkip struct {
	Sequence string
	Value    string
	Reason   string
}

// collectPhaseWorkingDirs reads fest_working_dir from every sequence in the
// phase. Sequences that declare none are skipped rather than defaulted: a
// missing working dir means the work is in the festival itself (a design or
// docs phase), which the judge already sees through phase_path.
//
// Best-effort by design. An unreadable sequence or a malformed value is skipped
// rather than failing the checkpoint, because a judge with partial working-dir
// context is strictly better than a gate that will not run. Skips carrying a
// non-empty declaration are returned so the caller can report them.
func collectPhaseWorkingDirs(phasePath string) ([]judgeWorkingDir, []workingDirSkip) {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return nil, nil
	}

	var dirs []judgeWorkingDir
	var skips []workingDirSkip
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		goalPath := filepath.Join(phasePath, entry.Name(), "SEQUENCE_GOAL.md")
		raw, err := os.ReadFile(goalPath)
		if err != nil {
			continue
		}
		fm, _, err := frontmatter.Parse(raw)
		if err != nil || fm == nil {
			continue
		}
		declared := strings.TrimSpace(fm.WorkingDir)
		if declared == "" {
			continue
		}

		rel, err := pathutil.NormalizeWorkingDir(fm.WorkingDir)
		if err != nil {
			skips = append(skips, workingDirSkip{Sequence: entry.Name(), Value: declared, Reason: err.Error()})
			continue
		}
		if rel == "" {
			skips = append(skips, workingDirSkip{
				Sequence: entry.Name(),
				Value:    declared,
				Reason:   "normalized to an empty path",
			})
			continue
		}

		dirs = append(dirs, judgeWorkingDir{Sequence: entry.Name(), Path: rel})
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].Sequence != dirs[j].Sequence {
			return dirs[i].Sequence < dirs[j].Sequence
		}
		return dirs[i].Path < dirs[j].Path
	})
	sort.Slice(skips, func(i, j int) bool { return skips[i].Sequence < skips[j].Sequence })
	return dirs, skips
}

// reportWorkingDirSkips warns about declarations that were dropped, loudest
// when dropping them left the judge with nothing.
func reportWorkingDirSkips(w io.Writer, skips []workingDirSkip, kept int) {
	for _, skip := range skips {
		_, _ = fmt.Fprintf(w, "fest: ignoring fest_working_dir %q in %s: %s\n", skip.Value, skip.Sequence, skip.Reason)
	}
	if len(skips) > 0 && kept == 0 {
		_, _ = fmt.Fprintln(w, "fest: the judge will see no working directories for this phase")
	}
}

// campaignRootFor resolves the campaign containing the festival.
//
// Delegates to workspace.DetectCampaign so the judge's notion of the campaign
// root is the same one the rest of fest uses, including the CAMP_ROOT override
// and absolute-path resolution. Empty when detection fails, which leaves the
// relative working-dir paths intact and costs the judge only the join.
func campaignRootFor(ctx context.Context, festivalPath string) string {
	root, err := workspace.DetectCampaign(ctx, festivalPath)
	if err != nil {
		return ""
	}
	return root
}
