package itestenv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	obeyDirName  = ".obey"
	locksDirName = "locks"
	// lockNamePrefix matches camp's itestenv so a camp suite and a fest suite
	// targeting the same daemon serialize on one flock. A fest-specific prefix
	// would be a parallel lock namespace, which is the thing Q3 forbids.
	lockNamePrefix = "camp-itest-"
	lockNameSuffix = ".lock"
	lockKeyBytes   = 6

	// EnvLockWait overrides how long a second run waits for the first to
	// finish before giving up.
	EnvLockWait = "FEST_ITEST_LOCK_WAIT"

	// DefaultLockWait comfortably exceeds a healthy full-suite wall time.
	DefaultLockWait = 30 * time.Minute

	lockPollInterval   = 500 * time.Millisecond
	lockNoticeInterval = 30 * time.Second
	lockFileMode       = 0o644
	lockDirMode        = 0o755
)

var (
	errLockBusy        = festerrors.New("lock is held by another process")
	errLockUnsupported = festerrors.New("file locking is unsupported on this platform")
)

func isLockBusy(err error) bool {
	return errors.Is(err, errLockBusy)
}

func isLockUnsupported(err error) bool {
	return errors.Is(err, errLockUnsupported)
}

// SuiteLockPath keys the suite lock by the daemon it protects, so two suites
// aimed at the same VM serialize while suites on different daemons do not wait
// on each other.
func SuiteLockPath(home, dockerHost string) string {
	key := strings.TrimSpace(dockerHost)
	if path := SocketPath(dockerHost); path != "" {
		key = path
	}
	if key == "" {
		key = "docker-default"
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(home, obeyDirName, locksDirName,
		lockNamePrefix+hex.EncodeToString(sum[:lockKeyBytes])+lockNameSuffix)
}

func profileLockPath(home, profile string) (string, error) {
	if profile == "" {
		return "", festerrors.New("profile name is required for the start lock")
	}
	return filepath.Join(home, obeyDirName, locksDirName,
		lockNamePrefix+"profile-"+profile+lockNameSuffix), nil
}

// LockStatus reports whether a lock is held, and by whom, without taking it.
func LockStatus(path string) (held bool, description string) {
	file, err := os.OpenFile(path, os.O_RDWR, lockFileMode)
	if err != nil {
		return false, "free"
	}
	defer func() { _ = file.Close() }()

	if err := lockFileNB(file); err != nil {
		if isLockBusy(err) {
			h, ok := readHolder(path)
			return true, "held by " + describeHolder(h, ok)
		}
		return false, "unknown: " + err.Error()
	}
	_ = unlockFile(file)
	return false, "free"
}

// LockOptions configures Acquire.
type LockOptions struct {
	Wait  time.Duration
	Out   io.Writer
	Label string
	Poll  time.Duration
	Now   func() time.Time
}

func (o LockOptions) normalize() LockOptions {
	if o.Wait <= 0 {
		o.Wait = DefaultLockWait
	}
	if o.Poll <= 0 {
		o.Poll = lockPollInterval
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.Label == "" {
		o.Label = "integration suite"
	}
	return o
}

// LockWait reads the wait budget from the environment, falling back to
// DefaultLockWait when unset or unparseable.
func LockWait(getenv func(string) string) time.Duration {
	if getenv == nil {
		getenv = os.Getenv
	}
	value := strings.TrimSpace(getenv(EnvLockWait))
	if value == "" {
		return DefaultLockWait
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return DefaultLockWait
	}
	return d
}

// Lock is a held cross-process lock. The zero value is not usable; Acquire
// returns one.
type Lock struct {
	path string
	file *os.File
}

// Path returns the lock file's location.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Acquire takes an exclusive lock on path, waiting for the current holder up
// to the configured budget and naming it while waiting.
func Acquire(ctx context.Context, path string, opts LockOptions) (*Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o := opts.normalize()
	if err := os.MkdirAll(filepath.Dir(path), lockDirMode); err != nil {
		return nil, festerrors.Wrapf(err, "create lock directory for %s", path)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, lockFileMode)
	if err != nil {
		return nil, festerrors.Wrapf(err, "open lock file %s", path)
	}
	lock, err := waitForLock(ctx, file, path, o)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return lock, nil
}

func waitForLock(ctx context.Context, file *os.File, path string, o LockOptions) (*Lock, error) {
	start := o.Now()
	deadline := start.Add(o.Wait)
	var lastNotice time.Time
	for {
		switch err := lockFileNB(file); {
		case err == nil:
			writeHolder(file, o.Label)
			return &Lock{path: path, file: file}, nil
		case isLockUnsupported(err):
			writeLine(o.Out, "WARNING integration suite lock unavailable on this platform: "+
				"concurrent runs against one daemon cannot be serialized")
			return &Lock{path: path}, nil
		case !isLockBusy(err):
			return nil, festerrors.Wrapf(err, "lock %s", path)
		}

		waited := o.Now().Sub(start)
		current, known := readHolder(path)
		if lastNotice.IsZero() || o.Now().Sub(lastNotice) >= lockNoticeInterval {
			writeLine(o.Out, waitingNotice(o.Label, waited, current, known))
			lastNotice = o.Now()
		}
		if o.Now().After(deadline) {
			return nil, festerrors.New(fmt.Sprintf("%s is still locked after %s by %s; "+
				"re-run once it finishes, or raise %s",
				o.Label, waited.Round(time.Second), describeHolder(current, known), EnvLockWait))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(o.Poll):
		}
	}
}

// Release drops the lock. The record is truncated first so a reader of the
// file never attributes a finished run's pid to a live holder.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = l.file.Truncate(0)
	err := unlockFile(l.file)
	if closeErr := l.file.Close(); err == nil {
		err = closeErr
	}
	l.file = nil
	if err != nil {
		return festerrors.Wrapf(err, "release lock %s", l.path)
	}
	return nil
}

type holder struct {
	PID     int       `json:"pid"`
	Label   string    `json:"label"`
	Started time.Time `json:"started"`
}

func writeHolder(file *os.File, label string) {
	record, err := json.Marshal(holder{PID: os.Getpid(), Label: label, Started: time.Now()})
	if err != nil {
		return
	}
	_ = file.Truncate(0)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return
	}
	_, _ = file.Write(append(record, '\n'))
	_ = file.Sync()
}

func readHolder(path string) (h holder, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return holder{}, false
	}
	return parseHolder(data)
}

func parseHolder(data []byte) (holder, bool) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return holder{}, false
	}
	var h holder
	if err := json.Unmarshal([]byte(trimmed), &h); err != nil || h.PID <= 0 {
		return holder{}, false
	}
	return h, true
}

func describeHolder(h holder, ok bool) string {
	if !ok {
		return "another process"
	}
	description := "pid " + strconv.Itoa(h.PID)
	if !h.Started.IsZero() {
		description += " (started " + h.Started.Format(time.TimeOnly) + ")"
	}
	return description
}

func waitingNotice(label string, waited time.Duration, h holder, ok bool) string {
	notice := "waiting for " + label + ": held by " + describeHolder(h, ok)
	if waited >= time.Second {
		notice += "; waited " + waited.Round(time.Second).String()
	}
	return notice
}
