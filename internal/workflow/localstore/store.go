// Package localstore manages a single .workflow/ runtime directory for
// standalone workflow execution. See workflow/design/workitem-workflows/
// LOCAL_RUN_STATE.md for the canonical contract.
//
// The store is filesystem-only: append-only event stream, derived state via
// replay, cached summary fields repaired on read.
package localstore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	manifestName    = "workflow.yaml"
	runsDir         = "runs"
	runManifestName = "run.yaml"
	eventsName      = "progress_events.jsonl"
)

// Store manages a single .workflow/ runtime directory.
type Store struct {
	root        string // absolute path to .workflow/
	workflowDoc string // absolute path to WORKFLOW.md (sibling of .workflow/)
}

// Open returns a Store anchored at the given .workflow/ directory.
// workflowDir may not yet exist (Init will create it).
func Open(workflowDir, workflowDocPath string) *Store {
	return &Store{root: workflowDir, workflowDoc: workflowDocPath}
}

// InitOptions controls Store.Init.
type InitOptions struct {
	WorkflowID string
	WorkitemID string
	Force      bool
}

// Init creates .workflow/ and writes workflow.yaml.
// Refuses to overwrite an existing manifest unless Force is true.
func (s *Store) Init(ctx context.Context, opts InitOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.WorkflowID == "" {
		return festerrors.Validation("workflow_id is required")
	}
	if opts.WorkitemID == "" {
		return festerrors.Validation("workitem_id is required")
	}

	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return festerrors.IO("creating .workflow", err)
	}
	manifestPath := filepath.Join(s.root, manifestName)
	if _, err := os.Stat(manifestPath); err == nil && !opts.Force {
		return festerrors.Validation("workflow.yaml already exists; pass Force to overwrite")
	}

	docHash := ""
	if s.workflowDoc != "" {
		if h, err := HashWorkflowDoc(s.workflowDoc); err == nil {
			docHash = h
		}
	}

	m := Manifest{
		Version:    ManifestVersion,
		Kind:       ManifestKind,
		WorkflowID: opts.WorkflowID,
		WorkitemID: opts.WorkitemID,
		DocPath:    filepath.Base(s.workflowDoc),
		DocHash:    docHash,
	}
	return writeYAML(manifestPath, &m)
}

// LoadManifest reads workflow.yaml.
func (s *Store) LoadManifest(ctx context.Context) (*Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.root, manifestName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, festerrors.Wrap(err, "reading workflow manifest")
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, festerrors.Parse("parsing workflow manifest", err)
	}
	return &m, nil
}

// StartRun creates a new run directory, writes run.yaml, appends the
// workflow_run_started event, and updates the manifest's active_run_id.
// Returns the new run id.
func (s *Store) StartRun(ctx context.Context, startedBy string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return "", err
	}

	runID := newRunID(time.Now().UTC())
	runDir := filepath.Join(s.root, runsDir, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", festerrors.IO("creating run directory", err)
	}

	docHash := ""
	if s.workflowDoc != "" {
		docHash, _ = HashWorkflowDoc(s.workflowDoc)
	}

	now := time.Now().UTC()
	run := RunManifest{
		Version:    RunVersion,
		Kind:       RunKind,
		RunID:      runID,
		WorkflowID: m.WorkflowID,
		WorkitemID: m.WorkitemID,
		Status:     "active",
		StartedAt:  now,
		StartedBy:  startedBy,
		Executor:   "fest",
		Source: RunSource{
			WorkflowPath: "../../" + filepath.Base(s.workflowDoc),
			WorkflowHash: docHash,
			WorkitemPath: "../..",
		},
	}
	if err := writeYAML(filepath.Join(runDir, runManifestName), &run); err != nil {
		return "", err
	}

	if err := s.appendEvent(ctx, runDir, Event{
		Version:    1,
		EventID:    "evt_" + uuid.NewString()[:8],
		EventType:  EventWorkflowRunStarted,
		RunID:      runID,
		WorkflowID: m.WorkflowID,
		WorkitemID: m.WorkitemID,
		Timestamp:  now,
	}); err != nil {
		return "", err
	}

	m.ActiveRunID = runID
	m.Runs = append(m.Runs, RunRecord{
		RunID:     runID,
		Status:    "active",
		Path:      filepath.Join(runsDir, runID),
		StartedAt: now.Format(time.RFC3339),
	})
	if err := writeYAML(filepath.Join(s.root, manifestName), m); err != nil {
		return "", err
	}
	return runID, nil
}

// AppendEvent appends an event to the active run's event stream.
func (s *Store) AppendEvent(ctx context.Context, evt Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return err
	}
	if m.ActiveRunID == "" {
		return festerrors.Validation("no active run")
	}
	runDir := filepath.Join(s.root, runsDir, m.ActiveRunID)
	if evt.RunID == "" {
		evt.RunID = m.ActiveRunID
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	if evt.EventID == "" {
		evt.EventID = "evt_" + uuid.NewString()[:8]
	}
	if evt.Version == 0 {
		evt.Version = 1
	}
	return s.appendEvent(ctx, runDir, evt)
}

func (s *Store) appendEvent(_ context.Context, runDir string, evt Event) error {
	path := filepath.Join(runDir, eventsName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return festerrors.IO("opening events file", err)
	}
	defer f.Close()
	line, err := json.Marshal(evt)
	if err != nil {
		return festerrors.Parse("encoding event", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return festerrors.IO("writing event", err)
	}
	return nil
}

// LoadActive returns the active run's replayed state.
// If summary cached in run.yaml disagrees with the event stream, the file
// is rewritten with corrected values; events are never modified.
func (s *Store) LoadActive(ctx context.Context) (*RunState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return nil, err
	}
	if m.ActiveRunID == "" {
		return nil, nil
	}
	runDir := filepath.Join(s.root, runsDir, m.ActiveRunID)
	runPath := filepath.Join(runDir, runManifestName)
	raw, err := os.ReadFile(runPath)
	if err != nil {
		return nil, festerrors.Wrap(err, "reading run manifest")
	}
	var rm RunManifest
	if err := yaml.Unmarshal(raw, &rm); err != nil {
		return nil, festerrors.Parse("parsing run manifest", err)
	}

	state := replayEvents(filepath.Join(runDir, eventsName), rm)

	// Repair stale summary on disk if it disagrees with replay.
	if rm.Summary.CurrentStep != state.CurrentStep ||
		rm.Summary.CompletedSteps != state.CompletedSteps ||
		rm.Summary.Blocked != state.Blocked ||
		(state.Status != "" && state.Status != "active" && state.Status != rm.Status) {
		rm.Summary.CurrentStep = state.CurrentStep
		rm.Summary.CompletedSteps = state.CompletedSteps
		rm.Summary.Blocked = state.Blocked
		if state.Status != "" {
			rm.Status = state.Status
		}
		_ = writeYAML(runPath, &rm)
	}

	state.RunID = rm.RunID
	state.StartedAt = rm.StartedAt
	state.CompletedAt = rm.CompletedAt
	state.TotalSteps = rm.Summary.TotalSteps
	if state.TotalSteps == 0 {
		state.TotalSteps = state.CurrentStep
	}

	// Detect doc-hash drift.
	if s.workflowDoc != "" && m.DocHash != "" {
		if cur, hashErr := HashWorkflowDoc(s.workflowDoc); hashErr == nil && cur != m.DocHash {
			state.DocHashChanged = true
		}
	}
	return state, nil
}

// CheckDocHash compares the stored workflow.yaml doc_hash against the
// current WORKFLOW.md hash.
func (s *Store) CheckDocHash(ctx context.Context) (changed bool, current, stored string, err error) {
	if err := ctx.Err(); err != nil {
		return false, "", "", err
	}
	m, mErr := s.LoadManifest(ctx)
	if mErr != nil {
		return false, "", "", mErr
	}
	cur, hErr := HashWorkflowDoc(s.workflowDoc)
	if hErr != nil {
		return false, "", m.DocHash, hErr
	}
	return cur != m.DocHash, cur, m.DocHash, nil
}

// RunSummaryView is the public view returned by ListRuns.
type RunSummaryView struct {
	RunID       string
	Status      string
	Path        string
	StartedAt   string
	CompletedAt string
}

// ListRuns enumerates runs from the manifest in their stored order.
func (s *Store) ListRuns(ctx context.Context) ([]RunSummaryView, error) {
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RunSummaryView, 0, len(m.Runs))
	for _, r := range m.Runs {
		out = append(out, RunSummaryView{
			RunID:       r.RunID,
			Status:      r.Status,
			Path:        r.Path,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
		})
	}
	return out, nil
}

// replayEvents walks the event stream and derives current state.
// Returns zero values for missing or unreadable events.
func replayEvents(eventsPath string, cache RunManifest) *RunState {
	state := &RunState{
		Status:         "active",
		CurrentStep:    cache.Summary.CurrentStep,
		CompletedSteps: cache.Summary.CompletedSteps,
		Blocked:        cache.Summary.Blocked,
	}
	f, err := os.Open(eventsPath)
	if err != nil {
		return state
	}
	defer f.Close()

	// Replay overrides cache when events exist.
	state.CurrentStep = 0
	state.CompletedSteps = 0
	state.Blocked = false
	state.Status = "active"

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var evt struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		switch evt.EventType {
		case EventStepStart:
			state.CurrentStep++
		case EventStepDone:
			state.CompletedSteps++
			state.Blocked = false
		case EventStepBlock, EventCheckpointRejected:
			state.Blocked = true
		case EventCheckpointApproved:
			state.Blocked = false
		case EventWorkflowRunCompleted:
			state.Status = "completed"
		case EventWorkflowRunAbandoned:
			state.Status = "abandoned"
		}
	}
	if state.Blocked && state.Status == "active" {
		state.Status = "blocked"
	}
	return state
}

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return festerrors.Parse("marshal yaml", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return festerrors.IO("writing "+filepath.Base(path), err)
	}
	return nil
}

func newRunID(t time.Time) string {
	return "run-" + t.Format("20060102T150405Z")
}

// ManifestPath is the on-disk path to workflow.yaml under this store.
func (s *Store) ManifestPath() string { return filepath.Join(s.root, manifestName) }

// IsInitialized reports whether workflow.yaml exists in this store.
func (s *Store) IsInitialized() bool {
	_, err := os.Stat(s.ManifestPath())
	return !errors.Is(err, os.ErrNotExist)
}
