// Package standalone resolves workflow context outside of a festival.
//
// Resolution priority:
//  1. Festival context (delegates to shared.ResolveFestivalPath). Wins
//     unconditionally when both festival and standalone signals exist.
//     Festival detection walks up parent directories.
//  2. Tracked standalone workflow: a directory with .workflow/workflow.yaml.
//     Detected at cwd only (D013) — does not walk up.
//  3. Anonymous standalone workflow: a directory with WORKFLOW.md only.
//     Detected at cwd only (D013).
//  4. None: neither signal found.
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

// resolveFestivalPath is a package-level indirection over
// shared.ResolveFestivalPath so tests can inject failures (permission
// denied, I/O) without mutating the host filesystem. Production code
// always uses the real implementation.
var resolveFestivalPath = shared.ResolveFestivalPath

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
	festPath, festErr := resolveFestivalPath(absStart, "")
	if festErr == nil && festPath != "" {
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
	// A NotFound error means "not inside a festival" — fall through to
	// standalone resolution. Any other error (permission denied, I/O failure
	// while walking) is real and must propagate so the caller does not
	// silently degrade to ModeAnonymous/ModeNone.
	if festErr != nil && !festerrors.Is(festErr, festerrors.ErrCodeNotFound) {
		return nil, festerrors.Wrap(festErr, "resolving festival path")
	}

	// Priority 2 + 3: standalone cwd-only check (D013). Unlike festival
	// detection, standalone WORKFLOW.md detection does not walk up parent
	// directories — `fest next` from a child directory of a workflow does
	// not pick it up.
	return checkCwdForStandalone(ctx, absStart)
}

func checkCwdForStandalone(ctx context.Context, startDir string) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	manifest := filepath.Join(startDir, ".workflow", "workflow.yaml")
	if _, err := os.Stat(manifest); err == nil {
		return &Result{
			Mode:        ModeTracked,
			StartDir:    startDir,
			WorkflowDoc: filepath.Join(startDir, "WORKFLOW.md"),
			RuntimeDir:  filepath.Join(startDir, ".workflow"),
		}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, festerrors.Wrap(err, "stat workflow manifest")
	}

	doc := filepath.Join(startDir, "WORKFLOW.md")
	if _, err := os.Stat(doc); err == nil {
		return &Result{
			Mode:        ModeAnonymous,
			StartDir:    startDir,
			WorkflowDoc: doc,
		}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, festerrors.Wrap(err, "stat workflow doc")
	}

	return &Result{Mode: ModeNone, StartDir: startDir}, nil
}
