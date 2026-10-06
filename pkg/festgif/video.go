package festgif

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	shareWidth  = 1080
	shareHeight = 1920
	shareFPS    = 30
	// holdTimebase is 1 ms. image2 defaults to 25 fps, and an H.264 time base
	// at that rate is 40 ms, which cannot store the 10 ms grid from holdSeconds
	// (one replay frame at 30 fps is 30 ms).
	holdTimebase = 1000
)

// ErrFFmpegMissing means the ffmpeg binary is not on PATH.
var ErrFFmpegMissing = errors.New("ffmpeg is not on PATH")

// lookFFmpeg is replaced in tests.
var lookFFmpeg = exec.LookPath

// RenderMP4 paints each visual change and encodes an H.264 MP4 through ffmpeg.
// Holds match the GIF: only frames that change are painted, and each keeps the
// replay's beat timing. The picture is fitted inside 1080x1920 and padded with
// the replay background. A silent stereo track is muxed so feeds that require
// audio accept the file.
func RenderMP4(ctx context.Context, out string, r *Replay) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	bin, err := lookFFmpeg("ffmpeg")
	if err != nil {
		return Result{}, ErrFFmpegMissing
	}
	if r == nil || r.Frames <= 0 {
		return Result{}, fmt.Errorf("replay has no frames")
	}

	faces, err := loadFaces()
	if err != nil {
		return Result{}, err
	}
	p := newPainter(r, faces)
	img := image.NewRGBA(p.bounds())

	dir, err := os.MkdirTemp("", "fest-mp4-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	fps := max(1, r.Timing.FPS)
	cur := newCursor(r)
	frames := r.paintFrames()
	held := make([]heldFrame, 0, len(frames))
	for i, frame := range frames {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		p.paint(img, frame, cur.Advance(frame))
		path := filepath.Join(dir, fmt.Sprintf("frame-%06d.png", i))
		if err := writePNG(path, img); err != nil {
			return Result{}, err
		}
		end := r.Frames
		if i+1 < len(frames) {
			end = frames[i+1]
		}
		held = append(held, heldFrame{path: path, seconds: holdSeconds(frame, end, fps)})
	}

	// The concat demuxer keeps each still as one frame and stores the hold in
	// the timestamp gap. A single fps filter does not fill those gaps, so the
	// first pass records the holds at a 1 ms time base and the second expands
	// them to 30 fps.
	timed := filepath.Join(dir, "timed.mp4")
	if err := writeHoldVideo(ctx, bin, held, timed); err != nil {
		return Result{}, err
	}
	seconds := float64(centiseconds(r.Frames, fps)) / 100
	if err := runFFmpeg(ctx, bin, shareEncodeArgs(timed, seconds, out)); err != nil {
		return Result{}, err
	}
	return Result{Frames: r.Frames, Width: p.width, Height: p.height}, nil
}

func runFFmpeg(ctx context.Context, bin string, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 2000 {
			msg = msg[len(msg)-2000:]
		}
		if msg == "" {
			return fmt.Errorf("ffmpeg: %w", err)
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, msg)
	}
	return nil
}

// frameSyncArgs selects the flag that keeps concat timestamp gaps. ffmpeg 8
// dropped -vsync in favor of -fps_mode.
func frameSyncArgs(ctx context.Context, bin string) ([]string, error) {
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-h", "full")
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}
	if bytes.Contains(out, []byte("-fps_mode")) {
		return []string{"-fps_mode", "passthrough"}, nil
	}
	return []string{"-vsync", "0"}, nil
}

func writePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, img)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

type heldFrame struct {
	path    string
	seconds float64
}

// holdSeconds is how long a painted frame stays up, on the same centisecond
// grid the GIF encoder uses.
func holdSeconds(start, end, fps int) float64 {
	d := centiseconds(end, fps) - centiseconds(start, fps)
	if d < 1 {
		d = 1
	}
	return float64(d) / 100
}

func concatScript(frames []heldFrame) string {
	var b strings.Builder
	b.WriteString("ffconcat version 1.0\n")
	for _, frame := range frames {
		writeConcatFile(&b, frame, true)
	}
	if len(frames) > 0 {
		writeConcatFile(&b, frames[len(frames)-1], false)
	}
	return b.String()
}

// writeConcatFile names one still. option comes after file and applies to that
// file: without it, image2's 25 fps time base rounds the hold.
func writeConcatFile(b *strings.Builder, frame heldFrame, duration bool) {
	fmt.Fprintf(b, "file %s\n", quoteConcatPath(frame.path))
	fmt.Fprintf(b, "option framerate %d\n", holdTimebase)
	if duration {
		fmt.Fprintf(b, "duration %.3f\n", frame.seconds)
	}
}

// writeHoldVideo stores each still for its hold. The image and the MP4 track
// both use a 1 ms clock so short holds are not rounded to 40 ms.
func writeHoldVideo(ctx context.Context, bin string, frames []heldFrame, out string) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames")
	}
	sync, err := frameSyncArgs(ctx, bin)
	if err != nil {
		return err
	}
	concatFile, err := os.CreateTemp(filepath.Dir(out), ".fest-holds-*.ffconcat")
	if err != nil {
		return err
	}
	concatPath := concatFile.Name()
	defer func() { _ = os.Remove(concatPath) }()
	if _, err := concatFile.WriteString(concatScript(frames)); err != nil {
		_ = concatFile.Close()
		return err
	}
	if err := concatFile.Close(); err != nil {
		return err
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", concatPath}
	args = append(args, sync...)
	args = append(args,
		"-c:v", "libx264", "-qp", "0", "-preset", "ultrafast",
		"-enc_time_base", "0.001",
		"-video_track_timescale", strconv.Itoa(holdTimebase),
		"-an", out,
	)
	return runFFmpeg(ctx, bin, args)
}

func quoteConcatPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func shareFilter() string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease:flags=lanczos,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:0x%02X%02X%02X,fps=%d,format=yuv420p",
		shareWidth, shareHeight, shareWidth, shareHeight, colorBg.R, colorBg.G, colorBg.B, shareFPS,
	)
}

func shareEncodeArgs(timed string, seconds float64, out string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", timed,
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000",
		"-t", fmt.Sprintf("%.3f", seconds),
		"-map", "0:v:0", "-map", "1:a:0",
		"-vf", shareFilter(),
		"-c:v", "libx264", "-profile:v", "high", "-level", "4.1",
		"-preset", "fast", "-crf", "18", "-pix_fmt", "yuv420p", "-g", "60",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000",
		"-shortest", "-movflags", "+faststart",
		out,
	}
}
