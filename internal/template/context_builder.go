package template

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
)

// BuildContextFromPath builds a complete Context by parsing the directory hierarchy.
// Given a target path (e.g., a sequence directory), it extracts festival, phase, and
// sequence information from the directory structure.
//
// Parameters:
//   - targetPath: absolute path to sequence directory (for task creation)
//   - festivalPath: absolute path to festival root (can be empty for auto-detection)
//
// Returns a Context populated with all hierarchy information.
func BuildContextFromPath(targetPath, festivalPath string) (*Context, error) {
	ctx := NewContext()

	// If festival path not provided, try to find it
	if festivalPath == "" {
		var err error
		festivalPath, err = FindFestivalRoot(targetPath)
		if err != nil {
			// Not inside a festival - return minimal context
			return ctx, nil
		}
	}

	// Get relative path from festival to target
	relPath, err := filepath.Rel(festivalPath, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return ctx, nil
	}

	parts := strings.Split(relPath, string(filepath.Separator))

	// Try reading fest.yaml for accurate festival metadata
	if festCfg, cfgErr := config.LoadFestivalConfig(festivalPath, ""); cfgErr == nil && festCfg.Metadata.HasMetadata() {
		ctx.SetFestival(festCfg.Metadata.Name, festCfg.Metadata.Goal, nil)
		ctx.SetFestivalID(festCfg.Metadata.ID)
	} else {
		// Fall back to parsing festival info from directory name
		festivalName := filepath.Base(festivalPath)
		ctx.SetFestival(parseFestivalName(festivalName), "", nil)
		if festID := extractFestivalID(festivalName); festID != "" {
			ctx.SetFestivalID(festID)
		}
	}

	// Parse phase info (first path component)
	if len(parts) >= 1 && parts[0] != "." {
		phaseName := parts[0]
		phaseNum, phaseParsed := parsePhaseInfo(phaseName)
		if phaseParsed != "" {
			phaseType := detectPhaseType(phaseName)
			ctx.SetPhase(phaseNum, phaseParsed, phaseType)
		}
	}

	// Parse sequence info (second path component)
	if len(parts) >= 2 {
		seqName := parts[1]
		seqNum, seqParsed := parseSequenceInfo(seqName)
		if seqParsed != "" {
			ctx.SetSequence(seqNum, seqParsed)
		}
	}

	ctx.ComputeStructureVariables()
	return ctx, nil
}

// parseFestivalName extracts the human-readable festival name from directory name.
// Handles formats like:
//   - "my-festival-AF0001" -> "my festival"
//   - "address-feedback-from-last-fest-festival-AF0001" -> "address feedback from last fest festival"
//   - "my_festival_GU0002" -> "my festival"
func parseFestivalName(dirName string) string {
	// Remove festival ID suffix if present (e.g., "-AF0001" or "_GU0002")
	idPattern := regexp.MustCompile(`[-_][A-Z]{2,4}\d{4}$`)
	name := idPattern.ReplaceAllString(dirName, "")

	// Convert dashes/underscores to spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Trim and clean up
	name = strings.TrimSpace(name)

	return name
}

// extractFestivalID extracts the festival ID from directory name.
// Looks for patterns like "AF0001", "GU0002" at the end.
func extractFestivalID(dirName string) string {
	idPattern := regexp.MustCompile(`([A-Z]{2,4}\d{4})$`)
	matches := idPattern.FindStringSubmatch(dirName)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// parsePhaseInfo extracts phase number and name from directory name.
// Handles formats like "001_PLANNING", "002_IMPLEMENTATION", "01_Research".
func parsePhaseInfo(dirName string) (int, string) {
	// Pattern: NNN_PHASE_NAME or NN_phase_name
	parts := strings.SplitN(dirName, "_", 2)
	if len(parts) < 2 {
		return 0, ""
	}

	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, ""
	}

	// Convert name: "PLANNING" -> "Planning", "research" -> "research"
	name := parts[1]
	// Handle UPPER_CASE names
	if name == strings.ToUpper(name) {
		// Title case for all-caps names
		name = strings.ToLower(name)
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
	}
	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")

	return num, name
}

// parseSequenceInfo extracts sequence number and name from directory name.
// Handles formats like "01_requirements", "02_api_design".
func parseSequenceInfo(dirName string) (int, string) {
	parts := strings.SplitN(dirName, "_", 2)
	if len(parts) < 2 {
		return 0, ""
	}

	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, ""
	}

	// Convert underscores to spaces
	name := strings.ReplaceAll(parts[1], "_", " ")

	return num, name
}

// detectPhaseType determines the phase type from the phase directory name.
func detectPhaseType(dirName string) string {
	lower := strings.ToLower(dirName)

	switch {
	case strings.Contains(lower, "planning"):
		return "planning"
	case strings.Contains(lower, "research"):
		return "research"
	case strings.Contains(lower, "implementation"):
		return "implementation"
	case strings.Contains(lower, "review"):
		return "review"
	case strings.Contains(lower, "deployment"):
		return "deployment"
	default:
		return ""
	}
}

// LoadConfigMarkers loads config-based markers from fest.yaml.
// Returns a map of marker hint -> value for Category B markers.
// The config parameter should come from FestivalConfig.
func LoadConfigMarkers(festivalPath string) map[string]string {
	markers := make(map[string]string)

	if festivalPath == "" {
		return markers
	}

	// Try to load fest.yaml
	configPath := filepath.Join(festivalPath, "fest.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return markers
	}

	// Note: The actual config loading is done by config.LoadFestivalConfig()
	// This function just returns the path - callers should load config and
	// pass values to ToReplacementMapWithConfig()

	return markers
}

// BuildTaskContext builds a complete Context for task creation.
// This is a convenience function that combines path parsing with task info.
//
// Parameters:
//   - sequencePath: absolute path to the sequence directory
//   - festivalPath: absolute path to festival root (can be empty)
//   - taskNumber: the task's order number
//   - taskName: the task's human-readable name
func BuildTaskContext(sequencePath, festivalPath string, taskNumber int, taskName string) (*Context, error) {
	ctx, err := BuildContextFromPath(sequencePath, festivalPath)
	if err != nil {
		return nil, err
	}

	ctx.SetTask(taskNumber, taskName)
	ctx.ComputeStructureVariables()

	return ctx, nil
}
