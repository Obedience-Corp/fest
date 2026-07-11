package research

import "testing"

func TestResearchTreeHiddenAndDeprecated(t *testing.T) {
	cmd := NewResearchCommand()
	if !cmd.Hidden {
		t.Error("research command should be hidden")
	}
	if cmd.Deprecated == "" {
		t.Error("research command should carry a deprecation notice")
	}

	wantChildren := map[string]bool{"create": false, "summary": false, "link": false}
	for _, sub := range cmd.Commands() {
		if _, tracked := wantChildren[sub.Name()]; !tracked {
			continue
		}
		wantChildren[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("research subcommand %q should be hidden", sub.Name())
		}
		if sub.Deprecated == "" {
			t.Errorf("research subcommand %q should carry a deprecation notice", sub.Name())
		}
	}
	for name, seen := range wantChildren {
		if !seen {
			t.Errorf("expected research subcommand %q to still be registered", name)
		}
	}
}
