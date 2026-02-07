package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SchemaValidEvaluator validates a file against a schema.
type SchemaValidEvaluator struct{}

func (e *SchemaValidEvaluator) Evaluate(ctx context.Context, phasePath string, params map[string]any) (*ConditionResult, error) {
	file, ok := params["file"].(string)
	if !ok {
		return nil, fmt.Errorf("schema_valid condition requires 'file' parameter")
	}

	fullPath := filepath.Join(phasePath, file)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return &ConditionResult{
			Pass:           false,
			FailureContext: fmt.Sprintf("Cannot read file %s: %v", file, err),
		}, nil
	}

	// Validate it's valid YAML. A proper JSON schema validator can be added later.
	var parsed any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return &ConditionResult{
			Pass:           false,
			FailureContext: fmt.Sprintf("Invalid YAML in %s: %v", file, err),
		}, nil
	}

	return &ConditionResult{
		Pass:           true,
		FailureContext: "",
	}, nil
}
