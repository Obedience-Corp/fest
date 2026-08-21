package commit

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Obedience-Corp/fest/internal/progress"
)

// position is where inside a festival a commit happened: the phase and
// sequence directory numbers, kept exactly as the directories spell them
// ("001", "02") rather than re-padded.
type position struct {
	Phase    string
	Sequence string
}

func (p position) isZero() bool {
	return p.Phase == "" && p.Sequence == ""
}

// numberedDirRe matches the leading number of a phase or sequence directory
// name such as "001_IMPLEMENT" or "02_camp_pilot".
var numberedDirRe = regexp.MustCompile(`^(\d+)_`)

// commitPosition is the single place that decides which festival the position
// is measured against. festivalPath is whichever festival the command settled
// on: the one in scope, or the one --festival resolved when scope had none.
//
// An empty ref means there is no festival reference to hang the segments off
// (--no-tag, or no detectable ID), so the position is skipped entirely.
func commitPosition(ctx context.Context, ref, festivalPath string, hasFestival bool) position {
	if ref == "" || !hasFestival || festivalPath == "" {
		return position{}
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	return resolvePosition(ctx, festivalPath, cwd)
}

// resolvePosition locates the commit inside the festival, preferring the
// caller's working directory and falling back to the single in-progress
// sequence recorded in the progress store.
//
// Every failure is soft: an unreadable store, an ambiguous set of in-progress
// tasks, or a working directory outside the festival all yield the zero
// position so the tag simply omits the segments instead of blocking a commit.
func resolvePosition(ctx context.Context, festivalPath, cwd string) position {
	if ctx.Err() != nil || festivalPath == "" {
		return position{}
	}

	if pos := positionFromCwd(festivalPath, cwd); !pos.isZero() {
		return pos
	}

	return positionFromProgress(ctx, festivalPath)
}

// positionFromCwd derives the position from where the caller stands. A working
// directory at the festival root or outside it carries no position.
func positionFromCwd(festivalPath, cwd string) position {
	if cwd == "" {
		return position{}
	}

	rel, err := filepath.Rel(festivalPath, cwd)
	if err != nil {
		return position{}
	}
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return position{}
	}

	parts := strings.Split(rel, string(os.PathSeparator))
	phase := numberedDirPrefix(parts[0])
	if phase == "" {
		return position{}
	}

	pos := position{Phase: phase}
	if len(parts) > 1 {
		pos.Sequence = numberedDirPrefix(parts[1])
	}
	return pos
}

// positionFromProgress reads the festival's in-progress tasks and returns the
// single phase/sequence pair they share. Zero in-progress tasks, or tasks
// spread across parallel sequences, are ambiguous and yield nothing.
func positionFromProgress(ctx context.Context, festivalPath string) position {
	store := progress.NewStore(festivalPath)
	if err := store.LoadReadOnly(ctx); err != nil {
		return position{}
	}

	data := store.Data()
	if data == nil {
		return position{}
	}

	var found position
	for key, task := range data.Tasks {
		if task == nil || task.Status != progress.StatusInProgress {
			continue
		}
		pos, ok := positionFromTaskKey(key)
		if !ok {
			continue
		}
		if found.isZero() {
			found = pos
			continue
		}
		if found != pos {
			return position{}
		}
	}
	return found
}

// positionFromTaskKey reads a progress store key such as
// "001_IMPLEMENT/02_camp_pilot/01_task.md". Legacy keys are bare filenames
// with no directories and carry no position.
func positionFromTaskKey(key string) (position, bool) {
	parts := strings.Split(filepath.ToSlash(key), "/")
	if len(parts) < 3 {
		return position{}, false
	}
	phase := numberedDirPrefix(parts[0])
	sequence := numberedDirPrefix(parts[1])
	if phase == "" || sequence == "" {
		return position{}, false
	}
	return position{Phase: phase, Sequence: sequence}, true
}

func numberedDirPrefix(name string) string {
	match := numberedDirRe.FindStringSubmatch(name)
	if match == nil {
		return ""
	}
	return match[1]
}

// positionSummary renders a resolved position for human output.
func positionSummary(phase, sequence string) string {
	if phase == "" {
		return ""
	}
	if sequence == "" {
		return "phase " + phase
	}
	return "phase " + phase + ", sequence " + sequence
}
