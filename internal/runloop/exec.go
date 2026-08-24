package runloop

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// InvokeExec runs one user-supplied worker. Fest does not know any agent
// CLI. The slice prompt is written to stdin.
func InvokeExec(ctx context.Context, spec, prompt, workDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return errors.Validation("exec command is required")
	}
	parts := strings.Fields(spec)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(prompt)
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
		return errors.Wrap(err, "exec failed").
			WithField("exec", spec).
			WithField("stderr", msg)
	}
	return nil
}

func buildPrompt(snap Snapshot) string {
	var b strings.Builder
	b.WriteString("Do only this Festival slice. Do not skip human gates. Do not invent the next task.\n")
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
