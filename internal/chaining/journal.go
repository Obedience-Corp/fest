package chaining

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const chainJournalVersion = 1

type chainJournal struct {
	Version            int    `json:"version"`
	PendingName        string `json:"pending_name"`
	PendingType        string `json:"pending_type"`
	CompletedPhasePath string `json:"completed_phase_path"`
	CompletedPhaseType string `json:"completed_phase_type"`
	PhaseDir           string `json:"phase_dir,omitempty"`
}

func chainJournalDir(festivalPath string) string {
	return filepath.Join(festivalPath, ".fest", "chaining")
}

func chainJournalPath(festivalPath, pendingName string) string {
	// Pending phase names come from fest.yaml. Hashing keeps the journal under
	// .fest/chaining even when a malformed name contains path separators.
	id := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(pendingName))))
	return filepath.Join(chainJournalDir(festivalPath), fmt.Sprintf("%x.json", id))
}

func loadChainJournal(festivalPath, pendingName string) (*chainJournal, error) {
	path := chainJournalPath(festivalPath, pendingName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read chain journal: %w", err)
	}
	var j chainJournal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("parse chain journal: %w", err)
	}
	if j.Version != chainJournalVersion || !strings.EqualFold(j.PendingName, pendingName) {
		return nil, fmt.Errorf("chain journal for %s has an unexpected version or phase name", pendingName)
	}
	if j.PhaseDir != "" {
		festivalAbs, festErr := filepath.Abs(festivalPath)
		phaseAbs, phaseErr := filepath.Abs(j.PhaseDir)
		if festErr != nil || phaseErr != nil {
			return nil, fmt.Errorf("resolve chain journal phase directory: festival=%v phase=%v", festErr, phaseErr)
		}
		rel, relErr := filepath.Rel(festivalAbs, phaseAbs)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("chain journal phase directory escapes festival: %s", j.PhaseDir)
		}
	}
	return &j, nil
}

func writeChainJournal(festivalPath string, j *chainJournal) error {
	if j == nil {
		return fmt.Errorf("chain journal is nil")
	}
	dir := chainJournalDir(festivalPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chain journal directory: %w", err)
	}
	j.Version = chainJournalVersion
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal chain journal: %w", err)
	}
	data = append(data, '\n')

	path := chainJournalPath(festivalPath, j.PendingName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write chain journal: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace chain journal: %w", err)
	}
	return nil
}

func clearChainJournal(festivalPath, pendingName string) error {
	path := chainJournalPath(festivalPath, pendingName)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func newChainJournal(target *ChainTarget) (*chainJournal, error) {
	completedAbs, err := filepath.Abs(target.CompletedPhasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve completed phase path: %w", err)
	}
	return &chainJournal{
		PendingName:        target.PendingPhase.Name,
		PendingType:        target.PendingPhase.Type,
		CompletedPhasePath: completedAbs,
		CompletedPhaseType: target.CompletedPhaseType,
	}, nil
}

func journalMatchesTarget(j *chainJournal, target *ChainTarget) (bool, error) {
	if j == nil {
		return false, nil
	}
	completedAbs, err := filepath.Abs(target.CompletedPhasePath)
	if err != nil {
		return false, fmt.Errorf("resolve completed phase path: %w", err)
	}
	journalCompleted, err := filepath.Abs(j.CompletedPhasePath)
	if err != nil {
		return false, fmt.Errorf("resolve journal completed phase path: %w", err)
	}
	if !strings.EqualFold(j.PendingName, target.PendingPhase.Name) {
		return false, nil
	}
	if !strings.EqualFold(j.PendingType, target.PendingPhase.Type) {
		return false, nil
	}
	if journalCompleted != completedAbs {
		same := runtime.GOOS == "windows" && strings.EqualFold(journalCompleted, completedAbs)
		if !same {
			journalInfo, journalErr := os.Stat(journalCompleted)
			completedInfo, completedErr := os.Stat(completedAbs)
			same = journalErr == nil && completedErr == nil && os.SameFile(journalInfo, completedInfo)
		}
		if !same {
			return false, nil
		}
	}
	if !strings.EqualFold(j.CompletedPhaseType, target.CompletedPhaseType) {
		return false, nil
	}
	return true, nil
}
