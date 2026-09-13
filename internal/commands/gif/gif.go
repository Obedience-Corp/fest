// Package gif implements `fest gif`: render a festival's execution replay as
// an animated GIF.
package gif

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/Obedience-Corp/fest/pkg/festgif"
	replay "github.com/Obedience-Corp/fest/pkg/festgif/festival"
	"github.com/spf13/cobra"
)

type options struct {
	festival string
	out      string
	speed    float64
}

// NewGifCommand creates the `fest gif` command.
func NewGifCommand() *cobra.Command {
	opts := &options{}
	cmd := &cobra.Command{
		Use:   "gif [festival]",
		Short: "Render a festival's execution as an animated GIF",
		Long: `Render the festival tree as an animated GIF that replays its execution.
Each task, step, and gate changes state in the order fest recorded it, the
way fest watch shows it live. A gate waiting on the approval judge shows the
judge glyph and "Judge: waiting", then the verdict, including reject and
recheck loops. Each lifecycle hook run appears under the row it fired on. The
last frame matches fest show.

Works on any festival with a progress log, including completed festivals in
the dungeon. The festival can be the current directory, a name, a path, or a
--festival selector. The GIF is written to ./<festival>.gif unless --out is
given.

Every change holds long enough to read, and festivals with more changes than
fit show consecutive ordinary changes together rather than flashing past. Use
--speed to play it faster or slower.`,
		Example: `  fest gif                          # festival in the current directory
  fest gif my-festival              # by name, from anywhere in a camp
  fest gif festivals/.dungeon/completed/2026-01-01/my-festival   # by path
  fest gif --festival DM0001        # by selector
  fest gif -o docs/replay.gif       # choose the output file
  fest gif --speed 2                # twice as fast
  fest gif --speed 0.5              # half speed, easier to follow`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) > 0 {
				target = args[0]
			}
			return run(cmd, target, opts)
		},
	}
	cmd.Flags().StringVar(&opts.festival, "festival", "", "festival selector (name or ID) from within a camp")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "output file (default ./<festival>.gif)")
	cmd.Flags().Float64Var(&opts.speed, "speed", 1, "playback speed: 2 is twice as fast, 0.5 is half speed")
	return cmd
}

func run(cmd *cobra.Command, target string, opts *options) error {
	ctx := cmd.Context()
	festival, err := resolveFestival(ctx, target, opts.festival)
	if err != nil {
		return err
	}
	if opts.speed <= 0 {
		return errors.Validation("speed must be greater than zero").WithOp("gif").
			WithHintf("got %v; 2 is twice as fast, 0.5 is half speed", opts.speed)
	}
	in, err := replay.Load(ctx, festival.Path)
	if err != nil {
		return errors.Wrap(err, "loading festival replay").WithOp("gif")
	}
	plan := festgif.Plan(in, festgif.DefaultTiming.Scaled(1/opts.speed))

	out := opts.out
	if out == "" {
		out = festival.Name + ".gif"
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return errors.IO("resolving output path", err).WithOp("gif")
	}
	result, size, err := writeGIF(ctx, out, plan)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s Wrote %s (%d frames, %s)\n",
		ui.Success("✓"), out, result.Frames, formatBytes(size))
	return err
}

// gifMode is the permission the finished GIF gets. os.CreateTemp creates the
// scratch file 0600, which would otherwise survive the rename.
const gifMode = 0o644

// writeGIF renders into a temporary file beside out and renames it into place,
// so an interrupted render never leaves a partial GIF.
func writeGIF(ctx context.Context, out string, replay *festgif.Replay) (festgif.Result, int64, error) {
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return festgif.Result{}, 0, errors.IO("creating output directory", err).WithHintf("path: %s", dir)
	}
	tmp, err := os.CreateTemp(dir, ".fest-gif-*.tmp")
	if err != nil {
		return festgif.Result{}, 0, errors.IO("creating output file", err).WithHintf("path: %s", dir)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	w := bufio.NewWriter(tmp)
	result, err := festgif.Render(ctx, w, replay)
	if err == nil {
		err = w.Flush()
	}
	if err == nil {
		err = tmp.Chmod(gifMode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return festgif.Result{}, 0, errors.Wrap(err, "rendering GIF").WithOp("gif")
	}
	info, err := os.Stat(tmp.Name())
	if err != nil {
		return festgif.Result{}, 0, errors.IO("reading rendered GIF", err)
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		return festgif.Result{}, 0, errors.IO("writing GIF", err).WithHintf("path: %s", out)
	}
	return result, info.Size(), nil
}

func resolveFestival(ctx context.Context, target, selector string) (*show.FestivalInfo, error) {
	if target != "" && selector != "" {
		return nil, errors.Validation("cannot use positional festival with --festival").WithOp("gif")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.IO("getting current directory", err)
	}
	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	switch {
	case selector != "":
		path, err := shared.ResolveFestivalSelector(ctx, cwd, selector)
		if err != nil {
			return nil, err
		}
		return show.DetectCurrentFestival(ctx, path, campaignRoot)
	case target != "":
		if path, ok := existingDir(cwd, target); ok {
			festival, err := show.DetectCurrentFestival(ctx, path, campaignRoot)
			if err == nil || !errors.Is(err, errors.ErrCodeNotFound) {
				return festival, err
			}
		}
		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil {
			return nil, errors.Wrap(err, "finding festivals directory")
		}
		if festivalsDir == "" {
			return nil, errors.NotFound("festivals directory").WithOp("gif").
				WithHint("run fest gif from inside a camp, or pass --festival <selector>")
		}
		return show.FindFestivalByName(ctx, festivalsDir, target, campaignRoot)
	}

	start := cwd
	if path, err := shared.ResolveFestivalPath(cwd, ""); err == nil && path != "" {
		start = path
	}
	festival, err := show.DetectCurrentFestival(ctx, start, campaignRoot)
	if errors.Is(err, errors.ErrCodeNotFound) {
		return nil, errors.NotFound("festival").WithOp("gif").
			WithHint("run fest gif inside a festival, pass its name or path, or use --festival <selector>; fest list shows festivals")
	}
	return festival, err
}

// existingDir reports whether target names a directory, relative to cwd or
// absolute. A path inside a festival resolves the way the current directory
// does; a bare name that matches no festival on disk goes through the name
// lookup.
func existingDir(cwd, target string) (string, bool) {
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return path, true
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
