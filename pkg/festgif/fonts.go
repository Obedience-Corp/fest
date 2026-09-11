package festgif

import (
	"embed"
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// JetBrains Mono, SIL Open Font License 1.1 (fonts/OFL.txt).
//
//go:embed fonts/JetBrainsMono-Regular.ttf fonts/JetBrainsMono-Bold.ttf
var fontFiles embed.FS

type faces struct {
	title   font.Face
	suffix  font.Face
	percent font.Face
	stats   font.Face
	phase   font.Face
	row     font.Face
	rowBold font.Face
	sub     font.Face
	badge   font.Face
}

func loadFaces() (*faces, error) {
	regular, err := parseFont("fonts/JetBrainsMono-Regular.ttf")
	if err != nil {
		return nil, err
	}
	bold, err := parseFont("fonts/JetBrainsMono-Bold.ttf")
	if err != nil {
		return nil, err
	}
	f := &faces{}
	for _, spec := range []struct {
		dst  *font.Face
		font *opentype.Font
		size float64
	}{
		{&f.title, bold, 30},
		{&f.suffix, regular, 18},
		{&f.percent, bold, 18},
		{&f.stats, regular, 15},
		{&f.phase, bold, 19},
		{&f.row, regular, 17},
		{&f.rowBold, bold, 17},
		{&f.sub, regular, 15},
		{&f.badge, regular, 14},
	} {
		face, err := opentype.NewFace(spec.font, &opentype.FaceOptions{Size: spec.size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
		*spec.dst = &cachedFace{Face: face, size: spec.size, glyphs: map[rune]cachedGlyph{}}
	}
	return f, nil
}

func parseFont(name string) (*opentype.Font, error) {
	data, err := fontFiles.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return opentype.Parse(data)
}

type cachedGlyph struct {
	bounds  image.Rectangle
	mask    *image.Alpha
	advance fixed.Int26_6
	ok      bool
}

// cachedFace rasterizes each rune once. The opentype face reuses its mask
// buffer between calls, so cached masks are copies.
type cachedFace struct {
	font.Face
	size   float64
	glyphs map[rune]cachedGlyph
}

func (f *cachedFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	g, hit := f.glyphs[r]
	if !hit {
		bounds, mask, maskp, advance, ok := f.Face.Glyph(fixed.Point26_6{}, r)
		g = cachedGlyph{bounds: bounds, advance: advance, ok: ok}
		if ok && !bounds.Empty() {
			g.mask = image.NewAlpha(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
			draw.Draw(g.mask, g.mask.Bounds(), mask, maskp, draw.Src)
		}
		f.glyphs[r] = g
	}
	if g.mask == nil {
		return image.Rectangle{}, image.Transparent, image.Point{}, g.advance, g.ok
	}
	at := image.Pt(dot.X.Round(), dot.Y.Round())
	return g.bounds.Add(at), g.mask, image.Point{}, g.advance, g.ok
}

// lineHeight matches CSS "normal" line height for a monospace terminal font.
const lineHeight = 1.2

// lineMetrics are a face's CSS-style line box: its height, and the baseline's
// offset from the top of the box with the glyphs centered in it.
func lineMetrics(face font.Face) (height, baseline int) {
	m := face.Metrics()
	asc, desc := float64(m.Ascent)/64, float64(m.Descent)/64
	h := asc + desc
	if cf, ok := face.(*cachedFace); ok {
		h = cf.size * lineHeight
	}
	return int(math.Round(h)), int(math.Round((h-asc-desc)/2 + asc))
}
