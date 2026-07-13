package workflow

import (
	"strings"
	"testing"
)

// Keep host coverage pure. Real payload/state writes and process launch live
// in tests/integration/async_judge_test.go's container boundary.
func TestLooksLikeGoTestBinary(t *testing.T) {
	cases := []struct {
		exe  string
		want bool
	}{
		{"/tmp/go-build123/b001/workflow.test", true},
		{`C:\Users\x\go-build\workflow.test.exe`, true},
		{"/usr/local/bin/fest", false},
		{"/Users/me/bin/fest", false},
		{"workflow.test.backup", false},
	}
	for _, tc := range cases {
		if got := looksLikeGoTestBinary(tc.exe); got != tc.want {
			t.Fatalf("looksLikeGoTestBinary(%q) = %v, want %v", tc.exe, got, tc.want)
		}
	}
}

func TestLaunchJudgeProcessDefault_RefusesGoTestBinary(t *testing.T) {
	// Static paths are intentional: the guard must return before performing
	// any filesystem mutation.
	_, err := launchJudgeProcessDefault("/nonexistent/payload.json", "/nonexistent", "/nonexistent/judge.log")
	if err == nil {
		t.Fatal("expected refusal when executable is a go test binary")
	}
	if !strings.Contains(err.Error(), "go test binary") {
		t.Fatalf("error = %v, want go test binary refusal", err)
	}
}
