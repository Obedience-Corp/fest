package festgif

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

type pt struct{ x, y float32 }

// fillRoundRect draws an antialiased rounded rectangle.
func fillRoundRect(dst *image.RGBA, x, y, w, h, radius float64, c color.NRGBA) {
	if w <= 0 || h <= 0 || c.A == 0 {
		return
	}
	radius = math.Min(radius, math.Min(w, h)/2)
	pts := roundRectPath(x, y, w, h, radius)
	fillPolygons(dst, c, pts)
}

func roundRectPath(x, y, w, h, r float64) []pt {
	var pts []pt
	corner := func(cx, cy, from float64) {
		const steps = 6
		for i := 0; i <= steps; i++ {
			a := from + float64(i)*(math.Pi/2)/steps
			pts = append(pts, pt{float32(cx + r*math.Cos(a)), float32(cy + r*math.Sin(a))})
		}
	}
	corner(x+w-r, y+r, -math.Pi/2)
	corner(x+w-r, y+h-r, 0)
	corner(x+r, y+h-r, math.Pi/2)
	corner(x+r, y+r, math.Pi)
	return pts
}

// fillPolygons rasterizes closed polygons in one pass. Each polygon is wound
// the same way so overlaps add coverage instead of cancelling.
func fillPolygons(dst *image.RGBA, c color.NRGBA, polys ...[]pt) {
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, poly := range polys {
		for _, p := range poly {
			minX, minY = min(minX, p.x), min(minY, p.y)
			maxX, maxY = max(maxX, p.x), max(maxY, p.y)
		}
	}
	box := image.Rect(int(math.Floor(float64(minX))), int(math.Floor(float64(minY))),
		int(math.Ceil(float64(maxX))), int(math.Ceil(float64(maxY)))).Intersect(dst.Bounds())
	if box.Empty() {
		return
	}
	z := vector.NewRasterizer(box.Dx(), box.Dy())
	z.DrawOp = draw.Over
	ox, oy := float32(box.Min.X), float32(box.Min.Y)
	for _, poly := range polys {
		if len(poly) < 3 {
			continue
		}
		if signedArea(poly) < 0 {
			poly = reversed(poly)
		}
		z.MoveTo(poly[0].x-ox, poly[0].y-oy)
		for _, p := range poly[1:] {
			z.LineTo(p.x-ox, p.y-oy)
		}
		z.ClosePath()
	}
	z.Draw(dst, box, image.NewUniform(c), image.Point{})
}

func signedArea(poly []pt) float32 {
	var a float32
	for i := range poly {
		j := (i + 1) % len(poly)
		a += poly[i].x*poly[j].y - poly[j].x*poly[i].y
	}
	return a / 2
}

func reversed(poly []pt) []pt {
	out := make([]pt, len(poly))
	for i, p := range poly {
		out[len(poly)-1-i] = p
	}
	return out
}

// stroke is a straight line of width w as a polygon.
func stroke(x0, y0, x1, y1, w float32) []pt {
	dx, dy := x1-x0, y1-y0
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return nil
	}
	nx, ny := -dy/l*w/2, dx/l*w/2
	return []pt{{x0 + nx, y0 + ny}, {x1 + nx, y1 + ny}, {x1 - nx, y1 - ny}, {x0 - nx, y0 - ny}}
}

// arcStroke is a circular arc of width w, from angle a0 to a1 (radians).
func arcStroke(cx, cy, r, a0, a1, w float32) []pt {
	const steps = 16
	var outer, inner []pt
	for i := 0; i <= steps; i++ {
		a := float64(a0 + (a1-a0)*float32(i)/steps)
		cos, sin := float32(math.Cos(a)), float32(math.Sin(a))
		outer = append(outer, pt{cx + (r+w/2)*cos, cy + (r+w/2)*sin})
		inner = append(inner, pt{cx + (r-w/2)*cos, cy + (r-w/2)*sin})
	}
	return append(outer, reversed(inner)...)
}

// halfDisc is the lower half of a disc: a judge scale's pan.
func halfDisc(cx, cy, r float32) []pt {
	const steps = 10
	pts := make([]pt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := math.Pi * float64(i) / steps
		pts = append(pts, pt{cx + r*float32(math.Cos(a)), cy + r*float32(math.Sin(a))})
	}
	return pts
}

// drawScales draws fest's judge glyph (⚖), which the font lacks, in an em box
// whose left edge is x and whose vertical center is cy.
func drawScales(dst *image.RGBA, x, cy, em float32, c color.NRGBA) {
	s := em * 0.78
	left, top := x+em*0.02, cy-s/2
	mid := left + s/2
	w := max(1.2, s*0.1)
	beamY := top + s*0.2
	panY := top + s*0.62
	panR := s * 0.2
	lx, rx := left+panR, left+s-panR
	fillPolygons(dst, c,
		stroke(mid, top+s*0.06, mid, top+s*0.94, w),
		stroke(mid-s*0.26, top+s*0.94, mid+s*0.26, top+s*0.94, w),
		stroke(left+s*0.04, beamY, left+s*0.96, beamY, w),
		stroke(lx, beamY, lx-panR*0.9, panY, w*0.8),
		stroke(lx, beamY, lx+panR*0.9, panY, w*0.8),
		stroke(rx, beamY, rx-panR*0.9, panY, w*0.8),
		stroke(rx, beamY, rx+panR*0.9, panY, w*0.8),
		halfDisc(lx, panY, panR),
		halfDisc(rx, panY, panR),
	)
}

// drawSkipArrow draws fest's skipped glyph (⤼), which the font lacks: an arc
// over the top ending in an arrowhead.
func drawSkipArrow(dst *image.RGBA, x, cy, em float32, c color.NRGBA) {
	s := em * 0.62
	cx := x + em*0.02 + s/2
	r := s * 0.42
	w := max(1.3, s*0.13)
	end := float32(math.Pi * 0.12)
	ex, ey := cx+r*float32(math.Cos(float64(end))), cy+r*float32(math.Sin(float64(end)))
	head := s * 0.3
	fillPolygons(dst, c,
		arcStroke(cx, cy, r, float32(math.Pi), end+float32(2*math.Pi), w),
		[]pt{{ex - head*0.6, ey - head*0.2}, {ex + head*0.6, ey - head*0.2}, {ex, ey + head*0.7}},
	)
}
