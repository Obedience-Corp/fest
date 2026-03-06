// Package yamlutil provides YAML marshaling helpers with standard 2-space indentation.
package yamlutil

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

// Marshal serializes v to YAML with 2-space indentation (community standard).
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
