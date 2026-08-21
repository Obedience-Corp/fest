package validator

// Levels for validation issues
const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelInfo    = "info"
)

// Issue codes
const (
	CodeMissingFile        = "missing_file"
	CodeMissingTaskFiles   = "missing_task_files"
	CodeMissingSequence    = "missing_sequence"
	CodeMissingQualityGate = "missing_quality_gates"
	CodeNamingConvention   = "naming_convention"
	CodeUnfilledTemplate   = "unfilled_template"
	CodeMissingGoal        = "missing_goal"
	CodeNumberingGap       = "numbering_gap"

	// Auto-link issue codes
	CodeAutoLinkMissingWorkingDir  = "autolink_missing_working_dir"
	CodeAutoLinkAbsolutePath       = "autolink_absolute_path"
	CodeAutoLinkPathTraversal      = "autolink_path_traversal"
	CodeAutoLinkPathNotFound       = "autolink_path_not_found"
	CodeAutoLinkPathNotDir         = "autolink_path_not_dir"
	CodeAutoLinkUnrequiredSet      = "autolink_unrequired_set"
	CodeAutoLinkProjectPathInvalid = "autolink_project_path_invalid"

	// Hooks issue codes
)

// Issue represents a single validation problem
type Issue struct {
	Level       string `json:"level"`
	Code        string `json:"code"`
	Path        string `json:"path"`
	Message     string `json:"message"`
	Fix         string `json:"fix,omitempty"`
	AutoFixable bool   `json:"auto_fixable"`
}

// Checklist represents post-completion checklist results
type Checklist struct {
	TemplatesFilled *bool `json:"templates_filled"`
	GoalsAchievable *bool `json:"goals_achievable"`
	TaskFilesExist  *bool `json:"task_files_exist"`
	OrderCorrect    *bool `json:"order_correct"`
	ParallelCorrect *bool `json:"parallel_correct"`
}

// FixApplied represents a fix that was automatically applied
type FixApplied struct {
	Code   string `json:"code"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// Result is the aggregated validation result from validators
type Result struct {
	OK bool `json:"ok"`
	// Metadata for command context
	Action   string `json:"action,omitempty"`
	Festival string `json:"festival,omitempty"`

	// Aggregated result
	Valid        bool         `json:"valid"`
	Score        int          `json:"score"`
	Issues       []Issue      `json:"issues,omitempty"`
	Checklist    *Checklist   `json:"checklist,omitempty"`
	FixesApplied []FixApplied `json:"fixes_applied,omitempty"`
	Warnings     []string     `json:"warnings,omitempty"`
	Suggestions  []string     `json:"suggestions,omitempty"`
}

// NewResult creates a baseline result object for the given action/festival
func NewResult(action, festival string) *Result {
	return &Result{
		Action:   action,
		Festival: festival,
		Issues:   []Issue{},
		Warnings: []string{},
	}
}

// HasErrors returns true if the result contains at least one error-level issue.
// This includes unfilled-template findings, which remain error-level in
// implementation phases so fest next can still block on them.
func (r *Result) HasErrors() bool {
	if r == nil {
		return false
	}
	for _, is := range r.Issues {
		if is.Level == LevelError {
			return true
		}
	}
	return false
}

// HasStructuralErrors reports error-level issues that are not pending
// template markers. Missing files, missing tasks, and missing gates fail
// structure validation; unfilled [REPLACE] markers do not.
func (r *Result) HasStructuralErrors() bool {
	if r == nil {
		return false
	}
	for _, is := range r.Issues {
		if is.Level == LevelError && !IsPendingMarker(is.Code) {
			return true
		}
	}
	return false
}

// HasPendingMarkers reports whether any unfilled-template issues are present.
func (r *Result) HasPendingMarkers() bool {
	if r == nil {
		return false
	}
	for _, is := range r.Issues {
		if IsPendingMarker(is.Code) {
			return true
		}
	}
	return false
}
