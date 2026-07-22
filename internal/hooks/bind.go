package hooks

import "strings"

type SkipReason string

const (
	SkipUndeclared SkipReason = "undeclared"
	SkipDisabled   SkipReason = "disabled"   // declared but enabled=false or level/layer off
	SkipHumanGate  SkipReason = "human-gate" // set by sequence 04
)

// PlannedHook is a bound name classified against the effective hook set.
type PlannedHook struct {
	Name   string
	Timing Timing
	Skip   SkipReason // empty means it will run
	Hook   ResolvedHook
}

// PlanBindings resolves a step's pre/post name lists against the effective set
// for a given level, in binding-list order.
func (e *Effective) PlanBindings(level Level, pre, post []string) []PlannedHook {
	if e == nil {
		var planned []PlannedHook
		for _, name := range pre {
			planned = append(planned, PlannedHook{Name: name, Timing: TimingPre, Skip: SkipUndeclared})
		}
		for _, name := range post {
			planned = append(planned, PlannedHook{Name: name, Timing: TimingPost, Skip: SkipUndeclared})
		}
		return planned
	}
	var planned []PlannedHook
	add := func(names []string, t Timing) {
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			h, ok := e.Hooks[name]
			switch {
			case !ok:
				planned = append(planned, PlannedHook{Name: name, Timing: t, Skip: SkipUndeclared})
			case !e.Runnable(name, string(level)):
				planned = append(planned, PlannedHook{Name: name, Timing: t, Skip: SkipDisabled, Hook: h})
			default:
				planned = append(planned, PlannedHook{Name: name, Timing: t, Hook: h})
			}
		}
	}
	add(pre, TimingPre)
	add(post, TimingPost)
	return planned
}

// UndeclaredNames returns bound names skipped because they are not declared.
func UndeclaredNames(planned []PlannedHook) []string {
	var out []string
	for _, p := range planned {
		if p.Skip == SkipUndeclared {
			out = append(out, p.Name)
		}
	}
	return out
}

// FormatSkippedUndeclaredLine returns guidance text for fest next, or empty.
func FormatSkippedUndeclaredLine(planned []PlannedHook) string {
	names := UndeclaredNames(planned)
	if len(names) == 0 {
		return ""
	}
	return "Skipped hooks (undeclared): " + strings.Join(names, ", ")
}
