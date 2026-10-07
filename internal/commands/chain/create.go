package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	chaintpl "github.com/Obedience-Corp/fest/embedded/templates/chain"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

const chainIDPrefix = "CH"

type chainTemplateData struct {
	ID        string
	Name      string
	Goal      string
	CreatedAt string
}

type createResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Goal   string `json:"goal,omitempty"`
	Path   string `json:"path"`
}

func newCreateCmd() *cobra.Command {
	var (
		name    string
		goal    string
		jsonOut bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new festival chain",
		Long:  "Create a new, empty chain YAML definition file in festivals/chains/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(cmd, name, goal, jsonOut)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "chain name (required)")
	cmd.Flags().StringVar(&goal, "goal", "", "chain goal description")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit structured JSON result")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runCreate(cmd *cobra.Command, name, goal string, jsonOut bool) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := festivalsRoot()
	if err != nil {
		return err
	}

	chainsDir := filepath.Join(root, "chains")
	if err := os.MkdirAll(chainsDir, 0o755); err != nil {
		return errors.IO("creating chains directory", err)
	}

	// Generate chain IDs from the fixed chain namespace.
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	id := nextChainID(chainsDir, chainIDPrefix)

	rendered, err := renderChainTemplate(chainTemplateData{
		ID:        id,
		Name:      slug,
		Goal:      goal,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}

	path := filepath.Join(chainsDir, fmt.Sprintf("%s-%s.yaml", slug, id))
	if err := writeNewChainFile(path, rendered); err != nil {
		return err
	}

	if jsonOut {
		return emitCreateJSON(createResult{
			ID:     id,
			Name:   slug,
			Status: "planning",
			Goal:   goal,
			Path:   path,
		})
	}

	fmt.Println(ui.Label("CHAIN CREATED"))
	fmt.Printf("  ID:   %s\n", id)
	fmt.Printf("  Name: %s\n", slug)
	fmt.Printf("  File: %s\n", path)
	fmt.Println()
	fmt.Println("Add festivals with 'fest chain add --chain " + id + " --festival <id> [--after <ref>]',")
	fmt.Println("then run 'fest chain validate " + id + "' to verify.")

	return nil
}

func renderChainTemplate(data chainTemplateData) ([]byte, error) {
	tplData, err := chaintpl.Templates.ReadFile("chain_template.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "reading chain template").WithCode(errors.ErrCodeTemplate)
	}

	tmpl, err := template.New("chain").Parse(string(tplData))
	if err != nil {
		return nil, errors.Wrap(err, "parsing chain template").WithCode(errors.ErrCodeTemplate)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, errors.Wrap(err, "rendering chain template").WithCode(errors.ErrCodeTemplate)
	}
	return buf.Bytes(), nil
}

func writeNewChainFile(path string, data []byte) error {
	f, err := createChainFile(path)
	if err != nil {
		if os.IsExist(err) {
			return errors.Validation("chain file already exists").WithField("path", path)
		}
		return errors.IO("creating chain file", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return errors.IO("writing chain file", err)
	}
	if err := f.Close(); err != nil {
		return errors.IO("closing chain file", err)
	}
	return nil
}

func emitCreateJSON(result createResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling create result")
	}
	fmt.Println(string(data))
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

func createChainFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
}
