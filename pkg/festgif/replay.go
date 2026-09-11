// Package festgif renders a festival's execution replay as an animated GIF:
// the tree `fest show` draws, with each task, step, and gate changing state in
// the order fest recorded it, including approval judge runs and hook runs.
package festgif

import (
	"math"
	"sort"
)

// Kind is the type of a tree row.
type Kind int

const (
	KindPhase Kind = iota
	KindSequence
	KindStep
	KindTask
)

func (k Kind) leaf() bool { return k == KindStep || k == KindTask }

// Node is one row of the festival tree. Tasks and steps are the leaves whose
// state the replay changes; phases and sequences roll up from them.
type Node struct {
	Key      string
	Kind     Kind
	Label    string
	Children []*Node
}

// Status values, as `fest show` displays them.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusBlocked    = "blocked"
	StatusSkipped    = "skipped"
)

// JudgeRunning is the judge status while an approval judge is working.
const JudgeRunning = "running"

// State is a leaf's display state: its status plus, for steps, the judge
// status recorded on it (empty when there is none).
type State struct {
	Status string
	Judge  string
}

func (s State) done() bool {
	return s.Status == StatusCompleted || s.Status == StatusSkipped
}

// Change sets one leaf's state.
type Change struct {
	Key   string
	State State
}

// HookRun is one lifecycle hook execution, pinned to the row it fired on.
type HookRun struct {
	Key     string
	Name    string
	Timing  string
	Verb    string
	Outcome string
	Skip    string
	Millis  int64
	Blocked bool
}

func (h HookRun) failed() bool { return h.Outcome == "fail" || h.Outcome == "timeout" }

// Hold marks a beat that stays on screen longer than an ordinary change.
type Hold int

const (
	HoldNone Hold = iota
	HoldJudgeWait
	HoldVerdict
	HoldRejection
	HoldBlocked
	HoldHookFail
)

// Beat is one recorded event that changed what the tree shows.
type Beat struct {
	Changes []Change
	Hook    *HookRun
	Hold    Hold
}

// Input is everything a replay needs.
type Input struct {
	Title  string
	Phases []*Node
	Beats  []Beat
	// Final is each leaf's current state; the held last frame always shows it.
	Final map[string]State
}

// Dwell is extra frames that hold judge and hook moments on screen.
type Dwell struct {
	JudgeWait float64
	Verdict   float64
	Rejection float64
	Blocked   float64
	HookFail  float64
}

func (d Dwell) of(h Hold) float64 {
	switch h {
	case HoldJudgeWait:
		return d.JudgeWait
	case HoldVerdict:
		return d.Verdict
	case HoldRejection:
		return d.Rejection
	case HoldBlocked:
		return d.Blocked
	case HoldHookFail:
		return d.HookFail
	default:
		return 0
	}
}

// Timing controls pacing, in frames.
type Timing struct {
	FPS           int
	IntroFrames   int
	FramesPerBeat float64
	MinBody       int
	MaxBody       int
	TailFrames    int
	Dwell         Dwell
	// MaxDwell caps the dwell frames added on top of the body.
	MaxDwell int
	// HookFrames is how long a hook run's line stays under its row.
	HookFrames int
	// HeatFrames is how long a changed row stays highlighted.
	HeatFrames int
}

// DefaultTiming paces a replay for reading: each change shows for 0.2s, a
// judge wait for 1s, a verdict for 0.8s, a rejection for 1.5s plus 1s blocked,
// and a failed hook for 1.5s; hook lines stay up for 2s and the final state
// holds for 3s. Very long festivals compress ordinary changes past MaxBody and
// judge moments past MaxDwell, so a replay stays around a minute at most.
var DefaultTiming = Timing{
	FPS:           30,
	IntroFrames:   24,
	FramesPerBeat: 6,
	MinBody:       150,
	MaxBody:       750,
	TailFrames:    90,
	Dwell:         Dwell{JudgeWait: 30, Verdict: 24, Rejection: 45, Blocked: 30, HookFail: 45},
	MaxDwell:      1050,
	HookFrames:    60,
	HeatFrames:    15,
}

// Row is one flattened tree row with its drawn tree guides.
type Row struct {
	Key        string
	Parent     int
	Kind       Kind
	Label      string
	Prefix     string
	SubPrefix  string
	Aggregates bool
}

// transition sets a leaf row's state at a frame.
type transition struct {
	Frame int
	Row   int
	State State
}

// hookMark is a hook run shown under a row from a frame on.
type hookMark struct {
	Frame int
	Row   int
	Run   HookRun
}

// Stats counts the festival's structure for the header line.
type Stats struct {
	Phases    int
	Sequences int
	Tasks     int
}

// Replay is a planned replay: rows, their state changes on the frame
// timeline, and the frame counts.
type Replay struct {
	Title       string
	Rows        []Row
	transitions []transition
	hooks       []hookMark
	Stats       Stats
	IntroFrames int
	BodyFrames  int
	Frames      int
	Timing      Timing
}

// Plan lays out the rows and places every beat on the frame timeline. Beats
// share the body evenly; each beat's dwell holds it on screen before the next.
// The last frame is clamped to Input.Final so it always matches the tree.
func Plan(in Input, t Timing) *Replay {
	r := &Replay{Title: in.Title, Timing: t, IntroFrames: t.IntroFrames}
	index := map[string]int{}
	var walk func(n *Node, parent int, guides []bool, last bool)
	walk = func(n *Node, parent int, guides []bool, last bool) {
		switch n.Kind {
		case KindSequence:
			r.Stats.Sequences++
		case KindTask:
			r.Stats.Tasks++
		}
		i := len(r.Rows)
		r.Rows = append(r.Rows, Row{
			Key:        n.Key,
			Parent:     parent,
			Kind:       n.Kind,
			Label:      n.Label,
			Prefix:     treePrefix(guides, last),
			SubPrefix:  subPrefix(guides, last),
			Aggregates: n.Kind == KindPhase || n.Kind == KindSequence,
		})
		index[n.Key] = i
		childGuides := append(append([]bool{}, guides...), !last)
		for ci, c := range n.Children {
			walk(c, i, childGuides, ci == len(n.Children)-1)
		}
	}
	r.Stats.Phases = len(in.Phases)
	for i, p := range in.Phases {
		walk(p, -1, nil, i == len(in.Phases)-1)
	}

	n := len(in.Beats)
	plain := math.Min(float64(t.MaxBody), math.Max(float64(t.MinBody), float64(n)*t.FramesPerBeat))
	plain = math.Round(plain)
	var dwellTotal float64
	for i := 0; i < n-1; i++ {
		dwellTotal += t.Dwell.of(in.Beats[i].Hold)
	}
	scale := 1.0
	if t.MaxDwell > 0 && dwellTotal > float64(t.MaxDwell) {
		scale = float64(t.MaxDwell) / dwellTotal
	}
	step := 0.0
	if n > 1 {
		step = plain / float64(n-1)
	}
	frames := make([]int, n)
	pos := 0.0
	for i := range in.Beats {
		frames[i] = t.IntroFrames + int(math.Round(pos))
		if i < n-1 {
			pos += step + t.Dwell.of(in.Beats[i].Hold)*scale
		}
	}
	r.BodyFrames = int(plain)
	if n > 1 {
		r.BodyFrames = int(math.Round(pos))
	}

	touched := map[int]bool{}
	for i, b := range in.Beats {
		for _, c := range b.Changes {
			row, ok := index[c.Key]
			if !ok || !r.Rows[row].Kind.leaf() {
				continue
			}
			touched[row] = true
			r.transitions = append(r.transitions, transition{Frame: frames[i], Row: row, State: c.State})
		}
		if b.Hook != nil {
			if row, ok := index[b.Hook.Key]; ok {
				r.hooks = append(r.hooks, hookMark{Frame: frames[i], Row: row, Run: *b.Hook})
			}
		}
	}

	// A leaf the events never touched (older logs, renamed files) still gets
	// its final state, near its siblings' activity when there is any.
	parentLast := map[int]int{}
	for _, tr := range r.transitions {
		if p := r.Rows[tr.Row].Parent; p >= 0 && tr.Frame > parentLast[p] {
			parentLast[p] = tr.Frame
		}
	}
	var leaves []int
	for i, row := range r.Rows {
		if row.Kind.leaf() {
			leaves = append(leaves, i)
		}
	}
	for li, row := range leaves {
		final := finalState(in.Final, r.Rows[row].Key)
		if touched[row] || final.Status == StatusPending {
			continue
		}
		frame, ok := parentLast[r.Rows[row].Parent]
		if !ok {
			frame = t.IntroFrames + int(math.Round(float64(li)/math.Max(1, float64(len(leaves)-1))*float64(r.BodyFrames)))
		}
		r.transitions = append(r.transitions, transition{Frame: frame, Row: row, State: final})
	}

	clamp := t.IntroFrames + r.BodyFrames
	for _, row := range leaves {
		r.transitions = append(r.transitions, transition{Frame: clamp, Row: row, State: finalState(in.Final, r.Rows[row].Key)})
	}
	sort.SliceStable(r.transitions, func(a, b int) bool { return r.transitions[a].Frame < r.transitions[b].Frame })
	r.Frames = t.IntroFrames + r.BodyFrames + t.TailFrames
	return r
}

func finalState(final map[string]State, key string) State {
	s, ok := final[key]
	if !ok || s.Status == "" {
		s.Status = StatusPending
	}
	return s
}

func treePrefix(guides []bool, last bool) string {
	s := guideRun(guides)
	if last {
		return s + "└── "
	}
	return s + "├── "
}

func subPrefix(guides []bool, last bool) string {
	s := guideRun(guides)
	if last {
		return s + "    "
	}
	return s + "│   "
}

func guideRun(guides []bool) string {
	s := ""
	for _, next := range guides {
		if next {
			s += "│   "
		} else {
			s += "    "
		}
	}
	return s
}
