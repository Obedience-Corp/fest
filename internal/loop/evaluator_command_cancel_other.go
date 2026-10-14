//go:build !windows

package loop

import "os/exec"

func configureCommandCancellation(_ *exec.Cmd) {}
