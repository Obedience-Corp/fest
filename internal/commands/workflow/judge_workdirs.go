package workflow

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/pathutil"
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
	Path string `json:"path"`
	// AbsolutePath is resolved for the judge's convenience; it is the campaign
	// root joined with Path, and is empty when the campaign root is unknown.
	AbsolutePath string `json:"absolute_path,omitempty"`
}

// collectPhaseWorkingDirs reads fest_working_dir from every sequence in the
// phase. Sequences that declare none are skipped rather than defaulted: a
// missing working dir means the work is in the festival itself (a design or
// docs phase), which the judge already sees through phase_path.
//
// Best-effort by design. An unreadable sequence or a malformed value is skipped
// rather than failing the checkpoint, because a judge with partial working-dir
// context is strictly better than a gate that will not run.
func collectPhaseWorkingDirs(phasePath, campaignRoot string) []judgeWorkingDir {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var dirs []judgeWorkingDir
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
		rel, err := pathutil.NormalizeWorkingDir(fm.WorkingDir)
		if err != nil || rel == "" {
			continue
		}
		key := entry.Name() + "\x00" + rel
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		wd := judgeWorkingDir{Sequence: entry.Name(), Path: rel}
		if campaignRoot != "" {
			wd.AbsolutePath = filepath.Join(campaignRoot, rel)
		}
		dirs = append(dirs, wd)
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].Sequence != dirs[j].Sequence {
			return dirs[i].Sequence < dirs[j].Sequence
		}
		return dirs[i].Path < dirs[j].Path
	})
	return dirs
}

// campaignRootFor walks up from the festival path looking for the campaign
// marker, so working dirs can be handed to the judge as absolute paths it can
// actually open. Empty when no campaign root is found, which leaves the
// relative paths intact and costs the judge only the join.
func campaignRootFor(festivalPath string) string {
	dir := festivalPath
	for {
		if dir == "" || dir == string(filepath.Separator) {
			return ""
		}
		if info, err := os.Stat(filepath.Join(dir, ".campaign")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
