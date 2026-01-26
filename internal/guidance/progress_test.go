package guidance

import (
	"testing"
)

func TestNewProgress(t *testing.T) {
	p := NewProgress(ModeImplementation)

	if p.Mode != ModeImplementation {
		t.Errorf("Mode = %v, want ModeImplementation", p.Mode)
	}
	if p.PhaseProgress == nil {
		t.Error("PhaseProgress should not be nil")
	}
	if p.Completed != 0 {
		t.Errorf("Completed = %d, want 0", p.Completed)
	}
	if p.Total != 0 {
		t.Errorf("Total = %d, want 0", p.Total)
	}
}

func TestProgress_Calculate(t *testing.T) {
	tests := []struct {
		name       string
		completed  int
		total      int
		wantApprox float64
	}{
		{"zero total", 0, 0, 0},
		{"50%", 5, 10, 50.0},
		{"100%", 10, 10, 100.0},
		{"33.3%", 1, 3, 33.33},
		{"25%", 1, 4, 25.0},
		{"0%", 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Completed = tt.completed
			p.Total = tt.total
			p.Calculate()
			// Use approximate comparison for floating point
			diff := p.Percentage - tt.wantApprox
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				t.Errorf("Percentage = %v, want approximately %v", p.Percentage, tt.wantApprox)
			}
		})
	}
}

func TestProgress_IsComplete(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		total     int
		want      bool
	}{
		{"not started", 0, 10, false},
		{"in progress", 5, 10, false},
		{"complete", 10, 10, true},
		{"over complete", 11, 10, true},
		{"no tasks", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Completed = tt.completed
			p.Total = tt.total
			if got := p.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgress_IsStarted(t *testing.T) {
	tests := []struct {
		name       string
		completed  int
		inProgress int
		want       bool
	}{
		{"nothing started", 0, 0, false},
		{"has completed", 1, 0, true},
		{"has in progress", 0, 1, true},
		{"has both", 1, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Completed = tt.completed
			p.InProgress = tt.inProgress
			if got := p.IsStarted(); got != tt.want {
				t.Errorf("IsStarted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgress_Remaining(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		completed int
		skipped   int
		want      int
	}{
		{"normal case", 10, 5, 2, 3},
		{"all complete", 10, 10, 0, 0},
		{"negative clamps to zero", 10, 8, 5, 0},
		{"zero total", 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Total = tt.total
			p.Completed = tt.completed
			p.Skipped = tt.skipped
			if got := p.Remaining(); got != tt.want {
				t.Errorf("Remaining() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgress_AddPhase(t *testing.T) {
	p := NewProgress(ModeImplementation)
	info := &PhaseProgressInfo{
		Name:       "Implementation",
		Type:       "implementation",
		Number:     1,
		Completed:  5,
		Total:      15,
		Percentage: 33.3,
		Status:     "in_progress",
	}

	p.AddPhase("001_Implementation", info)

	if len(p.PhaseProgress) != 1 {
		t.Errorf("PhaseProgress count = %d, want 1", len(p.PhaseProgress))
	}
	if p.PhaseProgress["001_Implementation"] != info {
		t.Error("PhaseProgress should contain the added info")
	}
}

func TestProgress_AddPhase_NilMap(t *testing.T) {
	p := &Progress{Mode: ModeImplementation}
	info := &PhaseProgressInfo{Name: "Test"}

	// Should not panic with nil map
	p.AddPhase("test", info)

	if p.PhaseProgress == nil {
		t.Error("AddPhase should initialize PhaseProgress map")
	}
	if p.PhaseProgress["test"] != info {
		t.Error("AddPhase should add the info")
	}
}

func TestProgress_IncrementCompleted(t *testing.T) {
	p := NewProgress(ModeImplementation)
	p.Total = 10
	p.Completed = 0

	p.IncrementCompleted()

	if p.Completed != 1 {
		t.Errorf("Completed = %d, want 1", p.Completed)
	}
	if p.Percentage != 10.0 {
		t.Errorf("Percentage = %v, want 10.0", p.Percentage)
	}
}

func TestProgress_IncrementFailed(t *testing.T) {
	p := NewProgress(ModeImplementation)
	p.Failed = 0

	p.IncrementFailed()

	if p.Failed != 1 {
		t.Errorf("Failed = %d, want 1", p.Failed)
	}
}

func TestProgress_IncrementSkipped(t *testing.T) {
	p := NewProgress(ModeImplementation)
	p.Skipped = 0

	p.IncrementSkipped()

	if p.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", p.Skipped)
	}
}

func TestProgress_SetPosition(t *testing.T) {
	p := NewProgress(ModeImplementation)
	p.SetPosition("Phase1", "Sequence1", "Task1")

	if p.CurrentPhase != "Phase1" {
		t.Errorf("CurrentPhase = %v, want Phase1", p.CurrentPhase)
	}
	if p.CurrentSequence != "Sequence1" {
		t.Errorf("CurrentSequence = %v, want Sequence1", p.CurrentSequence)
	}
	if p.CurrentTask != "Task1" {
		t.Errorf("CurrentTask = %v, want Task1", p.CurrentTask)
	}
}

func TestProgress_ProgressBar(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		width      int
		want       string
	}{
		{"50% width 10", 50.0, 10, "[=====>    ]"},
		{"0% width 10", 0.0, 10, "[>         ]"},
		{"100% width 10", 100.0, 10, "[==========]"},
		{"25% width 8", 25.0, 8, "[==>     ]"},
		{"default width", 50.0, 0, "[==========>         ]"}, // default 20
		{"negative width uses default", 50.0, -5, "[==========>         ]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Percentage = tt.percentage
			got := p.ProgressBar(tt.width)
			if got != tt.want {
				t.Errorf("ProgressBar(%d) = %q, want %q", tt.width, got, tt.want)
			}
		})
	}
}

func TestProgress_ProgressBar_OverFilled(t *testing.T) {
	p := NewProgress(ModeImplementation)
	p.Percentage = 150.0 // Over 100%

	bar := p.ProgressBar(10)
	// Should cap at full
	if bar != "[==========]" {
		t.Errorf("ProgressBar with 150%% = %q, want [==========]", bar)
	}
}

func TestProgress_Summary(t *testing.T) {
	tests := []struct {
		name       string
		total      int
		completed  int
		percentage float64
		want       string
	}{
		{"no tasks", 0, 0, 0, "No tasks"},
		{"50%", 10, 5, 50.0, "50.0% (5/10 tasks)"},
		{"100%", 10, 10, 100.0, "100.0% (10/10 tasks)"},
		{"33.3%", 3, 1, 33.3, "33.3% (1/3 tasks)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(ModeImplementation)
			p.Total = tt.total
			p.Completed = tt.completed
			p.Percentage = tt.percentage
			got := p.Summary()
			if got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPhaseProgressInfo(t *testing.T) {
	info := &PhaseProgressInfo{
		Name:       "Implementation",
		Type:       "implementation",
		Number:     1,
		Completed:  5,
		Total:      10,
		Percentage: 50.0,
		Status:     "in_progress",
		Sequences: []*SequenceProgressInfo{
			{
				Name:       "core",
				Number:     1,
				Completed:  3,
				Total:      5,
				Percentage: 60.0,
				Status:     "in_progress",
			},
		},
	}

	if info.Name != "Implementation" {
		t.Errorf("Name = %v, want Implementation", info.Name)
	}
	if len(info.Sequences) != 1 {
		t.Errorf("Sequences count = %d, want 1", len(info.Sequences))
	}
	if info.Sequences[0].Name != "core" {
		t.Errorf("Sequence name = %v, want core", info.Sequences[0].Name)
	}
}

func TestSequenceProgressInfo(t *testing.T) {
	info := &SequenceProgressInfo{
		Name:       "core",
		Number:     1,
		Completed:  3,
		Total:      5,
		Percentage: 60.0,
		Status:     "in_progress",
	}

	if info.Name != "core" {
		t.Errorf("Name = %v, want core", info.Name)
	}
	if info.Number != 1 {
		t.Errorf("Number = %d, want 1", info.Number)
	}
	if info.Completed != 3 {
		t.Errorf("Completed = %d, want 3", info.Completed)
	}
}
