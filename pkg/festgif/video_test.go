package festgif

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestConcatScriptRepeatsTheLastFrame(t *testing.T) {
	got := concatScript([]heldFrame{
		{path: "/tmp/a.png", seconds: 2},
		{path: "/tmp/it's.png", seconds: 1},
	})
	want := "ffconcat version 1.0\n" +
		"file '/tmp/a.png'\n" +
		"option framerate 1000\n" +
		"duration 2.000\n" +
		"file '/tmp/it'\\''s.png'\n" +
		"option framerate 1000\n" +
		"duration 1.000\n" +
		"file '/tmp/it'\\''s.png'\n" +
		"option framerate 1000\n"
	if got != want {
		t.Fatalf("concat script:\n%s\nwant:\n%s", got, want)
	}
}

func TestHoldSecondsUsesGIFCentiseconds(t *testing.T) {
	if got, want := holdSeconds(0, 30, 30), 1.0; got != want {
		t.Fatalf("first second = %v, want %v", got, want)
	}
	if got, want := holdSeconds(30, 90, 30), 2.0; got != want {
		t.Fatalf("two seconds = %v, want %v", got, want)
	}
	var sum float64
	frames := []int{0, 30, 90}
	for i, frame := range frames {
		end := 120
		if i+1 < len(frames) {
			end = frames[i+1]
		}
		sum += holdSeconds(frame, end, 30)
	}
	if got, want := sum, float64(centiseconds(120, 30))/100; got != want {
		t.Fatalf("holds sum to %v, GIF length is %v", got, want)
	}
}

func TestShareFilterFitsAVerticalFrame(t *testing.T) {
	got := shareFilter()
	for _, part := range []string{"scale=1080:1920", "pad=1080:1920", "0x0D1117", "fps=30", "format=yuv420p"} {
		if !strings.Contains(got, part) {
			t.Errorf("filter %q missing %s", got, part)
		}
	}
}

func TestRenderMP4ReportsMissingFFmpeg(t *testing.T) {
	orig := lookFFmpeg
	lookFFmpeg = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { lookFFmpeg = orig })

	out := filepath.Join(t.TempDir(), "replay.mp4")
	_, err := RenderMP4(t.Context(), out, Plan(Input{Title: "demo"}, DefaultTiming))
	if !errors.Is(err, ErrFFmpegMissing) {
		t.Fatalf("error = %v, want ErrFFmpegMissing", err)
	}
	if _, statErr := os.Stat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing ffmpeg left %s: %v", out, statErr)
	}
}

func TestWriteHoldVideoKeepsShortHolds(t *testing.T) {
	bin := ffmpegBin(t)
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "frame.png")
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// 10 ms and 30 ms are on the GIF grid and shorter than image2's 40 ms tick.
	pattern := []float64{0.01, 0.03}
	var frames []heldFrame
	var want []float64
	var at float64
	for range 20 {
		for _, hold := range pattern {
			frames = append(frames, heldFrame{path: pngPath, seconds: hold})
			want = append(want, at)
			at += hold
		}
	}
	out := filepath.Join(dir, "holds.mp4")
	if err := writeHoldVideo(t.Context(), bin, frames, out); err != nil {
		t.Fatal(err)
	}
	base := probeTimebase(t, out)
	if base < holdTimebase {
		t.Fatalf("time base 1/%d cannot store a 1 ms hold", base)
	}
	got := framePTS(t, out)
	if len(got) < len(want) {
		t.Fatalf("frames %d, want at least %d", len(got), len(want))
	}
	for i, pts := range want {
		if math.Abs(got[i]-pts) > 0.001 {
			t.Fatalf("frame %d at %v, want %v", i, got[i], pts)
		}
	}
}

func TestRenderMP4DenseReplayMatchesDuration(t *testing.T) {
	ffmpegBin(t)
	const beats = 40
	tasks := make([]*Node, beats)
	changes := make([]Beat, beats)
	final := make(map[string]State, beats)
	for i := range beats {
		key := fmt.Sprintf("t%d", i)
		tasks[i] = &Node{Key: key, Kind: KindTask, Label: key}
		changes[i] = Beat{Changes: []Change{{Key: key, State: State{Status: StatusCompleted}}}}
		final[key] = State{Status: StatusCompleted}
	}
	timing := DefaultTiming
	timing.IntroFrames = 1
	timing.FramesPerBeat = 1
	timing.MinFramesPerBeat = 1
	timing.MinBody = 1
	timing.MaxBody = 0
	timing.TailFrames = 1
	timing.MaxDwell = 0
	timing.HookFrames = 1
	timing.HeatFrames = 0
	timing.Dwell = Dwell{}
	replay := Plan(Input{
		Title: "dense",
		Phases: []*Node{{
			Key: "p", Kind: KindPhase, Label: "p",
			Children: []*Node{{Key: "s", Kind: KindSequence, Label: "s", Children: tasks}},
		}},
		Beats: changes,
		Final: final,
	}, timing)
	if gaps := shortHolds(replay); gaps < beats {
		t.Fatalf("dense replay has %d one-frame holds, want at least %d", gaps, beats)
	}
	out := filepath.Join(t.TempDir(), "replay.mp4")
	if _, err := RenderMP4(t.Context(), out, replay); err != nil {
		t.Fatal(err)
	}
	probe := probeVideo(t, out)
	want := float64(centiseconds(replay.Frames, max(1, replay.Timing.FPS))) / 100
	if probe.duration < want-0.05 || probe.duration > want+0.05 {
		t.Fatalf("duration %v, replay is %v seconds across %d frames", probe.duration, want, replay.Frames)
	}
}

// shortHolds counts painted gaps of a single replay frame. Those are 30 ms at
// 30 fps, the holds a 25 fps clock rounds away.
func shortHolds(r *Replay) int {
	frames := r.paintFrames()
	n := 0
	for i, frame := range frames {
		end := r.Frames
		if i+1 < len(frames) {
			end = frames[i+1]
		}
		if end-frame == 1 {
			n++
		}
	}
	return n
}

func ffmpegBin(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	return bin
}

func probeTimebase(t *testing.T, path string) int {
	t.Helper()
	out := probeValue(t, path, "stream=time_base")
	_, den, ok := strings.Cut(strings.TrimSpace(out), "/")
	if !ok {
		t.Fatalf("time base %q", out)
	}
	n, err := strconv.Atoi(den)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func framePTS(t *testing.T, path string) []float64 {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_frames", "-show_entries", "frame=pts_time", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Frames []struct {
			PtsTime string `json:"pts_time"`
		} `json:"frames"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	pts := make([]float64, 0, len(raw.Frames))
	for _, frame := range raw.Frames {
		v, err := strconv.ParseFloat(frame.PtsTime, 64)
		if err != nil {
			t.Fatal(err)
		}
		pts = append(pts, v)
	}
	return pts
}

func probeValue(t *testing.T, path, entries string) string {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", entries, "-of", "default=nw=1:nk=1", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRenderMP4WritesAVerticalVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	timing := DefaultTiming
	timing.IntroFrames = 2
	timing.MinBody = 4
	timing.TailFrames = 2
	timing.MaxBody = 4
	replay := Plan(Input{Title: "demo"}, timing)
	out := filepath.Join(t.TempDir(), "replay.mp4")
	if _, err := RenderMP4(t.Context(), out, replay); err != nil {
		t.Fatal(err)
	}
	probe := probeVideo(t, out)
	if probe.video.CodecName != "h264" || probe.video.Width != shareWidth || probe.video.Height != shareHeight {
		t.Fatalf("video = %+v", probe.video)
	}
	if probe.video.PixFmt != "yuv420p" {
		t.Fatalf("pix_fmt = %s", probe.video.PixFmt)
	}
	if rate, err := parseRate(probe.video.AvgFrameRate); err != nil || rate < 29 || rate > 31 {
		t.Fatalf("frame rate %s", probe.video.AvgFrameRate)
	}
	if probe.audio.CodecName != "aac" || probe.audio.Channels != 2 || probe.audio.SampleRate != "48000" {
		t.Fatalf("audio = %+v", probe.audio)
	}
	want := float64(centiseconds(replay.Frames, max(1, replay.Timing.FPS))) / 100
	if probe.duration < want-0.05 || probe.duration > want+0.05 {
		t.Fatalf("duration %v, replay is %v seconds", probe.duration, want)
	}
}

type probeStream struct {
	CodecName    string `json:"codec_name"`
	CodecType    string `json:"codec_type"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	PixFmt       string `json:"pix_fmt"`
	AvgFrameRate string `json:"avg_frame_rate"`
	Channels     int    `json:"channels"`
	SampleRate   string `json:"sample_rate"`
}

type probed struct {
	video, audio probeStream
	duration     float64
}

func probeVideo(t *testing.T, path string) probed {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries",
		"format=duration:stream=codec_name,codec_type,width,height,pix_fmt,avg_frame_rate,channels,sample_rate",
		"-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Streams []probeStream `json:"streams"`
		Format  struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		t.Fatal(err)
	}
	var got probed
	for _, stream := range raw.Streams {
		switch stream.CodecType {
		case "video":
			got.video = stream
		case "audio":
			got.audio = stream
		}
	}
	duration, err := strconv.ParseFloat(raw.Format.Duration, 64)
	if err != nil {
		t.Fatal(err)
	}
	got.duration = duration
	return got
}

func parseRate(rate string) (float64, error) {
	num, den, ok := strings.Cut(rate, "/")
	if !ok {
		return strconv.ParseFloat(rate, 64)
	}
	n, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, err
	}
	d, err := strconv.ParseFloat(den, 64)
	if err != nil || d == 0 {
		return 0, err
	}
	return n / d, nil
}
