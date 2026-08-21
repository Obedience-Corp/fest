package hooks

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Obedience-Corp/fest/internal/config"
)

// ContextSchemaVersion is the JSON schema identifier written on hook stdin.
const ContextSchemaVersion = "fest.hook.context/v1"

// Coord is the festival location a lifecycle hook is firing for.
// Empty fields are omitted from stdin JSON and from FEST_* env.
type Coord struct {
	FestivalPath string
	FestivalID   string
	Phase        string
	Step         int
	Task         string
}

// Payload is the per-execution hook context. Hook commands such as
// `camp buzz post --from-hook` read this from stdin JSON and/or FEST_* env
// instead of scraping the TUI.
type Payload struct {
	SchemaVersion string `json:"schema_version"`
	FestivalPath  string `json:"festival_path,omitempty"`
	FestivalID    string `json:"festival_id,omitempty"`
	Phase         string `json:"phase,omitempty"`
	Step          int    `json:"step,omitempty"`
	Task          string `json:"task,omitempty"`
	Level         string `json:"level"`
	Verb          string `json:"verb"`
	Timing        string `json:"timing"`
	Hook          string `json:"hook,omitempty"`
}

// BuildPayload assembles the context for one planned hook execution.
func BuildPayload(coord Coord, level Level, verb Verb, p PlannedHook) Payload {
	return Payload{
		SchemaVersion: ContextSchemaVersion,
		FestivalPath:  strings.TrimSpace(coord.FestivalPath),
		FestivalID:    strings.TrimSpace(coord.FestivalID),
		Phase:         strings.TrimSpace(coord.Phase),
		Step:          coord.Step,
		Task:          strings.TrimSpace(coord.Task),
		Level:         string(level),
		Verb:          string(verb),
		Timing:        string(p.Timing),
		Hook:          strings.TrimSpace(p.Name),
	}
}

// JSON is the stdin body for a hook process (one JSON object plus newline).
func (p Payload) JSON() []byte {
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	return append(b, '\n')
}

// Env is the extra environment merged onto the hook process.
// Empty optional fields are omitted so consumers can distinguish "unset"
// from "explicitly blank". FEST_HOOK=1 always marks a hook invocation.
func (p Payload) Env() []string {
	env := []string{
		"FEST_HOOK=1",
		"FEST_HOOK_SCHEMA=" + ContextSchemaVersion,
	}
	add := func(key, val string) {
		if val == "" {
			return
		}
		env = append(env, key+"="+val)
	}
	add("FEST_HOOK_NAME", p.Hook)
	add("FEST_TASK", p.Task)
	add("FEST_VERB", p.Verb)
	add("FEST_LEVEL", p.Level)
	add("FEST_TIMING", p.Timing)
	add("FEST_PHASE", p.Phase)
	if p.Step > 0 {
		add("FEST_STEP", strconv.Itoa(p.Step))
	}
	add("FEST_FESTIVAL_PATH", p.FestivalPath)
	add("FEST_FESTIVAL", p.FestivalID)
	return env
}

// FestivalID returns metadata.id from fest.yaml, or empty when unknown.
func FestivalID(festivalPath string) string {
	festivalPath = strings.TrimSpace(festivalPath)
	if festivalPath == "" {
		return ""
	}
	cfg, err := config.LoadFestivalConfig(festivalPath, "")
	if err != nil || cfg == nil {
		return ""
	}
	if id := strings.TrimSpace(cfg.Metadata.ID); id != "" {
		return id
	}
	return strings.TrimSpace(cfg.Metadata.Name)
}
