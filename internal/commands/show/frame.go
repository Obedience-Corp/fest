package show

import "sync"

// FrameState carries live scroll state shared between the cycle key loop and
// the watch renderer. The key loop moves the viewport; the renderer records
// content extents on each draw (used for clamping) and redraws when Redraw
// fires, so scrolling never tears down the underlying file-watch session.
type FrameState struct {
	mu     sync.Mutex
	offset int
	total  int
	view   int
	redraw chan struct{}
}

// NewFrameState returns scroll state starting at the top of the content.
func NewFrameState() *FrameState {
	return &FrameState{redraw: make(chan struct{}, 1)}
}

// ScrollLines moves the viewport by delta lines, clamped to the content
// extents from the most recent draw, and signals a redraw when the offset
// changed.
func (f *FrameState) ScrollLines(delta int) {
	f.mu.Lock()
	changed := f.applyOffset(f.offset + delta)
	f.mu.Unlock()
	if changed {
		f.signal()
	}
}

// ScrollPages moves the viewport by whole viewports.
func (f *FrameState) ScrollPages(delta int) {
	f.mu.Lock()
	page := f.view
	if page < 1 {
		page = 1
	}
	changed := f.applyOffset(f.offset + delta*page)
	f.mu.Unlock()
	if changed {
		f.signal()
	}
}

// Reset returns the viewport to the top without signaling; the caller is
// about to start a fresh render (e.g. cycling to another festival).
func (f *FrameState) Reset() {
	f.mu.Lock()
	f.offset = 0
	f.total = 0
	f.view = 0
	f.mu.Unlock()
}

// Redraw exposes the signal channel the renderer selects on.
func (f *FrameState) Redraw() <-chan struct{} {
	return f.redraw
}

// Viewport slices content lines for a draw at the given viewport height and
// records the extents used to clamp later scrolls. It returns the visible
// lines with the 1-based from/to positions, and whether the content overflows
// the viewport at all.
func (f *FrameState) Viewport(lines []string, height int) (visible []string, from, to, total int, overflow bool) {
	if height < 1 {
		height = 1
	}
	total = len(lines)

	f.mu.Lock()
	f.total = total
	f.view = height
	f.applyOffset(f.offset)
	offset := f.offset
	f.mu.Unlock()

	if total <= height {
		return lines, 1, total, total, false
	}
	end := offset + height
	if end > total {
		end = total
	}
	return lines[offset:end], offset + 1, end, total, true
}

// applyOffset clamps and stores the offset, reporting whether it changed.
// Callers must hold f.mu.
func (f *FrameState) applyOffset(offset int) bool {
	maxOffset := f.total - f.view
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	if offset == f.offset {
		return false
	}
	f.offset = offset
	return true
}

func (f *FrameState) signal() {
	select {
	case f.redraw <- struct{}{}:
	default:
	}
}
