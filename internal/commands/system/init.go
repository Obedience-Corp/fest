package system

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/bundled"
	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/config"
	festcontract "github.com/Obedience-Corp/fest/internal/contract"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/fileops"
	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/Obedience-Corp/obey-shared/contract"
	"github.com/spf13/cobra"
)

// InitOptions holds options for the init command.
type InitOptions struct {
	Force       bool
	From        string
	Minimal     bool
	NoChecksums bool
	Register    bool
	Unregister  bool
}

// NewInitCommand creates the init command
func NewInitCommand() *cobra.Command {
	opts := &InitOptions{}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new festival directory structure",
		Annotations: map[string]string{
			"scope": "global",
		},
		Long: `Initialize a new festival directory structure in the current or specified directory.

This command copies the festival template structure from your local cache
(populated by 'fest sync') and creates initial checksum tracking.

Workspace Registration:
  Use --register to mark an existing festivals/ directory as your active workspace.
  This enables 'fest go' to navigate directly to it from anywhere in the project.

  Use --unregister to remove the workspace marker, making the festivals/
  directory invisible to 'fest go' (useful for source repositories).`,
		Example: `  fest init                      # Initialize in current directory
  fest init ./my-project         # Initialize in specified directory
  fest init --force              # Overwrite existing festival
  fest init --minimal            # Create minimal structure only
  fest init --register           # Register existing festivals as workspace
  fest init --unregister         # Remove workspace registration`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}
			return RunInit(cmd.Context(), targetPath, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite existing festival directory")
	cmd.Flags().StringVar(&opts.From, "from", "", "source directory (default: ~/.obey/fest)")
	cmd.Flags().BoolVar(&opts.Minimal, "minimal", false, "create minimal structure only")
	cmd.Flags().BoolVar(&opts.NoChecksums, "no-checksums", false, "skip checksum generation")
	cmd.Flags().BoolVar(&opts.Register, "register", false, "register existing festivals as active workspace")
	cmd.Flags().BoolVar(&opts.Unregister, "unregister", false, "remove workspace registration")

	return cmd
}

// RunInit executes the init command logic.
func RunInit(ctx context.Context, targetPath string, opts *InitOptions) error {
	// Create UI handler
	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// Convert to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return errors.Wrap(err, "resolving path").WithField("path", targetPath)
	}

	// Prefer campaign-relative display paths; fall back to the init target
	// (festival workspace root) when not inside a campaign.
	campaignRoot, campaignErr := workspace.DetectCampaign(ctx, absPath)
	displayRoot := resolveDisplayRoot(campaignRoot, campaignErr, absPath)
	showPath := func(p string) string {
		return pathutil.DisplayPath(p, displayRoot)
	}

	// Handle --register flag: register existing festivals directory
	if opts.Register {
		return runRegister(ctx, absPath, display)
	}

	// Handle --unregister flag: remove workspace marker
	if opts.Unregister {
		return runUnregister(ctx, absPath, display)
	}

	// Check if festival already exists
	festivalPath := filepath.Join(absPath, "festivals")
	if fileops.Exists(festivalPath) && !opts.Force {
		if !display.Confirm("Festival directory already exists at %s. Overwrite?", showPath(festivalPath)) {
			display.Warning("Initialization cancelled")
			return nil
		}
	}

	// Determine source directory
	sourceDir := opts.From
	if sourceDir == "" {
		sourceDir = filepath.Join(config.ConfigDir(), "festivals")
	}

	// An explicit --from names a directory the operator expects to exist.
	// Syncing writes to the config dir instead, so it could never satisfy this
	// path; say what is missing rather than attempt an unrelated download.
	if opts.From != "" && !fileops.Exists(sourceDir) {
		return errors.NotFound("source directory").
			WithField("path", sourceDir).
			WithHint("--from must point at an existing festival template directory")
	}

	// Check if source exists - if not, auto-sync to populate it
	if !fileops.Exists(sourceDir) {
		display.Info("Template cache not found, syncing from GitHub...")

		// Run sync to populate the source directory
		syncOpts := &syncOptions{
			dryRun: false,
		}
		syncErr := runSync(ctx, nil, syncOpts)
		if syncErr != nil {
			// Templates only ever existed after a successful sync, so a machine
			// that cannot reach GitHub could not run 'fest init' at all. The
			// same scaffold ships inside the binary; use it and let sync become
			// the way to update the methodology rather than to obtain it.
			if !usesBundledMethodology(ctx) {
				return errors.Wrap(syncErr, "auto-sync failed").
					WithField("path", sourceDir).
					WithHint("check your internet connection or run 'fest sync' manually")
			}
			display.Warning("Could not reach the methodology repository: %s", errors.Message(syncErr))
			display.Info("Using the methodology bundled with this fest build.")
			display.Info("Run 'fest sync' once the network is available to pick up newer templates.")
			if err := bundled.Seed(ctx, sourceDir); err != nil {
				return errors.Wrap(err, "seeding bundled methodology").
					WithField("path", sourceDir).
					WithHint("could not reach GitHub and could not write the bundled copy; check permissions on " + sourceDir)
			}
		}

		// Verify sync worked
		if !fileops.Exists(sourceDir) {
			return errors.NotFound("source directory after sync").
				WithField("path", sourceDir).
				WithHint("sync completed but templates not found - check 'fest sync' output")
		}
	}

	display.Info("Initializing festival structure at %s...", showPath(festivalPath))

	// Create festivals directory if it doesn't exist
	if err := os.MkdirAll(festivalPath, 0755); err != nil {
		return errors.IO("creating directory", err).WithField("path", festivalPath)
	}

	// Copy structure
	copier := fileops.NewCopier()
	if opts.Minimal {
		// Copy only essential directories
		essentialDirs := []string{".festival", "active", "planning"}
		for _, dir := range essentialDirs {
			src := filepath.Join(sourceDir, dir)
			dst := filepath.Join(festivalPath, dir)
			if fileops.Exists(src) {
				if err := copier.CopyDirectory(ctx, src, dst); err != nil {
					return errors.IO("copying directory", err).WithField("source", src).WithField("destination", dst)
				}
			}
		}
	} else {
		// Copy everything
		if err := copier.CopyDirectory(ctx, sourceDir, festivalPath); err != nil {
			return errors.IO("copying festival structure", err).WithField("source", sourceDir).WithField("destination", festivalPath)
		}
	}

	// The template tree always ships the visible dungeon/ spelling. Match it to
	// the campaign so a dungeon_hidden campaign scaffolds festivals/.dungeon
	// rather than a stray visible dungeon while the rest of the campaign is
	// hidden. No-op for visible campaigns and standalone (non-campaign) trees.
	if err := workspace.NormalizeNewDungeonSpelling(festivalPath); err != nil {
		return err
	}

	// Generate checksums unless disabled
	if !opts.NoChecksums {
		display.Info("Generating .festival checksums...")

		// Ensure .state directory exists
		stateDir := filepath.Join(festivalPath, ".festival", workspace.StateDir)
		if err := os.MkdirAll(stateDir, 0755); err != nil {
			return errors.IO("creating state directory", err).WithField("path", stateDir)
		}

		checksumFile := filepath.Join(stateDir, ".fest-checksums.json")

		// Only checksum the .festival directory
		festivalMetaDir := filepath.Join(festivalPath, ".festival")
		checksums, err := fileops.GenerateChecksums(ctx, festivalMetaDir)
		if err != nil {
			return errors.Wrap(err, "generating checksums").WithField("path", festivalMetaDir)
		}

		if err := fileops.SaveChecksums(ctx, checksumFile, checksums); err != nil {
			return errors.IO("saving checksums", err).WithField("path", checksumFile)
		}

		display.Info("Created checksum tracking at %s", showPath(checksumFile))
	}

	// Scaffold a user-owned .festival/config.yaml with a commented hooks example
	// if none exists. Written after checksums so it stays user-owned rather than
	// tracked as a synced methodology resource.
	if !config.WorkspaceConfigExists(festivalPath) {
		if err := config.SaveWorkspaceConfig(festivalPath, config.DefaultWorkspaceConfig()); err != nil {
			display.Warning("Could not write workspace config: %v", err)
		}
	}

	// Auto-register the new festivals directory as workspace
	if err := workspace.RegisterFestivals(festivalPath); err != nil {
		display.Warning("Could not register workspace marker: %v", err)
	} else {
		marker, _ := workspace.ReadMarker(festivalPath)
		if marker != nil {
			display.Info("Registered as workspace: %s", marker.Workspace)
		}
	}

	// Write fest entries to the campaign contract if inside a campaign.
	// This declares fest's state files and directories so the daemon knows
	// what to watch. If no campaign exists (standalone fest workspace),
	// skip gracefully -- the contract only matters when a daemon is present.
	if campaignErr == nil {
		contractPath := contract.ContractPath(campaignRoot)
		if err := contract.WriteEntries(contractPath, contract.OwnerFest, festcontract.FestEntries()); err != nil {
			display.Warning("Could not write contract entries: %v", err)
		} else {
			if shared.IsVerbose() {
				display.Info("Wrote fest entries to %s", showPath(contractPath))
			}
		}
	}

	// Show summary — user-facing paths stay campaign- or workspace-relative.
	display.Success("Successfully initialized festival structure at %s", showPath(festivalPath))
	display.Info("\nNext steps:")
	display.Info("  1. cd %s", showPath(absPath))
	display.Info("  2. Review festivals/.festival/README.md")
	display.Info("  3. Start planning your festival in festivals/planning/")
	display.Info("\nWorkspace navigation:")
	display.Info("  cd \"$(fest go --print)\"   # Navigate to festivals from anywhere")

	return nil
}

// resolveDisplayRoot chooses the root used for user-facing path display.
// Prefer campaign root when present; otherwise the festival workspace root
// (init target / parent of festivals/).
func resolveDisplayRoot(campaignRoot string, campaignErr error, festivalWorkspaceRoot string) string {
	if campaignErr == nil && campaignRoot != "" {
		return campaignRoot
	}
	return festivalWorkspaceRoot
}

// runRegister registers an existing festivals directory as the active workspace
func runRegister(ctx context.Context, targetPath string, display *ui.UI) error {
	// Find the festivals directory
	festivalsDir, err := findFestivalsDir(targetPath)
	if err != nil {
		return err
	}

	campaignRoot, campaignErr := workspace.DetectCampaign(ctx, festivalsDir)
	displayRoot := resolveDisplayRoot(campaignRoot, campaignErr, filepath.Dir(festivalsDir))
	showPath := func(p string) string {
		return pathutil.DisplayPath(p, displayRoot)
	}

	// Check if already registered
	if workspace.HasMarker(festivalsDir) {
		marker, _ := workspace.ReadMarker(festivalsDir)
		if marker != nil {
			display.Info("Already registered as workspace: %s", marker.Workspace)
			return nil
		}
	}

	// Register
	if err := workspace.RegisterFestivals(festivalsDir); err != nil {
		return errors.Wrap(err, "registering workspace").WithField("path", festivalsDir)
	}

	marker, _ := workspace.ReadMarker(festivalsDir)
	wsName := ""
	if marker != nil {
		wsName = marker.Workspace
	}

	display.Success("Registered %s as workspace: %s", showPath(festivalsDir), wsName)
	display.Info("You can now use 'cd \"$(fest go --print)\"' from anywhere in this project")

	return nil
}

// runUnregister removes the workspace marker from a festivals directory
func runUnregister(ctx context.Context, targetPath string, display *ui.UI) error {
	// Find the festivals directory
	festivalsDir, err := findFestivalsDir(targetPath)
	if err != nil {
		return err
	}

	campaignRoot, campaignErr := workspace.DetectCampaign(ctx, festivalsDir)
	displayRoot := resolveDisplayRoot(campaignRoot, campaignErr, filepath.Dir(festivalsDir))
	showPath := func(p string) string {
		return pathutil.DisplayPath(p, displayRoot)
	}

	// Check if registered
	if !workspace.HasMarker(festivalsDir) {
		display.Info("No workspace marker found at %s", showPath(festivalsDir))
		return nil
	}

	// Get workspace name before removing
	marker, _ := workspace.ReadMarker(festivalsDir)
	wsName := ""
	if marker != nil {
		wsName = marker.Workspace
	}

	// Unregister
	if err := workspace.UnregisterFestivals(festivalsDir); err != nil {
		return errors.Wrap(err, "unregistering workspace").WithField("path", festivalsDir)
	}

	display.Success("Unregistered workspace: %s", wsName)
	display.Info("This festivals directory will no longer be found by 'fest go'")

	return nil
}

// findFestivalsDir locates the festivals directory from a given path
func findFestivalsDir(targetPath string) (string, error) {
	// Check if target is already a festivals directory
	if filepath.Base(targetPath) == "festivals" {
		if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
			return targetPath, nil
		}
	}

	// Check if target contains a festivals directory
	festivalsDir := filepath.Join(targetPath, "festivals")
	if info, err := os.Stat(festivalsDir); err == nil && info.IsDir() {
		return festivalsDir, nil
	}

	// Walk up looking for festivals directory
	nearest, err := workspace.FindNearestFestivals(targetPath)
	if err != nil {
		return "", errors.Wrap(err, "finding festivals directory").WithField("path", targetPath)
	}
	if nearest == "" {
		return "", errors.NotFound("festivals directory").WithField("path", targetPath)
	}

	return nearest, nil
}
