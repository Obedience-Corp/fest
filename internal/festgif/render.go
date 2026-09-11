package festgif

import (
	"context"
	"image"
	"image/gif"
	"io"
	"math"
)

// paletteSamples is how many evenly spaced frames choose the palette.
const paletteSamples = 48

// Result describes a written GIF.
type Result struct {
	Frames int
	Width  int
	Height int
}

// Render paints every frame of the replay and writes it to w as a looping GIF.
// It checks ctx between frames.
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
	cursor := NewCursor(r)
	for frame := 0; frame < r.Frames; frame++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		p.paint(img, frame, cursor.Advance(frame))
		enc.add(img, centiseconds(frame+1, fps)-centiseconds(frame, fps))
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
