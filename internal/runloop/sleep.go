package runloop

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// startSleepGuard keeps the machine from sleeping during a live run.
// Failure is ignored: leaveable is still useful in tmux without caffeinate.
func startSleepGuard() func() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("caffeinate", "-dimsu", "-w", strconv.Itoa(os.Getpid()))
	case "linux":
		cmd = exec.Command("systemd-inhibit", "--what=idle:sleep", "--who=fest", "--why=fest run", "sleep", "infinity")
	default:
		return func() {}
	}
	if err := cmd.Start(); err != nil {
		return func() {}
	}
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}
}
