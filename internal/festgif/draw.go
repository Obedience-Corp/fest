package festgif

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"regexp"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const (
	canvasWidth    = 860
	padding        = 48
	rowHeight      = 27
	glyphBox       = 22
	badgeGap       = 10
	percentBox     = 56
	barGap         = 12
	barHeight      = 10
	barRadius      = 6
	rowRadius      = 4
	titleGap       = 14
	statsGap       = 10
	headerGap      = 16
	introLift      = 12
	pendingOpacity = 0.42
	heatAlpha      = 0.16
)

// Palette: a terminal-inspired dark card, matching fest's colors.
var (
	colorBg         = rgb(0x0d1117)
	colorBgAccent   = rgb(0x161b22)
	colorGuide      = rgb(0x3b4252)
	colorText       = rgb(0xc9d1d9)
	colorDim        = rgb(0x8b949e)
	colorPhase      = rgb(0x79c0ff)
	colorSequence   = rgb(0xd2a8ff)
	colorCompleted  = rgb(0x3fb950)
	colorInProgress = rgb(0xd29922)
	colorPending    = rgb(0x6e7681)
	colorFailed     = rgb(0xf85149)
	colorGate       = rgb(0x39c5cf)
	colorJudge      = rgb(0xb388ff)
	colorHeat       = rgb(0x58a6ff)
)

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	c.A = uint8(math.Round(float64(c.A) * math.Max(0, math.Min(1, a))))
	return c
}

var gateLabel = regexp.MustCompile(`(?i)\bgate\b`)

func labelColor(row Row) color.NRGBA {
	switch {
	case gateLabel.MatchString(row.Label):
		return colorGate
	case row.Kind == KindPhase:
		return colorPhase
	case row.Kind == KindSequence:
		return colorSequence
	case row.Kind == KindStep:
		return colorDim
	default:
		return colorText
	}
}

func statusColor(status string) color.NRGBA {
	switch status {
	case StatusCompleted:
		return colorCompleted
	case StatusInProgress, StatusSkipped:
		return colorInProgress
	case StatusBlocked:
		return colorFailed
	default:
		return colorPending
	}
}

// statusGlyph is fest's glyph: steps use ✗ for blocked, other rows use ■.
func statusGlyph(status string, kind Kind) string {
	switch status {
	case StatusCompleted:
		return "✓"
	case StatusInProgress:
		return "●"
	case StatusBlocked:
		if kind == KindStep {
			return "✗"
		}
		return "■"
	case StatusSkipped:
		return "⤼"
	default:
		return "○"
	}
}

func toneColor(t Tone) color.NRGBA {
	switch t {
	case ToneJudge:
		return colorJudge
	case ToneFailed:
		return colorFailed
	default:
		return colorDim
	}
}

type painter struct {
	r          *Replay
	f          *faces
	width      int
	height     int
	content    int
	titleLines []string
	headerH    int
}

func newPainter(r *Replay, f *faces) *painter {
	p := &painter{r: r, f: f, width: canvasWidth, content: canvasWidth - 2*padding}
	suffix := "  festival"
	titleW := font.MeasureString(f.title, r.Title).Ceil()
	suffixW := font.MeasureString(f.suffix, suffix).Ceil()
	p.titleLines = []string{r.Title}
	if titleW+suffixW > p.content {
		p.titleLines = append(p.titleLines, "")
	}
	titleLH, _ := lineMetrics(f.title)
	pctLH, _ := lineMetrics(f.percent)
	statsLH, _ := lineMetrics(f.stats)
	p.headerH = len(p.titleLines)*titleLH + titleGap + pctLH + statsGap + statsLH + headerGap
	p.height = padding + p.headerH + (r.MaxLines()+1)*rowHeight + padding
	return p
}

func (p *painter) bounds() image.Rectangle { return image.Rect(0, 0, p.width, p.height) }

// paint draws one frame.
func (p *painter) paint(img *image.RGBA, frame int, leaf []LeafState) {
	draw.Draw(img, img.Bounds(), image.NewUniform(colorBg), image.Point{}, draw.Src)
	drive := clamp01(float64(frame) / math.Max(1, float64(p.r.IntroFrames)))
	roll := p.r.Rollups(leaf)

	done, total := 0, 0
	for i, row := range p.r.Rows {
		if row.Kind == KindPhase {
			done += roll[i].Done
			total += roll[i].Total
		}
	}
	progress := 0
	if total > 0 {
		progress = int(math.Round(float64(done) / float64(total) * 100))
	}

	y := padding + int(math.Round((1-drive)*-introLift))
	y = p.paintHeader(img, y, progress, drive)
	p.paintTree(img, y, frame, leaf, roll, drive)
}

func (p *painter) paintHeader(img *image.RGBA, top, progress int, alpha float64) int {
	titleLH, titleAsc := lineMetrics(p.f.title)
	x := p.textAt(img, p.f.title, padding, top+titleAsc, p.r.Title, withAlpha(colorCompleted, alpha))
	if len(p.titleLines) > 1 {
		top += titleLH
		p.textAt(img, p.f.suffix, padding, top+titleAsc, "festival", withAlpha(colorDim, alpha))
	} else {
		p.textAt(img, p.f.suffix, x, top+titleAsc, "  festival", withAlpha(colorDim, alpha))
	}
	top += titleLH + titleGap

	pctLH, pctAsc := lineMetrics(p.f.percent)
	barW := p.content - percentBox - barGap
	barTop := float64(top) + float64(pctLH-barHeight)/2
	fillRoundRect(img, padding, barTop, float64(barW), barHeight, barRadius, withAlpha(colorBgAccent, alpha))
	if progress > 0 {
		fillRoundRect(img, padding, barTop, float64(barW)*float64(progress)/100, barHeight, barRadius, withAlpha(colorCompleted, alpha))
	}
	label := fmt.Sprintf("%d%%", progress)
	labelW := font.MeasureString(p.f.percent, label).Ceil()
	p.textAt(img, p.f.percent, padding+p.content-labelW, top+pctAsc, label, withAlpha(colorCompleted, alpha))
	top += pctLH + statsGap

	statsLH, statsAsc := lineMetrics(p.f.stats)
	stats := fmt.Sprintf("%d phases · %d sequences · %d tasks", p.r.Stats.Phases, p.r.Stats.Sequences, p.r.Stats.Tasks)
	p.textAt(img, p.f.stats, padding, top+statsAsc, stats, withAlpha(colorDim, alpha))
	return top + statsLH + headerGap
}

func (p *painter) paintTree(img *image.RGBA, top, frame int, leaf []LeafState, roll []Rollup, alpha float64) {
	heatFrames := float64(max(1, p.r.Timing.HeatFrames))
	for _, i := range p.r.Visible(leaf) {
		row := p.r.Rows[i]
		a := roll[i]
		opacity := alpha * rowOpacity(row, a, leaf[i])
		heat := 0.0
		if a.Last >= 0 {
			heat = clamp01(1 - float64(frame-a.Last)/heatFrames)
		}
		if heat > 0 {
			fillRoundRect(img, padding, float64(top), float64(p.content), rowHeight, rowRadius, withAlpha(colorHeat, heatAlpha*heat*opacity))
		}
		p.paintRow(img, top, row, a, leaf[i], opacity)
		top += rowHeight

		for _, line := range SubLines(row, leaf[i], p.r.HookAt(i, frame)) {
			x := p.centered(img, p.f.row, padding, top, row.SubPrefix, withAlpha(colorGuide, opacity))
			p.centered(img, p.f.sub, x, top, line.Text, withAlpha(toneColor(line.Tone), opacity))
			top += rowHeight
		}
	}
}

func (p *painter) paintRow(img *image.RGBA, top int, row Row, a Rollup, s LeafState, opacity float64) {
	face, labelFace := p.f.row, p.f.row
	switch row.Kind {
	case KindPhase:
		face, labelFace = p.f.phase, p.f.phase
	case KindSequence:
		labelFace = p.f.rowBold
	}
	x := p.centered(img, face, padding, top, row.Prefix, withAlpha(colorGuide, opacity))

	em := float32(face.Metrics().Height.Ceil()) * 0.76
	lh, asc := lineMetrics(face)
	cy := float32(top) + float32(rowHeight)/2
	switch {
	case row.Kind == KindStep && s.Judge == JudgeRunning:
		drawScales(img, float32(x), cy, em, withAlpha(colorJudge, opacity))
	case a.Status == StatusSkipped:
		drawSkipArrow(img, float32(x), cy, em, withAlpha(colorInProgress, opacity))
	default:
		baseline := top + (rowHeight-lh)/2 + asc
		p.textAt(img, face, x, baseline, statusGlyph(a.Status, row.Kind), withAlpha(statusColor(a.Status), opacity))
	}
	x += glyphBox

	x = p.centered(img, labelFace, x, top, row.Label, withAlpha(labelColor(row), opacity))
	if row.Aggregates && a.Total > 1 {
		badge := colorDim
		switch a.Status {
		case StatusCompleted:
			badge = colorCompleted
		case StatusInProgress, StatusBlocked:
			badge = colorInProgress
		}
		p.centered(img, p.f.badge, x+badgeGap, top, fmt.Sprintf("%d/%d", a.Done, a.Total), withAlpha(badge, opacity))
	}
}

// centered draws text vertically centered in a row box and returns the x
// where it ended.
func (p *painter) centered(img *image.RGBA, face font.Face, x, top int, s string, c color.NRGBA) int {
	lh, asc := lineMetrics(face)
	return p.textAt(img, face, x, top+(rowHeight-lh)/2+asc, s, c)
}

func (p *painter) textAt(img *image.RGBA, face font.Face, x, baseline int, s string, c color.NRGBA) int {
	if c.A == 0 {
		return x + font.MeasureString(face, s).Ceil()
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
	return d.Dot.X.Round()
}

// rowOpacity dims rows that have not started. A step whose judge is running
// is active even if its status was reset to pending.
func rowOpacity(row Row, a Rollup, s LeafState) float64 {
	if a.Status != StatusPending || (row.Kind == KindStep && s.Judge == JudgeRunning) {
		return 1
	}
	return pendingOpacity
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }
