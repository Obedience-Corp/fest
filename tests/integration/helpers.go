//go:build integration
// +build integration

package integration

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
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestContainer wraps container operations for testing
type TestContainer struct {
	container testcontainers.Container
	ctx       context.Context
	t         *testing.T
}

const (
	integrationGitName  = "Fest Integration Tests"
	integrationGitEmail = "fest-integration@example.invalid"
)

// integrationGitEnv gives fixture repositories deterministic identity and
// transport defaults without reading or writing ~/.gitconfig. Keeping the
// settings in the container environment prevents host Git configuration from
// leaking in either direction while covering every git process spawned by
// fest or camp.
func integrationGitEnv() map[string]string {
	return map[string]string{
		"GIT_AUTHOR_NAME":     integrationGitName,
		"GIT_AUTHOR_EMAIL":    integrationGitEmail,
		"GIT_COMMITTER_NAME":  integrationGitName,
		"GIT_COMMITTER_EMAIL": integrationGitEmail,
		"GIT_CONFIG_COUNT":    "2",
		"GIT_CONFIG_KEY_0":    "init.defaultBranch",
		"GIT_CONFIG_VALUE_0":  "main",
		"GIT_CONFIG_KEY_1":    "protocol.file.allow",
		"GIT_CONFIG_VALUE_1":  "always",
	}
}

// NewTestContainer creates a new Alpine container for testing fest
func NewTestContainer(t *testing.T) (*TestContainer, error) {
	ctx := context.Background()

	// Get the absolute path to the Linux binary
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	festBinaryPath := filepath.Join(cwd, "../../bin/linux", "fest")
	festBinaryPath, err = filepath.Abs(festBinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get path to templates (used by fest init)
	templatesPath := filepath.Join(cwd, "../../methodology/festivals/.festival")
	templatesPath, err = filepath.Abs(templatesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get templates path: %w", err)
	}

	// Create Linux binary directory if it doesn't exist
	linuxBinDir := filepath.Dir(festBinaryPath)
	if err := os.MkdirAll(linuxBinDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create linux bin directory: %w", err)
	}

	// Check if binary exists
	if _, err := os.Stat(festBinaryPath); err != nil {
		return nil, fmt.Errorf("fest binary not found at %s: %w - run 'just build test-binary' first", festBinaryPath, err)
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Cmd:        []string{"sleep", "3600"}, // Keep container running
		Env:        integrationGitEnv(),
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(30 * time.Second),
		AutoRemove: true,
		Mounts: testcontainers.ContainerMounts{
			{
				Source:   testcontainers.GenericBindMountSource{HostPath: festBinaryPath},
				Target:   "/fest",
				ReadOnly: false,
			},
			{
				// Mount templates so fest init can use them without network
				Source:   testcontainers.GenericBindMountSource{HostPath: templatesPath},
				Target:   "/root/.obey/fest/festivals/.festival",
				ReadOnly: true,
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

	// Make fest executable in container
	exitCode, output, err := container.Exec(ctx, []string{"chmod", "+x", "/fest"})
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to make fest executable: %w", err)
	}
	if exitCode != 0 {
		outputBytes, _ := io.ReadAll(output)
		container.Terminate(ctx)
		return nil, fmt.Errorf("chmod failed with exit code %d, output: %s", exitCode, string(outputBytes))
	}

	return &TestContainer{
		container: container,
		ctx:       ctx,
		t:         t,
	}, nil
}

// RunFest executes the fest command in the container
func (tc *TestContainer) RunFest(args ...string) (string, error) {
	cmd := append([]string{"/fest"}, args...)

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute fest: %w", err)
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

	if exitCode != 0 {
		return output, fmt.Errorf("fest exited with code %d: %s", exitCode, output)
	}

	return output, nil
}

// RunFestInDir runs fest command from a specific directory
func (tc *TestContainer) RunFestInDir(dir string, args ...string) (string, error) {
	// Use sh -c to change directory and run fest
	cmd := []string{"sh", "-c", "cd " + dir + " && /fest " + strings.Join(args, " ")}

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute fest in dir: %w", err)
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

	// Return error on non-zero exit (like RunFest does)
	if exitCode != 0 {
		return output, fmt.Errorf("fest exited with code %d: %s", exitCode, output)
	}

	return output, nil
}

// RunFestTTY executes fest command in the container with a pseudo-TTY.
func (tc *TestContainer) RunFestTTY(args ...string) (string, error) {
	cmd := append([]string{"/fest"}, args...)

	options := []tcexec.ProcessOption{
		tcexec.ProcessOptionFunc(func(opts *tcexec.ProcessOptions) {
			opts.ExecConfig.TTY = true
			opts.ExecConfig.AttachStdin = true
		}),
	}

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd, options...)
	if err != nil {
		return "", fmt.Errorf("failed to execute fest in TTY: %w", err)
	}

	outputBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read TTY output: %w", err)
	}
	output := string(outputBytes)

	if exitCode != 0 {
		return output, fmt.Errorf("fest exited with code %d: %s", exitCode, output)
	}

	return output, nil
}

// RunFestInDirTTY runs fest command from a specific directory with a pseudo-TTY.
func (tc *TestContainer) RunFestInDirTTY(dir string, args ...string) (string, error) {
	cmd := []string{"sh", "-c", "cd " + dir + " && /fest " + strings.Join(args, " ")}

	options := []tcexec.ProcessOption{
		tcexec.ProcessOptionFunc(func(opts *tcexec.ProcessOptions) {
			opts.ExecConfig.TTY = true
			opts.ExecConfig.AttachStdin = true
		}),
	}

	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd, options...)
	if err != nil {
		return "", fmt.Errorf("failed to execute fest in dir with TTY: %w", err)
	}

	outputBytes, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read TTY output: %w", err)
	}
	output := string(outputBytes)

	if exitCode != 0 {
		return output, fmt.Errorf("fest exited with code %d: %s", exitCode, output)
	}

	return output, nil
}

// Exec runs an arbitrary shell command inside the container via `sh -c`.
// Returns combined stdout+stderr and any execution error. Non-zero exit
// codes are surfaced via the error so tests can assert on them.
func (tc *TestContainer) Exec(cmd ...string) (string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("failed to read command output: %w", err)
	}
	output := stdout.String()
	if stderr.Len() > 0 {
		output += stderr.String()
	}

	if exitCode != 0 {
		return output, fmt.Errorf("command exited with code %d: %s", exitCode, output)
	}
	return output, nil
}

// WriteFile writes content into a path inside the container, creating
// any missing parent directories.
func (tc *TestContainer) WriteFile(path, content string) error {
	parent := filepath.Dir(path)
	if _, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", parent}); err != nil {
		return fmt.Errorf("mkdir -p %s: %w", parent, err)
	}
	// Use a temp host file and CopyToContainer to avoid shell-escaping content.
	f, err := os.CreateTemp("", "fest-helper-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return tc.CopyToContainer(f.Name(), path)
}

// runCommand executes a command in the container
// Returns output even on non-zero exit for test debugging
func (tc *TestContainer) runCommand(cmd []string) (string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	// Demultiplex Docker stream (removes the \x01 header bytes)
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", fmt.Errorf("failed to demultiplex output: %w", err)
	}

	// Combine stdout and stderr
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

// CopyToContainer copies a file to the container
func (tc *TestContainer) CopyToContainer(sourcePath, targetPath string) error {
	fileContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	return tc.container.CopyToContainer(
		tc.ctx,
		fileContent,
		targetPath,
		0644,
	)
}

// CopyDirToContainer copies a directory to the container
func (tc *TestContainer) CopyDirToContainer(sourceDir, targetDir string) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetDir, relPath)

		if info.IsDir() {
			// Create directory in container
			exitCode, _, err := tc.container.Exec(tc.ctx, []string{"mkdir", "-p", targetPath})
			if err != nil || exitCode != 0 {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			return nil
		}

		// Copy file
		return tc.CopyToContainer(path, targetPath)
	})
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

	// Demultiplex Docker stream
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

// ReadFile reads a file from the container
func (tc *TestContainer) ReadFile(path string) (string, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"cat", path})
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Demultiplex Docker stream
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

// CheckFileExists checks if a file exists in the container
func (tc *TestContainer) CheckFileExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-f", path})
	if err != nil {
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return exitCode == 0, nil
}

// CheckDirExists checks if a directory exists in the container
func (tc *TestContainer) CheckDirExists(path string) (bool, error) {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{"test", "-d", path})
	if err != nil {
		return false, fmt.Errorf("failed to check directory: %w", err)
	}

	return exitCode == 0, nil
}

// FileSystemState captures the state of a directory tree
type FileSystemState struct {
	Directories []string          // directory paths
	Files       map[string]string // path -> content
}

// CaptureState captures the complete filesystem state in the container
func (tc *TestContainer) CaptureState(rootPath string) (*FileSystemState, error) {
	state := &FileSystemState{
		Files:       make(map[string]string),
		Directories: []string{},
	}

	// Find all directories
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"find", rootPath, "-type", "d"})
	if err != nil {
		return nil, fmt.Errorf("failed to find directories: %w", err)
	}

	if exitCode == 0 {
		output, _ := io.ReadAll(reader)
		for _, dir := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if dir != "" && dir != rootPath {
				relPath, _ := filepath.Rel(rootPath, dir)
				if relPath != "." && relPath != "" {
					state.Directories = append(state.Directories, relPath)
				}
			}
		}
	}

	// Find all files and read their content
	files, err := tc.ListDirectory(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		content, err := tc.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		relPath, _ := filepath.Rel(rootPath, file)
		state.Files[relPath] = content
	}

	return state, nil
}

// CountPhases counts the number of phases in a festival directory
func (tc *TestContainer) CountPhases(festivalPath string) (int, error) {
	// First check what directories exist
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("find %s -maxdepth 1 -type d -name '[0-9][0-9][0-9]_*' | wc -l", festivalPath),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count phases: %w", err)
	}

	if exitCode != 0 {
		return 0, nil // No phases found
	}

	output, _ := io.ReadAll(reader)
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// CountSequences counts the number of sequences in a phase
func (tc *TestContainer) CountSequences(phasePath string) (int, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("ls -d %s/[0-9][0-9]_* 2>/dev/null | wc -l", phasePath),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count sequences: %w", err)
	}

	if exitCode != 0 {
		return 0, nil // No sequences found
	}

	output, _ := io.ReadAll(reader)
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// CountTasks counts the number of tasks in a sequence
func (tc *TestContainer) CountTasks(sequencePath string) (int, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("ls %s/[0-9][0-9]_*.md 2>/dev/null | wc -l", sequencePath),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	if exitCode != 0 {
		return 0, nil // No tasks found
	}

	output, _ := io.ReadAll(reader)
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// VerifyParallelItems checks that parallel tasks/sequences with the same number exist
func (tc *TestContainer) VerifyParallelItems(path string, prefix string) (int, error) {
	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c",
		fmt.Sprintf("ls %s/%s* 2>/dev/null | wc -l", path, prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to verify parallel items: %w", err)
	}

	if exitCode != 0 {
		return 0, nil // No items found
	}

	output, _ := io.ReadAll(reader)
	count := 0
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, nil
}

// VerifyStructure validates the directory structure is correct
func (tc *TestContainer) VerifyStructure(festivalPath string) error {
	// Check if festival directory exists
	exists, err := tc.CheckDirExists(festivalPath)
	if err != nil {
		return fmt.Errorf("failed to check festival directory: %w", err)
	}
	if !exists {
		return fmt.Errorf("festival directory does not exist: %s", festivalPath)
	}

	// Get all phases
	phaseCount, err := tc.CountPhases(festivalPath)
	if err != nil {
		return fmt.Errorf("failed to count phases: %w", err)
	}

	if phaseCount == 0 {
		// Valid - empty festival
		return nil
	}

	// Check that phases are sequentially numbered
	for i := 1; i <= phaseCount; i++ {
		phasePattern := fmt.Sprintf("%s/%03d_*", festivalPath, i)
		exitCode, _, err := tc.container.Exec(tc.ctx, []string{
			"sh", "-c",
			fmt.Sprintf("ls -d %s 2>/dev/null | head -1", phasePattern),
		})
		if err != nil || exitCode != 0 {
			return fmt.Errorf("phase %03d not found or not sequential", i)
		}
	}

	return nil
}

// Cleanup terminates the container
func (tc *TestContainer) Cleanup() {
	if tc.container != nil {
		tc.container.Terminate(tc.ctx)
	}
}

// NewSharedContainer creates a test container for shared use across tests.
// Unlike NewTestContainer, it doesn't require a *testing.T during creation.
func NewSharedContainer() (*TestContainer, error) {
	ctx := context.Background()

	festBinary, err := buildFestBinaryShared()
	if err != nil {
		return nil, fmt.Errorf("failed to build fest binary: %w", err)
	}

	// Get path to templates (used by fest init)
	templatesPath, err := buildTemplatesPathShared()
	if err != nil {
		return nil, fmt.Errorf("failed to get templates path: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:      "alpine:latest",
		Cmd:        []string{"sleep", "3600"},
		Env:        integrationGitEnv(),
		WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(30 * time.Second),
		AutoRemove: true,
		Mounts: testcontainers.ContainerMounts{
			{
				Source:   testcontainers.GenericBindMountSource{HostPath: festBinary},
				Target:   "/fest",
				ReadOnly: false,
			},
			{
				// Mount templates so fest init can use them without network
				Source:   testcontainers.GenericBindMountSource{HostPath: templatesPath},
				Target:   "/root/.obey/fest/festivals/.festival",
				ReadOnly: true,
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
	festBinaryPath := filepath.Join(cwd, "../../bin/linux", "fest")
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

// buildTemplatesPathShared returns the path to the templates directory without testing.T
func buildTemplatesPathShared() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	templatesPath := filepath.Join(cwd, "../../methodology/festivals/.festival")
	templatesPath, err = filepath.Abs(templatesPath)
	if err != nil {
		return "", fmt.Errorf("failed to get templates path: %w", err)
	}

	// Check if templates exist
	if _, err := os.Stat(templatesPath); err != nil {
		return "", fmt.Errorf("templates not found at %s: %w", templatesPath, err)
	}

	return templatesPath, nil
}

// Reset clears container state between tests.
// This removes all test artifacts while keeping the container and binary intact.
// The trailing `sync` ensures filesystem buffers are flushed before the next test
// begins — required for consistency on macOS/Colima where Docker exec runs through
// a virtualization layer (overlayfs in a Linux VM).
func (tc *TestContainer) Reset() error {
	exitCode, _, err := tc.container.Exec(tc.ctx, []string{
		"sh", "-c", "rm -rf /test /output /festivals /workspace /testproject /outer /tmp/* /repair-* /sysupdate-* 2>/dev/null; mkdir -p /test; sync",
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
// This should be called at the start of each test to ensure clean state.
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
			// Remove trailing slash and get base name
			line = strings.TrimSuffix(line, "/")
			dirs = append(dirs, filepath.Base(line))
		}
	}

	return dirs, nil
}

// ValidateSnapshot compares actual state with expected state
func ValidateSnapshot(t *testing.T, actual, expected *FileSystemState) {
	// Check directories
	require.ElementsMatch(t, expected.Directories, actual.Directories, "directory structure mismatch")

	// Check files
	require.Equal(t, len(expected.Files), len(actual.Files), "file count mismatch")

	for path, expectedContent := range expected.Files {
		actualContent, exists := actual.Files[path]
		require.True(t, exists, "missing file: %s", path)
		require.Equal(t, expectedContent, actualContent, "content mismatch in file: %s", path)
	}
}

// CreateFestivalGoalFile creates a basic FESTIVAL_GOAL.md content
func CreateFestivalGoalFile() string {
	return `---
id: COMPLEX_TEST_FESTIVAL
---

# Complex Test Festival

## Goal

Test the fest CLI with a complex, realistic festival structure including:

- Multiple implementation phases
- Parallel sequences and tasks
- Deep nesting (3 levels)
- Renumbering operations
- Phase/sequence removal

## Success Criteria

- All fest commands work correctly in isolation
- Renumbering handles parallel items correctly
- Structure remains valid after operations
`
}

// CreatePhaseGoalFile creates a PHASE_GOAL.md for a given phase
func CreatePhaseGoalFile(phaseName string) string {
	return fmt.Sprintf(`---
id: %s_PHASE
---

# %s Phase

## Objective

Complete all %s tasks and sequences.

## Sequences

Multiple sequences with parallel execution paths.
`, strings.ToUpper(phaseName), phaseName, strings.ToLower(phaseName))
}

// CreateTaskFile creates a task markdown file
func CreateTaskFile(taskName string) string {
	return fmt.Sprintf(`---
id: %s
---

# %s

## Task Description

This is a test task for %s.

## Implementation

- Step 1
- Step 2
- Step 3

## Verification

- Test passes
- Code reviewed
`, strings.ToUpper(taskName), taskName, taskName)
}

// setupWorkspace creates a minimal workspace structure for testing.
// We create the structure manually instead of using fest init because
// fest init requires network access for auto-sync which isn't available in containers.
//
// Note: This creates a festivals/ directory UNDER the given path.
// So setupWorkspace(t, tc, "/") creates /festivals/.
//
// Returns the path to the festivals directory.
func setupWorkspace(t *testing.T, tc *TestContainer, basePath string) string {
	return setupWorkspaceFixture(t, tc, basePath, true)
}

// setupWorkspaceWithoutTemplates creates the deliberately incomplete fixture
// used by the missing-core-template regression tests.
func setupWorkspaceWithoutTemplates(t *testing.T, tc *TestContainer, basePath string) string {
	return setupWorkspaceFixture(t, tc, basePath, false)
}

func setupWorkspaceFixture(t *testing.T, tc *TestContainer, basePath string, seedTemplates bool) string {
	t.Helper()

	festivalsPath := filepath.Join(basePath, "festivals")

	// Create minimal workspace structure
	_, err := tc.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf("mkdir -p %s/.festival/.state %s/active %s/planning",
			festivalsPath, festivalsPath, festivalsPath),
	})
	require.NoError(t, err, "should create workspace directories")

	// Create .workspace marker file to register as workspace (JSON format expected by workspace.ReadMarker)
	markerPath := filepath.Join(festivalsPath, ".festival", ".state", ".workspace")
	markerContent := `{"workspace": "` + filepath.Base(basePath) + `", "registered": "2024-01-01T00:00:00Z"}`
	cmd := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", markerPath, markerContent)
	_, err = tc.runCommand([]string{"sh", "-c", cmd})
	require.NoError(t, err, "should create workspace marker")

	if seedTemplates {
		coreDir := filepath.Join(festivalsPath, ".festival", "templates", "festival")
		_, err = tc.runCommand([]string{"mkdir", "-p", coreDir})
		require.NoError(t, err, "should create core festival template directory")
		for name, content := range map[string]string{
			"OVERVIEW.md": "# Festival Overview\n\nIntegration fixture overview.\n",
			"GOAL.md":     "# Festival Goal\n\nIntegration fixture goal.\n",
			"RULES.md":    "# Festival Rules\n\n- Exercise the real CLI.\n",
			"TODO.md":     "# Festival TODO\n\n- [ ] Exercise the workflow.\n",
		} {
			require.NoError(t, tc.WriteFile(filepath.Join(coreDir, name), content),
				"should write marker-free core template %s", name)
		}
	}

	return festivalsPath
}
