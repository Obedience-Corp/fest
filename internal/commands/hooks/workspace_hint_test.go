package hooks

import (
	"strings"
	"testing"
)

// When no hooks are configured because the workspace has no
// festivals/.festival/config.yaml at all, the text listing names the absent
// file and the commands that create it. The JSON view is a pinned contract and
// carries no such line.

func TestWorkspaceConfigHintLine_NamesFileAndCommands(t *testing.T) {
	line := workspaceConfigHintLine("festivals/.festival/config.yaml")
	for _, want := range []string{"festivals/.festival/config.yaml", "fest system update", "fest init"} {
		if !strings.Contains(line, want) {
			t.Errorf("hint %q lacks %q", line, want)
		}
	}
}

func TestWorkspaceConfigHintLine_IsOneLine(t *testing.T) {
	if line := workspaceConfigHintLine("x"); strings.Contains(line, "\n") {
		t.Fatalf("hint spans lines: %q", line)
	}
}
