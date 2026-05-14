package workflow

import (
	"regexp"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

var workflowIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)

// ValidateWorkflowID returns a festerrors.Validation error if id does not
// match the slug-safety pattern required for filesystem-derived workflow
// identifiers. The pattern is lowercase ASCII alphanumeric plus underscore
// and dash, must start with [a-z0-9], 1-80 chars.
func ValidateWorkflowID(id string) error {
	if id == "" {
		return festerrors.Validation("workflow_id must not be empty").
			WithHint("pass --workflow-id or run from a directory with a slug-safe basename")
	}
	if !workflowIDPattern.MatchString(id) {
		return festerrors.Validation("invalid workflow_id").
			WithField("workflow_id", id).
			WithHint("use lowercase letters, digits, '-', '_'; start with [a-z0-9]; max 80 chars")
	}
	return nil
}

var slugInvalidPattern = regexp.MustCompile(`[^a-z0-9_-]+`)

// SanitizeBasenameAsSlug lowercases name and replaces runs of invalid
// characters with a single '-'. Leading dashes are stripped. Used when
// auto-deriving a workflow_id from a directory basename so e.g.
// "My Project!" becomes "my-project".
func SanitizeBasenameAsSlug(name string) string {
	if name == "" {
		return ""
	}
	lower := strings.ToLower(name)
	cleaned := slugInvalidPattern.ReplaceAllString(lower, "-")
	cleaned = strings.Trim(cleaned, "-")
	return cleaned
}
