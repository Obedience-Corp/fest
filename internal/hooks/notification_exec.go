package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Notification output is informational only. Discard stdout and retain only a
// bounded stderr tail. WaitDelay bounds inherited pipes if a custom executable
// spawns a child that outlives the cancelled command.
func newNotificationRunner(dir string) *Runner {
	r := NewRunner(dir)
	r.Exec = func(ctx context.Context, command string, stdin []byte, dir string) CommandResult {
		args := strings.Fields(command)
		if len(args) == 0 {
			return CommandResult{ExitCode: -1, Err: fmt.Errorf("notification command is empty")}
		}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), r.execEnv...)
		cmd.Stdin = bytes.NewReader(stdin)
		cmd.Stdout = io.Discard
		stderr := &notificationTail{}
		cmd.Stderr = stderr
		cmd.WaitDelay = 100 * time.Millisecond
		err := cmd.Run()
		result := CommandResult{Err: err}
		if err != nil {
			result.ExitCode = -1
			if cmd.ProcessState != nil {
				result.ExitCode = cmd.ProcessState.ExitCode()
			}
			if len(stderr.data) > 0 {
				result.Err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(stderr.data)))
			}
		}
		return result
	}
	return r
}

type notificationTail struct{ data []byte }

func (b *notificationTail) Write(p []byte) (int, error) {
	size := len(p)
	if size >= stderrTailLimit {
		b.data = append(b.data[:0], p[size-stderrTailLimit:]...)
	} else {
		excess := len(b.data) + size - stderrTailLimit
		if excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, p...)
	}
	return size, nil
}
