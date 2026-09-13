// Package gif implements `fest gif`: render a festival's execution replay as
// an animated GIF.
package gif

import (
	"context"
	"fmt"
	"math"
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
	embed    bool
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
the dungeon. The GIF is written to ./<festival>.gif unless --out is given.
Use --embed to save festival-replay.gif inside the festival and add a relative
image link to FESTIVAL_OVERVIEW.md (creating the overview if needed). Repeating
--embed refreshes the replay without duplicating the link. --embed and --out
cannot be combined.

Promoting or setting a festival to completed does this automatically before
the status change is committed. Use --embed to refresh or retry that replay.

At default speed, related task changes are grouped by sequence and each
update holds for at least 2 seconds. Row backgrounds stay steady. Replays
target about a minute; distinct sequences and important outcomes can extend
that. Rejections and hook results get extra reading time.
Use --speed to play it faster or slower.`,
		Example: `  fest gif                          # festival in the current directory
  fest gif my-festival              # by name, from anywhere in a camp
  fest gif --festival DM0001        # by selector
  fest gif -o docs/replay.gif       # choose the output file
  fest gif --embed                  # save and embed the replay in the overview
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
	cmd.Flags().BoolVar(&opts.embed, "embed", false, "save festival-replay.gif in the festival and embed it in FESTIVAL_OVERVIEW.md")
	cmd.MarkFlagsMutuallyExclusive("embed", "out")
	cmd.Flags().Float64Var(&opts.speed, "speed", 1, "playback speed: 2 is twice as fast, 0.5 is half speed")
	return cmd
}

func run(cmd *cobra.Command, target string, opts *options) error {
	ctx := cmd.Context()
	festival, err := resolveFestival(ctx, target, opts.festival)
	if err != nil {
		return err
	}
	if opts.speed <= 0 || math.IsNaN(opts.speed) || math.IsInf(opts.speed, 0) {
		return errors.Validation("speed must be finite and greater than zero").WithOp("gif").
			WithHintf("got %v; 2 is twice as fast, 0.5 is half speed", opts.speed)
	}
	var result festgif.Result
	var size int64
	out := opts.out
	if opts.embed {
		out = filepath.Join(festival.Path, replay.ReplayFilename)
		result, size, err = replay.Embed(ctx, festival.Path, opts.speed)
	} else {
		in, loadErr := replay.Load(ctx, festival.Path)
		if loadErr != nil {
			return errors.Wrap(loadErr, "loading festival replay").WithOp("gif")
		}
		plan := festgif.Plan(in, festgif.DefaultTiming.Scaled(1/opts.speed))
		if out == "" {
			out = festival.Name + ".gif"
		}
		out, err = filepath.Abs(out)
		if err != nil {
			return errors.IO("resolving output path", err).WithOp("gif")
		}
		result, size, err = replay.WriteGIF(ctx, out, plan)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s Wrote %s (%d frames, %s)\n",
		ui.Success("✓"), out, result.Frames, formatBytes(size))
	return err
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
			WithHint("run fest gif inside a festival, pass its name, or use --festival <selector>; fest list shows festivals")
	}
	return festival, err
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
