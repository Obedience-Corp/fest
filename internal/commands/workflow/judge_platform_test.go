package workflow

import (
	"os"
	"testing"
)

func TestJudgeProcessAlive_CurrentProcess(t *testing.T) {
	if !judgeProcessAlive(os.Getpid()) {
		t.Fatal("current process must be reported alive")
	}
}

func TestJudgeProcessAlive_InvalidPID(t *testing.T) {
	if judgeProcessAlive(0) || judgeProcessAlive(-1) {
		t.Fatal("non-positive PIDs must not be reported alive")
	}
}
