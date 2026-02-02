package github

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/registry"
)

// ProgressFunc is called during download to report progress
type ProgressFunc func(current, total int64, file string)

// GitHubTreeResponse represents the GitHub API tree response
type GitHubTreeResponse struct {
	SHA  string           `json:"sha"`
	Tree []GitHubTreeItem `json:"tree"`
}

// GitHubTreeItem represents a single item in the GitHub tree
type GitHubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
	SHA  string `json:"sha,omitempty"`
}

// FileInfo contains path and SHA for a file
type FileInfo struct {
	Path string
	SHA  string
}

// Downloader handles downloading from GitHub
type Downloader struct {
	repoURL  string
	branch   string
	repoPath string // Path within the repository to the festivals directory
	client   *http.Client
	timeout  int
	retry    int
}

// NewDownloader creates a new GitHub downloader
// repoPath is the path within the repository to the festivals directory (e.g., "methodology/festivals")
func NewDownloader(repoURL, branch, repoPath string) *Downloader {
	return &Downloader{
		repoURL:  repoURL,
		branch:   branch,
		repoPath: repoPath,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		timeout: 30,
		retry:   3,
	}
}

// SetTimeout sets the download timeout in seconds
func (d *Downloader) SetTimeout(seconds int) {
	d.timeout = seconds
	d.client.Timeout = time.Duration(seconds) * time.Second
}

// SetRetry sets the number of retry attempts
func (d *Downloader) SetRetry(count int) {
	d.retry = count
}

// Download downloads the festival directory structure
func (d *Downloader) Download(targetDir string, progress ProgressFunc) error {
	// Parse repository URL to get owner and repo name
	owner, repo, err := parseRepoURL(d.repoURL)
	if err != nil {
		return errors.Wrap(err, "invalid repository URL")
	}

	// Build raw content base URL
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", owner, repo, d.branch)

	// Create target directory
	if err := os.MkdirAll(targetDir, registry.DirPermissions); err != nil {
		return errors.IO("creating target directory", err).WithField("path", targetDir)
	}

	// Get file list from GitHub API
	files, err := d.getFilesFromGitHub(owner, repo)
	if err != nil {
		return errors.Wrap(err, "getting file list from GitHub")
	}

	if len(files) == 0 {
		return errors.NotFound("files in " + d.repoPath + " directory")
	}

	totalFiles := int64(len(files))
	currentFile := int64(0)

	// Download each file
	for _, file := range files {
		currentFile++
		if progress != nil {
			progress(currentFile, totalFiles, file)
		}

		fileURL := fmt.Sprintf("%s/%s/%s", baseURL, d.repoPath, file)
		targetPath := filepath.Join(targetDir, file)

		// Download with retry
		var lastErr error
		for attempt := 0; attempt < d.retry; attempt++ {
			if err := d.downloadFile(fileURL, targetPath); err != nil {
				lastErr = err
				time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			return errors.Wrap(lastErr, "downloading file after retries").WithField("file", file).WithField("attempts", d.retry)
		}
	}

	return nil
}

// downloadFile downloads a single file
func (d *Downloader) downloadFile(url, targetPath string) error {
	// Create directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, registry.DirPermissions); err != nil {
		return errors.IO("creating directory", err).WithField("path", targetDir)
	}

	// Make HTTP request
	resp, err := d.client.Get(url)
	if err != nil {
		return errors.IO("downloading from URL", err).WithField("url", url)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return errors.IO("download failed", nil).WithField("status", resp.StatusCode).WithField("url", url)
	}

	// Create target file
	out, err := os.Create(targetPath)
	if err != nil {
		return errors.IO("creating file", err).WithField("path", targetPath)
	}
	defer out.Close()

	// Copy content
	if _, err := io.Copy(out, resp.Body); err != nil {
		return errors.IO("writing file", err).WithField("path", targetPath)
	}

	return out.Sync()
}

// parseRepoURL parses a GitHub repository URL
func parseRepoURL(url string) (owner, repo string, err error) {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Remove github.com
	url = strings.TrimPrefix(url, "github.com/")

	// Split owner and repo
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", "", errors.Validation("invalid repository URL format").WithField("url", url)
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")

	return owner, repo, nil
}

// getFilesFromGitHub fetches the file list from GitHub API
func (d *Downloader) getFilesFromGitHub(owner, repo string) ([]string, error) {
	// Build API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, d.branch)

	// Make API request
	resp, err := d.client.Get(apiURL)
	if err != nil {
		return nil, errors.IO("fetching file tree from GitHub API", err).WithField("url", apiURL)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.IO("GitHub API request failed", nil).WithField("status", resp.StatusCode)
	}

	// Parse response
	var treeResp GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return nil, errors.Parse("parsing GitHub API response", err)
	}

	// Filter for files in the configured repository path
	pathPrefix := d.repoPath + "/"
	var files []string
	for _, item := range treeResp.Tree {
		// Only include files (blobs), not directories (trees)
		if item.Type != "blob" {
			continue
		}

		// Only include files under the configured path
		if strings.HasPrefix(item.Path, pathPrefix) {
			// Remove path prefix to get relative path
			relativePath := strings.TrimPrefix(item.Path, pathPrefix)
			files = append(files, relativePath)
		}
	}

	return files, nil
}

// getFilesWithSHA fetches the file list with SHA hashes from GitHub API
func (d *Downloader) getFilesWithSHA(owner, repo string) (map[string]string, error) {
	// Build API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, d.branch)

	// Make API request
	resp, err := d.client.Get(apiURL)
	if err != nil {
		return nil, errors.IO("fetching file tree from GitHub API", err).WithField("url", apiURL)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.IO("GitHub API request failed", nil).WithField("status", resp.StatusCode)
	}

	// Parse response
	var treeResp GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return nil, errors.Parse("parsing GitHub API response", err)
	}

	// Build map of path -> SHA for files in the configured repository path
	pathPrefix := d.repoPath + "/"
	files := make(map[string]string)
	for _, item := range treeResp.Tree {
		// Only include files (blobs), not directories (trees)
		if item.Type != "blob" {
			continue
		}

		// Only include files under the configured path
		if strings.HasPrefix(item.Path, pathPrefix) {
			// Remove path prefix to get relative path
			relativePath := strings.TrimPrefix(item.Path, pathPrefix)
			files[relativePath] = item.SHA
		}
	}

	return files, nil
}

// calculateGitBlobSHA calculates the Git blob SHA1 hash for a file
// Git blob SHA = SHA1("blob <size>\0<content>")
func calculateGitBlobSHA(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.IO("reading file", err).WithField("path", filePath)
	}

	// Git blob format: "blob <size>\0<content>"
	header := fmt.Sprintf("blob %d\x00", len(content))

	h := sha1.New()
	h.Write([]byte(header))
	h.Write(content)

	return hex.EncodeToString(h.Sum(nil)), nil
}

// DeleteOrphaned removes local files that don't exist in the remote repository
func (d *Downloader) DeleteOrphaned(owner, repo, targetDir string) ([]string, error) {
	// Get remote file list
	remoteFiles, err := d.getFilesWithSHA(owner, repo)
	if err != nil {
		return nil, errors.Wrap(err, "getting remote file list")
	}

	deleted := []string{}

	// Walk local directory and find files not in remote
	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil // Skip directories (will be cleaned up if empty)
		}

		// Get relative path
		relPath, err := filepath.Rel(targetDir, path)
		if err != nil {
			return nil
		}

		// Skip hidden files and checksums
		if strings.HasPrefix(filepath.Base(relPath), ".") {
			return nil
		}

		// Check if file exists in remote
		if _, exists := remoteFiles[relPath]; !exists {
			// Delete the file
			if err := os.Remove(path); err != nil {
				return nil // Skip if can't delete
			}
			deleted = append(deleted, relPath)
		}

		return nil
	})

	if err != nil {
		return deleted, errors.IO("walking local directory", err).WithField("path", targetDir)
	}

	// Clean up empty directories
	cleanEmptyDirs(targetDir)

	return deleted, nil
}

// cleanEmptyDirs removes empty directories recursively
func cleanEmptyDirs(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == root {
			return nil
		}

		// Try to remove the directory (will only succeed if empty)
		os.Remove(path)
		return nil
	})

	// Walk again in reverse to clean nested empty dirs
	// (simple approach: just run it a few times)
	for i := 0; i < 5; i++ {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() || path == root {
				return nil
			}
			os.Remove(path)
			return nil
		})
	}
}

// CheckForUpdates checks if remote repository has changes compared to local cache
func (d *Downloader) CheckForUpdates(owner, repo, targetDir string) (bool, []string, error) {
	// Get remote file list with SHAs
	remoteFiles, err := d.getFilesWithSHA(owner, repo)
	if err != nil {
		return false, nil, errors.Wrap(err, "getting remote file list")
	}

	changes := []string{}

	// Check for new or modified files
	for remotePath, remoteSHA := range remoteFiles {
		localPath := filepath.Join(targetDir, remotePath)
		info, err := os.Stat(localPath)
		if os.IsNotExist(err) {
			// File doesn't exist locally (new file)
			changes = append(changes, fmt.Sprintf("+ %s (new)", remotePath))
			continue
		}
		if err != nil {
			// Other error, skip this file
			continue
		}
		if info.IsDir() {
			// Unexpected: remote is a file but local is a directory
			continue
		}

		// File exists locally - compare SHA hashes
		localSHA, err := calculateGitBlobSHA(localPath)
		if err != nil {
			// Can't calculate local SHA, skip comparison
			continue
		}

		if localSHA != remoteSHA {
			changes = append(changes, fmt.Sprintf("~ %s (modified)", remotePath))
		}
	}

	// Check for deleted files (in local but not in remote)
	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil // Skip directories
		}

		// Get relative path
		relPath, err := filepath.Rel(targetDir, path)
		if err != nil {
			return nil
		}

		// Skip hidden files and checksums
		if strings.HasPrefix(filepath.Base(relPath), ".") {
			return nil
		}

		// Check if file exists in remote
		if _, exists := remoteFiles[relPath]; !exists {
			changes = append(changes, fmt.Sprintf("- %s (deleted)", relPath))
		}

		return nil
	})

	if err != nil {
		return false, nil, errors.IO("walking local directory", err).WithField("path", targetDir)
	}

	hasUpdates := len(changes) > 0
	return hasUpdates, changes, nil
}

// IsGitAvailable checks if the git command is available
func IsGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// SHAMarkerFile is the name of the file that stores sync state
const SHAMarkerFile = ".last-sync-sha"

// SyncState stores both commit SHA and content hash for accurate update detection
type SyncState struct {
	CommitSHA   string `json:"commit_sha"`
	ContentHash string `json:"content_hash"`
}

// GetRemoteSHA gets the current HEAD SHA of the branch using git ls-remote
// This works with private repos using SSH keys
func (d *Downloader) GetRemoteSHA() (string, error) {
	if !IsGitAvailable() {
		return "", errors.Validation("git command not found")
	}

	cmd := exec.Command("git", "ls-remote", d.repoURL, "refs/heads/"+d.branch)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.IO("getting remote SHA", err).WithField("url", d.repoURL)
	}

	// Output format: "<sha>\trefs/heads/<branch>"
	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return "", errors.NotFound("branch " + d.branch)
	}

	return parts[0], nil
}

// ReadSyncState reads the stored sync state from the last sync
func ReadSyncState(targetDir string) (*SyncState, error) {
	stateFile := filepath.Join(targetDir, SHAMarkerFile)
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No previous sync
		}
		return nil, err
	}

	// Try to parse as JSON (new format)
	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		// Fall back to old format (just commit SHA)
		state.CommitSHA = strings.TrimSpace(string(data))
		state.ContentHash = ""
	}
	return &state, nil
}

// WriteSyncState stores the sync state after a successful sync
func WriteSyncState(targetDir string, state *SyncState) error {
	stateFile := filepath.Join(targetDir, SHAMarkerFile)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

// ReadLastSyncSHA reads the stored commit SHA from the last sync (for backward compatibility)
func ReadLastSyncSHA(targetDir string) (string, error) {
	state, err := ReadSyncState(targetDir)
	if err != nil {
		return "", err
	}
	if state == nil {
		return "", nil
	}
	return state.CommitSHA, nil
}

// WriteLastSyncSHA stores the SHA after a successful sync (for backward compatibility)
func WriteLastSyncSHA(targetDir, sha string) error {
	// Read existing state to preserve content hash
	state, _ := ReadSyncState(targetDir)
	if state == nil {
		state = &SyncState{}
	}
	state.CommitSHA = sha
	return WriteSyncState(targetDir, state)
}

// ComputeContentHash computes a hash of directory contents
// This hash changes only when the actual file contents change, not when the repo has unrelated commits
func ComputeContentHash(dir string) (string, error) {
	h := sha1.New()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		// Skip hidden files (including .last-sync-sha)
		if strings.HasPrefix(filepath.Base(relPath), ".") {
			return nil
		}

		// Add path to hash (sorted by Walk)
		h.Write([]byte(relPath))
		h.Write([]byte{0})

		// Add file content hash
		fileHash, err := calculateGitBlobSHA(path)
		if err != nil {
			return nil // Skip files we can't hash
		}
		h.Write([]byte(fileHash))
		h.Write([]byte{0})

		return nil
	})

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// CheckForUpdatesWithGit checks if the remote has new commits using git ls-remote
// Returns (hasUpdates, remoteSHA, error)
// Note: This only checks commit SHA. Use CheckForTemplateUpdates for accurate detection.
func (d *Downloader) CheckForUpdatesWithGit(targetDir string) (bool, string, error) {
	remoteSHA, err := d.GetRemoteSHA()
	if err != nil {
		return false, "", err
	}

	state, err := ReadSyncState(targetDir)
	if err != nil {
		return false, "", err
	}

	// If no local state, we need to sync
	if state == nil {
		return true, remoteSHA, nil
	}

	// If commit SHA is same, definitely no updates
	if state.CommitSHA == remoteSHA {
		return false, remoteSHA, nil
	}

	// Commit SHA changed - might have template updates
	return true, remoteSHA, nil
}

// CheckForTemplateUpdates performs a full check by cloning and comparing content hashes
// Returns (hasUpdates, newState, error)
// If hasUpdates is false, the caller should just update the commit SHA without copying files
func (d *Downloader) CheckForTemplateUpdates(targetDir string, remoteSHA string) (bool, *SyncState, error) {
	// Read local state
	localState, err := ReadSyncState(targetDir)
	if err != nil {
		return false, nil, err
	}

	// If no local state, definitely need to sync
	if localState == nil || localState.ContentHash == "" {
		return true, nil, nil
	}

	// Clone to temp directory to compute content hash
	tempDir, err := os.MkdirTemp("", "fest-sync-check-*")
	if err != nil {
		return false, nil, errors.IO("creating temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone with depth=1
	cmd := exec.Command("git", "clone",
		"--depth=1",
		"--single-branch",
		"-b", d.branch,
		d.repoURL,
		tempDir,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return false, nil, errors.IO("cloning repository for update check", err).WithField("url", d.repoURL)
	}

	// Compute content hash of remote templates
	sourceDir := filepath.Join(tempDir, d.repoPath)
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return false, nil, errors.NotFound("directory " + d.repoPath + " in repository")
	}

	remoteContentHash, err := ComputeContentHash(sourceDir)
	if err != nil {
		return false, nil, errors.Wrap(err, "computing remote content hash")
	}

	newState := &SyncState{
		CommitSHA:   remoteSHA,
		ContentHash: remoteContentHash,
	}

	// Compare content hashes
	hasUpdates := localState.ContentHash != remoteContentHash
	return hasUpdates, newState, nil
}

// DownloadWithGit clones the repository using git and copies the target directory
// This method uses existing git credentials (SSH keys, credential helpers) automatically
func (d *Downloader) DownloadWithGit(targetDir string, progress ProgressFunc) error {
	if !IsGitAvailable() {
		return errors.Validation("git command not found")
	}

	// Create temp directory for clone
	tempDir, err := os.MkdirTemp("", "fest-sync-*")
	if err != nil {
		return errors.IO("creating temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	if progress != nil {
		progress(1, 3, "Cloning repository...")
	}

	// Clone with depth=1 for speed
	cmd := exec.Command("git", "clone",
		"--depth=1",
		"--single-branch",
		"-b", d.branch,
		d.repoURL,
		tempDir,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return errors.IO("cloning repository", err).WithField("url", d.repoURL)
	}

	if progress != nil {
		progress(2, 3, "Copying files...")
	}

	// Source directory within the clone
	sourceDir := filepath.Join(tempDir, d.repoPath)

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return errors.NotFound("directory " + d.repoPath + " in repository")
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, registry.DirPermissions); err != nil {
		return errors.IO("creating target directory", err).WithField("path", targetDir)
	}

	// Clear target directory (except protected hidden files) before copying
	// This ensures deleted files from source are also deleted from target
	if err := clearTargetDir(targetDir); err != nil {
		return errors.IO("clearing target directory", err).WithField("path", targetDir)
	}

	// Copy files from source to target
	if err := copyDir(sourceDir, targetDir); err != nil {
		return errors.Wrap(err, "copying files")
	}

	if progress != nil {
		progress(3, 3, "Done")
	}

	return nil
}

// protectedFiles are state files that should never be deleted during sync.
var protectedFiles = map[string]bool{
	".fest-checksums.json": true,
	".workspace":           true,
	"id_registry.yaml":     true,
}

// clearTargetDir removes all files from target except protected state files.
// This ensures the target directory exactly mirrors the source after copying.
// It handles the .festival directory specially - clearing its contents while
// preserving protected state files.
func clearTargetDir(targetDir string) error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to clear
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(targetDir, entry.Name())

		// Handle .festival directory specially - clear its contents
		if entry.Name() == ".festival" && entry.IsDir() {
			if err := clearFestivalDir(path); err != nil {
				return err
			}
			continue
		}

		// Skip other hidden files (not .festival)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// clearFestivalDir clears the .festival directory contents while preserving
// protected state files like .fest-checksums.json and .workspace.
func clearFestivalDir(festivalDir string) error {
	entries, err := os.ReadDir(festivalDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip protected state files
		if protectedFiles[entry.Name()] {
			continue
		}
		path := filepath.Join(festivalDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// copyDir recursively copies a directory tree
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip .git directory
		if strings.HasPrefix(relPath, ".git") || relPath == ".git" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		return copyFile(path, targetPath)
	})
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Get source file info for permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Create destination file
	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
