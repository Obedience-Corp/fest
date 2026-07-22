package hooks

import (
	"time"

	"github.com/Obedience-Corp/fest/internal/config"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

type Layer string

const (
	LayerMachine   Layer = "machine"
	LayerFestivals Layer = "festivals"
	LayerFestival  Layer = "festival"
)

type FailPolicy string

const (
	FailClosed FailPolicy = "closed"
	FailOpen   FailPolicy = "open"
)

type EvidenceMode string

const (
	EvidencePaths EvidenceMode = "paths"
	EvidenceEmbed EvidenceMode = "embed"
)

// DefaultTimeout is applied to newly declared hooks when timeout is unset.
const DefaultTimeout = 120 * time.Second

// ResolvedHook is a fully-defaulted, typed hook definition plus provenance.
type ResolvedHook struct {
	Name     string
	Command  string
	Fail     FailPolicy
	Timeout  time.Duration
	Evidence EvidenceMode
	Enabled  bool
	Source   Layer         // the layer the winning definition came from
	Shadowed []ShadowedDef // upper-layer definitions this one replaced, when they DIFFER
}

// ShadowedDef records a replaced upper-layer definition for drift reporting (D10).
type ShadowedDef struct {
	Source Layer
	Def    config.HookDefinition
}

// Effective is the resolved hook set plus resolved switches.
type Effective struct {
	Enabled bool                    // layer-wide switch, most-specific-wins, default true
	Levels  map[string]bool         // phase/sequence/task, most-specific-wins, default true
	Hooks   map[string]ResolvedHook // by name

	// Legacy alias (flat hooks.approval_judge.command) metadata for warnings.
	LegacyAliasActive  bool
	LegacyAliasCommand string
}

type layerCfg struct {
	name Layer
	cfg  *config.HooksConfig
}

type prior struct {
	src Layer
	def config.HookDefinition
}

// Resolve merges three declaration layers into one effective hook set.
// Nil or empty layers are skipped (D7: empty defaults at every layer).
// The festivals-layer legacy flat key is expanded in-memory before merge (D6/R6).
func Resolve(machine, festivals, festival *config.HooksConfig) (*Effective, error) {
	eff := &Effective{
		Enabled: true,
		Levels:  map[string]bool{"phase": true, "sequence": true, "task": true},
		Hooks:   map[string]ResolvedHook{},
	}

	festivals = cloneHooksConfig(festivals)
	if aliased, cmd := applyApprovalJudgeAlias(festivals); aliased {
		eff.LegacyAliasActive = true
		eff.LegacyAliasCommand = cmd
	}

	layers := []layerCfg{
		{LayerMachine, machine},
		{LayerFestivals, festivals},
		{LayerFestival, festival},
	}

	winners := map[string]prior{}

	for _, l := range layers {
		if emptyLayer(l.cfg) {
			continue
		}
		if l.cfg.Enabled != nil {
			eff.Enabled = *l.cfg.Enabled
		}
		for level, on := range l.cfg.Levels {
			eff.Levels[level] = on
		}
		for name, def := range l.cfg.Definitions {
			winners[name] = prior{src: l.name, def: def}
		}
	}

	for name, w := range winners {
		rh, err := resolveOne(name, w.src, w.def)
		if err != nil {
			return nil, err
		}
		rh.Shadowed = differingShadows(name, w, layers)
		eff.Hooks[name] = rh
	}
	return eff, nil
}

// Runnable reports whether the named hook should run at the given lifecycle level.
func (e *Effective) Runnable(name, level string) bool {
	if e == nil {
		return false
	}
	h, ok := e.Hooks[name]
	if !ok || !e.Enabled || !h.Enabled {
		return false
	}
	on, known := e.Levels[level]
	return !known || on // unknown level defaults true
}

func emptyLayer(cfg *config.HooksConfig) bool {
	if cfg == nil {
		return true
	}
	return cfg.IsZero() &&
		cfg.ApprovalJudge.Command == "" &&
		cfg.Enabled == nil &&
		len(cfg.Levels) == 0 &&
		len(cfg.Definitions) == 0
}

func resolveOne(name string, src Layer, def config.HookDefinition) (ResolvedHook, error) {
	rh := ResolvedHook{Name: name, Command: def.Command, Source: src, Enabled: true}
	switch def.Fail {
	case "", string(FailClosed):
		rh.Fail = FailClosed
	case string(FailOpen):
		rh.Fail = FailOpen
	default:
		return rh, festerrors.Validation("invalid hook fail policy").
			WithField("hook", name).WithField("fail", def.Fail).
			WithHint("fail must be closed or open")
	}
	switch def.Evidence {
	case "", string(EvidencePaths):
		rh.Evidence = EvidencePaths
	case string(EvidenceEmbed):
		rh.Evidence = EvidenceEmbed
	default:
		return rh, festerrors.Validation("invalid hook evidence mode").
			WithField("hook", name).WithField("evidence", def.Evidence).
			WithHint("evidence must be paths or embed")
	}
	rh.Timeout = DefaultTimeout
	if def.Timeout != "" {
		d, err := time.ParseDuration(def.Timeout)
		if err != nil {
			return rh, festerrors.Validation("invalid hook timeout").
				WithField("hook", name).WithField("timeout", def.Timeout)
		}
		rh.Timeout = d
	}
	if def.Enabled != nil {
		rh.Enabled = *def.Enabled
	}
	return rh, nil
}

func defsEqual(a, b config.HookDefinition) bool {
	if a.Command != b.Command || a.Fail != b.Fail || a.Timeout != b.Timeout || a.Evidence != b.Evidence {
		return false
	}
	return boolPtrEqual(a.Enabled, b.Enabled)
}

func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// differingShadows walks layers before the winner and collects differing defs.
func differingShadows(name string, winner prior, layers []layerCfg) []ShadowedDef {
	var out []ShadowedDef
	for _, l := range layers {
		if l.name == winner.src {
			break
		}
		if emptyLayer(l.cfg) {
			continue
		}
		def, ok := l.cfg.Definitions[name]
		if !ok {
			continue
		}
		if !defsEqual(def, winner.def) {
			out = append(out, ShadowedDef{Source: l.name, Def: def})
		}
	}
	return out
}
