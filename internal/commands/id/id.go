// Package id provides the fest id command for displaying the current festival's ID.
package id

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	idpkg "github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

var jsonOutput bool

// NewIDCommand creates the id command.
func NewIDCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "id",
		Short: "Show the festival ID for the current context",
		Annotations: map[string]string{
			"scope": string(scope.Festival),
		},
		Long: `Display the festival ID for the current location.

Works from inside a festival directory or from a project linked to one.
The ID is read from fest.yaml metadata, falling back to the directory name.

Examples:
  fest id          # Print the festival ID (e.g., SR0001)
  fest id --json   # Output as JSON with id, name, and path`,
		RunE: runID,
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func runID(cmd *cobra.Command, args []string) error {
	festivalPath, ok := scope.FestivalFrom(cmd.Context())
	if !ok || festivalPath == "" {
		return errors.NotFound("festival context").
			WithHint("Run from inside a festival directory or a linked project")
	}

	// Primary: load from fest.yaml metadata
	cfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err == nil && cfg.Metadata.HasMetadata() {
		return outputID(cfg.Metadata.ID, cfg.Metadata.Name, festivalPath)
	}

	// Fallback: extract from directory name
	dirName := filepath.Base(festivalPath)
	festID, err := idpkg.ExtractIDFromDirName(dirName)
	if err != nil {
		return errors.NotFound("festival ID").
			WithHint("No metadata.id in fest.yaml and directory name has no ID suffix")
	}

	return outputID(festID, "", festivalPath)
}

func outputID(festID, name, path string) error {
	if jsonOutput {
		result := map[string]string{"id": festID, "path": path}
		if name != "" {
			result["name"] = name
		}
		return shared.EncodeJSON(os.Stdout, result)
	}
	fmt.Println(festID)
	return nil
}
