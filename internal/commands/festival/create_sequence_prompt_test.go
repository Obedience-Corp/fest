package festival

import "testing"

func TestShouldPromptCreateTaskFiles(t *testing.T) {
	orig := createSequenceStdinIsTTY
	t.Cleanup(func() { createSequenceStdinIsTTY = orig })

	cases := []struct {
		name string
		opts *CreateSequenceOptions
		tty  bool
		want bool
	}{
		{name: "nil opts", opts: nil, tty: true, want: false},
		{name: "interactive default", opts: &CreateSequenceOptions{}, tty: true, want: true},
		{name: "non-tty skips prompt", opts: &CreateSequenceOptions{}, tty: false, want: false},
		{name: "json skips prompt", opts: &CreateSequenceOptions{JSONOutput: true}, tty: true, want: false},
		{name: "no-prompt skips", opts: &CreateSequenceOptions{NoPrompt: true}, tty: true, want: false},
		{name: "skip-markers skips", opts: &CreateSequenceOptions{SkipMarkers: true}, tty: true, want: false},
		{name: "json and non-tty", opts: &CreateSequenceOptions{JSONOutput: true}, tty: false, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			createSequenceStdinIsTTY = func() bool { return tc.tty }
			if got := shouldPromptCreateTaskFiles(tc.opts); got != tc.want {
				t.Fatalf("shouldPromptCreateTaskFiles() = %v, want %v", got, tc.want)
			}
		})
	}
}
