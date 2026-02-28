package chain

import "context"

// FestivalStatus represents the lifecycle state of a single festival.
type FestivalStatus string

const (
	FestivalPlanning  FestivalStatus = "planning"
	FestivalReady     FestivalStatus = "ready"
	FestivalActive    FestivalStatus = "active"
	FestivalCompleted FestivalStatus = "completed"
)

// ChainProgress holds computed progress information for a chain.
type ChainProgress struct {
	State       ChainStatus
	Completed   int
	Total       int
	Percentage  float64
	WaveStatus  map[int]WaveProgress
	Unblocked   []string // refs that can now start
}

// WaveProgress holds the progress of a single wave.
type WaveProgress struct {
	WaveID    int
	Name      string
	State     string // "pending", "active", "completed"
	Completed int
	Total     int
}

// ComputeProgress determines the chain's current progress based on festival statuses.
// The statuses map is keyed by festival ref.
func ComputeProgress(ctx context.Context, c *Chain, statuses map[string]FestivalStatus) (*ChainProgress, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	progress := &ChainProgress{
		Total:      len(c.Festivals),
		WaveStatus: make(map[int]WaveProgress),
	}

	// Count completed festivals.
	for _, f := range c.Festivals {
		if statuses[f.Ref] == FestivalCompleted {
			progress.Completed++
		}
	}

	if progress.Total > 0 {
		progress.Percentage = float64(progress.Completed) / float64(progress.Total) * 100
	}

	// Determine overall state.
	switch {
	case progress.Completed == progress.Total:
		progress.State = StatusCompleted
	case progress.Completed > 0 || hasActiveStatus(statuses):
		progress.State = StatusActive
	default:
		progress.State = StatusPlanning
	}

	// Compute wave progress.
	for _, w := range c.Waves {
		wp := WaveProgress{
			WaveID: w.ID,
			Name:   w.Name,
			Total:  len(w.Festivals),
		}
		allCompleted := true
		anyActive := false
		for _, ref := range w.Festivals {
			s := statuses[ref]
			if s == FestivalCompleted {
				wp.Completed++
			} else {
				allCompleted = false
			}
			if s == FestivalActive || s == FestivalReady {
				anyActive = true
			}
		}
		switch {
		case allCompleted:
			wp.State = "completed"
		case anyActive || wp.Completed > 0:
			wp.State = "active"
		default:
			wp.State = "pending"
		}
		progress.WaveStatus[w.ID] = wp
	}

	// Find unblocked festivals: all hard upstream deps are completed.
	progress.Unblocked = findUnblocked(c, statuses)

	return progress, nil
}

// findUnblocked returns festivals whose hard dependencies are all completed
// and that haven't started yet.
func findUnblocked(c *Chain, statuses map[string]FestivalStatus) []string {
	// Build hard-dep map: ref -> list of upstream refs with hard edges.
	hardDeps := make(map[string][]string)
	for _, e := range c.Edges {
		if e.Type == EdgeHard {
			hardDeps[e.To] = append(hardDeps[e.To], e.From)
		}
	}

	var unblocked []string
	for _, f := range c.Festivals {
		status := statuses[f.Ref]
		if status == FestivalActive || status == FestivalCompleted {
			continue // already started
		}

		allDepsMet := true
		for _, dep := range hardDeps[f.Ref] {
			if statuses[dep] != FestivalCompleted {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			unblocked = append(unblocked, f.Ref)
		}
	}

	return unblocked
}

func hasActiveStatus(statuses map[string]FestivalStatus) bool {
	for _, s := range statuses {
		if s == FestivalActive || s == FestivalReady {
			return true
		}
	}
	return false
}
