package progress

import (
	"testing"

	progresspkg "github.com/Obedience-Corp/fest/internal/progress"
)

func TestParsePercentage_Valid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"zero", "0", 0, false},
		{"zero_percent", "0%", 0, false},
		{"fifty", "50", 50, false},
		{"fifty_percent", "50%", 50, false},
		{"hundred", "100", 100, false},
		{"hundred_percent", "100%", 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePercentage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePercentage(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parsePercentage(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsePercentage_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"negative", "-1"},
		{"over_hundred", "101"},
		{"not_number", "abc"},
		{"empty", ""},
		{"decimal", "50.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePercentage(tt.input)
			if err == nil {
				t.Errorf("parsePercentage(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestStatusForProgress(t *testing.T) {
	tests := []struct {
		name     string
		progress int
		want     string
	}{
		{"zero_is_pending", 0, "pending"},
		{"partial_is_in_progress", 50, "in_progress"},
		{"almost_is_in_progress", 99, "in_progress"},
		{"hundred_is_completed", 100, "completed"},
		{"over_hundred_is_completed", 150, "completed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusForProgress(tt.progress)
			if got != tt.want {
				t.Errorf("statusForProgress(%d) = %q, want %q", tt.progress, got, tt.want)
			}
		})
	}
}

func TestValidateTaskOptions_Valid(t *testing.T) {
	tests := []struct {
		name string
		opts *progressOptions
	}{
		{"empty_options", &progressOptions{}},
		{"task_only", &progressOptions{taskID: "01_design.md"}},
		{"path_only", &progressOptions{taskPath: "/path/to/task.md"}},
		{"phase_sequence_task", &progressOptions{
			taskID:   "01_design.md",
			phase:    "002_FOUNDATION",
			sequence: "01_alpha",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskOptions(tt.opts)
			if err != nil {
				t.Errorf("validateTaskOptions() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateTaskOptions_Invalid(t *testing.T) {
	tests := []struct {
		name string
		opts *progressOptions
	}{
		{"path_and_task", &progressOptions{
			taskPath: "/path/to/task.md",
			taskID:   "01_design.md",
		}},
		{"path_and_phase", &progressOptions{
			taskPath: "/path/to/task.md",
			phase:    "002_FOUNDATION",
		}},
		{"phase_without_task", &progressOptions{
			phase:    "002_FOUNDATION",
			sequence: "01_alpha",
		}},
		{"phase_only", &progressOptions{
			phase: "002_FOUNDATION",
		}},
		{"sequence_only", &progressOptions{
			sequence: "01_alpha",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskOptions(tt.opts)
			if err == nil {
				t.Errorf("validateTaskOptions() expected error, got nil")
			}
		})
	}
}

func TestEnsureTaskProgress_WithTask(t *testing.T) {
	taskID := "test_task"
	existing := &progresspkg.TaskProgress{
		TaskID:   "existing",
		Status:   "completed",
		Progress: 100,
	}
	fallback := &progresspkg.TaskProgress{
		Status:   "pending",
		Progress: 0,
	}

	result := ensureTaskProgress(taskID, existing, fallback)
	if result != existing {
		t.Error("ensureTaskProgress() should return existing task when not nil")
	}
}

func TestEnsureTaskProgress_WithFallback(t *testing.T) {
	taskID := "test_task"
	fallback := &progresspkg.TaskProgress{
		Status:   "pending",
		Progress: 0,
	}

	result := ensureTaskProgress(taskID, nil, fallback)
	if result != fallback {
		t.Error("ensureTaskProgress() should return fallback when task is nil")
	}
	if result.TaskID != taskID {
		t.Errorf("ensureTaskProgress() should set TaskID, got %q", result.TaskID)
	}
}

func TestEnsureTaskProgress_WithNilFallback(t *testing.T) {
	taskID := "test_task"

	result := ensureTaskProgress(taskID, nil, nil)
	if result == nil {
		t.Fatal("ensureTaskProgress() should return non-nil result")
	}
	if result.TaskID != taskID {
		t.Errorf("ensureTaskProgress() TaskID = %q, want %q", result.TaskID, taskID)
	}
	if result.Status != "pending" {
		t.Errorf("ensureTaskProgress() Status = %q, want pending", result.Status)
	}
	if result.Progress != 0 {
		t.Errorf("ensureTaskProgress() Progress = %d, want 0", result.Progress)
	}
}
