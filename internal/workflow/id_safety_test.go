package workflow

import (
	"strings"
	"testing"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

func TestValidateWorkflowID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty", "", true},
		{"simple", "wf", false},
		{"with-dash", "wf-foo", false},
		{"with-date", "my-project-2026-05-13", false},
		{"with-underscore", "wf_bar", false},
		{"alnum", "abc123", false},
		{"leading-dash", "-leading", true},
		{"leading-underscore", "_leading", true},
		{"uppercase", "WfFoo", true},
		{"space", "Has Spaces", true},
		{"path-separator", "wf/foo", true},
		{"double-dot", "..", true},
		{"single-char", "a", false},
		{"max-length", "a" + strings.Repeat("b", 79), false},
		{"too-long", "a" + strings.Repeat("b", 80), true},
		{"non-ascii", "résumé", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWorkflowID(c.id)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateWorkflowID(%q): got err=%v, wantErr=%v", c.id, err, c.wantErr)
			}
			if c.wantErr && err != nil {
				if !festerrors.Is(err, festerrors.ErrCodeValidation) {
					t.Errorf("expected festerrors.Validation, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestSanitizeBasenameAsSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"foo", "foo"},
		{"Foo", "foo"},
		{"My Project", "my-project"},
		{"My Project!", "my-project"},
		{"my_thing", "my_thing"},
		{"-leading-", "leading"},
		{"  spaces  ", "spaces"},
		{"foo//bar", "foo-bar"},
		{"résumé", "r-sum"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := SanitizeBasenameAsSlug(c.in)
			if got != c.want {
				t.Errorf("SanitizeBasenameAsSlug(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
