package festgif

import (
	"context"
	"image"
	"image/gif"
	"io"
	"math"
	"sort"
)

// paletteSamples is how many evenly spaced frames choose the palette.
const paletteSamples = 48

// Result describes a written GIF.
type Result struct {
	Frames int
	Width  int
	Height int
}

// Render paints each visual change and writes it to w as a looping GIF.
// Static reading holds are encoded as delays, without repainting identical
// frames. It checks ctx between frames.
func Render(ctx context.Context, w io.Writer, r *Replay) (Result, error) {
	f, err := loadFaces()
	if err != nil {
		return Result{}, err
	}
	p := newPainter(r, f)
	img := image.NewRGBA(p.bounds())

	samples := make([]*image.RGBA, 0, paletteSamples+1)
	for i := 0; i <= paletteSamples; i++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		frame := int(math.Round(float64(i) / paletteSamples * float64(r.Frames-1)))
		sample := image.NewRGBA(p.bounds())
		p.paint(sample, frame, r.StateAt(frame))
		samples = append(samples, sample)
	}
	enc := newEncoder(buildPalette(samples), p.bounds())

	fps := max(1, r.Timing.FPS)
	cur := newCursor(r)
	frames := r.paintFrames()
	for i, frame := range frames {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		p.paint(img, frame, cur.Advance(frame))
		end := r.Frames
		if i+1 < len(frames) {
			end = frames[i+1]
		}
		enc.add(img, centiseconds(end, fps)-centiseconds(frame, fps))
	}
	if err := gif.EncodeAll(w, &enc.out); err != nil {
		return Result{}, err
	}
	return Result{Frames: r.Frames, Width: p.width, Height: p.height}, nil
}

// centiseconds is when frame starts, in GIF delay units, so rounded delays
// never drift from the frame rate.
func centiseconds(frame, fps int) int {
	return int(math.Round(float64(frame) * 100 / float64(fps)))
}

// paintFrames lists every frame where the painter can change: intro animation,
// state changes (including optional pulses), hook appearances and expirations.
// Long reading holds therefore cost no more rendering work than short holds.
func (r *Replay) paintFrames() []int {
	frames := map[int]bool{0: true}
	add := func(frame int) {
		if frame >= 0 && frame < r.Frames {
			frames[frame] = true
		}
	}
	for frame := 1; frame <= r.IntroFrames && frame < r.Frames; frame++ {
		add(frame)
	}
	for _, tr := range r.transitions {
		add(tr.Frame)
		for offset := 1; offset <= r.Timing.HeatFrames && tr.Frame+offset < r.Frames; offset++ {
			add(tr.Frame + offset)
		}
	}
	for _, h := range r.hooks {
		add(h.Frame)
		add(h.Frame + r.Timing.HookFrames)
	}
	ordered := make([]int, 0, len(frames))
	for frame := range frames {
		ordered = append(ordered, frame)
	}
	sort.Ints(ordered)
	return ordered
}
