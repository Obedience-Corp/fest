package guidance

import "fmt"

// Progress represents current progress through the guidance workflow.
type Progress struct {
	// Mode that generated this progress
	Mode Mode `json:"mode"`

	// Overall counts
	Completed  int     `json:"completed"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`

	// Current position
	CurrentPhase    string `json:"current_phase,omitempty"`
	CurrentSequence string `json:"current_sequence,omitempty"`
	CurrentTask     string `json:"current_task,omitempty"`

	// Status breakdown
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`

	// Phase-level progress
	PhaseProgress map[string]*PhaseProgressInfo `json:"phase_progress,omitempty"`
}

// PhaseProgressInfo holds progress information for a specific phase.
type PhaseProgressInfo struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"` // Phase type (implementation, planning, etc.)
	Number     int     `json:"number"`
	Completed  int     `json:"completed"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
	Status     string  `json:"status"` // pending, in_progress, completed

	// Sequence-level progress within this phase
	Sequences []*SequenceProgressInfo `json:"sequences,omitempty"`
}

// SequenceProgressInfo holds progress information for a sequence.
type SequenceProgressInfo struct {
	Name       string  `json:"name"`
	Number     int     `json:"number"`
	Completed  int     `json:"completed"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
	Status     string  `json:"status"`
}

// NewProgress creates a new Progress with initial values.
func NewProgress(mode Mode) *Progress {
	return &Progress{
		Mode:          mode,
		PhaseProgress: make(map[string]*PhaseProgressInfo),
	}
}

// Calculate updates the percentage based on completed/total.
func (p *Progress) Calculate() {
	if p.Total > 0 {
		p.Percentage = float64(p.Completed) / float64(p.Total) * 100
	}
}

// IsComplete returns true if all work is done.
func (p *Progress) IsComplete() bool {
	return p.Total > 0 && p.Completed >= p.Total
}

// IsStarted returns true if any work has begun.
func (p *Progress) IsStarted() bool {
	return p.Completed > 0 || p.InProgress > 0
}

// Remaining returns the number of remaining tasks.
func (p *Progress) Remaining() int {
	remaining := p.Total - p.Completed - p.Skipped
	if remaining < 0 {
		return 0
	}
	return remaining
}

// AddPhase adds or updates phase progress.
func (p *Progress) AddPhase(name string, info *PhaseProgressInfo) {
	if p.PhaseProgress == nil {
		p.PhaseProgress = make(map[string]*PhaseProgressInfo)
	}
	p.PhaseProgress[name] = info
}

// IncrementCompleted increments the completed count and recalculates.
func (p *Progress) IncrementCompleted() {
	p.Completed++
	p.Calculate()
}

// IncrementFailed increments the failed count.
func (p *Progress) IncrementFailed() {
	p.Failed++
}

// IncrementSkipped increments the skipped count.
func (p *Progress) IncrementSkipped() {
	p.Skipped++
}

// SetPosition updates the current position.
func (p *Progress) SetPosition(phase, sequence, task string) {
	p.CurrentPhase = phase
	p.CurrentSequence = sequence
	p.CurrentTask = task
}

// ProgressBar returns a simple text progress bar.
func (p *Progress) ProgressBar(width int) string {
	if width <= 0 {
		width = 20
	}

	filled := min(int(p.Percentage/100*float64(width)), width)

	bar := make([]byte, width)
	for i := 0; i < width; i++ {
		if i < filled {
			bar[i] = '='
		} else if i == filled {
			bar[i] = '>'
		} else {
			bar[i] = ' '
		}
	}

	return "[" + string(bar) + "]"
}

// Summary returns a one-line summary string.
func (p *Progress) Summary() string {
	if p.Total == 0 {
		return "No tasks"
	}
	return fmt.Sprintf("%.1f%% (%d/%d tasks)", p.Percentage, p.Completed, p.Total)
}
