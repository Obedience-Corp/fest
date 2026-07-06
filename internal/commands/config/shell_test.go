package config

import (
	"strings"
	"testing"
)

func TestShellInitValidShells(t *testing.T) {
	validShells := []string{"zsh", "bash", "fish"}

	for _, shell := range validShells {
		t.Run(shell, func(t *testing.T) {
			cmd := NewShellInitCommand()
			cmd.SetArgs([]string{shell})
			err := cmd.Execute()
			if err != nil {
				t.Errorf("shell-init %s should not error: %v", shell, err)
			}
		})
	}
}

func TestShellInitInvalidShell(t *testing.T) {
	cmd := NewShellInitCommand()
	cmd.SetArgs([]string{"powershell"})

	err := cmd.Execute()
	if err == nil {
		t.Error("shell-init with invalid shell should return error")
	}

	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error should mention 'unsupported shell', got: %v", err)
	}
}

func TestShellInitNoArgs(t *testing.T) {
	cmd := NewShellInitCommand()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("shell-init with no args should return error")
	}
}
