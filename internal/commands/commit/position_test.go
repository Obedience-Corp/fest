package commit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
)

type storedTask struct {
	key    string
	status string
}

// writeProgress persists tasks through the store API so the resolver reads a
// real JSONL event log rather than a hand-written fixture.
func writeProgress(t *testing.T, festivalPath string, tasks ...storedTask) {
	t.Helper()

	ctx := context.Background()
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	ts := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	next := func() time.Time {
		ts = ts.Add(time.Minute)
		return ts
	}

	for _, task := range tasks {
		store.SetTask(&progress.TaskProgress{TaskID: task.key, Status: task.status})
		switch task.status {
		case progress.StatusInProgress:
			store.QueueEvent(&progress.ProgressEvent{Timestamp: next(), Event: progress.EventStarted, Task: task.key})
		case progress.StatusCompleted:
			store.QueueEvent(&progress.ProgressEvent{Timestamp: next(), Event: progress.EventStarted, Task: task.key})
			store.QueueEvent(&progress.ProgressEvent{Timestamp: next(), Event: progress.EventCompleted, Task: task.key})
		default:
			store.QueueEvent(&progress.ProgressEvent{Timestamp: next(), Event: progress.EventProgress, Task: task.key})
		}
	}

	if err := store.Save(ctx); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func festivalDir(t *testing.T, subdirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range subdirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}
	return root
}

func TestResolvePosition_CancelledContext(t *testing.T) {
	t.Parallel()

	festival := festivalDir(t, filepath.Join("001_IMPLEMENT", "02_camp_pilot"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cwd := filepath.Join(festival, "001_IMPLEMENT", "02_camp_pilot")
	if got := resolvePosition(ctx, festival, cwd); !got.isZero() {
		t.Errorf("resolvePosition() on cancelled context = %+v, want zero", got)
	}
}

func TestResolvePosition_EmptyFestivalPath(t *testing.T) {
	t.Parallel()

	if got := resolvePosition(context.Background(), "", "/anywhere"); !got.isZero() {
		t.Errorf("resolvePosition() without a festival = %+v, want zero", got)
	}
}

func TestResolvePosition_FromCwd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		dirs   []string
		cwdRel string
		cwdAbs string
		want   position
	}{
		{
			name:   "outside the festival",
			dirs:   []string{filepath.Join("001_IMPLEMENT", "02_camp_pilot")},
			cwdAbs: t.TempDir(),
		},
		{
			name: "empty cwd",
			dirs: []string{filepath.Join("001_IMPLEMENT", "02_camp_pilot")},
		},
		{
			name:   "festival root",
			dirs:   []string{filepath.Join("001_IMPLEMENT", "02_camp_pilot")},
			cwdRel: ".",
		},
		{
			name:   "non-numeric phase directory",
			dirs:   []string{filepath.Join("planning", "02_camp_pilot")},
			cwdRel: filepath.Join("planning", "02_camp_pilot"),
		},
		{
			name:   "numeric phase with non-numeric sequence",
			dirs:   []string{filepath.Join("001_IMPLEMENT", "camp_pilot")},
			cwdRel: filepath.Join("001_IMPLEMENT", "camp_pilot"),
			want:   position{Phase: "001"},
		},
		{
			name:   "phase only",
			dirs:   []string{"001_IMPLEMENT"},
			cwdRel: "001_IMPLEMENT",
			want:   position{Phase: "001"},
		},
		{
			name:   "phase and sequence",
			dirs:   []string{filepath.Join("001_IMPLEMENT", "02_camp_pilot")},
			cwdRel: filepath.Join("001_IMPLEMENT", "02_camp_pilot"),
			want:   position{Phase: "001", Sequence: "02"},
		},
		{
			name:   "below the sequence directory",
			dirs:   []string{filepath.Join("001_IMPLEMENT", "02_camp_pilot", "evidence")},
			cwdRel: filepath.Join("001_IMPLEMENT", "02_camp_pilot", "evidence"),
			want:   position{Phase: "001", Sequence: "02"},
		},
		{
			name:   "numbers keep their written padding",
			dirs:   []string{filepath.Join("12_ROLLOUT", "007_wide")},
			cwdRel: filepath.Join("12_ROLLOUT", "007_wide"),
			want:   position{Phase: "12", Sequence: "007"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			festival := festivalDir(t, tt.dirs...)
			cwd := tt.cwdAbs
			if tt.cwdRel != "" {
				cwd = filepath.Join(festival, tt.cwdRel)
			}

			if got := resolvePosition(context.Background(), festival, cwd); got != tt.want {
				t.Errorf("resolvePosition() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolvePosition_FromProgressStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		tasks []storedTask
		want  position
	}{
		{
			name: "no tasks at all",
		},
		{
			name: "nothing in progress",
			tasks: []storedTask{
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusCompleted},
				{key: "001_IMPLEMENT/02_camp_pilot/02_task.md", status: progress.StatusPending},
			},
		},
		{
			name: "legacy bare filename key",
			tasks: []storedTask{
				{key: "01_task.md", status: progress.StatusInProgress},
			},
		},
		{
			name: "non-numeric directory prefixes",
			tasks: []storedTask{
				{key: "implement/camp_pilot/01_task.md", status: progress.StatusInProgress},
			},
		},
		{
			name: "parallel sequences are ambiguous",
			tasks: []storedTask{
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
				{key: "001_IMPLEMENT/03_rollout/01_task.md", status: progress.StatusInProgress},
			},
		},
		{
			name: "parallel phases are ambiguous",
			tasks: []storedTask{
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
				{key: "002_VERIFY/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
			},
		},
		{
			name: "single in-progress task",
			tasks: []storedTask{
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
				{key: "001_IMPLEMENT/01_setup/01_task.md", status: progress.StatusCompleted},
			},
			want: position{Phase: "001", Sequence: "02"},
		},
		{
			name: "two in-progress tasks in one sequence",
			tasks: []storedTask{
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
				{key: "001_IMPLEMENT/02_camp_pilot/02_task.md", status: progress.StatusInProgress},
			},
			want: position{Phase: "001", Sequence: "02"},
		},
		{
			name: "legacy key alongside a positioned task",
			tasks: []storedTask{
				{key: "01_task.md", status: progress.StatusInProgress},
				{key: "001_IMPLEMENT/02_camp_pilot/01_task.md", status: progress.StatusInProgress},
			},
			want: position{Phase: "001", Sequence: "02"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			festival := festivalDir(t)
			if len(tt.tasks) > 0 {
				writeProgress(t, festival, tt.tasks...)
			}

			if got := resolvePosition(context.Background(), festival, festival); got != tt.want {
				t.Errorf("resolvePosition() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolvePosition_CwdWinsOverProgressStore(t *testing.T) {
	t.Parallel()

	festival := festivalDir(t, filepath.Join("001_IMPLEMENT", "03_rollout"))
	writeProgress(t, festival, storedTask{
		key:    "001_IMPLEMENT/02_camp_pilot/01_task.md",
		status: progress.StatusInProgress,
	})

	cwd := filepath.Join(festival, "001_IMPLEMENT", "03_rollout")
	want := position{Phase: "001", Sequence: "03"}
	if got := resolvePosition(context.Background(), festival, cwd); got != want {
		t.Errorf("resolvePosition() = %+v, want %+v", got, want)
	}
}

func TestResolvePosition_LeavesWorkflowStateInPlace(t *testing.T) {
	t.Parallel()

	festival := festivalDir(t, progress.ProgressDir)
	writeProgress(t, festival, storedTask{
		key:    "001_IMPLEMENT/02_camp_pilot/01_task.md",
		status: progress.StatusInProgress,
	})

	workflowYAML := filepath.Join(festival, progress.ProgressDir, "workflow_state.yaml")
	if err := os.WriteFile(workflowYAML, []byte("festival: demo\nphases: {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolvePosition(context.Background(), festival, festival)

	if _, err := os.Stat(workflowYAML); err != nil {
		t.Fatalf("workflow_state.yaml must survive position resolution: %v", err)
	}
}

func TestPositionSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pos  position
		want string
	}{
		{name: "zero"},
		{name: "sequence without phase", pos: position{Sequence: "02"}},
		{name: "phase only", pos: position{Phase: "001"}, want: "phase 001"},
		{name: "phase and sequence", pos: position{Phase: "001", Sequence: "02"}, want: "phase 001, sequence 02"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := positionSummary(tt.pos.Phase, tt.pos.Sequence); got != tt.want {
				t.Errorf("positionSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommitPosition(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		// fromScope reports whether the festival came from scope. The --festival
		// strategy sets hasFestival itself after resolving the flag, so both
		// arrive at commitPosition the same way.
		hasFestival bool
		emptyPath   bool
		want        position
	}{
		{
			name:        "no reference suppresses the position",
			hasFestival: true,
		},
		{
			name: "no festival in scope and no flag",
			ref:  "FE-CC0008",
		},
		{
			name:        "festival flagged but the path is empty",
			ref:         "FE-CC0008",
			hasFestival: true,
			emptyPath:   true,
		},
		{
			name:        "festival path supplied by the --festival strategy",
			ref:         "FE-CC0008",
			hasFestival: true,
			want:        position{Phase: "001", Sequence: "02"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The caller stands outside the festival, which is the --festival
			// case: only the progress store can supply the position.
			outside := t.TempDir()
			t.Chdir(outside)

			festival := festivalDir(t)
			writeProgress(t, festival, storedTask{
				key:    "001_IMPLEMENT/02_camp_pilot/01_task.md",
				status: progress.StatusInProgress,
			})

			path := festival
			if tt.emptyPath {
				path = ""
			}

			got := commitPosition(context.Background(), tt.ref, path, tt.hasFestival)
			if got != tt.want {
				t.Errorf("commitPosition() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
