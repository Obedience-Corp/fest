package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

// addOptions holds the parsed flags for `fest chain add`.
type addOptions struct {
	chain    string
	festival string
	after    []string
	edgeType string
	note     string
	json     bool
}

// addedEdge is the JSON shape for an edge created by the command.
type addedEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
	Note string `json:"note,omitempty"`
}

// addedNode is the JSON shape for the festival node added to the chain.
type addedNode struct {
	Ref      string   `json:"ref"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects,omitempty"`
}

// validationSummary is the JSON shape for the in-memory validation result.
type validationSummary struct {
	Valid  bool     `json:"valid"`
	Score  int      `json:"score"`
	Errors []string `json:"errors,omitempty"`
}

// addResult is the structured result emitted by `fest chain add --json`.
type addResult struct {
	ChainID    string            `json:"chain_id"`
	ChainName  string            `json:"chain_name"`
	Node       addedNode         `json:"node"`
	Edges      []addedEdge       `json:"edges"`
	Validation validationSummary `json:"validation"`
}

func newAddCmd() *cobra.Command {
	opts := &addOptions{}

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a festival to a chain",
		Long: "Add a festival as a node in an existing chain, optionally with " +
			"prerequisite dependency edges. The mutated chain is validated in " +
			"memory and written only if valid.\n" +
			"The festival defaults to the current festival context when run inside " +
			"a festival or linked project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.chain, "chain", "", "target chain id or name")
	cmd.Flags().StringVar(&opts.festival, "festival", "", "festival to add (id, name, or path)")
	cmd.Flags().StringArrayVar(&opts.after, "after", nil, "prerequisite festival ref/id (repeatable)")
	cmd.Flags().StringVar(&opts.edgeType, "type", string(chainpkg.EdgeHard), "edge type: hard | soft")
	cmd.Flags().StringVar(&opts.note, "note", "", "optional edge note")
	cmd.Flags().BoolVar(&opts.json, "json", false, "emit structured JSON result")

	return cmd
}

func runAdd(ctx context.Context, opts *addOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	edgeType, err := parseEdgeType(opts.edgeType)
	if err != nil {
		return err
	}

	chainID, _, err := resolveChainID(ctx, opts.chain)
	if err != nil {
		return err
	}

	c, path, err := findChainByID(ctx, chainID)
	if err != nil {
		return err
	}

	festival, err := resolveAddFestival(ctx, opts.festival)
	if err != nil {
		return err
	}

	node, edges, err := mutateChain(ctx, c, festival, opts.after, edgeType, opts.note)
	if err != nil {
		return err
	}

	result := chainpkg.Validate(ctx, c)
	if !result.Valid {
		return validationError(result)
	}

	if err := chainpkg.Write(path, c); err != nil {
		return err
	}

	if opts.json {
		return emitAddJSON(c, node, edges, result)
	}
	printAddSummary(c, node, edges)
	return nil
}

// resolveAddFestival resolves the festival to add, falling back to the current
// festival context when --festival is omitted.
func resolveAddFestival(ctx context.Context, flag string) (resolvedFestival, error) {
	root, err := festivalsRoot()
	if err != nil {
		return resolvedFestival{}, err
	}

	value := flag
	if value == "" {
		festID, ok := resolveCurrentFestivalID(ctx)
		if !ok {
			return resolvedFestival{}, errors.Validation("festival is required").
				WithHint("pass --festival <id|name|path> or run from inside a festival or linked project")
		}
		value = festID
	}
	return resolveFestivalToAdd(ctx, root, value)
}

// mutateChain appends the festival node and any prerequisite edges to the chain
// in memory, rejecting duplicates and self-edges, and places the node in a
// trailing wave when the chain uses waves.
func mutateChain(ctx context.Context, c *chainpkg.Chain, festival resolvedFestival, after []string, edgeType chainpkg.EdgeType, note string) (chainpkg.FestivalNode, []chainpkg.Edge, error) {
	if err := ctx.Err(); err != nil {
		return chainpkg.FestivalNode{}, nil, err
	}

	for _, f := range c.Festivals {
		if f.ID == festival.ID {
			return chainpkg.FestivalNode{}, nil, errors.Validation("festival already in chain").
				WithField("id", festival.ID).
				WithField("ref", f.Ref).
				WithField("chainID", c.Metadata.ID)
		}
	}

	node := buildNode(c, festival.ID, festival.Name, festival.Projects)
	c.Festivals = append(c.Festivals, node)

	var edges []chainpkg.Edge
	for _, raw := range after {
		fromRef, err := resolvePrereqRef(c, raw)
		if err != nil {
			return chainpkg.FestivalNode{}, nil, err
		}
		edge, err := buildEdge(c, fromRef, node.Ref, edgeType, note)
		if err != nil {
			return chainpkg.FestivalNode{}, nil, err
		}
		c.Edges = append(c.Edges, edge)
		edges = append(edges, edge)
	}

	placeInTrailingWave(c, node.Ref)

	return node, edges, nil
}

// validationError converts a failed validation result into an actionable error
// listing each structural error.
func validationError(result *chainpkg.ValidationResult) error {
	msgs := make([]string, 0, len(result.Errors))
	for _, e := range result.Errors {
		msgs = append(msgs, e.Error())
	}
	return errors.Validation("chain would be invalid after add; no changes written").
		WithField("errors", msgs).
		WithHintf("fix the chain or adjust the add (%s)", strings.Join(msgs, "; "))
}

func emitAddJSON(c *chainpkg.Chain, node chainpkg.FestivalNode, edges []chainpkg.Edge, result *chainpkg.ValidationResult) error {
	out := addResult{
		ChainID:   c.Metadata.ID,
		ChainName: c.Metadata.Name,
		Node: addedNode{
			Ref:      node.Ref,
			ID:       node.ID,
			Name:     node.Name,
			Projects: node.Projects,
		},
		Edges:      make([]addedEdge, 0, len(edges)),
		Validation: summarize(result),
	}
	for _, e := range edges {
		out.Edges = append(out.Edges, addedEdge{
			From: e.From,
			To:   e.To,
			Type: string(e.Type),
			Note: e.Note,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling add result")
	}
	fmt.Println(string(data))
	return nil
}

func summarize(result *chainpkg.ValidationResult) validationSummary {
	s := validationSummary{Valid: result.Valid, Score: result.Score}
	for _, e := range result.Errors {
		s.Errors = append(s.Errors, e.Error())
	}
	return s
}

func printAddSummary(c *chainpkg.Chain, node chainpkg.FestivalNode, edges []chainpkg.Edge) {
	fmt.Println(ui.Label("FESTIVAL ADDED TO CHAIN"))
	fmt.Printf("  Chain:    %s (%s)\n", c.Metadata.Name, c.Metadata.ID)
	fmt.Printf("  Festival: %s (%s)\n", node.Name, node.ID)
	fmt.Printf("  Ref:      %s\n", node.Ref)
	if len(edges) == 0 {
		fmt.Println("  Edges:    none (added as a root)")
	} else {
		fmt.Printf("  Edges:    %d\n", len(edges))
		for _, e := range edges {
			fmt.Printf("    %s -> %s (%s)\n", e.From, e.To, e.Type)
		}
	}
}
