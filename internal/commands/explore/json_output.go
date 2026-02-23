package explore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// ExploreOutput is the top-level JSON output structure for fest explore --json.
type ExploreOutput struct {
	Status    string         `json:"status"`
	Count     int            `json:"count"`
	Festivals []FestivalJSON `json:"festivals"`
}

// FestivalJSON represents a festival in JSON output.
type FestivalJSON struct {
	Name     string      `json:"name"`
	Status   string      `json:"status"`
	Progress float64     `json:"progress"`
	Path     string      `json:"path"`
	Phases   []PhaseJSON `json:"phases,omitempty"`
}

// PhaseJSON represents a phase in JSON output.
type PhaseJSON struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Sequences []SequenceJSON `json:"sequences,omitempty"`
}

// SequenceJSON represents a sequence in JSON output.
type SequenceJSON struct {
	Name  string     `json:"name"`
	Path  string     `json:"path"`
	Tasks []TaskJSON `json:"tasks,omitempty"`
}

// TaskJSON represents a task in JSON output.
type TaskJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// outputExploreJSON outputs structured JSON for the explore command.
func outputExploreJSON(ctx context.Context, status string) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting working directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return errors.NotFound("festivals directory").
			WithHint("Run from inside a festivals workspace")
	}

	statuses := []string{status}
	if status == "" {
		statuses = id.StatusDirectories
	}

	parser := festival.NewParser()
	var allFestivals []FestivalJSON

	for _, s := range statuses {
		festivals, loadErr := show.ListFestivalsByStatus(ctx, festivalsDir, s)
		if loadErr != nil {
			continue
		}
		for _, f := range festivals {
			fj := festivalToJSON(ctx, parser, f)
			allFestivals = append(allFestivals, fj)
		}
	}

	output := ExploreOutput{
		Status:    status,
		Count:     len(allFestivals),
		Festivals: allFestivals,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return errors.IO("encoding JSON", err)
	}
	return nil
}

func festivalToJSON(ctx context.Context, parser *festival.Parser, f *show.FestivalInfo) FestivalJSON {
	var progress float64
	if f.Stats != nil {
		progress = f.Stats.Progress
	}

	// Get campaign root once for all path conversions
	campaignRoot, _ := workspace.DetectCampaign(ctx, "")
	displayPath := func(p string) string {
		if campaignRoot != "" {
			return pathutil.DisplayPath(p, campaignRoot)
		}
		return p
	}

	fj := FestivalJSON{
		Name:     f.Name,
		Status:   f.Status,
		Progress: progress,
		Path:     displayPath(f.Path),
	}

	phases, err := parser.ParsePhases(ctx, f.Path)
	if err != nil {
		return fj
	}

	for _, phase := range phases {
		pj := PhaseJSON{
			Name: phase.FullName,
			Path: displayPath(phase.Path),
		}

		sequences, seqErr := parser.ParseSequences(ctx, phase.Path)
		if seqErr != nil {
			fj.Phases = append(fj.Phases, pj)
			continue
		}

		for _, seq := range sequences {
			sj := SequenceJSON{
				Name: seq.FullName,
				Path: displayPath(seq.Path),
			}

			tasks, taskErr := parser.ParseTasks(ctx, seq.Path)
			if taskErr == nil {
				for _, task := range tasks {
					sj.Tasks = append(sj.Tasks, TaskJSON{
						Name: task.FullName,
						Path: displayPath(task.Path),
					})
				}
			}

			pj.Sequences = append(pj.Sequences, sj)
		}

		fj.Phases = append(fj.Phases, pj)
	}

	return fj
}

// outputFestivalJSON outputs JSON for a single festival's hierarchy.
func outputFestivalJSON(ctx context.Context, festivalPath string) error {
	parser := festival.NewParser()
	info := &show.FestivalInfo{
		Name: filepath.Base(festivalPath),
		Path: festivalPath,
	}

	fj := festivalToJSON(ctx, parser, info)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(fj)
}
