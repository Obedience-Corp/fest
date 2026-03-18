package gates

import (
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// CanonicalGateType maps built-in gate IDs to their optional descriptive type.
// This is metadata for generated frontmatter, not the source of identity.
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
