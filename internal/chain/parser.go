package chain

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a chain YAML file and returns the parsed Chain.
func Parse(ctx context.Context, path string) (*Chain, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading chain file %s: %w", path, err)
	}

	return ParseBytes(ctx, data)
}

// ParseBytes parses chain YAML from raw bytes.
func ParseBytes(ctx context.Context, data []byte) (*Chain, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var c Chain
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing chain YAML: %w", err)
	}

	if err := validateStructure(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

// validateStructure performs basic structural validation on a parsed chain.
func validateStructure(c *Chain) error {
	if c.ChainVersion == "" {
		return fmt.Errorf("chain_version is required")
	}
	if c.Metadata.ID == "" {
		return fmt.Errorf("metadata.id is required")
	}
	if c.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if len(c.Festivals) == 0 {
		return fmt.Errorf("at least one festival is required")
	}

	refs := c.RefSet()
	for _, f := range c.Festivals {
		if f.Ref == "" {
			return fmt.Errorf("festival ref is required (festival id=%s)", f.ID)
		}
		if f.ID == "" {
			return fmt.Errorf("festival id is required (ref=%s)", f.Ref)
		}
	}

	for i, e := range c.Edges {
		if !refs[e.From] {
			return fmt.Errorf("edge[%d] from=%q: unknown festival ref", i, e.From)
		}
		if !refs[e.To] {
			return fmt.Errorf("edge[%d] to=%q: unknown festival ref", i, e.To)
		}
		if e.Type != EdgeHard && e.Type != EdgeSoft {
			return fmt.Errorf("edge[%d] type=%q: must be %q or %q", i, e.Type, EdgeHard, EdgeSoft)
		}
	}

	return nil
}
