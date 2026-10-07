// Package yamlutil provides YAML marshaling helpers with standard 2-space indentation.
package yamlutil

import (
	"bytes"
	"strings"

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

// QuoteString renders s as a single-line YAML double-quoted scalar, escaping
// quotes, backslashes and control characters, for inline use in YAML templates.
func QuoteString(s string) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	node := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: s}
	if err := enc.Encode(node); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
