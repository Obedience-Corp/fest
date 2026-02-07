package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileExistsEvaluator checks that all listed files exist.
type FileExistsEvaluator struct{}

func (e *FileExistsEvaluator) Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error) {
	filesParam, ok := params["files"]
	if !ok {
		return nil, fmt.Errorf("file_exists condition requires 'files' parameter")
	}

	var files []string
	switch v := filesParam.(type) {
	case []string:
		files = v
	case []any:
		for _, f := range v {
			if str, ok := f.(string); ok {
				files = append(files, str)
			}
		}
	default:
		return nil, fmt.Errorf("files parameter must be a list of strings")
	}

	var missing []string
	for _, file := range files {
		fullPath := filepath.Join(phasePath, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missing = append(missing, file)
		}
	}

	if len(missing) == 0 {
		return &ConditionResult{
			Pass:           true,
			FailureContext: "",
		}, nil
	}

	return &ConditionResult{
		Pass:           false,
		FailureContext: fmt.Sprintf("Missing files:\n%s", strings.Join(missing, "\n")),
	}, nil
}
