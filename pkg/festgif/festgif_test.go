package festgif

import (
	"bytes"
	"context"
	"fmt"
	"image/gif"
	"sort"
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
	r.hooks = append(r.hooks, hookMark{Frame: r.IntroFrames + r.BodyFrames, Row: rowIndex(t, r, "g1"), Run: HookRun{Name: "final_check"}})
	// The held last frame: p1, s1, t1, t2, g1 with its judge and hook lines, p2.
	if got := r.maxLines(); got != 8 {
		t.Fatalf("maxLines = %d, want 8", got)
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
	previous := -1
	for _, tr := range r.transitions {
		if tr.Frame == previous {
			continue
		}
		if previous >= 0 && tr.Frame-previous < fps*2 {
			t.Fatalf("displayed updates %d frames apart, want at least 2s", tr.Frame-previous)
		}
		previous = tr.Frame
	}
	g := rowIndex(t, r, "g1")
	if wait := frameOf(t, r, g, "approved") - frameOf(t, r, g, JudgeRunning); wait < fps {
		t.Errorf("judge wait shows for %d frames, want at least 1s (%d)", wait, fps)
	}
	if r.Timing.HookFrames < 2*fps || r.Timing.TailFrames < 3*fps {
		t.Errorf("hook lines (%d) and the final hold (%d) should last at least 2s and 3s", r.Timing.HookFrames, r.Timing.TailFrames)
	}
}

// crowdedInput checks a large festival whose changes must remain separate: 400 tasks, then a judge run that is approved and one that is
// rejected before it passes.
func crowdedInput() Input {
	done := State{Status: StatusCompleted}
	seq := &Node{Key: "s1", Kind: KindSequence, Label: "01_build"}
	in := Input{Title: "crowded", Final: map[string]State{}}
	for i := 0; i < 400; i++ {
		key := fmt.Sprintf("t%03d", i)
		seq.Children = append(seq.Children, &Node{Key: key, Kind: KindTask, Label: key})
		in.Beats = append(in.Beats, Beat{Changes: []Change{{Key: key, State: done}}})
		in.Final[key] = done
	}
	g1 := &Node{Key: "g1", Kind: KindStep, Label: "Gate 1: PHASE GOAL"}
	g2 := &Node{Key: "g2", Kind: KindStep, Label: "Gate 2: QUALITY"}
	in.Phases = []*Node{{Key: "p1", Kind: KindPhase, Label: "001_BUILD", Children: []*Node{seq, g1, g2}}}
	in.Beats = append(in.Beats,
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusInProgress, Judge: "approved"}}}, Hold: HoldVerdict},
		Beat{Changes: []Change{{Key: "g1", State: State{Status: StatusCompleted, Judge: "approved"}}}},
		Beat{Changes: []Change{{Key: "g2", State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
		Beat{Changes: []Change{{Key: "g2", State: State{Status: StatusInProgress, Judge: "rejected"}}}, Hold: HoldRejection},
		Beat{Changes: []Change{{Key: "g2", State: State{Status: StatusBlocked, Judge: "rejected"}}}, Hold: HoldBlocked},
		Beat{Changes: []Change{{Key: "g2", State: State{Status: StatusCompleted, Judge: "approved"}}}},
	)
	in.Final["g1"] = State{Status: StatusCompleted, Judge: "approved"}
	in.Final["g2"] = State{Status: StatusCompleted, Judge: "approved"}
	return in
}

func TestCrowdedFestivalsStayReadable(t *testing.T) {
	r := Plan(crowdedInput(), DefaultTiming)
	floor := int(DefaultTiming.FramesPerBeat)
	seen := map[int]bool{}
	var frames []int
	for _, tr := range r.transitions {
		if tr.Frame < r.IntroFrames+r.BodyFrames && !seen[tr.Frame] {
			seen[tr.Frame] = true
			frames = append(frames, tr.Frame)
		}
	}
	sort.Ints(frames)
	for i := 1; i < len(frames); i++ {
		if gap := frames[i] - frames[i-1]; gap < floor {
			t.Fatalf("changes %d frames apart, want at least %d", gap, floor)
		}
	}
	if len(frames) > r.Timing.MaxBody/floor+fixedBeats(crowdedInput().Beats)+1 {
		t.Errorf("%d displayed updates exceed the compact body budget", len(frames))
	}
	// Every task still ends completed, and the rejection is still held.
	leaf := r.StateAt(r.Frames - 1)
	for i := 0; i < 400; i++ {
		if got := leaf[rowIndex(t, r, fmt.Sprintf("t%03d", i))].Status; got != StatusCompleted {
			t.Fatalf("task %d = %s after batching, want completed", i, got)
		}
	}
}

// judgeHeavyInput is a festival whose judge runs outnumber a short body
// budget when a caller explicitly requests batching: 60 gates the judge approves, and one it rejects before passing it.
func judgeHeavyInput() Input {
	in := Input{Title: "judged", Final: map[string]State{}}
	phase := &Node{Key: "p1", Kind: KindPhase, Label: "001_BUILD"}
	approved := State{Status: StatusCompleted, Judge: "approved"}
	for i := 0; i < 60; i++ {
		key := fmt.Sprintf("g%02d", i)
		phase.Children = append(phase.Children, &Node{Key: key, Kind: KindStep, Label: fmt.Sprintf("Gate %d: CHECK", i)})
		in.Beats = append(in.Beats,
			Beat{Changes: []Change{{Key: key, State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
			Beat{Hook: &HookRun{Key: key, Name: "approval_judge", Timing: "post", Verb: "gate_approve", Outcome: "pass", Millis: 9000}},
			Beat{Changes: []Change{{Key: key, State: State{Status: StatusInProgress, Judge: "approved"}}}, Hold: HoldVerdict},
			Beat{Changes: []Change{{Key: key, State: approved}}},
		)
		in.Final[key] = approved
	}
	rejected := &Node{Key: "gx", Kind: KindStep, Label: "Gate 60: QUALITY"}
	phase.Children = append(phase.Children, rejected)
	in.Beats = append(in.Beats,
		Beat{Changes: []Change{{Key: "gx", State: State{Status: StatusInProgress, Judge: JudgeRunning}}}, Hold: HoldJudgeWait},
		Beat{Changes: []Change{{Key: "gx", State: State{Status: StatusInProgress, Judge: "rejected"}}}, Hold: HoldRejection},
		Beat{Changes: []Change{{Key: "gx", State: State{Status: StatusBlocked, Judge: "rejected"}}}, Hold: HoldBlocked},
		Beat{Changes: []Change{{Key: "gx", State: approved}}},
	)
	in.Final["gx"] = approved
	in.Phases = []*Node{phase}
	return in
}

func TestJudgeHeavyFestivalsKeepRejectionsAndDropRoutineWaits(t *testing.T) {
	timing := DefaultTiming
	timing.MaxBody, timing.MaxDwell = 750, 900
	r := Plan(judgeHeavyInput(), timing)
	waits := map[int]bool{}
	for _, tr := range r.transitions {
		if tr.State.Judge == JudgeRunning {
			waits[tr.Row] = true
		}
	}
	if waits[rowIndex(t, r, "g00")] {
		t.Error("a routine judge wait should give way when judge runs alone exceed the budget")
	}
	gx := rowIndex(t, r, "gx")
	if !waits[gx] {
		t.Error("the rejected run keeps its waiting beat")
	}
	// Every verdict still shows, and the rejection holds longer than a verdict.
	verdicts := 0
	for _, tr := range r.transitions {
		if tr.State.Judge == "approved" && tr.State.Status == StatusInProgress {
			verdicts++
		}
	}
	if verdicts != 60 {
		t.Errorf("%d verdicts shown, want 60", verdicts)
	}
	var rejectHold, verdictHold int
	for i, tr := range r.transitions {
		if i+1 >= len(r.transitions) {
			break
		}
		gap := r.transitions[i+1].Frame - tr.Frame
		if tr.Row == gx && tr.State.Judge == "rejected" && tr.State.Status == StatusInProgress {
			rejectHold = gap
		}
		if tr.Row == rowIndex(t, r, "g00") && tr.State.Judge == "approved" && verdictHold == 0 {
			verdictHold = gap
		}
	}
	// Over the dwell budget, routine verdicts compress but the rejection keeps
	// its full hold, so they differ by nearly the full difference in dwell.
	want := 0.8 * (DefaultTiming.Dwell.Rejection - DefaultTiming.Dwell.Verdict)
	if got := float64(rejectHold - verdictHold); got < want {
		t.Errorf("rejection held %d frames and a routine verdict %d (difference %.0f, want at least %.0f)", rejectHold, verdictHold, got, want)
	}
}

func TestScaledStretchesEveryDuration(t *testing.T) {
	fast := Plan(judgedInput(), DefaultTiming.Scaled(0.5))
	slow := Plan(judgedInput(), DefaultTiming.Scaled(2))
	if slow.Frames < 3*fast.Frames {
		t.Errorf("half speed lasts %d frames and double speed %d; want about four times", slow.Frames, fast.Frames)
	}
	if got := DefaultTiming.Scaled(2).Dwell.JudgeWait; got != 2*DefaultTiming.Dwell.JudgeWait {
		t.Errorf("scaled judge wait = %v", got)
	}
}
