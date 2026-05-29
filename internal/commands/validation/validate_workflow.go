package validation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

const (
	CodeWorkflowNumbering = "workflow_numbering"
	CodeWorkflowScan      = "workflow_scan"
)

func validateWorkflowDocsChecks(ctx context.Context, festivalPath string, result *ValidationResult) {
	walkErr := filepath.WalkDir(festivalPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".fest" || name == ".workflow" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "WORKFLOW.md" {
			return nil
		}
		if err := standalone.ValidateStepNumbering(ctx, path); err != nil {
			result.Issues = append(result.Issues, workflowIssueFromError(path, err))
		}
		return nil
	})

	if walkErr != nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Level:   LevelError,
			Code:    CodeWorkflowScan,
			Path:    festivalPath,
			Message: fmt.Sprintf("Failed to scan for WORKFLOW.md files: %v", walkErr),
		})
	}
}

func workflowIssueFromError(path string, err error) ValidationIssue {
	issue := ValidationIssue{
		Level: LevelError,
		Code:  CodeWorkflowNumbering,
		Path:  path,
	}

	var ferr *festerrors.Error
	if errors.As(err, &ferr) {
		msg := ferr.Message
		if found, ok := ferr.Fields["found"].(string); ok && found != "" {
			msg = fmt.Sprintf("%s. Found: %s", msg, found)
		}
		issue.Message = msg
		issue.Fix = ferr.Hint
		return issue
	}

	issue.Message = err.Error()
	return issue
}
