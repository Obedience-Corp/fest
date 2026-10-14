package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CommandContext kills only the shell. A condition can launch child processes,
// so cancellation must terminate the shell's process tree on Windows.
func configureCommandCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		killCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		output, err := exec.CommandContext(killCtx, "taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").CombinedOutput()
		if err == nil {
			return nil
		}
		// Still stop the condition shell if taskkill is unavailable or fails.
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("terminate command tree: %w (%s); kill shell: %v", err, strings.TrimSpace(string(output)), killErr)
		}
		return fmt.Errorf("terminate command tree: %w (%s)", err, strings.TrimSpace(string(output)))
	}
}
