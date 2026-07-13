//go:build unix

package workflow

import (
	"os/exec"
	"testing"
)

func TestConfigureDetachedJudgeProcessUnix(t *testing.T) {
	cmd := exec.Command("fest")
	configureDetachedJudgeProcess(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("Unix judge runner must start in a new session")
	}
}
