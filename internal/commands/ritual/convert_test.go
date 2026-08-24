package ritual

import (
	"testing"
)

func TestSanitizeDirName(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"quarterly security review", "quarterly-security-review"},
		{"simple-name", "simple-name"},
		{"with/slash", "with-slash"},
		{"with\\backslash", "with-backslash"},
		{"  trim-me  ", "trim-me"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeDirName(tc.input)
			if got != tc.expect {
				t.Errorf("sanitizeDirName(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}

func TestIsDateDir(t *testing.T) {
	cases := []struct {
		input  string
		expect bool
	}{
		{"2026-01", true},
		{"2026-01-15", true},
		{"2026-1", false},
		{"2026", false},
		{"festival-name", false},
		{"", false},
		{"2026-13", true}, // format check only, not calendar validity
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := isDateDir(tc.input)
			if got != tc.expect {
				t.Errorf("isDateDir(%q) = %v, want %v", tc.input, got, tc.expect)
			}
		})
	}
}

func TestIsDigit(t *testing.T) {
	cases := []struct {
		input  byte
		expect bool
	}{
		{'0', true},
		{'9', true},
		{'a', false},
		{'/', false},
		{0, false},
	}
	for _, tc := range cases {
		got := isDigit(tc.input)
		if got != tc.expect {
			t.Errorf("isDigit(%d) = %v, want %v", tc.input, got, tc.expect)
		}
	}
}
