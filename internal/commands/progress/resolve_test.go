package progress

import (
	"testing"
)

func TestResolveTaskPath_Absolute(t *testing.T) {
	absPath := "/absolute/path/to/task.md"
	got, err := resolveTaskPath(absPath, "/festival", "/cwd")
	if err != nil {
		t.Fatalf("resolveTaskPath() error = %v", err)
	}
	if got != absPath {
		t.Errorf("resolveTaskPath() = %q, want %q", got, absPath)
	}
}

func TestResolveTaskPath_RelativeWithFestival(t *testing.T) {
	got, err := resolveTaskPath("phase/seq/task.md", "/festival", "/cwd")
	if err != nil {
		t.Fatalf("resolveTaskPath() error = %v", err)
	}
	want := "/festival/phase/seq/task.md"
	if got != want {
		t.Errorf("resolveTaskPath() = %q, want %q", got, want)
	}
}

func TestResolveTaskPath_RelativeWithoutFestival(t *testing.T) {
	got, err := resolveTaskPath("phase/seq/task.md", "", "/cwd")
	if err != nil {
		t.Fatalf("resolveTaskPath() error = %v", err)
	}
	want := "/cwd/phase/seq/task.md"
	if got != want {
		t.Errorf("resolveTaskPath() = %q, want %q", got, want)
	}
}

func TestResolveTaskPath_Empty(t *testing.T) {
	_, err := resolveTaskPath("", "/festival", "/cwd")
	if err == nil {
		t.Error("resolveTaskPath() expected error for empty path")
	}
}

func TestEnsureMarkdownFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already_has_md", "task.md", "task.md"},
		{"needs_md", "task", "task.md"},
		{"empty", "", ".md"},
		{"with_underscore", "01_design", "01_design.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureMarkdownFilename(tt.input)
			if got != tt.want {
				t.Errorf("ensureMarkdownFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsTaskFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid_underscore", "01_design.md", true},
		{"valid_dot", "01.design.md", true},
		{"valid_two_digit", "99_final.md", true},
		{"no_prefix", "design.md", false},
		{"single_digit", "1_design.md", false},
		{"not_md", "01_design.txt", false},
		{"uppercase", "01_DESIGN.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTaskFile(tt.input)
			if got != tt.want {
				t.Errorf("isTaskFile(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveTaskLocationPaths(t *testing.T) {
	tests := []struct {
		name         string
		festivalPath string
		taskID       string
		wantPhase    string
		wantSequence string
	}{
		{
			name:         "full_path",
			festivalPath: "/festival",
			taskID:       "002_FOUNDATION/01_alpha/01_design.md",
			wantPhase:    "/festival/002_FOUNDATION",
			wantSequence: "/festival/002_FOUNDATION/01_alpha",
		},
		{
			name:         "phase_only",
			festivalPath: "/festival",
			taskID:       "002_FOUNDATION",
			wantPhase:    "/festival/002_FOUNDATION",
			wantSequence: "",
		},
		{
			name:         "phase_and_seq",
			festivalPath: "/festival",
			taskID:       "002_FOUNDATION/01_alpha",
			wantPhase:    "/festival/002_FOUNDATION",
			wantSequence: "/festival/002_FOUNDATION/01_alpha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPhase, gotSeq := resolveTaskLocationPaths(tt.festivalPath, tt.taskID)
			if gotPhase != tt.wantPhase {
				t.Errorf("resolveTaskLocationPaths() phasePath = %q, want %q", gotPhase, tt.wantPhase)
			}
			if gotSeq != tt.wantSequence {
				t.Errorf("resolveTaskLocationPaths() sequencePath = %q, want %q", gotSeq, tt.wantSequence)
			}
		})
	}
}
