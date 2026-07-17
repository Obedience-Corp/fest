package continuation

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

const (
	// defaultObeyBinary is the installed CLI invoked as the notification
	// boundary. Fest never imports Obey's Go packages.
	defaultObeyBinary = "obey"
	// notifyTimeout bounds the submit call. Obey owns long-lived delivery
	// retry once it accepts the request, so the producer only needs a short
	// hand-off window.
	notifyTimeout = 5 * time.Second
)

// Notifier submits a session notification to the originating Obey session. A
// failure must never change the judge outcome; callers isolate the error.
type Notifier interface {
	Notify(ctx context.Context, n SessionNotification) error
}

// CLINotifier submits the envelope by invoking 'obey session notify' with the
// JSON envelope on stdin. Feedback is never interpolated into shell syntax.
type CLINotifier struct {
	// Binary overrides the executable name; empty uses "obey" from PATH.
	Binary string
}

// Notify marshals the envelope and hands it to 'obey session notify' over
// stdin within a bounded timeout. A missing binary, a non-zero exit, or a
// timeout is returned as an error for the caller to isolate from the judge
// outcome.
func (c CLINotifier) Notify(ctx context.Context, n SessionNotification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	binary := c.Binary
	if binary == "" {
		binary = defaultObeyBinary
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return festerrors.Wrap(err, "resolving obey binary for session notify").
			WithField("binary", binary).
			WithHint("continuation delivery is skipped; the judge result remains recorded")
	}
	data, err := json.Marshal(n)
	if err != nil {
		return festerrors.Wrap(err, "encoding session notification envelope")
	}

	submitCtx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()

	cmd := exec.CommandContext(submitCtx, path, "session", "notify")
	cmd.Stdin = bytes.NewReader(append(data, '\n'))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		wrapped := festerrors.Wrap(err, "submitting session notification").
			WithField("delivery_id", n.DeliveryID)
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			wrapped = wrapped.WithField("stderr", msg)
		}
		return wrapped
	}
	return nil
}
