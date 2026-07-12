package understand

import "testing"

func TestUnderstandDeadChildrenHiddenAndDeprecated(t *testing.T) {
	dead := map[string]bool{
		"chains":    false,
		"context":   false,
		"lifecycle": false,
		"nodeids":   false,
		"planning":  false,
		"resources": false,
		"rituals":   false,
		"roles":     false,
	}

	cmd := NewUnderstandCommand()
	for _, sub := range cmd.Commands() {
		if _, tracked := dead[sub.Name()]; !tracked {
			continue
		}
		dead[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("understand subcommand %q should be hidden", sub.Name())
		}
		if sub.Deprecated == "" {
			t.Errorf("understand subcommand %q should carry a deprecation notice", sub.Name())
		}
	}
	for name, seen := range dead {
		if !seen {
			t.Errorf("expected understand subcommand %q to still be registered", name)
		}
	}
}

func TestUnderstandHotChildrenStayVisible(t *testing.T) {
	hot := map[string]bool{
		"tasks":     false,
		"structure": false,
		"rules":     false,
		"templates": false,
		"loop":      false,
	}

	cmd := NewUnderstandCommand()
	for _, sub := range cmd.Commands() {
		if _, tracked := hot[sub.Name()]; !tracked {
			continue
		}
		hot[sub.Name()] = true
		if sub.Hidden {
			t.Errorf("understand subcommand %q should stay visible", sub.Name())
		}
		if sub.Deprecated != "" {
			t.Errorf("understand subcommand %q should not be deprecated", sub.Name())
		}
	}
	for name, seen := range hot {
		if !seen {
			t.Errorf("expected understand subcommand %q to still be registered", name)
		}
	}
}
