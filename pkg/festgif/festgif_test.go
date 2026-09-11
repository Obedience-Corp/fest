package festgif

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"
	"testing"
)

func tree() []*Node {
	return []*Node{
		{Key: "p1", Kind: KindPhase, Label: "001_PLAN", Children: []*Node{
			{Key: "s1", Kind: KindSequence, Label: "01_build", Children: []*Node{
				{Key: "t1", Kind: KindTask, Label: "01_task"},
				{Key: "t2", Kind: KindTask, Label: "02_task"},
			}},
			{Key: "g1", Kind: KindStep, Label: "Gate 1: PHASE GOAL"},
		}},
		{Key: "p2", Kind: KindPhase, Label: "002_SHIP", Children: []*Node{
			{Key: "s2", Kind: KindSequence, Label: "01_release", Children: []*Node{
				{Key: "t3", Kind: KindTask, Label: "01_task"},
			}},
		}},
	}
}

func judgedInput() Input {
	done := State{Status: StatusCompleted}
	return Input{
		Title:  "demo",
		Phases: tree(),
		Beats: []Beat{
			{Changes: []Change{{Key: "t1", State: done}}},
			{Changes: []Change{{Key: "t2", State: done}}},
			{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress}}}},
			{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
			{Hook: &HookRun{Key: "g1", Name: "approval_judge", Timing: "post", Verb: "gate_approve", Outcome: "pass", Millis: 18400}},
			{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: "approved"}}}, Hold: HoldVerdict},
			{Changes: []Change{{Key: "g1", State: State{Status: StatusCompleted, Judge: "approved"}}}},
		},
		Final: map[string]State{
			"t1": done, "t2": done, "t3": done,
			"g1": {Status: StatusCompleted, Judge: "approved"},
		},
	}
}

func rowIndex(t *testing.T, r *Replay, key string) int {
	t.Helper()
	for i, row := range r.Rows {
		if row.Key == key {
			return i
		}
	}
	t.Fatalf("no row %q", key)
	return -1
}

func frameOf(t *testing.T, r *Replay, row int, judge string) int {
	t.Helper()
	for _, tr := range r.transitions {
		if tr.Row == row && tr.State.Judge == judge {
			return tr.Frame
		}
	}
	t.Fatalf("no transition on row %d with judge %q", row, judge)
	return -1
}

func TestPlanDrawsFestShowTreeGuides(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	want := map[string][2]string{
		"p1": {"├── ", "│   "},
		"t2": {"│   │   └── ", "│   │       "},
		"g1": {"│   └── ", "│       "},
		"p2": {"└── ", "    "},
	}
	for key, w := range want {
		row := r.Rows[rowIndex(t, r, key)]
		if row.Prefix != w[0] || row.SubPrefix != w[1] {
			t.Errorf("%s: prefix %q sub %q, want %q %q", key, row.Prefix, row.SubPrefix, w[0], w[1])
		}
	}
	if r.Stats != (Stats{Phases: 2, Sequences: 2, Tasks: 3}) {
		t.Errorf("stats = %+v", r.Stats)
	}
}

func TestJudgeWaitHoldsOnScreen(t *testing.T) {
	in := judgedInput()
	without := DefaultTiming
	without.Dwell = Dwell{}
	with := Plan(in, DefaultTiming)
	plain := Plan(in, without)
	g := rowIndex(t, with, "g1")
	wait := func(r *Replay) int { return frameOf(t, r, g, "approved") - frameOf(t, r, g, JudgeRunning) }
	got := wait(with) - wait(plain)
	if want := int(DefaultTiming.Dwell.JudgeWait); got != want {
		t.Fatalf("judge wait grew by %d frames, want %d", got, want)
	}
}

func TestLastFrameMatchesFinalState(t *testing.T) {
	in := judgedInput()
	r := Plan(in, DefaultTiming)
	leaf := r.StateAt(r.Frames - 1)
	for key, want := range in.Final {
		if got := leaf[rowIndex(t, r, key)].State; got != want {
			t.Errorf("%s = %+v, want %+v", key, got, want)
		}
	}
	roll := r.Rollups(leaf)
	if p2 := roll[rowIndex(t, r, "p2")]; p2.Status != StatusCompleted || p2.Done != 1 {
		t.Errorf("t3 never changed in the log but its phase should still finish: %+v", p2)
	}
}

func TestRollupsFollowFestStatusRules(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	leaf := r.StateAt(r.IntroFrames)
	roll := r.Rollups(leaf)
	if got := roll[rowIndex(t, r, "s1")]; got.Status != StatusInProgress || got.Done != 1 || got.Total != 2 {
		t.Errorf("s1 after one task = %+v", got)
	}
	if got := roll[rowIndex(t, r, "p2")]; got.Status != StatusPending {
		t.Errorf("untouched phase = %+v", got)
	}
}

func TestVisibleExpandsOnlyTheFocusPath(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	g := rowIndex(t, r, "g1")
	leaf := r.StateAt(frameOf(t, r, g, JudgeRunning))
	var keys []string
	for _, i := range r.visible(leaf) {
		keys = append(keys, r.Rows[i].Key)
	}
	want := []string{"p1", "s1", "g1", "p2"}
	if len(keys) != len(want) {
		t.Fatalf("visible = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("visible = %v, want %v", keys, want)
		}
	}
}

func TestSubLinesReadLikeFest(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	g := rowIndex(t, r, "g1")
	mark := r.hooks[0]
	lines := subLines(r.Rows[g], r.StateAt(mark.Frame)[g], r.hookAt(g, mark.Frame))
	want := []subLine{
		{Text: "Judge: waiting", tone: toneJudge},
		{Text: "Hook: approval_judge (post gate_approve) pass 18.4s", tone: toneHook},
	}
	if len(lines) != 2 || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("lines = %+v, want %+v", lines, want)
	}
	if r.hookAt(g, mark.Frame+r.Timing.HookFrames-1) == nil {
		t.Error("hook line should still show inside its window")
	}
	if r.hookAt(g, mark.Frame+r.Timing.HookFrames) != nil {
		t.Error("hook line should clear after its window")
	}
}

func TestHookLineOutcomes(t *testing.T) {
	cases := []struct {
		run  HookRun
		want subLine
	}{
		{HookRun{Name: "lint", Timing: "post", Verb: "task_complete", Outcome: "fail", Blocked: true, Millis: 32},
			subLine{Text: "Hook: lint (post task_complete) fail, blocked 32ms", tone: toneFailed}},
		{HookRun{Name: "slow", Timing: "pre", Verb: "task_start", Outcome: "timeout", Millis: 65000},
			subLine{Text: "Hook: slow (pre task_start) timeout 1m 5s", tone: toneFailed}},
		{HookRun{Name: "approval_judge", Timing: "post", Verb: "gate_approve", Outcome: "skipped", Skip: "human-gate"},
			subLine{Text: "Hook: approval_judge (post gate_approve) skipped: human-gate", tone: toneHook}},
	}
	for _, c := range cases {
		if got := hookLine(c.run); got != c.want {
			t.Errorf("hookLine(%+v) = %+v, want %+v", c.run, got, c.want)
		}
	}
}

func TestMaxLinesCountsJudgeAndHookLines(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	// The held last frame: p1, s1, t1, t2, g1 and its judge line, p2. While
	// the gate is in focus its sequence collapses, so judge plus hook peaks at 6.
	if got := r.maxLines(); got != 7 {
		t.Fatalf("maxLines = %d, want 7", got)
	}
}

func TestRenderWritesAPlayableGIF(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	var buf bytes.Buffer
	res, err := Render(context.Background(), &buf, r)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	g, err := gif.DecodeAll(&buf)
	if err != nil {
		t.Fatalf("decoding GIF: %v", err)
	}
	if g.Config.Width != canvasWidth || g.Config.Height != res.Height {
		t.Errorf("size %dx%d, want %dx%d", g.Config.Width, g.Config.Height, canvasWidth, res.Height)
	}
	total := 0
	for _, d := range g.Delay {
		total += d
	}
	if want := centiseconds(r.Frames, r.Timing.FPS); total != want {
		t.Errorf("duration %dcs, want %dcs", total, want)
	}
	if len(g.Image) >= r.Frames {
		t.Errorf("identical frames should merge: %d images for %d frames", len(g.Image), r.Frames)
	}
}

func TestRenderStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, &bytes.Buffer{}, Plan(judgedInput(), DefaultTiming)); err == nil {
		t.Fatal("expected an error from a cancelled render")
	}
}

func TestBlockedLeafBlocksItsParents(t *testing.T) {
	in := judgedInput()
	in.Beats = append(in.Beats, Beat{Changes: []Change{{Key: "t2", State: State{Status: StatusBlocked}}}})
	in.Final["t2"] = State{Status: StatusBlocked}
	r := Plan(in, DefaultTiming)
	roll := r.Rollups(r.StateAt(r.Frames - 1))
	for _, key := range []string{"s1", "p1"} {
		if got := roll[rowIndex(t, r, key)].Status; got != StatusBlocked {
			t.Errorf("%s = %s, want blocked like fest show's determineStatus", key, got)
		}
	}
	if got := roll[rowIndex(t, r, "p2")].Status; got != StatusCompleted {
		t.Errorf("p2 = %s, want completed", got)
	}
}

func TestRunningJudgeKeepsAPendingStepBright(t *testing.T) {
	step := Row{Kind: KindStep}
	pending := Rollup{Status: StatusPending}
	if got := rowOpacity(step, pending, LeafState{State: State{Status: StatusPending, Judge: JudgeRunning}}); got != 1 {
		t.Errorf("pending step with a running judge opacity = %v, want 1", got)
	}
	if got := rowOpacity(step, pending, LeafState{State: State{Status: StatusPending}}); got != pendingOpacity {
		t.Errorf("idle pending step opacity = %v, want %v", got, pendingOpacity)
	}
}

// busyInput is a festival big enough that pacing is not stretched to MinBody:
// 60 tasks, then a gate the judge approves.
func busyInput() Input {
	done := State{Status: StatusCompleted}
	seq := &Node{Key: "s1", Kind: KindSequence, Label: "01_build"}
	in := Input{Title: "busy", Final: map[string]State{}}
	for i := 0; i < 60; i++ {
		key := fmt.Sprintf("t%02d", i)
		seq.Children = append(seq.Children, &Node{Key: key, Kind: KindTask, Label: key})
		in.Beats = append(in.Beats, Beat{Changes: []Change{{Key: key, State: done}}})
		in.Final[key] = done
	}
	gate := &Node{Key: "g1", Kind: KindStep, Label: "Gate 1: PHASE GOAL"}
	in.Phases = []*Node{{Key: "p1", Kind: KindPhase, Label: "001_BUILD", Children: []*Node{seq, gate}}}
	in.Beats = append(in.Beats,
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: "approved"}}}, Hold: HoldVerdict},
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusCompleted, Judge: "approved"}}}},
	)
	in.Final["g1"] = State{Status: StatusCompleted, Judge: "approved"}
	return in
}

func TestDefaultPacingIsReadable(t *testing.T) {
	r := Plan(busyInput(), DefaultTiming)
	fps := r.Timing.FPS
	for i := 1; i < 60; i++ {
		a, b := rowIndex(t, r, fmt.Sprintf("t%02d", i-1)), rowIndex(t, r, fmt.Sprintf("t%02d", i))
		if gap := frameOf(t, r, b, "") - frameOf(t, r, a, ""); gap < fps/5 {
			t.Fatalf("tasks %d and %d are %d frames apart, want at least 0.2s (%d)", i-1, i, gap, fps/5)
		}
	}
	g := rowIndex(t, r, "g1")
	if wait := frameOf(t, r, g, "approved") - frameOf(t, r, g, JudgeRunning); wait < fps {
		t.Errorf("judge wait shows for %d frames, want at least 1s (%d)", wait, fps)
	}
	if r.Timing.HookFrames < 2*fps || r.Timing.TailFrames < 3*fps {
		t.Errorf("hook lines (%d) and the final hold (%d) should last at least 2s and 3s", r.Timing.HookFrames, r.Timing.TailFrames)
	}
}
