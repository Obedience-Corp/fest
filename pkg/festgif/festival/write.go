package festival

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/pkg/festgif"
)

// WriteGIF renders into a temporary file beside out and renames it into place,
// so an interrupted render never leaves a partial GIF.
func WriteGIF(ctx context.Context, out string, replay *festgif.Replay) (festgif.Result, int64, error) {
	if err := ctx.Err(); err != nil {
		return festgif.Result{}, 0, err
	}
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
	if err := ctx.Err(); err != nil {
		return festgif.Result{}, 0, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return festgif.Result{}, 0, errors.IO("setting GIF permissions", err)
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		return festgif.Result{}, 0, errors.IO("writing GIF", err).WithHintf("path: %s", out)
	}
	return result, info.Size(), nil
}
