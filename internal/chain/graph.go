package chain

import (
	"context"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// DAG represents a directed acyclic graph of festival dependencies.
type DAG struct {
	// Nodes maps festival ref to its DAG node.
	Nodes map[string]*DAGNode
	// Adj maps a source ref to its outgoing (downstream) refs.
	Adj map[string][]string
	// InAdj maps a target ref to its incoming (upstream) refs.
	InAdj map[string][]string
}

// DAGNode is a single node in the dependency graph.
type DAGNode struct {
	Ref      string
	WaveID   int
	InDegree int
}

// BuildDAG constructs a DAG from a parsed Chain.
func BuildDAG(ctx context.Context, c *Chain) (*DAG, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dag := &DAG{
		Nodes: make(map[string]*DAGNode, len(c.Festivals)),
		Adj:   make(map[string][]string),
		InAdj: make(map[string][]string),
	}

	// Create nodes from festival list.
	for _, f := range c.Festivals {
		dag.Nodes[f.Ref] = &DAGNode{Ref: f.Ref}
	}

	// Assign wave IDs if waves are present.
	for _, w := range c.Waves {
		for _, ref := range w.Festivals {
			if node, ok := dag.Nodes[ref]; ok {
				node.WaveID = w.ID
			}
		}
	}

	// Build adjacency lists from edges.
	for _, e := range c.Edges {
		dag.Adj[e.From] = append(dag.Adj[e.From], e.To)
		dag.InAdj[e.To] = append(dag.InAdj[e.To], e.From)
		if node, ok := dag.Nodes[e.To]; ok {
			node.InDegree++
		}
	}

	return dag, nil
}

// TopologicalSort returns festival refs in dependency-safe execution order
// using Kahn's algorithm. Returns an error if a cycle is detected.
func (d *DAG) TopologicalSort(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int, len(d.Nodes))
	for ref, node := range d.Nodes {
		inDegree[ref] = node.InDegree
	}

	// Seed the queue with zero-indegree nodes.
	var queue []string
	for ref, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, ref)
		}
	}

	var sorted []string
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ref := queue[0]
		queue = queue[1:]
		sorted = append(sorted, ref)

		for _, downstream := range d.Adj[ref] {
			inDegree[downstream]--
			if inDegree[downstream] == 0 {
				queue = append(queue, downstream)
			}
		}
	}

	if len(sorted) != len(d.Nodes) {
		return nil, errors.Validation("cycle detected in dependency graph").
			WithField("processed", len(sorted)).
			WithField("total", len(d.Nodes))
	}

	return sorted, nil
}

// Roots returns refs with no incoming edges (zero in-degree).
func (d *DAG) Roots() []string {
	var roots []string
	for ref, node := range d.Nodes {
		if node.InDegree == 0 {
			roots = append(roots, ref)
		}
	}
	return roots
}

// Downstream returns all refs that directly depend on the given ref.
func (d *DAG) Downstream(ref string) []string {
	return d.Adj[ref]
}

// Upstream returns all refs that the given ref directly depends on.
func (d *DAG) Upstream(ref string) []string {
	return d.InAdj[ref]
}
