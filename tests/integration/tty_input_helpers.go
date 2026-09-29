//go:build integration
// +build integration

package integration

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
)

// ttyInputSettleDelay gives the command time to render its prompt before the
// answer is written. Writing immediately is harmless for a reader that blocks,
// but the delay keeps the captured transcript in the order an operator sees.
const (
	ttyInputSettleDelay   = 300 * time.Millisecond
	ttyInputExitPolls     = 100
	ttyInputExitPollDelay = 50 * time.Millisecond
)

// RunFestInDirTTYWithInput runs fest from dir on a real pseudo-terminal and
// writes input to its stdin.
//
// The existing TTY runners attach stdin but expose no writer, because
// testcontainers' Exec returns only a reader. fest task defer needs both: the
// operator guard refuses anything that is not a terminal, and the confirmation
// then asks the operator to type the task number. Supplying the answer any
// other way would mean weakening the guard, so the harness grows a runner
// instead. Nothing about the guard changes: the command still sees a terminal
// on stdin, still finds no agent marker in the container environment, and still
// walks a process ancestry with no agent binary in it.
func (tc *TestContainer) RunFestInDirTTYWithInput(dir, input string, args ...string) (string, error) {
	cli, err := testcontainers.NewDockerClientWithOpts(tc.ctx)
	if err != nil {
		return "", fmt.Errorf("docker client for TTY input: %w", err)
	}
	defer func() { _ = cli.Close() }()

	created, err := cli.ExecCreate(tc.ctx, tc.container.GetContainerID(), client.ExecCreateOptions{
		Cmd:          []string{"sh", "-c", "cd " + dir + " && /fest " + strings.Join(args, " ")},
		TTY:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("creating TTY exec: %w", err)
	}

	attached, err := cli.ExecAttach(tc.ctx, created.ID, client.ExecAttachOptions{TTY: true})
	if err != nil {
		return "", fmt.Errorf("attaching to TTY exec: %w", err)
	}
	defer attached.Close()

	writeErr := make(chan error, 1)
	go func() {
		time.Sleep(ttyInputSettleDelay)
		_, err := io.WriteString(attached.Conn, input)
		writeErr <- err
	}()

	// A TTY exec is a single unmultiplexed stream, so the transcript is read
	// straight off the connection until the process exits and closes it. The
	// write side is deliberately left open: closing it half way closes the
	// whole hijacked connection, which would cut the transcript short and leave
	// the exec still running when it is inspected.
	outputBytes, readErr := io.ReadAll(attached.Reader)
	output := string(outputBytes)
	if err := <-writeErr; err != nil {
		return output, fmt.Errorf("writing to the TTY: %w", err)
	}
	if readErr != nil {
		return output, fmt.Errorf("reading the TTY transcript: %w", readErr)
	}

	inspected, err := tc.waitForExec(cli, created.ID)
	if err != nil {
		return output, err
	}
	if inspected.ExitCode != 0 {
		return output, fmt.Errorf("fest exited with code %d: %s", inspected.ExitCode, output)
	}
	return output, nil
}

// waitForExec polls until the exec process has finished, so the exit code read
// back is the process's own and not the zero value of a running exec.
func (tc *TestContainer) waitForExec(cli *testcontainers.DockerClient, execID string) (client.ExecInspectResult, error) {
	var last client.ExecInspectResult
	for range ttyInputExitPolls {
		inspected, err := cli.ExecInspect(tc.ctx, execID, client.ExecInspectOptions{})
		if err != nil {
			return client.ExecInspectResult{}, fmt.Errorf("inspecting the TTY exec: %w", err)
		}
		if !inspected.Running {
			return inspected, nil
		}
		last = inspected
		time.Sleep(ttyInputExitPollDelay)
	}
	return last, fmt.Errorf("the TTY exec was still running after %d polls", ttyInputExitPolls)
}
