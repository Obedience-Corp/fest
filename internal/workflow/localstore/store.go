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
	"strconv"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	wfparser "github.com/Obedience-Corp/fest/internal/guidance/workflow"
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
		DocPath:    filepath.Base(s.workflowDoc),
		DocHash:    docHash,
	}
	return writeYAML(manifestPath, &m)
}

// LoadManifest reads workflow.yaml. Validates version, kind, and required
// IDs before returning so callers do not propagate corrupt state.
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
	if err := validateManifest(&m, path); err != nil {
		return nil, err
	}
	return &m, nil
}

// validateManifest enforces version/kind/workflow_id presence. Rejecting
// malformed manifests at the loader prevents compound corruption from
// downstream writes (D030 finding #2).
func validateManifest(m *Manifest, path string) error {
	if m.Version == 0 {
		return festerrors.Validation("workflow manifest missing version").
			WithField("path", path).
			WithHint("expected version: 1")
	}
	if m.Version != ManifestVersion {
		return festerrors.Validation("unsupported workflow manifest version").
			WithField("path", path).
			WithField("got", m.Version).
			WithField("supported", ManifestVersion)
	}
	if m.Kind != ManifestKind {
		return festerrors.Validation("workflow manifest kind mismatch").
			WithField("path", path).
			WithField("got", m.Kind).
			WithField("expected", ManifestKind)
	}
	if m.WorkflowID == "" {
		return festerrors.Validation("workflow manifest missing workflow_id").
			WithField("path", path)
	}
	return nil
}

// StartRun creates a new run directory, writes run.yaml, appends the
// workflow_run_started event, and updates the manifest's active_run_id.
// Returns the new run id.
//
// Pre-mutation contract: WORKFLOW.md must be hashable AND parseable before
// we create any run state. A missing or unparseable doc is a hard error,
// not a silent degradation to workflow_hash="" + total_steps=0, since
// downstream readers (camp dashboard, fest next) cannot interpret a run
// recorded with empty source provenance.
func (s *Store) StartRun(ctx context.Context, startedBy string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return "", err
	}

	// Single-active-run invariant: refuse to start a new run while one is
	// already active. The previous behavior silently appended a second
	// active record to runs[] while pointing active_run_id at the newest
	// run, leaving the prior run permanently labelled active but no longer
	// addressable by AppendEvent/LoadActive. Callers must complete, abandon,
	// or reset the existing run before starting another.
	if m.ActiveRunID != "" {
		return "", festerrors.Validation("a run is already active").
			WithField("active_run_id", m.ActiveRunID).
			WithHint("complete, abandon, or reset the active run before starting a new one (the manifest may have at most one active run)")
	}

	docHash := ""
	totalSteps := 0
	if s.workflowDoc != "" {
		h, hashErr := HashWorkflowDoc(s.workflowDoc)
		if hashErr != nil {
			return "", festerrors.Wrap(hashErr, "hashing WORKFLOW.md").
				WithField("path", s.workflowDoc).
				WithHint("restore WORKFLOW.md or rerun 'fest workflow init' after fixing the doc")
		}
		docHash = h

		n, parseErr := wfparser.NewParser().StepCount(ctx, s.workflowDoc)
		if parseErr != nil {
			return "", festerrors.Wrap(parseErr, "parsing WORKFLOW.md for step count").
				WithField("path", s.workflowDoc)
		}
		if n == 0 {
			return "", festerrors.Validation("WORKFLOW.md has no parseable steps").
				WithField("path", s.workflowDoc).
				WithHint("add at least one '## Step 1:' heading before starting a run")
		}
		totalSteps = n
	}

	// Allocate a unique run ID. Base is a UTC second-granularity timestamp;
	// if two starts land in the same second we append -2, -3, ... so older
	// runs are never overwritten.
	base := newRunID(time.Now().UTC())
	runID := base
	runDir := filepath.Join(s.root, runsDir, runID)
	for suffix := 2; ; suffix++ {
		if err := os.Mkdir(runDir, 0o755); err == nil {
			break
		} else if !os.IsExist(err) {
			// Parent .workflow/runs may not exist yet; create and retry.
			if errors.Is(err, os.ErrNotExist) {
				if mkErr := os.MkdirAll(filepath.Dir(runDir), 0o755); mkErr != nil {
					return "", festerrors.IO("creating runs directory", mkErr)
				}
				continue
			}
			return "", festerrors.IO("creating run directory", err)
		}
		runID = base + "-" + strconv.Itoa(suffix)
		runDir = filepath.Join(s.root, runsDir, runID)
	}

	now := time.Now().UTC()
	run := RunManifest{
		Version:    RunVersion,
		Kind:       RunKind,
		RunID:      runID,
		WorkflowID: m.WorkflowID,
		Status:     "active",
		StartedAt:  now,
		StartedBy:  startedBy,
		Executor:   "fest",
		Source: RunSource{
			WorkflowPath: "../../" + filepath.Base(s.workflowDoc),
			WorkflowHash: docHash,
			WorkitemPath: "../..",
		},
		Summary: RunSummary{TotalSteps: totalSteps},
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
	if err := s.appendEvent(ctx, runDir, evt); err != nil {
		return err
	}
	// Terminal events must also update the parent workflow.yaml so
	// `fest workflow runs` and subsequent LoadActive calls see a consistent
	// view. Without this, runs[i].Status stays "active" and ActiveRunID
	// keeps pointing at a finished run (D030 finding #3).
	if evt.EventType == EventWorkflowRunCompleted || evt.EventType == EventWorkflowRunAbandoned {
		if err := s.finalizeRunInManifest(ctx, evt); err != nil {
			return festerrors.Wrap(err, "finalize run in manifest")
		}
	}
	return nil
}

// finalizeRunInManifest updates workflow.yaml after a terminal event.
// Idempotent — re-finalizing an already-terminal run is a no-op.
func (s *Store) finalizeRunInManifest(ctx context.Context, evt Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m, err := s.LoadManifest(ctx)
	if err != nil {
		return err
	}
	terminalStatus := "completed"
	if evt.EventType == EventWorkflowRunAbandoned {
		terminalStatus = "abandoned"
	}
	for i := range m.Runs {
		if m.Runs[i].RunID == evt.RunID {
			m.Runs[i].Status = terminalStatus
			if m.Runs[i].EndedAt == "" {
				m.Runs[i].EndedAt = evt.Timestamp.Format(time.RFC3339)
			}
			break
		}
	}
	if m.ActiveRunID == evt.RunID {
		m.ActiveRunID = ""
	}
	return writeYAML(filepath.Join(s.root, manifestName), m)
}

func (s *Store) appendEvent(_ context.Context, runDir string, evt Event) error {
	path := filepath.Join(runDir, eventsName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return festerrors.IO("opening events file", err)
	}
	defer func() { _ = f.Close() }()
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

	state, replayErr := replayEvents(filepath.Join(runDir, eventsName), rm)
	if replayErr != nil {
		return nil, festerrors.Wrap(replayErr, "replaying event stream")
	}

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
//
// The event stream is the authoritative source of run state, so I/O failures
// on the events file must surface, not silently degrade to "no progress".
// Distinguishes between:
//   - missing file (run never appended events): returns cached summary, no error
//   - readable file with parseable lines: returns derived state
//   - permission/IO/scanner errors: returns the error so LoadActive can refuse
//     to repair from corrupt input
//
// Malformed JSON lines are intentionally tolerated (skipped) for legacy
// hand-edited streams.
func replayEvents(eventsPath string, cache RunManifest) (*RunState, error) {
	state := &RunState{
		Status:         "active",
		CurrentStep:    cache.Summary.CurrentStep,
		CompletedSteps: cache.Summary.CompletedSteps,
		Blocked:        cache.Summary.Blocked,
	}
	f, err := os.Open(eventsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return nil, festerrors.IO("opening event stream", err)
	}
	defer func() { _ = f.Close() }()

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
		case EventStepSkip:
			// Mirror camp's localrun.go skip handling: skipped steps count
			// as forward progress and clear any prior block. Without this
			// case, fest and camp would replay the same event stream into
			// divergent state (D030 finding #1).
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
	if err := scanner.Err(); err != nil {
		return nil, festerrors.IO("scanning event stream", err)
	}
	if state.Blocked && state.Status == "active" {
		state.Status = "blocked"
	}
	return state, nil
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
