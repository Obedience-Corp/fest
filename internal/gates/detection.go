package gates

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// DetectGateType resolves the canonical gate type for a task file.
// It prefers explicit frontmatter, but falls back to filename/body heuristics
// so older or custom gate files still match the expected gate slot.
func DetectGateType(filePath string) (frontmatter.GateType, bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", false
	}

	return detectGateTypeFromContent(filepath.Base(filePath), content)
}

func detectGateTypeFromContent(filename string, content []byte) (frontmatter.GateType, bool) {
	fm, body, err := frontmatter.Parse(content)
	if err == nil && fm != nil {
		explicit := CanonicalGateType(string(fm.GateType))
		inferred := inferGateTypeWithSignals(filename, body)

		// Only override explicit frontmatter for the known legacy mismatch where
		// commit gates were stamped as iterate. Broad override based on full body
		// text creates false positives for review gates with "security" checklist
		// items.
		if explicit != "" {
			if fm.Type == frontmatter.TypeGate && shouldOverrideExplicitGateType(explicit, inferred) {
				return inferred, true
			}
			return explicit, true
		}

		if fm.Type == frontmatter.TypeGate && inferred != "" {
			return inferred, true
		}
	}

	if inferred := inferGateTypeWithSignals(filename, content); inferred != "" {
		return inferred, true
	}

	return "", false
}

// CanonicalGateType maps gate IDs, filenames, and headings to a stable gate type.
func CanonicalGateType(value string) frontmatter.GateType {
	normalized := normalizeGateText(value)
	compact := strings.ReplaceAll(normalized, " ", "")

	switch {
	case strings.Contains(normalized, "performance") || strings.Contains(compact, "perf"):
		return frontmatter.GatePerformance
	case strings.Contains(normalized, "security"):
		return frontmatter.GateSecurity
	case strings.Contains(normalized, "iterate") ||
		strings.Contains(normalized, "iteration") ||
		strings.Contains(normalized, "review results") ||
		(strings.Contains(normalized, "review") && strings.Contains(normalized, "feedback")):
		return frontmatter.GateIterate
	case strings.Contains(normalized, "commit"):
		return frontmatter.GateCommit
	case strings.Contains(normalized, "code review") || strings.Contains(normalized, "review"):
		return frontmatter.GateReview
	case strings.Contains(normalized, "testing") ||
		strings.Contains(normalized, "test") ||
		strings.Contains(normalized, "verify") ||
		strings.Contains(normalized, "verification") ||
		strings.Contains(normalized, "qa"):
		return frontmatter.GateTesting
	default:
		return ""
	}
}

func inferGateTypeWithSignals(filename string, content []byte) frontmatter.GateType {
	if !hasGateSignal(filename, content) {
		return ""
	}

	signalText := filename
	if headings := extractHeadingSignals(content); headings != "" {
		signalText += "\n" + headings
	}

	return CanonicalGateType(signalText)
}

func hasGateSignal(filename string, content []byte) bool {
	if looksLikeGateFilename(filename) {
		return true
	}

	lower := strings.ToLower(string(content))
	signals := []string{
		"quality gate",
		"# gate:",
		"## gate:",
		"fest_type: gate",
		"fest_gate_type:",
	}
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			return true
		}
	}

	return false
}

func looksLikeGateFilename(filename string) bool {
	stem := normalizeGateText(extractTaskStem(filename))
	if stem == "" {
		return false
	}

	if strings.Contains(stem, "gate") {
		return true
	}

	switch strings.ReplaceAll(stem, " ", "") {
	case "testing", "testingandverify", "review", "codereview", "iterate", "reviewresultsiterate", "commit", "festcommit":
		return true
	default:
		return false
	}
}

func extractTaskStem(filename string) string {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(filename)), ".md")
	if len(name) >= 3 && isDigit(name[0]) && isDigit(name[1]) {
		switch name[2] {
		case '_', '-', '.', ' ':
			return name[3:]
		}
	}

	return name
}

func normalizeGateText(value string) string {
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		".", " ",
		"/", " ",
		":", " ",
		"\n", " ",
		"\r", " ",
	)
	return strings.Join(strings.Fields(strings.ToLower(replacer.Replace(value))), " ")
}

func extractHeadingSignals(content []byte) string {
	var headings []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		headings = append(headings, strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
	}

	return strings.Join(headings, "\n")
}

func shouldOverrideExplicitGateType(explicit, inferred frontmatter.GateType) bool {
	return explicit == frontmatter.GateIterate && inferred == frontmatter.GateCommit
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
