package festgif

import (
	"image"
	"image/color"
	"image/gif"
	"sort"
)

// transparentIndex marks pixels a delta frame leaves unchanged.
const transparentIndex = 255

type colorCount struct {
	rgb   uint32
	count int
}

// buildPalette picks 255 colors for the frames by median cut, seeding the
// exact theme colors so flat fills never shift. The last entry is transparent.
func buildPalette(samples []*image.RGBA) color.Palette {
	seeds := []color.NRGBA{
		colorBg, colorBgAccent, colorGuide, colorText, colorDim, colorPhase, colorSequence,
		colorCompleted, colorInProgress, colorPending, colorFailed, colorGate, colorJudge,
	}
	seeded := map[uint32]bool{}
	pal := color.Palette{}
	for _, c := range seeds {
		k := pack(c.R, c.G, c.B)
		if !seeded[k] {
			seeded[k] = true
			pal = append(pal, color.RGBA{R: c.R, G: c.G, B: c.B, A: 0xff})
		}
	}

	hist := map[uint32]int{}
	for _, img := range samples {
		pix := img.Pix
		for i := 0; i+3 < len(pix); i += 4 {
			k := pack(pix[i], pix[i+1], pix[i+2])
			if !seeded[k] {
				hist[k]++
			}
		}
	}
	colors := make([]colorCount, 0, len(hist))
	for k, n := range hist {
		colors = append(colors, colorCount{k, n})
	}
	sort.Slice(colors, func(a, b int) bool { return colors[a].rgb < colors[b].rgb })
	for _, c := range medianCut(colors, transparentIndex-len(pal)) {
		pal = append(pal, c)
	}
	for len(pal) < transparentIndex {
		pal = append(pal, color.RGBA{A: 0xff})
	}
	return append(pal, color.RGBA{})
}

func pack(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

func channel(rgb uint32, ch int) int { return int(rgb>>(16-8*ch)) & 0xff }

// medianCut splits the colors into at most n boxes along their widest channel
// at the pixel-weighted median, and returns each box's weighted mean.
func medianCut(colors []colorCount, n int) []color.RGBA {
	if len(colors) == 0 || n <= 0 {
		return nil
	}
	boxes := [][]colorCount{colors}
	for len(boxes) < n {
		best, bestScore, bestCh := -1, 0, 0
		for i, b := range boxes {
			if len(b) < 2 {
				continue
			}
			ch, span := widest(b)
			score := span * weight(b)
			if score > bestScore {
				best, bestScore, bestCh = i, score, ch
			}
		}
		if best < 0 {
			break
		}
		b := boxes[best]
		sort.Slice(b, func(x, y int) bool { return channel(b[x].rgb, bestCh) < channel(b[y].rgb, bestCh) })
		half, acc, cut := weight(b)/2, 0, 1
		for i, c := range b {
			acc += c.count
			if acc >= half {
				cut = min(max(i+1, 1), len(b)-1)
				break
			}
		}
		boxes[best] = b[:cut]
		boxes = append(boxes, b[cut:])
	}
	out := make([]color.RGBA, 0, len(boxes))
	for _, b := range boxes {
		var r, g, bl, w int
		for _, c := range b {
			r += channel(c.rgb, 0) * c.count
			g += channel(c.rgb, 1) * c.count
			bl += channel(c.rgb, 2) * c.count
			w += c.count
		}
		out = append(out, color.RGBA{R: uint8(r / w), G: uint8(g / w), B: uint8(bl / w), A: 0xff})
	}
	return out
}

func widest(b []colorCount) (ch, span int) {
	for c := 0; c < 3; c++ {
		lo, hi := 255, 0
		for _, x := range b {
			v := channel(x.rgb, c)
			lo, hi = min(lo, v), max(hi, v)
		}
		if hi-lo > span {
			ch, span = c, hi-lo
		}
	}
	return ch, span
}

func weight(b []colorCount) int {
	w := 0
	for _, c := range b {
		w += c.count
	}
	return w
}

// encoder turns full RGBA frames into a GIF of delta frames: each frame keeps
// only the rectangle that changed, with unchanged pixels transparent, and a
// frame identical to the last one just extends its delay.
type encoder struct {
	pal    color.Palette
	lookup map[uint32]uint8
	prev   *image.RGBA
	index  []uint8
	stamp  []int
	frames int
	out    gif.GIF
}

func newEncoder(pal color.Palette, bounds image.Rectangle) *encoder {
	return &encoder{
		pal:    pal,
		lookup: map[uint32]uint8{},
		prev:   image.NewRGBA(bounds),
		index:  make([]uint8, bounds.Dx()*bounds.Dy()),
		stamp:  make([]int, bounds.Dx()*bounds.Dy()),
		out:    gif.GIF{Config: image.Config{ColorModel: pal, Width: bounds.Dx(), Height: bounds.Dy()}},
	}
}

func (e *encoder) nearest(r, g, b uint8) uint8 {
	k := pack(r, g, b)
	if i, ok := e.lookup[k]; ok {
		return i
	}
	best, bestD := 0, 1<<30
	for i := 0; i < transparentIndex; i++ {
		c := e.pal[i].(color.RGBA)
		dr, dg, db := int(c.R)-int(r), int(c.G)-int(g), int(c.B)-int(b)
		if d := dr*dr + dg*dg + db*db; d < bestD {
			best, bestD = i, d
		}
	}
	e.lookup[k] = uint8(best)
	return uint8(best)
}

func (e *encoder) add(img *image.RGBA, delay int) {
	e.frames++
	w, h := img.Rect.Dx(), img.Rect.Dy()
	first := len(e.out.Image) == 0
	changed := image.Rectangle{}
	for y := 0; y < h; y++ {
		row := y * img.Stride
		for x := 0; x < w; x++ {
			o := row + x*4
			if !first && img.Pix[o] == e.prev.Pix[o] && img.Pix[o+1] == e.prev.Pix[o+1] && img.Pix[o+2] == e.prev.Pix[o+2] {
				continue
			}
			p := y*w + x
			idx := e.nearest(img.Pix[o], img.Pix[o+1], img.Pix[o+2])
			if !first && idx == e.index[p] {
				continue
			}
			e.index[p] = idx
			e.stamp[p] = e.frames
			changed = changed.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	copy(e.prev.Pix, img.Pix)

	if !first && changed.Empty() {
		e.out.Delay[len(e.out.Delay)-1] += delay
		return
	}
	if first {
		changed = img.Rect
	}
	frame := image.NewPaletted(changed, e.pal)
	for y := changed.Min.Y; y < changed.Max.Y; y++ {
		for x := changed.Min.X; x < changed.Max.X; x++ {
			p := y*w + x
			idx := uint8(transparentIndex)
			if e.stamp[p] == e.frames {
				idx = e.index[p]
			}
			frame.Pix[frame.PixOffset(x, y)] = idx
		}
	}
	e.out.Image = append(e.out.Image, frame)
	e.out.Delay = append(e.out.Delay, delay)
	e.out.Disposal = append(e.out.Disposal, gif.DisposalNone)
}
