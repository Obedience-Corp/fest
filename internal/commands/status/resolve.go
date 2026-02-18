package status

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// resolveTaskID normalizes a task identifier to a festival-relative path.
func resolveTaskID(festivalPath, cwd, taskInput string) (string, error) {
	// If it's already a full path within the festival, extract relative part
	if strings.HasPrefix(taskInput, festivalPath) {
		return strings.TrimPrefix(taskInput, festivalPath+"/"), nil
	}

	// If it's a relative path starting with ./ or ../
	if strings.HasPrefix(taskInput, "./") || strings.HasPrefix(taskInput, "../") {
		absPath := filepath.Join(cwd, taskInput)
		if strings.HasPrefix(absPath, festivalPath) {
			return strings.TrimPrefix(absPath, festivalPath+"/"), nil
		}
		return "", errors.Validation("path is outside festival").
			WithField("path", taskInput).
			WithField("festival", festivalPath)
	}

	// If it looks like a phase/sequence/task path (e.g., 001/01/01_task.md)
	if strings.Contains(taskInput, "/") || strings.HasSuffix(taskInput, ".md") {
		// Verify it exists
		fullPath := filepath.Join(festivalPath, taskInput)
		if _, err := os.Stat(fullPath); err == nil {
			return taskInput, nil
		}
	}

	// Try to find in current directory context
	// If cwd is within festival, try appending task name
	if strings.HasPrefix(cwd, festivalPath) {
		relCwd := strings.TrimPrefix(cwd, festivalPath+"/")
		testPath := filepath.Join(relCwd, taskInput)
		fullPath := filepath.Join(festivalPath, testPath)
		if _, err := os.Stat(fullPath); err == nil {
			return testPath, nil
		}
	}

	// Finally, try searching for the task
	return findTaskByName(festivalPath, taskInput)
}

// findTaskByName searches for a task file by name within a festival.
func findTaskByName(festivalPath, taskName string) (string, error) {
	var matches []string

	err := filepath.Walk(festivalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Check if this matches the task name
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			if info.Name() == taskName || strings.Contains(info.Name(), taskName) {
				relPath := strings.TrimPrefix(path, festivalPath+"/")
				matches = append(matches, relPath)
			}
		}

		return nil
	})
	if err != nil {
		return "", errors.Wrap(err, "searching for task")
	}

	if len(matches) == 0 {
		return "", errors.NotFound("task").
			WithField("name", taskName).
			WithField("hint", "use full path like '001/01/01_task.md'")
	}

	if len(matches) > 1 {
		return "", errors.Validation("ambiguous task name").
			WithField("name", taskName).
			WithField("matches", strings.Join(matches, ", ")).
			WithField("hint", "use full path to disambiguate")
	}

	return matches[0], nil
}

// resolvePhase finds a phase directory by name or number.
func resolvePhase(festivalPath, phaseInput string) (string, string, error) {
	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", "", errors.IO("reading festival directory", err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden and metadata directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Check for phase match (by prefix number or full name)
		if strings.HasPrefix(name, phaseInput) || name == phaseInput {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("phase").
			WithField("input", phaseInput).
			WithField("hint", "use phase number like '001' or full name like '001_CRITICAL'")
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous phase").
			WithField("input", phaseInput).
			WithField("matches", strings.Join(matches, ", "))
	}

	return filepath.Join(festivalPath, matches[0]), matches[0], nil
}

// resolveSequence finds a sequence directory by name or path.
func resolveSequence(festivalPath, cwd, seqInput string) (string, string, error) {
	// If input contains a slash, treat as phase/sequence path
	if strings.Contains(seqInput, "/") {
		parts := strings.SplitN(seqInput, "/", 2)
		phasePath, phaseName, err := resolvePhase(festivalPath, parts[0])
		if err != nil {
			return "", "", err
		}
		seqPath, seqName, err := findSequenceInPhase(phasePath, parts[1])
		if err != nil {
			return "", "", err
		}
		return seqPath, phaseName + "/" + seqName, nil
	}

	// Otherwise, search in current phase context or all phases
	// First check if we're in a phase directory
	if strings.HasPrefix(cwd, festivalPath) {
		relPath := strings.TrimPrefix(cwd, festivalPath+"/")
		parts := strings.Split(relPath, "/")
		if len(parts) >= 1 {
			// Try to find sequence in current phase
			phasePath := filepath.Join(festivalPath, parts[0])
			seqPath, seqName, err := findSequenceInPhase(phasePath, seqInput)
			if err == nil {
				return seqPath, parts[0] + "/" + seqName, nil
			}
		}
	}

	// Search all phases for the sequence
	return findSequenceGlobally(festivalPath, seqInput)
}

// findSequenceInPhase finds a sequence within a specific phase.
func findSequenceInPhase(phasePath, seqInput string) (string, string, error) {
	entries, err := os.ReadDir(phasePath)
	if err != nil {
		return "", "", errors.IO("reading phase directory", err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasPrefix(name, seqInput) || name == seqInput {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("sequence in phase").
			WithField("input", seqInput)
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous sequence").
			WithField("input", seqInput).
			WithField("matches", strings.Join(matches, ", "))
	}

	return filepath.Join(phasePath, matches[0]), matches[0], nil
}

// findSequenceGlobally searches all phases for a sequence.
func findSequenceGlobally(festivalPath, seqInput string) (string, string, error) {
	phases, err := os.ReadDir(festivalPath)
	if err != nil {
		return "", "", errors.IO("reading festival directory", err)
	}

	var matches []string
	for _, phase := range phases {
		if !phase.IsDir() || strings.HasPrefix(phase.Name(), ".") {
			continue
		}
		phasePath := filepath.Join(festivalPath, phase.Name())
		sequences, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}
		for _, seq := range sequences {
			if !seq.IsDir() || strings.HasPrefix(seq.Name(), ".") {
				continue
			}
			if strings.HasPrefix(seq.Name(), seqInput) || seq.Name() == seqInput {
				matches = append(matches, phase.Name()+"/"+seq.Name())
			}
		}
	}

	if len(matches) == 0 {
		return "", "", errors.NotFound("sequence").
			WithField("input", seqInput).
			WithField("hint", "use phase/sequence format like '001/01_api_design'")
	}

	if len(matches) > 1 {
		return "", "", errors.Validation("ambiguous sequence").
			WithField("input", seqInput).
			WithField("matches", strings.Join(matches, ", ")).
			WithField("hint", "use phase/sequence format to disambiguate")
	}

	parts := strings.SplitN(matches[0], "/", 2)
	return filepath.Join(festivalPath, parts[0], parts[1]), matches[0], nil
}
