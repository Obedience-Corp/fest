//go:build integration
// +build integration

package lifecycle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestContainer wraps container operations for testing
type TestContainer struct {
	container testcontainers.Container
	ctx       context.Context
	t         *testing.T
}

// NewSharedContainer creates a test container for shared use across tests.
func NewSharedContainer() (*TestContainer, error) {
	ctx := context.Background()

	festBinary, err := buildFestBinaryShared()
	if err != nil {
		return nil, fmt.Errorf("failed to build fest binary: %w", err)
	}
	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Cmd:        []string{"sleep", "3600"},
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(30 * time.Second),
		AutoRemove: true,
		Mounts: testcontainers.ContainerMounts{
			{
				Source:   testcontainers.GenericBindMountSource{HostPath: festBinary},
				Target:   "/fest",
				ReadOnly: false,
			},
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Make binary executable
	exitCode, _, err := container.Exec(ctx, []string{"chmod", "+x", "/fest"})
	if err != nil || exitCode != 0 {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to make binary executable: %w", err)
	}

	return &TestContainer{
		container: container,
		ctx:       ctx,
		t:         nil,
	}, nil
}

// buildFestBinaryShared returns the path to the fest binary without testing.T
func buildFestBinaryShared() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	// Note: lifecycle is one level deeper than integration
	festBinaryPath := filepath.Join(cwd, "../../../bin/linux", "fest")
	festBinaryPath, err = filepath.Abs(festBinaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if binary exists
	if _, err := os.Stat(festBinaryPath); err != nil {
		return "", fmt.Errorf("fest binary not found at %s - run 'just build-linux' first: %w", festBinaryPath, err)
	}

	return festBinaryPath, nil
}

// Cleanup terminates the container
func (tc *TestContainer) Cleanup() {
	if tc.container != nil {
		tc.container.Terminate(tc.ctx)
	}
}

// Reset clears container state between tests.
// The trailing `sync` ensures filesystem buffers are flushed before the next test
// begins — required for consistency on macOS/Colima where Docker exec runs through
// a virtualization layer (overlayfs in a Linux VM).
func (tc *TestContainer) Reset() error {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c", "rm -rf /test /output /festivals /workspace /testproject /outer /tmp/* 2>/dev/null; mkdir -p /test /festivals; sync",
	})
	if err != nil {
		return fmt.Errorf("failed to reset container: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("reset command failed with exit code %d", exitCode)
	}
	return nil
}

// GetSharedContainer returns the shared container, resetting state first.
func GetSharedContainer(t *testing.T) *TestContainer {
	t.Helper()
	if sharedContainer == nil {
		t.Fatal("shared container not initialized - TestMain not called?")
	}

	if err := sharedContainer.Reset(); err != nil {
		t.Fatalf("failed to reset container: %v", err)
	}

	return &TestContainer{
		container: sharedContainer.container,
		ctx:       sharedContainer.ctx,
		t:         t,
	}
}

// RunFestInDir runs fest command from a specific directory
func (tc *TestContainer) RunFestInDir(dir string, args ...string) (string, error) {
	cmd := []string{"sh", "-c", "cd " + dir + " && /fest " + strings.Join(args, " ")}
	return tc.runCommand(cmd)
}

// runCommand executes a command in the container
func (tc *TestContainer) runCommand(cmd []string) (string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	// Demultiplex Docker stream
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("failed to demultiplex output: %w", err)
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		output += stderr.String()
	}

	// Don't return error on non-zero exit - let tests check output
	if exitCode != 0 {
		return output, nil
	}

	return output, nil
}

// CheckDirExists checks if a directory exists in the container
func (tc *TestContainer) CheckDirExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-d", path})
	if err != nil {
		return false, fmt.Errorf("failed to check directory: %w", err)
	}
	return exitCode == 0, nil
}

// CheckFileExists checks if a file exists in the container
func (tc *TestContainer) CheckFileExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-f", path})
	if err != nil {
		return false, fmt.Errorf("failed to check file: %w", err)
	}
	return exitCode == 0, nil
}

// ReadFile reads a file from the container
func (tc *TestContainer) ReadFile(path string) (string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"cat", path})
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("failed to demultiplex output: %w", err)
	}

	output := stdout.String()

	if exitCode != 0 {
		return "", fmt.Errorf("cat command failed with exit code %d: %s", exitCode, output)
	}

	return output, nil
}

// ListDirectories lists directories in a path (sorted by name)
func (tc *TestContainer) ListDirectories(path string) ([]string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("ls -d %s/*/ 2>/dev/null | sort", path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list directories: %w", err)
	}

	if exitCode != 0 {
		return nil, nil // No directories found
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			line = strings.TrimSuffix(line, "/")
			dirs = append(dirs, filepath.Base(line))
		}
	}

	return dirs, nil
}

// ListDirectory lists files in a container directory recursively
func (tc *TestContainer) ListDirectory(path string) ([]string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"find", path, "-type", "f"})
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("find command failed with exit code %d", exitCode)
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return nil, fmt.Errorf("failed to demultiplex output: %w", err)
	}

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var files []string
	for _, line := range lines {
		if line != "" && line != path {
			files = append(files, line)
		}
	}

	return files, nil
}

// writeFileInContainer writes content to a file in the container
func writeFileInContainer(tc *TestContainer, path, content string) error {
	// Use base64 encoding to avoid shell escaping issues
	// First create the directory if needed
	dir := filepath.Dir(path)
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", dir})
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write using heredoc
	cmd := []string{"sh", "-c", fmt.Sprintf("cat > %s << 'EOFMARKER'\n%s\nEOFMARKER", path, content)}
	exitCode, _, err = tc.container.Exec(tc.ctx, cmd)
	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// containsAnyOf checks if the string contains any of the substrings
func containsAnyOf(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
