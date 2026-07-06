package shell

import (
	"strings"
	"testing"
)

func TestGenerateSupportedShells(t *testing.T) {
	for _, sh := range SupportedShells {
		out, err := Generate(sh)
		if err != nil {
			t.Fatalf("Generate(%q) error: %v", sh, err)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("Generate(%q) returned an empty script", sh)
		}
	}
}

func TestGenerateUnsupportedShell(t *testing.T) {
	_, err := Generate("powershell")
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error should mention 'unsupported shell', got: %v", err)
	}
}

func TestBashAndZshShareOneScript(t *testing.T) {
	bash, _ := Generate("bash")
	zsh, _ := Generate("zsh")
	if bash != zsh {
		t.Error("bash and zsh should render the same combined script")
	}
}

func TestIsSupported(t *testing.T) {
	for _, sh := range []string{"zsh", "bash", "fish"} {
		if !IsSupported(sh) {
			t.Errorf("IsSupported(%q) = false, want true", sh)
		}
	}
	if IsSupported("powershell") {
		t.Error("IsSupported(powershell) = true, want false")
	}
}
