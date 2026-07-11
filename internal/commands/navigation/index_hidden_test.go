package navigation

import "testing"

func TestIndexChildrenHiddenAndDeprecatedParentVisible(t *testing.T) {
	cmd := NewIndexCommand()
	if cmd.Hidden {
		t.Error("index parent command should stay visible")
	}
	if cmd.Deprecated != "" {
		t.Error("index parent command should not itself be marked deprecated")
	}

	wantChildren := map[string]bool{
		"write":    false,
		"validate": false,
		"show":     false,
		"tree":     false,
		"diff":     false,
	}
	for _, sub := range cmd.Commands() {
		if _, tracked := wantChildren[sub.Name()]; !tracked {
			continue
		}
		wantChildren[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("index subcommand %q should be hidden", sub.Name())
		}
		if sub.Deprecated == "" {
			t.Errorf("index subcommand %q should carry a deprecation notice", sub.Name())
		}
	}
	for name, seen := range wantChildren {
		if !seen {
			t.Errorf("expected index subcommand %q to still be registered", name)
		}
	}
}
