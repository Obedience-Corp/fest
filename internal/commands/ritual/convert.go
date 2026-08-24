package ritual

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	chainpkg "github.com/Obedience-Corp/fest/internal/chain"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type convertOptions struct {
	frequency  string
	name       string
	dryRun     bool
	moveSource bool
	json       bool
}

// newConvertCommand creates the fest ritual convert subcommand.
func newConvertCommand() *cobra.Command {
	opts := &convertOptions{}
	cmd := &cobra.Command{
		Use:   "convert <festival-name-or-id>",
		Short: "Convert a festival into a reusable ritual template",
		Long: `Copy an existing festival into ritual/ as a repeatable ritual template.

The source festival is preserved by default. The copy is placed in ritual/
with an RI-XX0001 ID suffix, its fest.yaml metadata.festival_type is set to
"ritual", and a ritual_config block with run_count: 0 is added.

Use --move-source to archive the original after conversion.`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		ValidArgsFunction: CompleteConvertSource,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConvert(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.frequency, "frequency", "", "Ritual frequency hint (e.g. weekly, quarterly); stored in ritual_config.schedule")
	cmd.Flags().StringVar(&opts.name, "name", "", "Override the festival name in the new ritual template (defaults to source name)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show what would change without writing")
	cmd.Flags().BoolVar(&opts.moveSource, "move-source", false, "Move the source festival to dungeon/archived after conversion (default: preserve source)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output as JSON")

	return cmd
}

func runConvert(ctx context.Context, nameOrID string, opts *convertOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	nameOrID = strings.TrimSpace(nameOrID)
	if nameOrID == "" {
		return errors.Validation("festival name or ID cannot be empty")
	}
	if strings.ContainsAny(nameOrID, "/\\") {
		return errors.Validation("festival name or ID cannot contain path separators").
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

	// Find the source festival across all status directories.
	sourcePath, sourceStatus, err := findFestivalForConvert(festivalsRoot, nameOrID)
	if err != nil {
		return err
	}

	sourceDirName := filepath.Base(sourcePath)

	// Load the source fest.yaml to get the clean name.
	sourceCfg, cfgErr := config.LoadFestivalConfig(sourcePath, "")
	if cfgErr != nil {
		return errors.Wrap(cfgErr, "loading source festival config").
			WithField("path", sourcePath)
	}

	// Determine the name for the new ritual.
	ritualName := opts.name
	if ritualName == "" {
		ritualName = sourceCfg.Metadata.Name
	}
	if ritualName == "" {
		// Fall back to the directory name without the ID suffix.
		ritualName = sourceDirName
		if extracted, extErr := id.ExtractIDFromDirName(sourceDirName); extErr == nil {
			ritualName = strings.TrimSuffix(sourceDirName, "-"+extracted)
		}
	}

	// Check if the source is already a ritual.
	if sourceCfg.Metadata.FestivalType == "ritual" {
		return errors.Validation("source festival is already a ritual").
			WithField("source", sourceDirName).
			WithHint("Use 'fest ritual run' to create a new run of an existing ritual")
	}

	// Check if the source is chain-linked.
	if chainBlocked, chainMsg := checkChainMembership(ctx, sourceCfg.Metadata.ID, festivalsRoot); chainBlocked {
		return errors.Validation("source festival is a node in a chain: "+chainMsg).
			WithField("source", sourceDirName).
			WithHint("Remove the festival from its chain before converting, or convert a non-chain festival")
	}

	// Generate the ritual ID.
	ritualID, err := id.GenerateRitualID(ctx, ritualName, festivalsRoot)
	if err != nil {
		return errors.Wrap(err, "generating ritual ID")
	}

	// Build the destination directory name: {name}-RI-{XX0001}
	// The RI- prefix is part of the directory naming convention, and the ID
	// portion is the base XX0001 (without the RI- prefix that the logical ID
	// carries).
	idSuffix := strings.TrimPrefix(ritualID, id.RitualPrefix)
	ritualDirName := fmt.Sprintf("%s-RI-%s", sanitizeDirName(ritualName), idSuffix)
	destPath := filepath.Join(festivalsRoot, "ritual", ritualDirName)

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// Warn if source is in active/ — converting from active means an agent
	// session may be pointing at it.
	if sourceStatus == "active" {
		display.Warning("Source festival is in active/ — ensure no agent session is using it")
	}

	if opts.dryRun {
		return reportConvertDryRun(display, opts, sourcePath, destPath, ritualDirName, ritualID, ritualName)
	}

	// Ensure ritual/ directory exists.
	ritualDir := filepath.Join(festivalsRoot, "ritual")
	if err := os.MkdirAll(ritualDir, 0o755); err != nil {
		return errors.IO("creating ritual directory", err)
	}

	// Check if destination already exists.
	if _, err := os.Stat(destPath); err == nil {
		return errors.Validation("a ritual with this name and ID already exists").
			WithField("dest", destPath).
			WithHint("Use --name to choose a different name, or remove the existing ritual first")
	}

	// Copy the source festival to ritual/.
	if err := copyDir(ctx, sourcePath, destPath); err != nil {
		return errors.Wrap(err, "copying festival to ritual").
			WithField("source", sourcePath).
			WithField("dest", destPath)
	}

	// Mutate the copied fest.yaml: set type to ritual, add ritual_config, drop status_history.
	convertedCfg, cfgErr := config.LoadFestivalConfig(destPath, "")
	if cfgErr != nil {
		return errors.Wrap(cfgErr, "loading copied festival config").
			WithField("path", destPath)
	}

	convertedCfg.Metadata.FestivalType = "ritual"
	convertedCfg.Metadata.ID = strings.TrimPrefix(ritualID, id.RitualPrefix)
	convertedCfg.Metadata.Name = ritualName
	convertedCfg.Metadata.StatusHistory = nil

	schedule := opts.frequency
	if schedule == "" {
		schedule = "manual"
	}
	convertedCfg.RitualConfig = &config.RitualConfig{
		Schedule: schedule,
		RunCount: 0,
	}

	// Resolve workspace root for path-relative saving.
	wsRoot := ""
	if ws, wsErr := workspace.FindWorkspace(ctx, cwd); wsErr == nil {
		wsRoot = ws.Root
	}

	if err := config.SaveFestivalConfig(destPath, wsRoot, convertedCfg); err != nil {
		return errors.Wrap(err, "saving converted festival config").
			WithField("path", destPath)
	}

	// Optionally move the source to dungeon/archived.
	var archivedPath string
	if opts.moveSource {
		archivedPath = filepath.Join(festivalsRoot, "dungeon", "archived", sourceDirName)
		if err := os.MkdirAll(filepath.Dir(archivedPath), 0o755); err != nil {
			return errors.IO("creating archive directory", err)
		}
		if err := os.Rename(sourcePath, archivedPath); err != nil {
			return errors.IO("moving source to archive", err)
		}
	}

	if opts.json {
		result := map[string]any{
			"success":      true,
			"action":       "ritual_convert",
			"source":       sourcePath,
			"source_dir":   sourceDirName,
			"dest":         destPath,
			"ritual_dir":   ritualDirName,
			"ritual_id":    ritualID,
			"ritual_name":  ritualName,
			"schedule":     schedule,
			"run_count":    0,
			"moved_source": opts.moveSource,
		}
		if archivedPath != "" {
			result["archived_to"] = archivedPath
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	display.Success("Converted festival to ritual: %s", ritualDirName)
	display.Info("  Source: %s", sourcePath)
	display.Info("  Destination: %s", destPath)
	display.Info("  Ritual ID: %s", ritualID)
	if opts.frequency != "" {
		display.Info("  Schedule: %s", opts.frequency)
	}
	if opts.moveSource {
		display.Info("  Source archived to: %s", archivedPath)
	}

	return nil
}

func reportConvertDryRun(display *ui.UI, opts *convertOptions, sourcePath, destPath, ritualDirName, ritualID, ritualName string) error {
	if opts.json {
		result := map[string]any{
			"success":           true,
			"action":            "ritual_convert_dry_run",
			"source":            sourcePath,
			"dest":              destPath,
			"ritual_dir":        ritualDirName,
			"ritual_id":         ritualID,
			"ritual_name":       ritualName,
			"schedule":          opts.frequency,
			"would_move_source": opts.moveSource,
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	display.Info("Dry run — no changes written")
	display.Info("  Would copy: %s", sourcePath)
	display.Info("  To: %s", destPath)
	display.Info("  Ritual ID: %s", ritualID)
	display.Info("  Name: %s", ritualName)
	if opts.frequency != "" {
		display.Info("  Schedule: %s", opts.frequency)
	}
	if opts.moveSource {
		display.Info("  Would move source to: dungeon/archived/")
	}
	return nil
}

// findFestivalForConvert searches all status directories for a matching festival.
// Returns the path and its status directory name.
func findFestivalForConvert(festivalsRoot, nameOrID string) (string, string, error) {
	for _, status := range id.StatusDirectories {
		statusPath := workspace.JoinStatus(festivalsRoot, status)

		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			entryPath := filepath.Join(statusPath, entry.Name())

			// Try exact directory name match.
			if entry.Name() == nameOrID {
				return entryPath, status, nil
			}

			// Try matching by festival ID suffix.
			if dirID, idErr := id.ExtractIDFromDirName(entry.Name()); idErr == nil && dirID == nameOrID {
				return entryPath, status, nil
			}

			// Try substring match.
			if contains(entry.Name(), nameOrID) {
				return entryPath, status, nil
			}
		}

		// For dungeon statuses, check date subdirectories.
		if strings.HasPrefix(status, "dungeon/") {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				if !isDateDir(entry.Name()) {
					continue
				}
				datePath := filepath.Join(statusPath, entry.Name())
				dateEntries, dateErr := os.ReadDir(datePath)
				if dateErr != nil {
					continue
				}
				for _, de := range dateEntries {
					if !de.IsDir() {
						continue
					}
					entryPath := filepath.Join(datePath, de.Name())
					if de.Name() == nameOrID {
						return entryPath, status, nil
					}
					if dirID, idErr := id.ExtractIDFromDirName(de.Name()); idErr == nil && dirID == nameOrID {
						return entryPath, status, nil
					}
					if contains(de.Name(), nameOrID) {
						return entryPath, status, nil
					}
				}
			}
		}
	}

	return "", "", errors.NotFound("festival").
		WithField("query", nameOrID).
		WithHint("Run 'fest list --all' to see available festivals")
}

// checkChainMembership checks if a festival is part of any chain.
// Returns (blocked, message). If not in a chain, returns (false, "").
func checkChainMembership(ctx context.Context, festivalID, festivalsRoot string) (bool, string) {
	if festivalID == "" {
		return false, ""
	}

	c, _, err := chainpkg.FindForFestival(ctx, festivalID, festivalsRoot)
	if err != nil || c == nil {
		return false, ""
	}

	return true, fmt.Sprintf("festival %s is a node in chain %q", festivalID, c.Metadata.Name)
}

// isDateDir checks if a directory name looks like a date bucket.
func isDateDir(name string) bool {
	if len(name) < 7 {
		return false
	}
	// YYYY-MM or YYYY-MM-DD
	if len(name) == 7 {
		return name[4] == '-' && isDigit(name[5]) && isDigit(name[6])
	}
	if len(name) == 10 {
		return name[4] == '-' && name[7] == '-'
	}
	return false
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// sanitizeDirName replaces spaces and slashes in a name for use as a directory component.
func sanitizeDirName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	return name
}

// CompleteConvertSource provides shell completions for the ritual convert argument.
// It offers festivals from all working status directories (planning, ready, active)
// since those are the most common conversion sources.
func CompleteConvertSource(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	festivalsRoot, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsRoot == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	needle := strings.ToLower(toComplete)
	for _, status := range []string{"planning", "ready", "active"} {
		statusPath := workspace.JoinStatus(festivalsRoot, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if needle == "" || strings.Contains(strings.ToLower(name), needle) {
				results = append(results, name)
			}
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}
