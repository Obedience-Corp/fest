// Package standalone resolves workflow context outside of a festival.
//
// Resolution priority:
//  1. Festival context (delegates to shared.ResolveFestivalPath) — always wins
//     when both festival and standalone signals exist.
//  2. Tracked standalone workflow — a directory with .workflow/workflow.yaml.
//  3. Anonymous standalone workflow — a directory with WORKFLOW.md only.
//  4. None — neither signal found.
package standalone

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

// Mode identifies which workflow context was resolved.
type Mode string

const (
	// ModeNone means no workflow context was found.
	ModeNone Mode = "none"
	// ModeFestival means resolution found a fest festival workflow.
	ModeFestival Mode = "festival"
	// ModeTracked means a standalone .workflow/ runtime is present.
	ModeTracked Mode = "tracked"
	// ModeAnonymous means a WORKFLOW.md exists with no .workflow/ runtime.
	ModeAnonymous Mode = "anonymous"
)

// Result is the typed output of resolution. All paths are absolute.
type Result struct {
	Mode         Mode
	StartDir     string
	FestivalPath string // set when Mode == ModeFestival
	PhasePath    string // set when Mode == ModeFestival and inside a phase
	WorkflowDoc  string // absolute path to WORKFLOW.md, set for tracked/anonymous/festival-phase
	RuntimeDir   string // absolute path to .workflow/, set when Mode == ModeTracked
}

// Resolve walks from startDir to find the nearest workflow context.
// Festival context wins when both festival and standalone signals exist for
// the same path.
func Resolve(ctx context.Context, startDir string) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		return nil, festerrors.Wrap(err, "resolving start dir")
	}

	// Priority 1: festival context.
	if festPath, festErr := shared.ResolveFestivalPath(absStart, ""); festErr == nil && festPath != "" {
		phasePath := shared.ResolvePhasePath(absStart, festPath)
		workflowDoc := ""
		if phasePath != "" {
			candidate := filepath.Join(phasePath, "WORKFLOW.md")
			if _, statErr := os.Stat(candidate); statErr == nil {
				workflowDoc = candidate
			}
		}
		return &Result{
			Mode:         ModeFestival,
			StartDir:     absStart,
			FestivalPath: festPath,
			PhasePath:    phasePath,
			WorkflowDoc:  workflowDoc,
		}, nil
	}

	// Priority 2 + 3: standalone walk upward.
	return walkForStandalone(ctx, absStart)
}

func walkForStandalone(ctx context.Context, startDir string) (*Result, error) {
	dir := startDir
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		manifest := filepath.Join(dir, ".workflow", "workflow.yaml")
		if _, err := os.Stat(manifest); err == nil {
			return &Result{
				Mode:        ModeTracked,
				StartDir:    startDir,
				WorkflowDoc: filepath.Join(dir, "WORKFLOW.md"),
				RuntimeDir:  filepath.Join(dir, ".workflow"),
			}, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, festerrors.Wrap(err, "stat workflow manifest")
		}

		doc := filepath.Join(dir, "WORKFLOW.md")
		if _, err := os.Stat(doc); err == nil {
			return &Result{
				Mode:        ModeAnonymous,
				StartDir:    startDir,
				WorkflowDoc: doc,
			}, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, festerrors.Wrap(err, "stat workflow doc")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return &Result{Mode: ModeNone, StartDir: startDir}, nil
		}
		dir = parent
	}
}
