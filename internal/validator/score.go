package validator

// MarkerPendingPenalty is the single score deduction applied when any
// unfilled template markers remain. Per-file unfilled_template issues used
// to each cost 15 points and floor a freshly scaffolded festival at 0/100.
const MarkerPendingPenalty = 10

// IsPendingMarker reports whether an issue code is an unfilled-template
// finding. Those mean "scaffolded, markers pending", not structural breakage.
func IsPendingMarker(code string) bool {
	return code == CodeUnfilledTemplate
}

// CalculateScore computes a validation score based on issues.
// Unfilled template markers are a capped pending-state penalty, not a
// per-file error that can wipe an otherwise sound structure score.
func CalculateScore(result *Result) int {
	if result == nil || len(result.Issues) == 0 {
		return 100
	}

	errorCount := 0
	warningCount := 0
	markersPending := false

	for _, issue := range result.Issues {
		if IsPendingMarker(issue.Code) {
			markersPending = true
			continue
		}
		switch issue.Level {
		case LevelError:
			errorCount++
		case LevelWarning:
			warningCount++
		}
	}

	// Base score of 100, minus 15 per structural error, minus 5 per warning.
	score := 100 - (errorCount * 15) - (warningCount * 5)
	if markersPending {
		score -= MarkerPendingPenalty
	}
	if score < 0 {
		score = 0
	}

	return score
}

// AddSuggestions adds helpful suggestions based on issues found.
func AddSuggestions(result *Result) {
	hasMissingTasks := false
	hasMissingGates := false
	hasUnfilledTemplates := false

	for _, issue := range result.Issues {
		switch issue.Code {
		case CodeMissingTaskFiles:
			hasMissingTasks = true
		case CodeMissingQualityGate:
			hasMissingGates = true
		case CodeUnfilledTemplate:
			hasUnfilledTemplates = true
		}
	}

	if hasMissingTasks {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest understand tasks' to learn about task file creation")
	}
	if hasMissingGates {
		result.Suggestions = append(result.Suggestions,
			"Run 'fest gates apply --approve' to add quality gates")
	}
	if hasUnfilledTemplates {
		result.Suggestions = append(result.Suggestions,
			"Edit files with unfilled template markers ([REPLACE:], [FILL:], or {{ }}) and add actual content")
	}
}
