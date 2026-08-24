package runloop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const (
	DefaultMaxTasks   = 8
	DefaultMaxMinutes = 240
	maxConsecutive    = 2
	statusSchema      = "fest.run.status/v1"
)

// Options control a leaveable run.
type Options struct {
	Dry        bool
	StatusOnly bool
	JSON       bool
	Exec       string
	MaxTasks   int
	MaxMinutes int
	Stdout     io.Writer
}

// StatusReport is the morning / --status / --json payload.
type StatusReport struct {
	SchemaVersion string   `json:"schema_version"`
	Outcome       string   `json:"outcome"`
	Reason        string   `json:"reason,omitempty"`
	Label         string   `json:"label,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	Current       int      `json:"current,omitempty"`
	Total         int      `json:"total,omitempty"`
	TasksDone     int      `json:"tasks_done"`
	Ledger        string   `json:"ledger,omitempty"`
	LastCommit    string   `json:"last_commit,omitempty"`
	NextHint      string   `json:"next_hint,omitempty"`
	Records       []Record `json:"records,omitempty"`
}

// Drive inspects the current plan and either reports, dry-runs, or loops.
func Drive(ctx context.Context, cwd string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.MaxTasks <= 0 {
		opts.MaxTasks = DefaultMaxTasks
	}
	if opts.MaxMinutes <= 0 {
		opts.MaxMinutes = DefaultMaxMinutes
	}
	classifyOnly := opts.Dry || strings.TrimSpace(opts.Exec) == ""

	snap, err := Inspect(ctx, cwd)
	if err != nil {
		return err
	}
	ledgerPath, err := LedgerPath(snap)
	if err != nil {
		return err
	}
	recs, err := ReadLedger(ctx, ledgerPath)
	if err != nil {
		return err
	}

	if opts.StatusOnly {
		verdict := Classify(snap)
		return emitStatus(opts, reportFrom(snap, verdict, recs, ledgerPath, ""))
	}

	if classifyOnly {
		verdict := Classify(snap)
		rec := Record{
			Iteration: TaskCount(recs) + 1,
			Outcome:   verdict.Outcome,
			Reason:    verdict.Reason,
			Label:     verdict.Label,
			Path:      snap.Path,
		}
		if err := AppendLedger(ctx, ledgerPath, rec); err != nil {
			return err
		}
		recs = append(recs, rec)
		return emitStatus(opts, reportFrom(snap, verdict, recs, ledgerPath, ""))
	}

	if snap.Kind == "festival" {
		return errors.New("fest run does not yet drive festival tasks").
			WithHint("fest run without --exec reports leaveability. Standalone WORKFLOW.md can loop with --exec <command>.")
	}

	stopSleep := startSleepGuard()
	defer stopSleep()

	deadline := time.Now().Add(time.Duration(opts.MaxMinutes) * time.Minute)
	consecutive := 0
	lastSHA := lastCommit(recs)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			verdict := Verdict{Outcome: OutcomeBudgetExhausted, Reason: "max minutes reached", Label: snap.Label}
			return finish(ctx, opts, snap, ledgerPath, recs, verdict, lastSHA)
		}
		snap, err = Inspect(ctx, cwd)
		if err != nil {
			return err
		}
		verdict := Classify(snap)
		if verdict.Outcome != OutcomeRunnable {
			return finish(ctx, opts, snap, ledgerPath, recs, verdict, lastSHA)
		}
		done := TaskCount(recs)
		if done >= opts.MaxTasks {
			verdict = Verdict{Outcome: OutcomeBudgetExhausted, Reason: "max tasks reached", Label: snap.Label}
			return finish(ctx, opts, snap, ledgerPath, recs, verdict, lastSHA)
		}

		iter := done + 1
		prompt := buildPrompt(snap)
		execErr := InvokeExec(ctx, opts.Exec, prompt, snap.WorkingDir)
		if execErr != nil {
			consecutive++
			rec := Record{
				Iteration: iter,
				Outcome:   OutcomeFailed,
				Reason:    "exec failed",
				Label:     snap.Label,
				Path:      snap.Path,
				Exec:      opts.Exec,
				Error:     execErr.Error(),
			}
			if err := AppendLedger(ctx, ledgerPath, rec); err != nil {
				return err
			}
			recs = append(recs, rec)
			if consecutive >= maxConsecutive {
				return finish(ctx, opts, snap, ledgerPath, recs, Verdict{
					Outcome: OutcomeFailed,
					Reason:  "consecutive exec failures",
					Label:   snap.Label,
				}, lastSHA)
			}
			continue
		}
		consecutive = 0

		sha, commitErr := maybeCommit(ctx, snap.WorkingDir, fmt.Sprintf("fest run: %s", snap.Label))
		if commitErr != nil {
			return errors.Wrap(commitErr, "committing fest run slice")
		}
		if sha != "" {
			lastSHA = sha
		}
		if err := AdvanceStandalone(ctx, snap.StandaloneRuntime, snap.StandaloneDoc); err != nil {
			return errors.Wrap(err, "advancing workflow")
		}
		rec := Record{
			Iteration: iter,
			Outcome:   "ok",
			Reason:    "slice driven",
			Label:     snap.Label,
			Path:      snap.Path,
			Commit:    sha,
			Exec:      opts.Exec,
		}
		if err := AppendLedger(ctx, ledgerPath, rec); err != nil {
			return err
		}
		recs = append(recs, rec)
	}
}

func finish(ctx context.Context, opts Options, snap Snapshot, ledgerPath string, recs []Record, verdict Verdict, lastSHA string) error {
	rec := Record{
		Iteration: TaskCount(recs) + 1,
		Outcome:   verdict.Outcome,
		Reason:    verdict.Reason,
		Label:     verdict.Label,
		Path:      snap.Path,
	}
	if err := AppendLedger(ctx, ledgerPath, rec); err != nil {
		return err
	}
	recs = append(recs, rec)
	return emitStatus(opts, reportFrom(snap, verdict, recs, ledgerPath, lastSHA))
}

func reportFrom(snap Snapshot, verdict Verdict, recs []Record, ledgerPath, lastSHA string) StatusReport {
	if lastSHA == "" {
		lastSHA = lastCommit(recs)
	}
	hint := "fest next"
	switch verdict.Outcome {
	case OutcomeWaitingHuman:
		hint = "handle the gate, then fest run --resume"
	case OutcomeCompleted:
		hint = "nothing queued"
	case OutcomeBudgetExhausted:
		hint = "fest run --resume to continue"
	case OutcomeRunnable:
		hint = "fest run --exec <command> to drive slices"
	}
	return StatusReport{
		SchemaVersion: statusSchema,
		Outcome:       verdict.Outcome,
		Reason:        verdict.Reason,
		Label:         verdict.Label,
		Kind:          snap.Kind,
		Current:       snap.Current,
		Total:         snap.Total,
		TasksDone:     TaskCount(recs),
		Ledger:        ledgerPath,
		LastCommit:    lastSHA,
		NextHint:      hint,
		Records:       recs,
	}
}

func lastCommit(recs []Record) string {
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Commit != "" {
			return recs[i].Commit
		}
	}
	return ""
}

func displayPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

func emitStatus(opts Options, report StatusReport) error {
	if opts.JSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return errors.Parse("encoding fest run status", err)
		}
		fmt.Fprintln(opts.Stdout, string(data))
		return nil
	}
	fmt.Fprintf(opts.Stdout, "outcome: %s\n", report.Outcome)
	if report.Reason != "" {
		fmt.Fprintf(opts.Stdout, "reason: %s\n", report.Reason)
	}
	if report.Label != "" {
		fmt.Fprintf(opts.Stdout, "next: %s", report.Label)
		if report.Total > 0 {
			fmt.Fprintf(opts.Stdout, " (%d/%d)", report.Current, report.Total)
		}
		fmt.Fprintln(opts.Stdout)
	}
	fmt.Fprintf(opts.Stdout, "did: %d tasks\n", report.TasksDone)
	if report.LastCommit != "" {
		fmt.Fprintf(opts.Stdout, "commit: %s\n", report.LastCommit)
	}
	if report.Ledger != "" {
		fmt.Fprintf(opts.Stdout, "log: %s\n", displayPath(report.Ledger))
	}
	if report.NextHint != "" {
		fmt.Fprintf(opts.Stdout, "hint: %s\n", report.NextHint)
	}
	return nil
}
