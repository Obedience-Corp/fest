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
	FPS         int
	IntroFrames int
	// FramesPerBeat is how long every shown change holds: the floor that keeps
	// a replay readable. Festivals with more changes than MaxBody allows batch
	// consecutive ordinary changes together rather than going below it.
	FramesPerBeat float64
	MinBody       int
	// MaxBody caps the frames spent on ordinary changes.
	MaxBody    int
	TailFrames int
	Dwell      Dwell
	// MaxDwell caps the dwell frames added on top of the body. Rejections,
	// blocks, and failed hooks keep their full dwell; judge waits and verdicts
	// are what compress.
	MaxDwell int
	// HookFrames is how long a hook run's line stays under its row.
	HookFrames int
	// HeatFrames is how long a changed row stays highlighted.
	HeatFrames int
}

// Scaled returns the timing with every duration multiplied by f: 0.5 plays a
// replay in half the time, 2 takes twice as long. Frame counts stay whole.
func (t Timing) Scaled(f float64) Timing {
	if f <= 0 {
		return t
	}
	scale := func(v int) int { return max(1, int(math.Round(float64(v)*f))) }
	t.IntroFrames = scale(t.IntroFrames)
	t.FramesPerBeat *= f
	t.MinBody, t.MaxBody = scale(t.MinBody), scale(t.MaxBody)
	t.TailFrames, t.MaxDwell = scale(t.TailFrames), scale(t.MaxDwell)
	t.HookFrames, t.HeatFrames = scale(t.HookFrames), scale(t.HeatFrames)
	t.Dwell = Dwell{
		JudgeWait: t.Dwell.JudgeWait * f,
		Verdict:   t.Dwell.Verdict * f,
		Rejection: t.Dwell.Rejection * f,
		Blocked:   t.Dwell.Blocked * f,
		HookFail:  t.Dwell.HookFail * f,
	}
	return t
}

// fixedBeats counts the beats that never merge: hook runs and held moments.
func fixedBeats(beats []Beat) int {
	n := 0
	for _, b := range beats {
		if b.Hook != nil || b.Hold != HoldNone {
			n++
		}
	}
	return n
}

// key reports whether a hold marks a moment that always keeps its full time.
func (h Hold) key() bool {
	return h == HoldRejection || h == HoldBlocked || h == HoldHookFail
}

// dropRoutineWaits removes the judge-waiting beat of a run that ends in a
// plain verdict, so a festival with many judge runs still shows every verdict
// and hook line. Rejections, blocks, and failed hooks keep their waiting beat.
func dropRoutineWaits(beats []Beat) []Beat {
	out := make([]Beat, 0, len(beats))
	for i, b := range beats {
		if b.Hold == HoldJudgeWait && endsInVerdict(beats[i+1:], b) {
			continue
		}
		out = append(out, b)
	}
	return out
}

// endsInVerdict reports whether the judge run that wait started returned a
// plain verdict, with no rejection or failure in between.
func endsInVerdict(rest []Beat, wait Beat) bool {
	if len(wait.Changes) != 1 {
		return false
	}
	key := wait.Changes[0].Key
	for _, b := range rest {
		if b.Hold.key() {
			return false
		}
		for _, c := range b.Changes {
			if c.Key == key {
				return b.Hold == HoldVerdict
			}
		}
	}
	return false
}

// batch merges runs of ordinary beats (no hook, no hold) so that at most limit
// beats remain, keeping every shown change above the readability floor. Beats
// that carry a hook or a hold are never merged.
func batch(beats []Beat, limit int) []Beat {
	ordinary := len(beats) - fixedBeats(beats)
	if limit < 1 {
		limit = 1
	}
	if ordinary <= limit {
		return beats
	}
	per := int(math.Ceil(float64(ordinary) / float64(limit)))
	out := make([]Beat, 0, len(beats))
	var merged *Beat
	for _, b := range beats {
		if b.Hook != nil || b.Hold != HoldNone {
			merged = nil
			out = append(out, b)
			continue
		}
		if merged == nil {
			out = append(out, b)
			merged = &out[len(out)-1]
			continue
		}
		merged.Changes = append(merged.Changes, b.Changes...)
		if len(merged.Changes) >= per*len(b.Changes) || len(merged.Changes) >= per {
			merged = nil
		}
	}
	return out
}

// DefaultTiming paces a replay for reading. Every shown change holds for at
// least 0.33s, a judge wait for 1.2s, a verdict for 0.8s, a rejection for 2s
// plus 1.5s blocked, and a failed hook for 2s. Hook lines stay up for 2.5s and
// the final state holds for 3s. A festival with more changes than fit batches
// consecutive ordinary changes instead of showing them too briefly to read.
var DefaultTiming = Timing{
	FPS:           30,
	IntroFrames:   24,
	FramesPerBeat: 10,
	MinBody:       150,
	MaxBody:       750,
	TailFrames:    90,
	Dwell:         Dwell{JudgeWait: 36, Verdict: 24, Rejection: 60, Blocked: 45, HookFail: 60},
	MaxDwell:      900,
	HookFrames:    75,
	HeatFrames:    8,
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

	// Fit the replay to MaxBody without ever showing a change for less than
	// FramesPerBeat: batch ordinary changes first, then let routine judge
	// waits go, and only then let the body run long.
	beats := in.Beats
	if t.FramesPerBeat > 0 {
		budget := int(float64(t.MaxBody) / t.FramesPerBeat)
		beats = batch(beats, budget-fixedBeats(beats))
		if len(beats) > budget {
			beats = dropRoutineWaits(beats)
			beats = batch(beats, budget-fixedBeats(beats))
		}
	}
	n := len(beats)
	plain := math.Max(float64(t.MinBody), float64(n)*t.FramesPerBeat)
	plain = math.Round(plain)

	// Rejections, blocks, and failed hooks keep their dwell; judge waits and
	// verdicts give way first when a festival has more than MaxDwell of them.
	var keyDwell, routineDwell float64
	for i := 0; i < n-1; i++ {
		d := t.Dwell.of(beats[i].Hold)
		if beats[i].Hold.key() {
			keyDwell += d
		} else {
			routineDwell += d
		}
	}
	keyScale, routineScale := 1.0, 1.0
	if budget := float64(t.MaxDwell); t.MaxDwell > 0 && keyDwell+routineDwell > budget {
		if keyDwell >= budget {
			keyScale = budget / keyDwell
			routineScale = 0
		} else if routineDwell > 0 {
			routineScale = (budget - keyDwell) / routineDwell
		}
	}
	dwellOf := func(h Hold) float64 {
		if h.key() {
			return t.Dwell.of(h) * keyScale
		}
		return t.Dwell.of(h) * routineScale
	}

	step := t.FramesPerBeat
	if n > 1 {
		step = plain / float64(n-1)
	}
	frames := make([]int, n)
	pos := 0.0
	for i := range beats {
		frames[i] = t.IntroFrames + int(math.Round(pos))
		if i < n-1 {
			pos += step + dwellOf(beats[i].Hold)
		}
	}
	r.BodyFrames = int(plain)
	if n > 1 {
		r.BodyFrames = int(math.Round(pos))
	}

	touched := map[int]bool{}
	for i, b := range beats {
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
