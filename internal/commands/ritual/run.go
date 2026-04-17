// Package ritual implements the fest ritual command for managing repeatable festivals.
package ritual

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/commitkit"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type runOptions struct {
	json bool
}

// NewRitualCommand creates the top-level fest ritual command.
func NewRitualCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ritual",
		Short: "Manage repeatable ritual festivals",
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
	}

	cmd.AddCommand(newRunCommand())

	return cmd
}

// newRunCommand creates the fest ritual run subcommand.
func newRunCommand() *cobra.Command {
	opts := &runOptions{}
	cmd := &cobra.Command{
		Use:   "run <ritual-name-or-id>",
		Short: "Create a new run of a ritual festival in active/",
		Long: `Copy a ritual festival from ritual/ to active/ with a hex run counter.

The ritual template stays in ritual/ untouched. A new copy is placed
in active/ with an appended hex counter (e.g., -0001, -000A, -00FF).

The counter auto-increments by scanning active/ and dungeon/completed/
for existing runs.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRitual(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "output as JSON")

	return cmd
}

func runRitual(ctx context.Context, nameOrID string, opts *runOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return errors.Validation("ritual name or ID cannot be empty")
	}
	if strings.ContainsAny(nameOrID, "/\\") {
		return errors.Validation("ritual name or ID cannot contain path separators").
			WithField("input", nameOrID)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsRoot, err := workspace.FindFestivals(cwd)
	if err != nil {
		return errors.Wrap(err, "finding festivals root")
	}
	if festivalsRoot == "" {
		return errors.NotFound("festivals directory")
	}

	// Find the ritual in ritual/
	ritualPath, err := findRitual(festivalsRoot, nameOrID)
	if err != nil {
		return err
	}

	ritualDirName := filepath.Base(ritualPath)

	// Find next run counter
	nextRun, err := id.FindNextRitualRun(ctx, festivalsRoot, ritualDirName)
	if err != nil {
		return errors.Wrap(err, "finding next run counter")
	}

	// Build run directory name: {ritual-dir-name}-{XXXX}
	hexCounter := id.FormatHexCounter(nextRun)
	runDirName := fmt.Sprintf("%s-%s", ritualDirName, hexCounter)
	destPath := filepath.Join(festivalsRoot, "active", runDirName)

	// Copy ritual to active/
	if err := copyDir(ctx, ritualPath, destPath); err != nil {
		return errors.Wrap(err, "copying ritual to active").
			WithField("source", ritualPath).
			WithField("dest", destPath)
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())
	ritualID, _ := id.ExtractIDFromDirName(ritualDirName)

	// Update ritual_config in the source ritual's fest.yaml
	ritualCfg, cfgErr := config.LoadFestivalConfig(ritualPath, "")
	if cfgErr == nil && ritualCfg.RitualConfig != nil {
		ritualCfg.RitualConfig.RunCount++
		ritualCfg.RitualConfig.LastRun = time.Now().UTC().Format("2006-01-02")
		if saveErr := config.SaveFestivalConfig(ritualPath, "", ritualCfg); saveErr != nil {
			display.Warning("Failed to update ritual run count: %v", saveErr)
		}
	}

	commitHash, commitErr := autoCommitRitualRun(ctx, ritualDirName, ritualID, ritualPath, destPath)
	if commitErr != nil {
		display.Warning("Failed to auto-commit ritual run: %v", commitErr)
	}

	// Output
	if opts.json {
		result := map[string]any{
			"success":     true,
			"action":      "ritual_run",
			"ritual":      ritualDirName,
			"ritual_id":   ritualID,
			"run_number":  nextRun,
			"hex_counter": hexCounter,
			"run_dir":     runDirName,
			"dest_path":   destPath,
		}
		if commitHash != "" {
			result["commit"] = commitHash
		}
		return shared.EncodeJSON(os.Stdout, result)
	}
	display.Success("Created ritual run: %s", runDirName)
	display.Info("  Source: %s", ritualPath)
	display.Info("  Destination: %s", destPath)
	display.Info("  Run #%d (0x%s)", nextRun, hexCounter)
	if commitHash != "" {
		display.Info("  Commit: %s", commitHash)
	}

	return nil
}

func autoCommitRitualRun(ctx context.Context, ritualName, ritualID, ritualPath, destPath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	ws, ok := scope.WorkspaceFrom(ctx)
	if !ok || ws == nil {
		return "", errors.New("workspace not available in context")
	}

	if err := commitkit.StageFiles(ctx, ws.Root, ritualPath, destPath); err != nil {
		return "", errors.Wrap(err, "stage")
	}

	hasChanges, err := commitkit.HasStagedChanges(ctx, ws.Root)
	if err != nil {
		return "", errors.Wrap(err, "check staged")
	}
	if !hasChanges {
		return "", nil
	}

	message := fmt.Sprintf("chore(fest): ritual run: %s -> %s", ritualName, filepath.Base(destPath))
	if ritualID != "" {
		message = fmt.Sprintf("chore(fest): ritual run: %s (%s) -> %s", ritualName, ritualID, filepath.Base(destPath))
	}

	if ws.Type == scope.WorkspaceTypeCampaign {
		campaignID, err := commitkit.LoadCampaignID(ctx, ws.Root)
		if err == nil && campaignID != "" {
			message = commitkit.PrependCampaignTag(campaignID, message)
		}
	}

	if err := commitkit.Commit(ctx, ws.Root, commitkit.CommitOptions{Message: message}); err != nil {
		return "", errors.Wrap(err, "commit")
	}

	hash, err := commitkit.ShortHash(ctx, ws.Root)
	if err != nil {
		return "committed", nil
	}
	return hash, nil
}

// findRitual searches ritual/ for a matching festival by name or ID.
func findRitual(festivalsRoot, nameOrID string) (string, error) {
	ritualDir := filepath.Join(festivalsRoot, "ritual")

	entries, err := os.ReadDir(ritualDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.NotFound("ritual directory").
				WithField("path", ritualDir).
				WithHint("Create a ritual festival first with: fest create festival --type ritual")
		}
		return "", errors.IO("reading ritual directory", err)
	}

	// Try exact directory name match first
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == nameOrID {
			return filepath.Join(ritualDir, entry.Name()), nil
		}
	}

	// Try matching by festival ID suffix
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirID, idErr := id.ExtractIDFromDirName(entry.Name())
		if idErr != nil {
			continue
		}
		if dirID == nameOrID {
			return filepath.Join(ritualDir, entry.Name()), nil
		}
	}

	// Try substring match on directory name
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if contains(entry.Name(), nameOrID) {
			return filepath.Join(ritualDir, entry.Name()), nil
		}
	}

	return "", errors.NotFound("ritual festival").
		WithField("query", nameOrID).
		WithHint("Available rituals are in festivals/ritual/")
}

// contains checks if haystack contains needle (case-insensitive).
func contains(haystack, needle string) bool {
	return len(needle) > 0 && strings.Contains(
		strings.ToLower(haystack),
		strings.ToLower(needle),
	)
}

// copyDir recursively copies a directory tree.
func copyDir(ctx context.Context, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(ctx, srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}
