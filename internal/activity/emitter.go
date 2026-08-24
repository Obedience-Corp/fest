package activity

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/version"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

const (
	// FestivalFileName is the activity log file name inside .fest/.
	FestivalFileName = "activity.jsonl"

	// CampaignSubDir is the campaign-level directory for fest activity.
	CampaignSubDir = "fest"

	// CampaignFileName is the activity log file name at the campaign level.
	CampaignFileName = "activity.jsonl"

	// RotationThreshold is the file size at which rotation kicks in (50 MiB).
	RotationThreshold = 50 * 1024 * 1024

	dirPerms  os.FileMode = 0755
	filePerms os.FileMode = 0644
)

// Emitter writes activity events to the festival-level and/or campaign-level
// activity.jsonl files. It is safe for concurrent use within a single process
// (mutex) and across processes (advisory flock).
type Emitter struct {
	festivalPath string
	campaignRoot string
	warn         func(error)
	disabled     bool
	mu           sync.Mutex
}

// Option customises an event before writing.
type Option func(*Event)

// WithData sets the event-specific data payload.
func WithData(data any) Option {
	return func(ev *Event) { ev.Data = data }
}

// WithError marks the result as failed with an error message. Failed
// invocations should still be logged so the activity log includes errors.
func WithError(err error) Option {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return func(ev *Event) {
		ev.Result = Result{OK: false, Error: msg}
	}
}

// NewFromFestival builds an emitter scoped to the festival at festivalPath.
// If festivalPath is empty or the campaign root cannot be detected, campaign-
// level emission is skipped (festival-only). If festivalPath is empty, the
// emitter is fully disabled.
func NewFromFestival(ctx context.Context, festivalPath string, warn func(error)) *Emitter {
	if warn == nil {
		warn = func(error) {}
	}
	e := &Emitter{
		festivalPath: festivalPath,
		warn:         warn,
	}
	if festivalPath == "" {
		e.disabled = true
		return e
	}
	// Detect campaign root for dual emission. Missing campaign is not an error —
	// we still emit festival-level events.
	root, err := workspace.DetectCampaign(ctx, festivalPath)
	if err == nil && root != "" {
		e.campaignRoot = root
	}
	return e
}

// NewCampaignOnly builds an emitter that writes only to the campaign-level
// file, for pure campaign events (festival.created when no festival dir exists
// yet). If campaignRoot is empty, the emitter is disabled.
func NewCampaignOnly(ctx context.Context, campaignRoot string, warn func(error)) *Emitter {
	if warn == nil {
		warn = func(error) {}
	}
	if campaignRoot == "" {
		// Try to detect from cwd.
		root, err := workspace.DetectCampaign(ctx, "")
		if err != nil || root == "" {
			return &Emitter{disabled: true, warn: warn}
		}
		campaignRoot = root
	}
	return &Emitter{
		campaignRoot: campaignRoot,
		warn:         warn,
	}
}

// Emit records one activity event. It determines the destination from the
// catalog and writes to the appropriate file(s). Write failures are reported
// via the warn function but never returned to the caller — activity logging
// is best-effort and must not fail the user's command.
func (e *Emitter) Emit(ctx context.Context, eventName string, scope Scope, sourceCmd string, opts ...Option) {
	if e == nil || e.disabled {
		return
	}
	if ctx.Err() != nil {
		e.warn(ctx.Err())
		return
	}

	ev := newEvent(eventName, scope, sourceCmd, nil, Result{OK: true})
	for _, o := range opts {
		if o == nil {
			continue
		}
		o(&ev)
	}

	dest := destination(eventName)
	e.mu.Lock()
	defer e.mu.Unlock()

	// Fill in scope paths that the emitter knows.
	if e.festivalPath != "" {
		ev.Scope.FestivalPathRelative = festivalRelativePath(e.campaignRoot, e.festivalPath)
		if ev.Scope.FestivalID == "" {
			ev.Scope.FestivalID = festivalIDFromPath(e.festivalPath)
		}
		if ev.Scope.FestivalName == "" {
			ev.Scope.FestivalName = filepath.Base(e.festivalPath)
		}
	}
	if e.campaignRoot != "" && ev.Scope.CampaignRoot == "" {
		ev.Scope.CampaignRoot = e.campaignRoot
	}

	// Always write to festival-level if we have a festival path.
	if e.festivalPath != "" {
		festFile := filepath.Join(e.festivalPath, ProgressDir, FestivalFileName)
		if err := e.writeFile(festFile, ev); err != nil {
			e.warn(err)
		}
	}

	// Write to campaign-level if the event destination is DestBoth and we
	// have a campaign root.
	if dest == DestBoth && e.campaignRoot != "" {
		campFile := filepath.Join(e.campaignRoot, workspace.CampaignDir, CampaignSubDir, CampaignFileName)
		if err := e.writeFile(campFile, ev); err != nil {
			e.warn(err)
		}
	}
}

// writeFile appends one event as a JSON line to the given file path. It uses
// an advisory flock to prevent interleaved writes from concurrent fest
// processes, and fsyncs after every write for durability.
func (e *Emitter) writeFile(path string, ev Event) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerms); err != nil {
		return errors.IO("creating activity log directory", err).WithField("path", filepath.Dir(path))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerms)
	if err != nil {
		return errors.IO("opening activity log", err).WithField("path", path)
	}
	defer func() { _ = f.Close() }()

	// Advisory file lock to prevent interleaved writes from concurrent processes.
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return errors.IO("locking activity log", err).WithField("path", path)
	}
	defer func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }()

	// Rotate if the file exceeds the threshold.
	if info, err := f.Stat(); err == nil && info.Size() > RotationThreshold {
		_ = rotateFile(path)
	}

	data, err := marshalEvent(ev)
	if err != nil {
		return errors.Wrap(err, "marshaling activity event")
	}
	data = append(data, '\n')

	if _, err := f.Write(data); err != nil {
		return errors.IO("writing activity event", err).WithField("path", path)
	}
	// fsync for durability — activity history matters more than throughput.
	if err := f.Sync(); err != nil {
		return errors.IO("fsyncing activity log", err).WithField("path", path)
	}
	return nil
}

// rotateFile renames the current file to an archived copy so a fresh log
// can start. It is a safety net, not a steady-state concern. Compression is
// a follow-up; for now we just move the old file aside.
func rotateFile(path string) error {
	dir := filepath.Dir(path)
	// Find the next available rotation slot.
	for n := 1; ; n++ {
		rotated := filepath.Join(dir, "activity."+itoa(n)+".jsonl")
		if _, err := os.Stat(rotated); os.IsNotExist(err) {
			return os.Rename(path, rotated)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// ProgressDir is the festival-level state directory. It mirrors
// progress.ProgressDir to avoid an import cycle (progress imports many
// command packages that would create a cycle if activity imported progress).
const ProgressDir = ".fest"

// festivalRelativePath returns the festival path relative to the campaign
// root, or the basename if the campaign root is empty.
func festivalRelativePath(campaignRoot, festivalPath string) string {
	if campaignRoot == "" {
		return filepath.Base(festivalPath)
	}
	rel, err := filepath.Rel(campaignRoot, festivalPath)
	if err != nil {
		return filepath.Base(festivalPath)
	}
	return rel
}

// festivalIDFromPath extracts the festival ID from the directory name suffix
// (e.g. "corp-site-build-CS0003" → "CS0003"), falling back to the basename.
func festivalIDFromPath(festivalPath string) string {
	base := filepath.Base(festivalPath)
	if i := lastIndexByte(base, '-'); i >= 0 && i < len(base)-1 {
		suffix := base[i+1:]
		if looksLikeFestID(suffix) {
			return suffix
		}
	}
	return base
}

func looksLikeFestID(s string) bool {
	if len(s) < 4 || len(s) > 12 {
		return false
	}
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// FestVersion returns the current fest version for external consumers.
func FestVersion() string { return version.Version }

// marshalEvent serializes an Event as compact JSON without HTML-escaping,
// so <REDACTED> appears literally in the log rather than as \u003cREDACTED\u003e.
func marshalEvent(ev Event) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(ev); err != nil {
		return nil, err
	}
	// json.Encoder.Encode appends a trailing newline; trim it since writeFile
	// adds its own.
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out, nil
}
