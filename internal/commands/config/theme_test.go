//go:build !no_charm

package config

import "testing"

func TestThemeTreeHiddenAndDeprecated(t *testing.T) {
	cmd := NewThemeCommand()
	if !cmd.Hidden {
		t.Error("theme command should be hidden")
	}
	if cmd.Deprecated == "" {
		t.Error("theme command should carry a deprecation notice")
	}

	wantChildren := map[string]bool{"show": false, "set": false, "test": false}
	for _, sub := range cmd.Commands() {
		if _, tracked := wantChildren[sub.Name()]; !tracked {
			continue
		}
		wantChildren[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("theme subcommand %q should be hidden", sub.Name())
		}
		if sub.Deprecated == "" {
			t.Errorf("theme subcommand %q should carry a deprecation notice", sub.Name())
		}
	}
	for name, seen := range wantChildren {
		if !seen {
			t.Errorf("expected theme subcommand %q to still be registered", name)
		}
	}
}
