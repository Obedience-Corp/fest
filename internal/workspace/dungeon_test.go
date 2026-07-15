package workspace

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStderr redirects os.Stderr for the duration of fn and returns
// whatever was written to it. Used to assert on the one-time dungeon
// conflict warning without depending on log/output package internals.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = original }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stderr: %v", err)
	}
	return string(data)
}

func TestResolveDungeonDir(t *testing.T) {
	tests := []struct {
		name       string
		makeDirs   []string // directories to create under festivalsRoot before resolving
		want       string
		wantWarned bool
	}{
		{
			name:     "neither exists defaults to visible",
			makeDirs: nil,
			want:     DungeonDir,
		},
		{
			name:     "only visible exists",
			makeDirs: []string{DungeonDir},
			want:     DungeonDir,
		},
		{
			name:     "only hidden exists",
			makeDirs: []string{HiddenDungeonDir},
			want:     HiddenDungeonDir,
		},
		{
			name:       "both exist prefers visible and warns",
			makeDirs:   []string{DungeonDir, HiddenDungeonDir},
			want:       DungeonDir,
			wantWarned: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalsRoot := t.TempDir()
			for _, dir := range tt.makeDirs {
				if err := os.MkdirAll(filepath.Join(festivalsRoot, dir), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			stderrPath := captureStderr(t, func() {
				got := ResolveDungeonDir(festivalsRoot)
				if got != tt.want {
					t.Errorf("ResolveDungeonDir() = %q, want %q", got, tt.want)
				}
			})

			warned := strings.Contains(stderrPath, "both")
			if warned != tt.wantWarned {
				t.Errorf("warning printed = %v, want %v (stderr: %q)", warned, tt.wantWarned, stderrPath)
			}
		})
	}
}

func TestResolveDungeonDir_WarnsOnlyOnce(t *testing.T) {
	festivalsRoot := t.TempDir()
	for _, dir := range []string{DungeonDir, HiddenDungeonDir} {
		if err := os.MkdirAll(filepath.Join(festivalsRoot, dir), 0755); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	stderrPath := captureStderr(t, func() {
		for i := 0; i < 3; i++ {
			if got := ResolveDungeonDir(festivalsRoot); got != DungeonDir {
				t.Fatalf("call %d: ResolveDungeonDir() = %q, want %q", i, got, DungeonDir)
			}
		}
	})

	if count := strings.Count(stderrPath, "both"); count != 1 {
		t.Errorf("warning printed %d times, want exactly 1 (stderr: %q)", count, stderrPath)
	}
}

func TestCheckDungeonConflict(t *testing.T) {
	tests := []struct {
		name     string
		makeDirs []string
		wantErr  bool
	}{
		{
			name:     "both exist errors",
			makeDirs: []string{DungeonDir, HiddenDungeonDir},
			wantErr:  true,
		},
		{
			name:     "only visible exists",
			makeDirs: []string{DungeonDir},
			wantErr:  false,
		},
		{
			name:     "only hidden exists",
			makeDirs: []string{HiddenDungeonDir},
			wantErr:  false,
		},
		{
			name:     "neither exists",
			makeDirs: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalsRoot := t.TempDir()
			for _, dir := range tt.makeDirs {
				if err := os.MkdirAll(filepath.Join(festivalsRoot, dir), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}

			err := CheckDungeonConflict(festivalsRoot)
			if tt.wantErr && err == nil {
				t.Fatal("CheckDungeonConflict() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CheckDungeonConflict() = %v, want nil", err)
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), DungeonDir) || !strings.Contains(err.Error(), HiddenDungeonDir) {
					t.Errorf("error should name both spellings, got: %v", err)
				}
				if !strings.Contains(err.Error(), "camp dungeon migrate") {
					t.Errorf("error should hint at the migration command, got: %v", err)
				}
			}
		})
	}
}

func TestIsDungeonDirName(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{name: "visible spelling", dir: "dungeon", want: true},
		{name: "hidden spelling", dir: ".dungeon", want: true},
		{name: "unrelated directory", dir: "active", want: false},
		{name: "empty string", dir: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDungeonDirName(tt.dir); got != tt.want {
				t.Errorf("IsDungeonDirName(%q) = %v, want %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestJoinDungeon(t *testing.T) {
	tests := []struct {
		name     string
		makeDirs []string
		elem     []string
		want     func(root string) string
	}{
		{
			name:     "defaults to visible when neither exists",
			makeDirs: nil,
			elem:     []string{"completed"},
			want: func(root string) string {
				return filepath.Join(root, "dungeon", "completed")
			},
		},
		{
			name:     "resolves to hidden when only hidden exists",
			makeDirs: []string{HiddenDungeonDir},
			elem:     []string{"completed", "2026-01-01"},
			want: func(root string) string {
				return filepath.Join(root, ".dungeon", "completed", "2026-01-01")
			},
		},
		{
			name:     "no additional elements",
			makeDirs: []string{HiddenDungeonDir},
			elem:     nil,
			want: func(root string) string {
				return filepath.Join(root, ".dungeon")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range tt.makeDirs {
				if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			got := JoinDungeon(root, tt.elem...)
			if want := tt.want(root); got != want {
				t.Errorf("JoinDungeon() = %q, want %q", got, want)
			}
		})
	}
}

func TestJoinStatus(t *testing.T) {
	tests := []struct {
		name     string
		makeDirs []string
		status   string
		want     func(root string) string
	}{
		{
			name:   "non-dungeon status passes through",
			status: "active",
			want: func(root string) string {
				return filepath.Join(root, "active")
			},
		},
		{
			name:   "bare dungeon status defaults to visible",
			status: "dungeon",
			want: func(root string) string {
				return filepath.Join(root, "dungeon")
			},
		},
		{
			name:     "nested dungeon status resolves hidden spelling",
			makeDirs: []string{HiddenDungeonDir},
			status:   "dungeon/completed",
			want: func(root string) string {
				return filepath.Join(root, ".dungeon", "completed")
			},
		},
		{
			name:   "status merely prefixed by the word dungeon is not treated as nested",
			status: "dungeonesque",
			want: func(root string) string {
				return filepath.Join(root, "dungeonesque")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range tt.makeDirs {
				if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			got := JoinStatus(root, tt.status)
			if want := tt.want(root); got != want {
				t.Errorf("JoinStatus(%q) = %q, want %q", tt.status, got, want)
			}
		})
	}
}
