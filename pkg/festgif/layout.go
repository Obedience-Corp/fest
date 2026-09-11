package festgif

import (
	"fmt"
	"sort"
)

// LeafState is a leaf row's state at a frame and the frame it last changed
// (-1 before any change).
type LeafState struct {
	State
	Last int
}

// Rollup is a row's aggregate state: leaves report their own state, phases
// and sequences roll up from the leaves under them.
type Rollup struct {
	Status  string
	Done    int
	Total   int
	Blocked int
	Active  int
	Last    int
}

// cursor walks a replay's transitions forward frame by frame.
type cursor struct {
	r    *Replay
	next int
	leaf []LeafState
}

// newCursor starts before the first frame.
func newCursor(r *Replay) *cursor {
	c := &cursor{r: r, leaf: make([]LeafState, len(r.Rows))}
	for i := range c.leaf {
		c.leaf[i] = LeafState{State: State{Status: StatusPending}, Last: -1}
	}
	return c
}

// Advance applies every transition up to and including frame.
func (c *cursor) Advance(frame int) []LeafState {
	for c.next < len(c.r.transitions) && c.r.transitions[c.next].Frame <= frame {
		t := c.r.transitions[c.next]
		c.leaf[t.Row] = LeafState{State: t.State, Last: t.Frame}
		c.next++
	}
	return c.leaf
}

// StateAt replays transitions up to frame from scratch.
func (r *Replay) StateAt(frame int) []LeafState {
	return newCursor(r).Advance(frame)
}

// Rollups resolves every row's aggregate state bottom-up. A phase or
// sequence takes its status from the leaf counts under it, exactly as
// `fest show` does (determineStatus): all done is completed, any blocked leaf
// is blocked, any started or finished leaf is in progress.
func (r *Replay) Rollups(leaf []LeafState) []Rollup {
	out := make([]Rollup, len(r.Rows))
	for i := range out {
		out[i].Last = -1
	}
	for i := len(r.Rows) - 1; i >= 0; i-- {
		row := r.Rows[i]
		if row.Kind.leaf() {
			s := leaf[i]
			out[i] = Rollup{Status: s.Status, Total: 1, Last: s.Last}
			switch {
			case s.done():
				out[i].Done = 1
			case s.Status == StatusBlocked:
				out[i].Blocked = 1
			case s.Status == StatusInProgress:
				out[i].Active = 1
			}
		} else {
			out[i].Status = rollupStatus(out[i])
		}
		if p := row.Parent; p >= 0 {
			out[p].Done += out[i].Done
			out[p].Total += out[i].Total
			out[p].Blocked += out[i].Blocked
			out[p].Active += out[i].Active
			if out[i].Last > out[p].Last {
				out[p].Last = out[i].Last
			}
		}
	}
	return out
}

func rollupStatus(r Rollup) string {
	switch {
	case r.Total == 0:
		return StatusPending
	case r.Done >= r.Total:
		return StatusCompleted
	case r.Blocked > 0:
		return StatusBlocked
	case r.Active > 0 || r.Done > 0:
		return StatusInProgress
	default:
		return StatusPending
	}
}

// visible returns the rows on screen, fest-watch style: every phase shows as a
// summary line, and only the focus path (the phase and sequence holding the
// most recent change) expands to its children.
func (r *Replay) visible(leaf []LeafState) []int {
	focus, best := -1, -1
	for i, row := range r.Rows {
		if row.Kind.leaf() && leaf[i].Last > best {
			best, focus = leaf[i].Last, i
		}
	}
	expanded := map[int]bool{}
	if focus >= 0 {
		for p := r.Rows[focus].Parent; p >= 0; p = r.Rows[p].Parent {
			expanded[p] = true
		}
	}
	shown := make([]bool, len(r.Rows))
	var out []int
	for i, row := range r.Rows {
		if row.Parent < 0 || (shown[row.Parent] && expanded[row.Parent]) {
			shown[i] = true
			out = append(out, i)
		}
	}
	return out
}

// hookAt is the latest hook run on row still inside its display window.
func (r *Replay) hookAt(row, frame int) *HookRun {
	var found *hookMark
	for i := range r.hooks {
		h := &r.hooks[i]
		if h.Frame > frame {
			break
		}
		if h.Row == row {
			found = h
		}
	}
	if found == nil || frame-found.Frame >= r.Timing.HookFrames {
		return nil
	}
	return &found.Run
}

// tone is the color role of a line drawn under a row.
type tone int

const (
	toneJudge tone = iota
	toneHook
	toneFailed
)

// subLine is a line drawn under a row.
type subLine struct {
	Text string
	tone tone
}

// subLines are the lines under a row: a step's judge state as `fest show`
// labels it, then the row's most recent hook run while it is on screen.
func subLines(row Row, s LeafState, hook *HookRun) []subLine {
	var out []subLine
	if row.Kind == KindStep && s.Judge != "" {
		label := s.Judge
		if label == JudgeRunning {
			label = "waiting"
		}
		out = append(out, subLine{Text: "Judge: " + label, tone: toneJudge})
	}
	if hook != nil {
		out = append(out, hookLine(*hook))
	}
	return out
}

// hookLine describes a hook run the way fest records it.
func hookLine(h HookRun) subLine {
	head := "Hook: " + h.Name
	where := h.Timing
	if h.Verb != "" {
		if where != "" {
			where += " "
		}
		where += h.Verb
	}
	if where != "" {
		head += " (" + where + ")"
	}
	took := ""
	if h.Millis > 0 {
		took = " " + formatMillis(h.Millis)
	}
	switch {
	case h.failed():
		blocked := ""
		if h.Blocked {
			blocked = ", blocked"
		}
		return subLine{Text: head + " " + h.Outcome + blocked + took, tone: toneFailed}
	case h.Outcome == "skipped":
		reason := ""
		if h.Skip != "" {
			reason = ": " + h.Skip
		}
		return subLine{Text: head + " skipped" + reason, tone: toneHook}
	case h.Outcome == "":
		return subLine{Text: head + " ran" + took, tone: toneHook}
	default:
		return subLine{Text: head + " " + h.Outcome + took, tone: toneHook}
	}
}

func formatMillis(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	s := float64(ms) / 1000
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	m := int(s) / 60
	return fmt.Sprintf("%dm %ds", m, int(s)-m*60)
}

// maxLines is the most lines (rows plus their judge and hook lines) on screen
// at any frame, which sizes the canvas.
func (r *Replay) maxLines() int {
	frames := map[int]bool{}
	for _, t := range r.transitions {
		frames[t.Frame] = true
	}
	for _, h := range r.hooks {
		frames[h.Frame] = true
	}
	ordered := make([]int, 0, len(frames))
	for f := range frames {
		ordered = append(ordered, f)
	}
	sort.Ints(ordered)
	max := r.Stats.Phases
	c := newCursor(r)
	for _, f := range ordered {
		leaf := c.Advance(f)
		lines := 0
		for _, i := range r.visible(leaf) {
			lines += 1 + len(subLines(r.Rows[i], leaf[i], r.hookAt(i, f)))
		}
		if lines > max {
			max = lines
		}
	}
	return max
}
