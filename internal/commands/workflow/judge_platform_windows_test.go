//go:build windows

package workflow

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestConfigureDetachedJudgeProcessWindows(t *testing.T) {
	cmd := exec.Command("fest.exe")
	configureDetachedJudgeProcess(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("Windows judge runner requires process attributes")
	}
	want := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS)
	if cmd.SysProcAttr.CreationFlags&want != want {
		t.Fatalf("creation flags = %#x, want detached process group bits %#x", cmd.SysProcAttr.CreationFlags, want)
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("detached Windows judge runner must not create a visible console window")
	}
}
