package campledger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/approvalkit"
	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/workspace"
)

// approvalWarnMsg matches the campaign-ledger warning wording.
const approvalWarnMsg = "campaign approvals not updated: %w"

// Waiting describes the checkpoint a fest source is stopped at, waiting on a
// human decision. Paths are absolute; SetWaiting makes them campaign-relative.
type Waiting struct {
	// Kind is an approvalkit source kind.
	Kind     string
	Step     int
	StepName string
	Title    string
	Body     string
	// Dir is the directory the source's approve and reject commands run from.
	Dir string
	// Doc is the WORKFLOW.md or GATES.md that defines the checkpoint.
	Doc   string
	Scope ledgerkit.Scope
}

// SetWaiting tells the campaign what a fest source is waiting on a human to
// approve: w, or nothing when w is nil.
//
// A source calls it after its own state change has been persisted. It never
// decides anything: approvals are granted only by the source's own commands,
// and this records the fact so the campaign can show one list of what is
// waiting. The call is idempotent (see approvalkit.Recorder.SetWaiting), so a
// source can make it on every transition.
//
// dir is where the source lives and identity distinguishes sources that share a
// directory (a phase's WORKFLOW.md and its GATES.md). Requests from this source
// that are no longer the one waiting are resolved with outcome and reason.
//
// When the call records anything, the approvals ledger is committed to the
// campaign repository straight away (approvalkit.Commit).
//
// Outside a campaign this is a silent no-op, as is the rest of this package. A
// failure to record or to commit warns and is never returned: it must not fail
// or undo the state change that already happened, and an event that could not
// be committed is committed by the next call or by camp.
func SetWaiting(ctx context.Context, dir, identity string, w *Waiting, outcome approvalkit.Outcome, reason string, warn func(error)) {
	if warn == nil {
		warn = WarnToStderr()
	}
	if ctx == nil || ctx.Err() != nil || strings.TrimSpace(dir) == "" {
		return
	}
	root, err := workspace.DetectCampaign(ctx, dir)
	if err != nil || root == "" {
		return // not in a campaign: there is no approvals ledger
	}
	relDir, ok := campaignRelative(root, dir)
	if !ok {
		return // the source resolves outside the campaign it was detected in
	}
	// Every workflow transition lands here, and most are not waiting on anyone.
	// With nothing to request and no ledger yet, there is nothing to resolve
	// either, so stop before resolving a writer id or opening the ledger.
	if w == nil {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(approvalkit.Dir))); err != nil {
			return
		}
	}
	source := approvalSource(relDir, identity)

	var want *approvalkit.Request
	if w != nil {
		campaignID := loadCampaignID(root)
		if campaignID == "" {
			warn(fmt.Errorf(approvalWarnMsg, fmt.Errorf("missing camp id in %s", filepath.Join(root, ".campaign", "campaign.yaml"))))
			return
		}
		cwd, ok := campaignRelative(root, w.Dir)
		if !ok {
			return
		}
		doc, _ := campaignRelative(root, w.Doc)
		scope := w.Scope
		scope.Campaign = campaignID
		want = &approvalkit.Request{
			Source:     source,
			SourceKind: w.Kind,
			Step:       w.Step,
			StepName:   w.StepName,
			Title:      w.Title,
			Body:       w.Body,
			Cwd:        cwd,
			Doc:        doc,
			Scope:      scope,
		}
	}

	writerID, err := ledgerkit.ResolveWriterID(ctx)
	if err != nil {
		warn(fmt.Errorf(approvalWarnMsg, err))
		return
	}
	rec, err := approvalkit.NewRecorder(root, writerID, resolveActor())
	if err != nil {
		warn(fmt.Errorf(approvalWarnMsg, err))
		return
	}
	change, err := rec.SetWaiting(ctx, source, want, outcome, reason)
	if err != nil {
		warn(fmt.Errorf(approvalWarnMsg, err))
		return
	}
	if change.Empty() {
		return
	}
	// Commit the event now. The ledger is committed so that a request reaches
	// the user's other machines; an event left uncommitted until camp next
	// looks at the ledger would not. Only .campaign/approvals is committed,
	// through a temporary index, so nothing staged or dirty in the camp is
	// touched. A camp that is not a git repository is not an error.
	if _, err := approvalkit.Commit(ctx, root); err != nil {
		warn(fmt.Errorf("campaign approvals recorded but not committed: %w", err))
	}
}

// approvalSource is the stable identity of a waiting source: its
// campaign-relative directory, plus identity when one directory holds more
// than one source. It carries no absolute path, so the same source produces the
// same value on every machine that shares the campaign.
func approvalSource(relDir, identity string) string {
	if identity = strings.TrimSpace(identity); identity != "" {
		return relDir + "|" + identity
	}
	return relDir
}

// campaignRelative returns path relative to root, slash-separated, and whether
// it lies inside root. Both sides are symlink-resolved when possible so a
// campaign reached through a symlink still matches.
func campaignRelative(root, path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if !approvalkit.IsCampRelative(rel) {
		return "", false
	}
	return rel, true
}
