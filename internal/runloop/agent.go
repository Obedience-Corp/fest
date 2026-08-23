package runloop

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// InvokeAgent runs one unattended slice. "claude" gets `claude -p`.
// Any other binary receives the prompt on stdin so tests can inject a fake.
func InvokeAgent(ctx context.Context, agent, prompt, workDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return errors.Validation("agent binary is required")
	}
	var cmd *exec.Cmd
	base := filepath.Base(agent)
	switch {
	case base == "claude" || base == "claude.exe":
		cmd = exec.CommandContext(ctx, agent, "-p", prompt)
	default:
		cmd = exec.CommandContext(ctx, agent)
		cmd.Stdin = strings.NewReader(prompt)
	}
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return errors.Wrap(err, "agent failed").
			WithField("agent", agent).
			WithField("stderr", msg)
	}
	return nil
}

func buildPrompt(snap Snapshot) string {
	var b strings.Builder
	b.WriteString("You are driving a Festival leaveable run. Do only this slice.\n")
	b.WriteString("Do not create festivals. Do not skip human gates. Do not invent the next task.\n")
	if snap.Label != "" {
		b.WriteString("\nSlice: ")
		b.WriteString(snap.Label)
		b.WriteString("\n")
	}
	if snap.Goal != "" {
		b.WriteString("\nGoal:\n")
		b.WriteString(snap.Goal)
		b.WriteString("\n")
	}
	if len(snap.Actions) > 0 {
		b.WriteString("\nActions:\n")
		for i, a := range snap.Actions {
			b.WriteString("  ")
			b.WriteString(strings.TrimSpace(a))
			if i < len(snap.Actions)-1 {
				b.WriteByte('\n')
			}
		}
		b.WriteByte('\n')
	}
	if snap.Content != "" && snap.Content != snap.Goal {
		b.WriteString("\n")
		b.WriteString(snap.Content)
		b.WriteString("\n")
	}
	return b.String()
}
