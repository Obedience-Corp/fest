// Package festival builds a festgif replay from a festival directory, with the
// same tree and state `fest show` reports at every point in the progress log.
//
//	in, err := festival.Load(ctx, "festivals/active/my-festival-MF0001")
//	if err != nil {
//		return err
//	}
//	_, err = festgif.Render(ctx, w, festgif.Plan(in, festgif.DefaultTiming))
package festival

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/pkg/festgif"
)

// Load reads the festival at dir, or the festival containing dir, and returns
// its replay: the rows and final state from the tree `fest show` builds, and
// one beat for every event in its progress log that changed what fest shows.
// The title is the festival's metadata name. Load never writes to disk.
func Load(ctx context.Context, dir string) (festgif.Input, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return festgif.Input{}, errors.IO("resolving festival path", err)
	}
	info, err := show.DetectCurrentFestival(ctx, abs, "")
	if err != nil {
		return festgif.Input{}, err
	}
	root, err := filepath.Abs(info.Path)
	if err != nil {
		return festgif.Input{}, errors.IO("resolving festival path", err)
	}
	tree, err := show.BuildFestivalTree(ctx, root)
	if err != nil {
		return festgif.Input{}, errors.Wrap(err, "building festival tree")
	}
	events, err := progress.NewStore(root).ReadEvents(ctx)
	if err != nil {
		return festgif.Input{}, errors.Wrap(err, "reading progress events")
	}
	title := info.MetadataName
	if title == "" {
		title = info.Name
	}
	return build(title, root, tree, events), nil
}

type leaf struct {
	key      string
	taskPath string
	stateKey string
	step     int
}

// state is the leaf's display state in store, read the way `fest show`
// reads it.
func (l leaf) state(store *progress.Store, root string) festgif.State {
	if l.taskPath != "" {
		task, ok := lookupTask(store, root, l.taskPath)
		if !ok {
			return festgif.State{Status: festgif.StatusPending}
		}
		return festgif.State{Status: show.TaskDisplayStatus(task.Status)}
	}
	ws, ok := store.WorkflowPhaseState(l.stateKey)
	if !ok {
		return festgif.State{Status: festgif.StatusPending}
	}
	ss := ws.GetStepState(l.step)
	if ss == nil {
		return festgif.State{Status: festgif.StatusPending}
	}
	s := festgif.State{Status: show.StepDisplayStatus(ss.Status)}
	if ss.Judge != nil {
		s.Judge = ss.Judge.Status
	}
	return s
}

// lookupTask mirrors progress.ResolveTaskProgress without its time inference,
// which reads the task file and never changes the status.
func lookupTask(store *progress.Store, root, taskPath string) (*progress.TaskProgress, bool) {
	if id, err := progress.NormalizeTaskID(root, taskPath); err == nil {
		if task, ok := store.GetTask(id); ok {
			return task, true
		}
	}
	return store.GetTask(filepath.Base(taskPath))
}

type index struct {
	phases map[string]string
	seqs   map[string]string
	tasks  map[string]string
	steps  map[string]bool
}

// build turns the tree `fest show` builds and the festival's event log into a
// replay. The tree supplies the rows and the final state; every event prefix
// is materialized with fest's own progress store, so each beat shows exactly
// what fest would have shown after that event.
func build(title, root string, tree *show.DisplayNode, events []progress.ProgressEvent) festgif.Input {
	in := festgif.Input{Title: title, Final: map[string]festgif.State{}}
	idx := index{phases: map[string]string{}, seqs: map[string]string{}, tasks: map[string]string{}, steps: map[string]bool{}}
	var leaves []leaf

	var convert func(n *show.DisplayNode, phase, seq string) *festgif.Node
	convert = func(n *show.DisplayNode, phase, seq string) *festgif.Node {
		node := &festgif.Node{Label: n.Name}
		switch n.NodeType {
		case "phase":
			node.Kind, node.Key = festgif.KindPhase, "phase:"+n.Name
			idx.phases[n.Name] = node.Key
			phase = n.Name
		case "sequence":
			node.Kind, node.Key = festgif.KindSequence, "seq:"+phase+"/"+n.Name
			idx.seqs[phase+"/"+n.Name] = node.Key
			seq = n.Name
		case "task":
			node.Kind, node.Label = festgif.KindTask, strings.TrimSuffix(n.Name, ".md")
			id, err := progress.NormalizeTaskID(root, n.TaskPath)
			if err != nil || n.TaskPath == "" {
				id = phase + "/" + seq + "/" + n.Name
			}
			node.Key = "task:" + id
			idx.tasks[id] = node.Key
			leaves = append(leaves, leaf{key: node.Key, taskPath: n.TaskPath})
			in.Final[node.Key] = festgif.State{Status: n.Status}
		case "step":
			node.Kind, node.Key = festgif.KindStep, stepKey(n.StateKey, n.StepNumber)
			idx.steps[node.Key] = true
			leaves = append(leaves, leaf{key: node.Key, stateKey: n.StateKey, step: n.StepNumber})
			in.Final[node.Key] = festgif.State{Status: n.Status, Judge: n.JudgeStatus}
		}
		for _, c := range n.Children {
			node.Children = append(node.Children, convert(c, phase, seq))
		}
		return node
	}
	for _, p := range tree.Children {
		in.Phases = append(in.Phases, convert(p, "", ""))
	}

	shown := map[string]festgif.State{}
	for _, l := range leaves {
		shown[l.key] = festgif.State{Status: festgif.StatusPending}
	}
	for i, e := range events {
		store := progress.Replay(root, events[:i+1])
		var changes []festgif.Change
		for _, l := range leaves {
			s := l.state(store, root)
			if s != shown[l.key] {
				shown[l.key] = s
				changes = append(changes, festgif.Change{Key: l.key, State: s})
			}
		}
		var hook *festgif.HookRun
		if e.Event == progress.EventWorkflowHookRun {
			hook = idx.hook(root, e)
		}
		if len(changes) == 0 && hook == nil {
			continue
		}
		in.Beats = append(in.Beats, festgif.Beat{Changes: changes, Hook: hook, Hold: hold(e, len(changes) > 0, hook)})
	}
	return in
}

func stepKey(stateKey string, step int) string {
	return "step:" + stateKey + "|" + strconv.Itoa(step)
}

// hook pins a hook run to the row it fired on: a step or gate (phase and
// step), a task or sequence (task coordinate), or a whole phase.
func (idx index) hook(root string, e progress.ProgressEvent) *festgif.HookRun {
	var key string
	switch {
	case e.Step > 0 && e.Phase != "":
		if k := stepKey(e.Phase, e.Step); idx.steps[k] {
			key = k
		}
	case e.Task != "":
		if id, err := progress.NormalizeTaskID(root, e.Task); err == nil {
			key = idx.tasks[id]
		}
		if key == "" {
			key = idx.seqs[strings.Trim(filepath.ToSlash(e.Task), "/")]
		}
	}
	if key == "" {
		key = idx.phases[strings.TrimPrefix(e.Phase, "gate:")]
	}
	if key == "" {
		return nil
	}
	return &festgif.HookRun{
		Key:     key,
		Name:    e.HookName,
		Timing:  e.HookTiming,
		Verb:    e.HookVerb,
		Outcome: e.HookOutcome,
		Skip:    e.HookSkip,
		Millis:  e.HookMillis,
		Blocked: e.HookBlocked,
	}
}

// hold keeps judge waits, verdicts, blocks, and failed hooks on screen longer.
func hold(e progress.ProgressEvent, changed bool, hook *festgif.HookRun) festgif.Hold {
	if hook != nil && (hook.Outcome == "fail" || hook.Outcome == "timeout") {
		return festgif.HoldHookFail
	}
	if !changed {
		return festgif.HoldNone
	}
	switch e.Event {
	case progress.EventWorkflowJudgeStarted:
		return festgif.HoldJudgeWait
	case progress.EventWorkflowJudgeReturned:
		if e.JudgeStatus == wf.JudgeApproved {
			return festgif.HoldVerdict
		}
		return festgif.HoldRejection
	case progress.EventWorkflowStepBlock, progress.EventBlocked:
		return festgif.HoldBlocked
	default:
		return festgif.HoldNone
	}
}
