package status

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/id"
	replay "github.com/Obedience-Corp/fest/pkg/festgif/festival"
)

// CompletionReplayResult reports the optional artifact separately from the
// successful lifecycle move, including in machine-readable output.
type CompletionReplayResult struct {
	Path    string `json:"path,omitempty"`
	Warning string `json:"warning,omitempty"`
}

// GenerateCompletionReplay runs after metadata updates and before the status
// commit in both completion commands. Rendering failure must not undo completed
// work; stderr and the JSON result describe the failure and how to retry it.
func GenerateCompletionReplay(ctx context.Context, festivalPath, newStatus string) CompletionReplayResult {
	if id.ResolveStatusPath(newStatus) != "dungeon/completed" {
		return CompletionReplayResult{}
	}
	if _, _, err := replay.Embed(ctx, festivalPath, 1); err != nil {
		warning := fmt.Sprintf("Festival completed, but its replay could not be embedded: %v. Retry from the festival directory with: fest gif --embed", err)
		fmt.Fprintln(os.Stderr, "Warning: "+warning)
		return CompletionReplayResult{Warning: warning}
	}
	return CompletionReplayResult{Path: filepath.Join(festivalPath, replay.ReplayFilename)}
}
