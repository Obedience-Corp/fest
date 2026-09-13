package festival

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/pkg/festgif"
)

const (
	// ReplayFilename is the generated asset beside the festival overview.
	ReplayFilename   = "festival-replay.gif"
	OverviewFilename = "FESTIVAL_OVERVIEW.md"
	replayStart      = "<!-- fest:replay:start -->"
	replayEnd        = "<!-- fest:replay:end -->"
	replayBlock      = replayStart + "\n## Execution replay\n\n![Festival execution replay](festival-replay.gif)\n" + replayEnd
)

// Embed renders the recorded execution and links it from the overview. Only
// the managed replay block is replaced; the rest of the document is preserved.
// The GIF is published before the overview so a failed render never adds a
// broken image link. A missing event log uses the current tree's final state.
func Embed(ctx context.Context, dir string, speed float64) (festgif.Result, int64, error) {
	if err := ctx.Err(); err != nil {
		return festgif.Result{}, 0, err
	}
	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		return festgif.Result{}, 0, errors.Validation("speed must be finite and greater than zero")
	}
	overview := filepath.Join(dir, OverviewFilename)
	mode := os.FileMode(0o644)
	info, err := os.Lstat(overview)
	if err != nil && !os.IsNotExist(err) {
		return festgif.Result{}, 0, errors.IO("checking festival overview", err)
	}
	content := []byte("# Festival Overview\n")
	if err == nil {
		if !info.Mode().IsRegular() {
			return festgif.Result{}, 0, errors.Validation("festival overview must be a regular file")
		}
		mode = info.Mode().Perm()
		content, err = os.ReadFile(overview)
		if err != nil {
			return festgif.Result{}, 0, errors.IO("reading festival overview", err)
		}
	}
	updated, managed, err := embedMarkdown(content)
	if err != nil {
		return festgif.Result{}, 0, err
	}
	out := filepath.Join(dir, ReplayFilename)
	if !managed {
		if _, err := os.Lstat(out); err == nil {
			return festgif.Result{}, 0, errors.Validation("replay filename is already in use").
				WithHintf("move %s aside before retrying; the existing file was preserved", out)
		} else if !os.IsNotExist(err) {
			return festgif.Result{}, 0, errors.IO("checking replay destination", err)
		}
	}
	in, err := Load(ctx, dir)
	if err != nil {
		return festgif.Result{}, 0, errors.Wrap(err, "loading festival replay")
	}
	result, size, err := WriteGIF(ctx, out, festgif.Plan(in, festgif.DefaultTiming.Scaled(1/speed)))
	if err != nil {
		return festgif.Result{}, 0, err
	}
	if bytes.Equal(content, updated) {
		return result, size, nil
	}
	if err := writeOverview(ctx, overview, updated, mode); err != nil {
		// A first embed has no ownership marker until the overview is written.
		// Remove our new asset on failure so --embed can safely retry.
		if !managed {
			if removeErr := os.Remove(out); removeErr != nil {
				return result, size, errors.Wrapf(err, "removing unpublished replay failed: %v", removeErr)
			}
		}
		return result, size, err
	}
	return result, size, nil
}

func embedMarkdown(content []byte) ([]byte, bool, error) {
	start, end := bytes.Index(content, []byte(replayStart)), bytes.Index(content, []byte(replayEnd))
	if start < 0 && end < 0 {
		updated := bytes.Clone(content)
		if len(updated) > 0 && updated[len(updated)-1] != '\n' {
			updated = append(updated, '\n')
		}
		updated = append(updated, '\n')
		return append(updated, []byte(replayBlock+"\n")...), false, nil
	}
	if start < 0 || end < start || bytes.Count(content, []byte(replayStart)) != 1 || bytes.Count(content, []byte(replayEnd)) != 1 {
		return nil, false, errors.Validation("festival overview has an incomplete or duplicate fest replay block").
			WithHint("repair the fest:replay:start and fest:replay:end markers before retrying")
	}
	updated := append(bytes.Clone(content[:start]), []byte(replayBlock)...)
	return append(updated, content[end+len(replayEnd):]...), true, nil
}

func writeOverview(ctx context.Context, path string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".fest-overview-*.tmp")
	if err != nil {
		return errors.IO("creating festival overview", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.Write(content)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.IO("writing festival overview", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.IO("publishing festival overview", err)
	}
	return nil
}
