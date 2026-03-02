package gates

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
)

// resolvePaths resolves festival, phase, and sequence paths from flags or cwd.
// Uses shared.ResolveFestivalPath which handles both linked festivals and direct detection.
func resolvePaths(festivalsRoot, cwd, phase, sequence string) (festivalPath, phasePath, sequencePath string, err error) {
	// Use shared resolution which handles both linked festivals and direct detection
	festivalPath, _ = shared.ResolveFestivalPath(cwd, "")

	if festivalPath == "" {
		return "", "", "", errors.NotFound("festival").
			WithHint("Run from within a festival or link a festival with 'fest link'")
	}

	if sequence != "" {
		// Parse sequence as "phase/sequence"
		parts := strings.SplitN(sequence, "/", 2)
		if len(parts) != 2 {
			return "", "", "", errors.Validation("sequence must be in format 'phase/sequence'").
				WithField("sequence", sequence)
		}
		phasePath = filepath.Join(festivalPath, parts[0])
		sequencePath = filepath.Join(phasePath, parts[1])

		// Verify paths exist
		if _, err := os.Stat(phasePath); os.IsNotExist(err) {
			return "", "", "", errors.NotFound("phase").WithField("phase", parts[0])
		}
		if _, err := os.Stat(sequencePath); os.IsNotExist(err) {
			return "", "", "", errors.NotFound("sequence").WithField("sequence", sequence)
		}
	} else if phase != "" {
		phasePath = filepath.Join(festivalPath, phase)
		if _, err := os.Stat(phasePath); os.IsNotExist(err) {
			return "", "", "", errors.NotFound("phase").WithField("phase", phase)
		}
	}

	return festivalPath, phasePath, sequencePath, nil
}

// gateOutput is the JSON output format for a gate.
type gateOutput struct {
	ID       string `json:"id"`
	Template string `json:"template"`
	Name     string `json:"name,omitempty"`
	Enabled  bool   `json:"enabled"`
	Removed  bool   `json:"removed,omitempty"`
	Source   string `json:"source,omitempty"`
}

// sourceOutput is the JSON output format for a policy source.
type sourceOutput struct {
	Level string `json:"level"`
	Path  string `json:"path,omitempty"`
	Name  string `json:"name,omitempty"`
}
