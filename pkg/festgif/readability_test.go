package festgif

import (
	"bytes"
	"fmt"
	"image"
	"testing"
)

func TestDefaultReadingHoldDoesNotFlash(t *testing.T) {
	for _, timing := range []Timing{DefaultTiming, DefaultTiming.Scaled(2)} {
		r := Plan(judgedInput(), timing)
		f, err := loadFaces()
		if err != nil {
			t.Fatal(err)
		}
		p := newPainter(r, f)
		start := r.transitions[0].Frame
		a, b := image.NewRGBA(p.bounds()), image.NewRGBA(p.bounds())
		p.paint(a, start, r.StateAt(start))
		p.paint(b, start+timing.FPS/2, r.StateAt(start+timing.FPS/2))
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Fatal("row or parent backgrounds changed during a reading hold")
		}
	}
}

func TestDefaultPreservesRejectionWaitAndLastBeat(t *testing.T) {
	in := judgeHeavyInput()
	r := Plan(in, DefaultTiming)
	waits := map[int]bool{}
	for _, tr := range r.transitions {
		if tr.State.Judge == JudgeRunning {
			waits[tr.Row] = true
		}
	}
	if !waits[rowIndex(t, r, "gx")] {
		t.Fatal("the rejected judge run lost its wait")
	}
	// The last event needs its own reading hold before the final-state clamp.
	last := r.transitions[len(r.transitions)-len(in.Final)-1]
	if gap := r.IntroFrames + r.BodyFrames - last.Frame; gap < 60 {
		t.Fatalf("last event held for %d frames, want at least 2s", gap)
	}
	for i, hook := range r.hooks {
		next := r.IntroFrames + r.BodyFrames
		for _, tr := range r.transitions {
			if tr.Frame > hook.Frame {
				next = tr.Frame
				break
			}
		}
		if next-hook.Frame < 90 {
			t.Fatalf("hook %d visible for %d frames, want at least 3s", i, next-hook.Frame)
		}
	}
}

// Sparse rendering must preserve every visible change, including optional
// pulses and hook expiration, while allowing arbitrarily long static holds.
func TestPaintFramesPreservesVisualChanges(t *testing.T) {
	for _, heat := range []int{0, 8} {
		timing := DefaultTiming
		timing.HeatFrames = heat
		r := Plan(judgedInput(), timing)
		frames := r.paintFrames()
		f, err := loadFaces()
		if err != nil {
			t.Fatal(err)
		}
		p := newPainter(r, f)
		a, b := image.NewRGBA(p.bounds()), image.NewRGBA(p.bounds())
		for i, frame := range frames {
			end := r.Frames
			if i+1 < len(frames) {
				end = frames[i+1]
			}
			p.paint(a, frame, r.StateAt(frame))
			for _, check := range []int{frame + (end-frame)/2, end - 1} {
				p.paint(b, check, r.StateAt(check))
				if !bytes.Equal(a.Pix, b.Pix) {
					t.Fatalf("heat=%d: skipped visual change between frames %d and %d", heat, frame, check)
				}
			}
		}
		if heat == 0 && len(frames) > r.IntroFrames+2*len(r.transitions)+2*len(r.hooks)+1 {
			t.Fatalf("static reading holds caused extra painted frames: %d", len(frames))
		}
	}
}

func TestMissingHistoryDoesNotInterruptReadingHolds(t *testing.T) {
	r := Plan(judgedInput(), DefaultTiming)
	missing := rowIndex(t, r, "t3")
	before := r.StateAt(r.IntroFrames + r.BodyFrames - 1)
	if before[missing].Status != StatusPending {
		t.Fatal("invented an execution time for a task absent from the event log")
	}
	if final := r.StateAt(r.Frames - 1); final[missing].Status != StatusCompleted {
		t.Fatal("final snapshot lost the unrecorded task's state")
	}
}

// Match the size of AS0006: 7 phases, 22 sequences, 138 tasks plus phase gates.
// Grouping must stay concise without hiding changes in a collapsed sequence.
func TestCompactReplayKeepsEachSequenceVisible(t *testing.T) {
	in := Input{Title: "sequence-summary", Final: map[string]State{}}
	sequence := 0
	for pi := 0; pi < 7; pi++ {
		phase := &Node{Key: fmt.Sprintf("p%d", pi), Kind: KindPhase}
		seqCount := 3
		if pi == 0 {
			seqCount = 4
		}
		for si := 0; si < seqCount; si++ {
			seq := &Node{Key: fmt.Sprintf("s%d", sequence), Kind: KindSequence}
			tasks := 6
			if sequence < 6 {
				tasks++
			}
			for ti := 0; ti < tasks; ti++ {
				key := fmt.Sprintf("s%d-t%d", sequence, ti)
				seq.Children = append(seq.Children, &Node{Key: key, Kind: KindTask})
				state := State{Status: StatusCompleted}
				in.Beats = append(in.Beats, Beat{Changes: []Change{{Key: key, State: state}}})
				in.Final[key] = state
			}
			sequence++
			phase.Children = append(phase.Children, seq)
		}
		for gi := 0; gi < 4; gi++ {
			key := fmt.Sprintf("p%d-g%d", pi, gi)
			phase.Children = append(phase.Children, &Node{Key: key, Kind: KindStep})
			for _, status := range []string{StatusInProgress, StatusCompleted} {
				in.Beats = append(in.Beats, Beat{Changes: []Change{{Key: key, State: State{Status: status}}}})
			}
			in.Final[key] = State{Status: StatusCompleted}
		}
		in.Phases = append(in.Phases, phase)
	}
	r := Plan(in, DefaultTiming)
	if r.Stats.Tasks != 138 || r.Stats.Sequences != 22 {
		t.Fatalf("bad fixture: %+v", r.Stats)
	}
	seconds := float64(r.Frames) / float64(r.Timing.FPS)
	if seconds < 45 || seconds > 75 {
		t.Fatalf("compact replay lasts %.1fs, want 45–75s", seconds)
	}
	clamp := r.IntroFrames + r.BodyFrames
	for _, tr := range r.transitions {
		if tr.Frame >= clamp {
			continue
		}
		visible := r.visible(r.StateAt(tr.Frame))
		found := false
		for _, row := range visible {
			if row == tr.Row {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("change to %s was hidden in a collapsed sequence", r.Rows[tr.Row].Key)
		}
	}
}
