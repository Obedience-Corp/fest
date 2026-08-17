package shell

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// bashOnlyConstructs are syntaxes a POSIX shell cannot parse. posix_core.sh
// must contain none of them: a single one aborts the whole eval, which is how
// `eval "$(fest shell-init bash)"` under busybox ash produced
//
//	syntax error: unexpected "(" (expecting "}")
//
// and left the user with no fgo and no fls at all.
var bashOnlyConstructs = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"array assignment", regexp.MustCompile(`(^|\s)(local\s+|export\s+|declare\s+|readonly\s+)?[A-Za-z_][A-Za-z0-9_]*=\(`)},
	{"[[ test", regexp.MustCompile(`\[\[`)},
	{"=~ match", regexp.MustCompile(`=~`)},
	{"&> redirect", regexp.MustCompile(`&>`)},
	{"process substitution", regexp.MustCompile(`<\(`)},
	{"complete builtin", regexp.MustCompile(`^\s*complete\s`)},
	{"compgen builtin", regexp.MustCompile(`\bcompgen\b`)},
	{"compdef builtin", regexp.MustCompile(`^\s*compdef\s`)},
	{"compadd builtin", regexp.MustCompile(`\bcompadd\b`)},
	{"shopt builtin", regexp.MustCompile(`^\s*shopt\s`)},
	{"function keyword", regexp.MustCompile(`^\s*function\s`)},
	{"bash/zsh completion variable", regexp.MustCompile(`\$\{?(BASH_|COMP_|COMPREPLY|CURRENT\b|words\[)`)},
	{"substring expansion", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*:\d`)},
	{"ANSI-C quoting", regexp.MustCompile(`\$'`)},
}

// The sh script is what fest hands a shell with no bash. Anything bash-only in
// it is a syntax error for the user, not a degraded feature.
//
// posix_core.sh is also the shared base for bash and zsh, so a bash-ism added
// there runs fine for the author and breaks only sh users. This is the guard
// that fails at the source instead of on someone's Kindle.
func TestShScriptHasNoBashOnlySyntax(t *testing.T) {
	script, err := Generate("sh")
	if err != nil {
		t.Fatalf("Generate(sh) error = %v", err)
	}
	for i, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue // comments describe the rule; they do not execute
		}
		for _, c := range bashOnlyConstructs {
			if c.pattern.MatchString(line) {
				t.Errorf("line %d uses %s, which POSIX sh cannot parse:\n  %s", i+1, c.name, line)
			}
		}
	}
}

// A static scan proves no known bash-ism is present; a real POSIX shell proves
// the script actually parses. Both matter, because the scan only knows the
// constructs it was taught.
//
// macOS /bin/sh is bash in POSIX mode and would accept the very syntax this
// guards against, so it is deliberately not a candidate.
func TestShScriptParsesUnderRealPosixShell(t *testing.T) {
	var sh string
	for _, candidate := range []string{"dash", "busybox", "ash"} {
		if path, err := exec.LookPath(candidate); err == nil {
			sh = path
			break
		}
	}
	if sh == "" {
		t.Skip("no dash/busybox/ash on PATH")
	}

	script, err := Generate("sh")
	if err != nil {
		t.Fatalf("Generate(sh) error = %v", err)
	}

	args := []string{"-n"}
	if strings.HasSuffix(sh, "busybox") {
		args = append([]string{"sh"}, args...)
	}
	cmd := exec.Command(sh, args...)
	cmd.Stdin = strings.NewReader(script)
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("%s rejected the sh script: %v\n%s", sh, runErr, out)
	}
}

// sh is a first-class supported shell, not an alias that happens to render.
func TestShIsSupported(t *testing.T) {
	if !IsSupported("sh") {
		t.Error(`IsSupported("sh") = false, want true`)
	}
	script, err := Generate("sh")
	if err != nil {
		t.Fatalf("Generate(sh) error = %v", err)
	}
	for _, want := range []string{"fgo() {", "fls() {", "fest() {"} {
		if !strings.Contains(script, want) {
			t.Errorf("sh script missing %q", want)
		}
	}
	// Completion is the only thing sh gives up. Carrying the machinery anyway
	// would reintroduce the syntax error this whole script exists to avoid.
	for _, unwanted := range []string{"_fgo_completions", "_fls_completions", "_fest_completions_bash", "compdef"} {
		if strings.Contains(script, unwanted) {
			t.Errorf("sh script must not carry bash/zsh completion machinery, found %q", unwanted)
		}
	}
}

// Every shell drives the same fgo, fls, and fest. The bug this prevents is a
// wrapper improved in one script and left behind in the other, which is how
// two copies of the same function always end.
func TestEveryBourneShellSharesTheSameWrappers(t *testing.T) {
	shScript, err := Generate("sh")
	if err != nil {
		t.Fatalf("Generate(sh) error = %v", err)
	}
	for _, shellType := range []string{"bash", "zsh"} {
		script, genErr := Generate(shellType)
		if genErr != nil {
			t.Fatalf("Generate(%q) error = %v", shellType, genErr)
		}
		if !strings.Contains(script, shScript) {
			t.Errorf("%s script does not embed posix_core verbatim; the wrappers have drifted", shellType)
		}
	}
}

// bash and zsh must still get their completions. Sharing the POSIX core is only
// safe if it costs them nothing.
func TestBashAndZshKeepTheirCompletions(t *testing.T) {
	for _, shellType := range []string{"bash", "zsh"} {
		script, err := Generate(shellType)
		if err != nil {
			t.Fatalf("Generate(%q) error = %v", shellType, err)
		}
		for _, want := range []string{"complete -F _fgo_completions fgo", "complete -F _fls_completions fls", "compdef _fgo_zsh fgo"} {
			if !strings.Contains(script, want) {
				t.Errorf("%s script lost %q", shellType, want)
			}
		}
	}
}

// The wrappers scope their working variables with 'local'. Where that is
// missing the failure is silent and wrong rather than loud: ksh93 does not
// abort on a failed 'local', so the assignment never happens and the function
// reads whatever the caller had in that name. The script must refuse instead.
func TestShScriptGuardsAgainstMissingLocal(t *testing.T) {
	script, err := Generate("sh")
	if err != nil {
		t.Fatalf("Generate(sh) error = %v", err)
	}

	if !strings.Contains(script, `[ "$_fest_v" = inner ]`) {
		t.Error("the local-support probe must assert the effect of local, not merely that it ran")
	}
	if !strings.Contains(script, "has no 'local' builtin") {
		t.Error("the script must say why it refused")
	}

	// The probe has to run before any wrapper is defined, or the shell installs
	// functions it cannot execute correctly before reaching the check.
	probeAt := strings.Index(script, "_fest_probe")
	wrapperAt := strings.Index(script, "fgo() {")
	if probeAt < 0 || wrapperAt < 0 {
		t.Fatalf("could not locate probe (%d) or wrapper (%d)", probeAt, wrapperAt)
	}
	if probeAt > wrapperAt {
		t.Error("the local-support probe must run before the wrappers are defined")
	}
}

// fls tab completion advertises the status vocabulary. It drifted once already:
// fish offered four statuses while bash offered seven plus flags, so the same
// keystroke produced a different menu depending on the shell.
func TestFlsCompletionVocabularyMatchesAcrossShells(t *testing.T) {
	statuses := []string{"active", "ready", "planning", "ritual", "completed", "dungeon", "all"}
	for _, shellType := range []string{"bash", "fish"} {
		script, err := Generate(shellType)
		if err != nil {
			t.Fatalf("Generate(%q) error = %v", shellType, err)
		}
		candidates := flsCompletionCandidates(t, shellType, script)
		for _, want := range statuses {
			if !containsExact(candidates, want) {
				t.Errorf("%s fls completion does not offer %q; it offers %v", shellType, want, candidates)
			}
		}
	}
}

// flsCompletionCandidates returns the exact tokens a script offers for fls.
//
// It returns tokens rather than a blob because a substring check cannot do this
// job: "all" appears inside "--all", and "ready" inside "already". Matching on
// whole tokens is what makes this able to fail.
func flsCompletionCandidates(t *testing.T, shellType, script string) []string {
	t.Helper()
	var candidates []string
	switch shellType {
	case "bash":
		m := regexp.MustCompile(`local completions="([^"]+)"`).FindStringSubmatch(script)
		if m != nil {
			candidates = strings.Fields(m[1])
		}
	case "fish":
		m := regexp.MustCompile(`complete -c fls -f -a "([^"]+)"`).FindStringSubmatch(script)
		if m != nil {
			candidates = strings.Fields(m[1])
		}
	}
	if len(candidates) == 0 {
		t.Fatalf("%s: could not extract the fls completion candidates", shellType)
	}
	return candidates
}

func containsExact(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
