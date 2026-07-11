package templates

import "testing"

func TestTemplatesTreeHiddenAndDeprecated(t *testing.T) {
	cmd := NewTemplatesCommand()
	if !cmd.Hidden {
		t.Error("templates command should be hidden")
	}
	if cmd.Deprecated == "" {
		t.Error("templates command should carry a deprecation notice")
	}

	wantChildren := map[string]bool{"create": false, "apply": false, "list": false}
	for _, sub := range cmd.Commands() {
		if _, tracked := wantChildren[sub.Name()]; !tracked {
			continue
		}
		wantChildren[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("templates subcommand %q should be hidden", sub.Name())
		}
		if sub.Deprecated == "" {
			t.Errorf("templates subcommand %q should carry a deprecation notice", sub.Name())
		}
	}
	for name, seen := range wantChildren {
		if !seen {
			t.Errorf("expected templates subcommand %q to still be registered", name)
		}
	}
}
