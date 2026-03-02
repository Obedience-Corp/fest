package chain

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
	"gopkg.in/yaml.v3"
)

// Parse reads a chain YAML file and returns the parsed Chain.
func Parse(ctx context.Context, path string) (*Chain, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.IO("reading chain file", err).WithField("path", path)
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
		return nil, errors.Parse("parsing chain YAML", err)
	}

	if err := validateStructure(&c); err != nil {
		return nil, err
	}

	return &c, nil
}

// validateStructure performs basic structural validation on a parsed chain.
func validateStructure(c *Chain) error {
	if c.ChainVersion == "" {
		return errors.Validation("chain_version is required")
	}
	if c.Metadata.ID == "" {
		return errors.Validation("metadata.id is required")
	}
	if c.Metadata.Name == "" {
		return errors.Validation("metadata.name is required")
	}
	if len(c.Festivals) == 0 {
		return errors.Validation("at least one festival is required")
	}

	refs := c.RefSet()
	for _, f := range c.Festivals {
		if f.Ref == "" {
			return errors.Validation("festival ref is required").WithField("festivalID", f.ID)
		}
		if f.ID == "" {
			return errors.Validation("festival id is required").WithField("ref", f.Ref)
		}
	}

	for i, e := range c.Edges {
		if !refs[e.From] {
			return errors.Validation("unknown festival ref in edge from").
				WithField("edge", i).WithField("from", e.From)
		}
		if !refs[e.To] {
			return errors.Validation("unknown festival ref in edge to").
				WithField("edge", i).WithField("to", e.To)
		}
		if e.Type != EdgeHard && e.Type != EdgeSoft {
			return errors.Validation(fmt.Sprintf("edge type must be %q or %q", EdgeHard, EdgeSoft)).
				WithField("edge", i).WithField("type", string(e.Type))
		}
	}

	return nil
}
