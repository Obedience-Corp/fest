package shell

import "testing"

// The mapping exists so a hint never names a shell that cannot run the script
// it offers. Bourne-family shells that are not bash or zsh all resolve to the
// POSIX script rather than to "unsupported".
func TestDetect(t *testing.T) {
	tests := map[string]string{
		"/bin/zsh":                 "zsh",
		"/bin/bash":                "bash",
		"/usr/local/bin/fish":      "fish",
		"/bin/sh":                  "sh",
		"/bin/ash":                 "sh",
		"/bin/dash":                "sh",
		"/bin/busybox":             "sh",
		"/bin/ksh":                 "sh",
		"-sh":                      "sh", // login shell form, as seen on the Kindle
		"-bash":                    "bash",
		"":                         FallbackShell,
		"/opt/weird/experimental7": FallbackShell,
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			if got := detect(in); got != want {
				t.Errorf("detect(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// Every value detect can return must be a shell Generate accepts, or the hint
// names a command that errors when the user runs it.
func TestDetectAlwaysReturnsAGeneratableShell(t *testing.T) {
	for _, in := range []string{"/bin/zsh", "/bin/bash", "/bin/fish", "/bin/sh", "/bin/ksh", "", "/nope"} {
		got := detect(in)
		if !IsSupported(got) {
			t.Errorf("detect(%q) = %q, which IsSupported rejects", in, got)
		}
		if _, err := Generate(got); err != nil {
			t.Errorf("Generate(detect(%q)) error = %v", in, err)
		}
	}
}

// fish reads the script from a pipe; the others eval it. Handing a fish user an
// eval line is the same class of dead end as handing a dash user bash syntax.
func TestInitCommand(t *testing.T) {
	tests := map[string]string{
		"zsh":  `eval "$(fest shell-init zsh)"`,
		"bash": `eval "$(fest shell-init bash)"`,
		"sh":   `eval "$(fest shell-init sh)"`,
		"fish": "fest shell-init fish | source",
		"nope": `eval "$(fest shell-init sh)"`, // unknown falls back to POSIX
	}
	for in, want := range tests {
		t.Run(in, func(t *testing.T) {
			if got := InitCommand(in); got != want {
				t.Errorf("InitCommand(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestInitHintFollowsShellEnv(t *testing.T) {
	t.Setenv("SHELL", "/bin/dash")
	if got, want := InitHint(), `eval "$(fest shell-init sh)"`; got != want {
		t.Errorf("InitHint() under dash = %q, want %q", got, want)
	}

	t.Setenv("SHELL", "/usr/bin/fish")
	if got, want := InitHint(), "fest shell-init fish | source"; got != want {
		t.Errorf("InitHint() under fish = %q, want %q", got, want)
	}
}
