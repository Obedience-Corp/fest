// Package campledger emits high-intent fest state changes into the campaign
// event ledger via camp's public pkg/ledgerkit (D003/D006 for fest).
//
// Emission is best-effort-but-loud: a ledger problem warns and never fails the
// caller's state change. Outside a campaign workspace emission is a silent
// no-op (standalone festivals have no campaign ledger).
//
// Workflow step noise (wf_step_start/done) stays only in
// .fest/progress_events.jsonl — this package must not be called from those paths.
package campledger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"
	"gopkg.in/yaml.v3"

	"github.com/Obedience-Corp/fest/internal/workspace"
)

// warnMsg matches camp's user-visible ledger warning wording (D003).
const warnMsg = "warning: campaign ledger not updated: %v\n"

// Emitter appends campaign ledger events for one high-intent fest action.
type Emitter struct {
	writer       *ledgerkit.Writer
	campaignID   string
	campaignRoot string
	actionID     string
	actor        ledgerkit.Actor
	warn         func(error)
	disabled     bool
}

// Option customises an event before append.
type Option func(*ledgerkit.Event)

// WithWhy sets the human-facing reason (D005).
func WithWhy(why string) Option {
	return func(ev *ledgerkit.Event) { ev.Why = why }
}

// WithPayload sets kind-specific payload fields.
func WithPayload(p map[string]any) Option {
	return func(ev *ledgerkit.Event) { ev.Payload = p }
}

// WithEvidence attaches typed evidence refs.
func WithEvidence(refs ...ledgerkit.Evidence) Option {
	return func(ev *ledgerkit.Event) { ev.Evidence = append(ev.Evidence, refs...) }
}

// WarnTo returns a loud, non-fatal warn function writing to w.
func WarnTo(w io.Writer) func(error) {
	return func(err error) {
		if err == nil {
			return
		}
		_, _ = fmt.Fprintf(w, warnMsg, err)
	}
}

// WarnToStderr prints ledger warnings to stderr.
func WarnToStderr() func(error) { return WarnTo(os.Stderr) }

// NewFromFestival builds an emitter scoped to the campaign that contains
// festivalPath. Missing campaign context disables emission (silent no-op).
func NewFromFestival(ctx context.Context, festivalPath string, warn func(error)) *Emitter {
	if warn == nil {
		warn = WarnToStderr()
	}
	e := &Emitter{
		actionID: ledgerkit.NewActionID(),
		actor:    resolveActor(),
		warn:     warn,
	}
	if festivalPath == "" {
		e.disabled = true
		return e
	}
	root, err := workspace.DetectCampaign(ctx, festivalPath)
	if err != nil || root == "" {
		// Standalone festival: no campaign ledger by design.
		e.disabled = true
		return e
	}
	campaignID := loadCampaignID(root)
	if campaignID == "" {
		warn(fmt.Errorf("ledger: missing campaign id in %s", filepath.Join(root, ".campaign", "campaign.yaml")))
		e.disabled = true
		return e
	}
	writerID, err := ledgerkit.ResolveWriterID(ctx)
	if err != nil {
		warn(fmt.Errorf("ledger: resolve writer id: %w", err))
		e.disabled = true
		return e
	}
	w, err := ledgerkit.NewWriter(root, writerID)
	if err != nil {
		warn(fmt.Errorf("ledger: init writer: %w", err))
		e.disabled = true
		return e
	}
	e.writer = w
	e.campaignRoot = root
	e.campaignID = campaignID
	return e
}

// Emit appends one command-sourced event. Failure warns; never returns to
// the caller as a hard error (D003).
func (e *Emitter) Emit(ctx context.Context, kind ledgerkit.Kind, scope ledgerkit.Scope, opts ...Option) {
	if e == nil || e.disabled || e.writer == nil {
		return
	}
	if ctx.Err() != nil {
		e.warn(ctx.Err())
		return
	}
	scope.Campaign = e.campaignID
	ev := &ledgerkit.Event{
		V:      ledgerkit.EnvelopeVersion,
		ID:     ledgerkit.NewEventID(),
		TS:     ledgerkit.NowUTC(time.Now()),
		Kind:   kind,
		Scope:  scope,
		Action: e.actionID,
		Actor:  e.actor,
		Source: ledgerkit.SourceCommand,
	}
	for _, o := range opts {
		o(ev)
	}
	if err := e.writer.Emit(ctx, ev, e.warn); err != nil {
		// writer.Emit already warns; keep signature symmetric with camp.
		_ = err
	}
}

// FestivalScope builds scope from a festival path and optional task path
// segments. taskID is the relative task path (phase/seq/task.md) when known.
func FestivalScope(festivalPath, taskID string) ledgerkit.Scope {
	scope := ledgerkit.Scope{
		Festival: festivalIDFromPath(festivalPath),
	}
	if taskID == "" {
		return scope
	}
	// taskID forms: "003_IMPLEMENT/05_views/02_graph_ingestion.md" or with fest_id
	parts := strings.Split(filepath.ToSlash(taskID), "/")
	switch {
	case len(parts) >= 3:
		scope.Phase = parts[0]
		scope.Sequence = parts[1]
		scope.Task = parts[len(parts)-1]
	case len(parts) == 2:
		scope.Phase = parts[0]
		scope.Sequence = parts[1]
	case len(parts) == 1:
		scope.Task = parts[0]
	}
	return scope
}

func festivalIDFromPath(festivalPath string) string {
	base := filepath.Base(festivalPath)
	// Prefer short id suffix when present (…-CA0002).
	if i := strings.LastIndex(base, "-"); i >= 0 && i < len(base)-1 {
		suffix := base[i+1:]
		if looksLikeFestID(suffix) {
			return suffix
		}
	}
	// Fall back to fest.yaml metadata.id
	data, err := os.ReadFile(filepath.Join(festivalPath, "fest.yaml"))
	if err == nil {
		var doc struct {
			Metadata struct {
				ID string `yaml:"id"`
			} `yaml:"metadata"`
		}
		if yaml.Unmarshal(data, &doc) == nil && doc.Metadata.ID != "" {
			return doc.Metadata.ID
		}
	}
	return base
}

func looksLikeFestID(s string) bool {
	if len(s) < 4 || len(s) > 12 {
		return false
	}
	// e.g. CA0002, RI-DR0001
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func loadCampaignID(campaignRoot string) string {
	path := filepath.Join(campaignRoot, ".campaign", "campaign.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.ID)
}

func resolveActor() ledgerkit.Actor {
	// Prefer USER / LOGNAME; agents often set both.
	name := os.Getenv("USER")
	if name == "" {
		name = os.Getenv("LOGNAME")
	}
	actorType := ledgerkit.ActorHuman
	// Heuristic: common agent env markers.
	if os.Getenv("OBEY_AGENT") != "" || os.Getenv("CLAUDE_CODE") != "" || os.Getenv("CODEX_TASK") != "" {
		actorType = ledgerkit.ActorAgent
	}
	if name == "" {
		return ledgerkit.Actor{Type: ledgerkit.ActorUnknown}
	}
	return ledgerkit.Actor{Type: actorType, Name: name}
}
