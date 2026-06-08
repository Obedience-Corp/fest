// Package chain implements inter-festival dependency management through chain
// YAML definitions. A chain groups related festivals into a directed acyclic
// graph (DAG) with typed dependency edges and parallel execution waves.
package chain

import "time"

// ChainStatus represents the lifecycle state of a chain.
type ChainStatus string

const (
	StatusPlanning  ChainStatus = "planning"
	StatusActive    ChainStatus = "active"
	StatusCompleted ChainStatus = "completed"
)

// EdgeType classifies the strength of a dependency between festivals.
type EdgeType string

const (
	// EdgeHard means the downstream festival cannot start ANY work.
	EdgeHard EdgeType = "hard"
	// EdgeSoft means the downstream festival can start with mocks/stubs.
	EdgeSoft EdgeType = "soft"
)

// Chain represents a festival chain definition — the top-level YAML document.
type Chain struct {
	ChainVersion  string         `yaml:"chain_version"`
	Metadata      Metadata       `yaml:"metadata"`
	Festivals     []FestivalNode `yaml:"festivals"`
	Edges         []Edge         `yaml:"edges"`
	Waves         []Wave         `yaml:"waves,omitempty"`
	SequenceHints []SequenceHint `yaml:"sequence_hints,omitempty"`
}

// Metadata holds chain identity and lifecycle information.
type Metadata struct {
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	Goal          string        `yaml:"goal,omitempty"`
	CreatedAt     time.Time     `yaml:"created_at,omitempty"`
	Status        ChainStatus   `yaml:"status"`
	StatusHistory []StatusEntry `yaml:"status_history,omitempty"`
}

// StatusEntry is a single lifecycle state transition in the audit trail.
type StatusEntry struct {
	Status    ChainStatus `yaml:"status"`
	Timestamp time.Time   `yaml:"timestamp"`
	Notes     string      `yaml:"notes,omitempty"`
}

// FestivalNode is a festival registered in the chain.
type FestivalNode struct {
	Ref      string   `yaml:"ref"`
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Projects []string `yaml:"projects,omitempty"`
}

// Edge is a directed dependency between two festival refs.
type Edge struct {
	From string   `yaml:"from"`
	To   string   `yaml:"to"`
	Type EdgeType `yaml:"type"`
	Gate string   `yaml:"gate,omitempty"`
	Note string   `yaml:"note,omitempty"`
}

// Wave is a group of festivals that can execute in parallel.
type Wave struct {
	ID        int      `yaml:"id"`
	Name      string   `yaml:"name"`
	Unlock    string   `yaml:"unlock"`
	Festivals []string `yaml:"festivals"`
}

// SequenceHint provides intra-festival parallelism guidance.
type SequenceHint struct {
	Festival            string       `yaml:"festival"`
	EarlyStart          *HintBlock   `yaml:"early_start,omitempty"`
	NeedsUpstream       *HintBlock   `yaml:"needs_upstream,omitempty"`
	InternalParallelism *Parallelism `yaml:"internal_parallelism,omitempty"`
}

// HintBlock describes sequences that can or cannot start early.
type HintBlock struct {
	Sequences []string `yaml:"sequences"`
	Condition string   `yaml:"condition,omitempty"`
}

// Parallelism describes intra-festival parallel and sequential groups.
type Parallelism struct {
	Parallel []string `yaml:"parallel"`
	Then     []string `yaml:"then,omitempty"`
}

// RefSet returns a set of all festival refs in the chain.
func (c *Chain) RefSet() map[string]bool {
	s := make(map[string]bool, len(c.Festivals))
	for _, f := range c.Festivals {
		s[f.Ref] = true
	}
	return s
}

// FestivalByRef returns the FestivalNode matching the given ref, or nil.
func (c *Chain) FestivalByRef(ref string) *FestivalNode {
	for i := range c.Festivals {
		if c.Festivals[i].Ref == ref {
			return &c.Festivals[i]
		}
	}
	return nil
}
