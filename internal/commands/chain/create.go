package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	id := nextChainID(chainsDir, prefix)

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

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("chain file already exists: %s", path)
		}
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

// nextChainID scans existing files in chainsDir to find the highest numeric
// suffix for the given prefix, then returns prefix + max+1 (zero-padded to 4).
// This avoids collisions with non-contiguous IDs.
func nextChainID(chainsDir, prefix string) string {
	entries, _ := os.ReadDir(chainsDir)
	maxNum := 0
	for _, e := range entries {
		name := e.Name()
		idx := strings.Index(name, prefix)
		if idx < 0 {
			continue
		}
		// Extract digits immediately after the prefix.
		after := name[idx+len(prefix):]
		// Take consecutive digits.
		numStr := ""
		for _, ch := range after {
			if ch >= '0' && ch <= '9' {
				numStr += string(ch)
			} else {
				break
			}
		}
		if n, err := strconv.Atoi(numStr); err == nil && n > maxNum {
			maxNum = n
		}
	}
	return fmt.Sprintf("%s%04d", prefix, maxNum+1)
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
