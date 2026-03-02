package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	chaintpl "github.com/Obedience-Corp/fest/embedded/templates/chain"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var (
		name string
		goal string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new festival chain",
		Long:  "Create a new chain YAML definition file in festivals/chains/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, name, goal)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "chain name (required)")
	cmd.Flags().StringVar(&goal, "goal", "", "chain goal description")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runCreate(cmd *cobra.Command, name, goal string) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	root, err := tpl.FindFestivalsRoot(cwd)
	if err != nil {
		return fmt.Errorf("finding festivals root: %w", err)
	}

	chainsDir := filepath.Join(root, "chains")
	if err := os.MkdirAll(chainsDir, 0o755); err != nil {
		return fmt.Errorf("creating chains directory: %w", err)
	}

	// Generate chain ID from name prefix.
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	prefix := chainIDPrefix(name)
	id := fmt.Sprintf("%s0001", prefix)

	// Check for existing chains to auto-increment.
	entries, _ := os.ReadDir(chainsDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), prefix) {
			// Increment: simple strategy, just bump to next number.
			id = fmt.Sprintf("%s%04d", prefix, len(entries)+1)
		}
	}

	// Render embedded chain template.
	tplData, err := chaintpl.Templates.ReadFile("chain_template.yaml")
	if err != nil {
		return fmt.Errorf("reading chain template: %w", err)
	}

	tmpl, err := template.New("chain").Parse(string(tplData))
	if err != nil {
		return fmt.Errorf("parsing chain template: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	data := struct {
		ID        string
		Name      string
		Goal      string
		CreatedAt string
	}{
		ID:        id,
		Name:      slug,
		Goal:      goal,
		CreatedAt: now,
	}

	filename := fmt.Sprintf("%s-%s.yaml", slug, id)
	path := filepath.Join(chainsDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating chain file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("rendering chain template: %w", err)
	}

	fmt.Println(ui.Label("CHAIN CREATED"))
	fmt.Printf("  ID:   %s\n", id)
	fmt.Printf("  Name: %s\n", slug)
	fmt.Printf("  File: %s\n", path)
	fmt.Println()
	fmt.Println("Add festivals and edges by editing the chain file directly,")
	fmt.Println("then run 'fest chain validate " + id + "' to verify.")

	return nil
}

// chainIDPrefix extracts a 2-letter uppercase prefix from the chain name.
func chainIDPrefix(name string) string {
	words := strings.Fields(strings.TrimSpace(name))
	if len(words) >= 2 {
		return strings.ToUpper(string(words[0][0])) + strings.ToUpper(string(words[1][0]))
	}
	if len(name) >= 2 {
		return strings.ToUpper(name[:2])
	}
	return "CH"
}
