package festival

import (
	"bufio"
	"context"
	stderrors "errors"
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

// WriteMP4 renders into a temporary file beside out and renames it into place,
// so an interrupted encode never leaves a partial MP4 where the caller asked.
func WriteMP4(ctx context.Context, out string, replay *festgif.Replay) (festgif.Result, int64, error) {
	if err := ctx.Err(); err != nil {
		return festgif.Result{}, 0, err
	}
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return festgif.Result{}, 0, errors.IO("creating output directory", err).WithHintf("path: %s", dir)
	}
	tmp, err := os.CreateTemp(dir, ".fest-mp4-*.mp4")
	if err != nil {
		return festgif.Result{}, 0, errors.IO("creating output file", err).WithHintf("path: %s", dir)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return festgif.Result{}, 0, errors.IO("creating output file", err).WithHintf("path: %s", tmpName)
	}
	defer func() { _ = os.Remove(tmpName) }()

	result, err := festgif.RenderMP4(ctx, tmpName, replay)
	if stderrors.Is(err, festgif.ErrFFmpegMissing) {
		return festgif.Result{}, 0, errors.NotFound("ffmpeg").WithOp("gif").
			WithHint("install ffmpeg, then rerun fest gif --mp4")
	}
	if err != nil {
		return festgif.Result{}, 0, errors.Wrap(err, "rendering MP4").WithOp("gif")
	}
	info, err := os.Stat(tmpName)
	if err != nil {
		return festgif.Result{}, 0, errors.IO("reading rendered MP4", err)
	}
	if err := ctx.Err(); err != nil {
		return festgif.Result{}, 0, err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return festgif.Result{}, 0, errors.IO("setting MP4 permissions", err)
	}
	if err := os.Rename(tmpName, out); err != nil {
		return festgif.Result{}, 0, errors.IO("writing MP4", err).WithHintf("path: %s", out)
	}
	return result, info.Size(), nil
}
