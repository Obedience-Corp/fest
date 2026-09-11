// Package tokencount wraps the tcount tokenizer to provide cached token counts
// for festival planning directories. Counts are cached under
// .campaign/cache/tokens/ so repeated `fest list` invocations do not re-walk
// and re-tokenize unchanged festivals.
//
// Token counts are decoration on list output, so this package is bounded on
// both axes that can make tokenizing expensive: a size guard keeps model
// weights, datasets, and other large proof artifacts out of the tokenizer, and
// a wall-clock budget caps how long one list invocation may spend counting.
// Either bound yields a count of 0, which renders as no token annotation.
package tokencount

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lancekrogers/tcount/tokenizer"
)

// cacheSubdir is the path under .campaign/ where token count caches live.
const cacheSubdir = "cache/tokens"

const (
	// maxFileBytes is the largest single file a festival may contain and still
	// be tokenized. Festival directories hold planning documents; a file this
	// large is a proof artifact (model weights, a dataset, a capture) whose
	// token count is meaningless and whose tokenization costs minutes of CPU
	// and gigabytes of resident memory.
	maxFileBytes int64 = 2 << 20 // 2 MiB

	// maxFestivalBytes is the largest total size of a festival directory that
	// will be tokenized. It catches directories built from many moderate files
	// that individually pass maxFileBytes.
	maxFestivalBytes int64 = 16 << 20 // 16 MiB

	// tokenCountBudget bounds the wall-clock time all token counting may
	// consume for one list invocation. Festivals left over when the budget is
	// gone are reported as 0.
	tokenCountBudget = 10 * time.Second

	// perFestivalBudget bounds one festival's share of tokenCountBudget so a
	// single expensive directory cannot starve the rest of the list.
	perFestivalBudget = 3 * time.Second
)

// CacheEntry is the on-disk cache record for one festival directory.
type CacheEntry struct {
	Tokens      int       `json:"tokens"`
	Method      string    `json:"method"`
	IsExact     bool      `json:"is_exact"`
	FileCount   int       `json:"file_count"`
	Fingerprint string    `json:"fingerprint"`
	CountedAt   time.Time `json:"counted_at"`
}

// Counter wraps a tcount tokenizer.Counter with a file-based cache.
// A zero-value Counter is usable; CountFestival returns 0 with no error when
// the campaign root is empty, so callers in list paths never fail on tokens.
type Counter struct {
	counter  *tokenizer.Counter
	cacheDir string
	enabled  bool
}

// NewCounter initializes a Counter for the given campaign root. When
// campaignRoot is empty the Counter is disabled (CountFestival returns 0).
func NewCounter(ctx context.Context, campaignRoot string) (*Counter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if campaignRoot == "" {
		return &Counter{enabled: false}, nil
	}
	tc, err := tokenizer.NewCounter(tokenizer.CounterOptions{})
	if err != nil {
		return nil, err
	}
	cacheDir := filepath.Join(campaignRoot, ".campaign", cacheSubdir)
	return &Counter{
		counter:  tc,
		cacheDir: cacheDir,
		enabled:  true,
	}, nil
}

// CountFestival returns the primary-method token count for the festival at
// festivalPath. It uses the on-disk cache when the directory fingerprint is
// unchanged, and skips tokenizing entirely when the directory exceeds the size
// guard. On any error, on a guard trip, or on a cancelled context it returns 0
// and nil error so list rendering never fails because of token counting.
func (c *Counter) CountFestival(ctx context.Context, festivalPath string) int {
	if c == nil || !c.enabled || c.counter == nil {
		return 0
	}
	scan, err := scanDir(ctx, festivalPath)
	if err != nil {
		return 0
	}
	if entry, ok := c.loadCache(festivalPath, scan.fingerprint); ok {
		return entry.Tokens
	}
	if !withinSizeLimits(scan) {
		return 0
	}
	info, err := os.Stat(festivalPath)
	if err != nil || !info.IsDir() {
		return 0
	}
	res, err := c.counter.CountDirectoryWithOptions(ctx, festivalPath, tokenizer.CountDirectoryOptions{MaxFileSize: maxFileBytes})
	if err != nil || len(res.Methods) == 0 {
		return 0
	}
	primary := res.Methods[0]
	c.saveCache(festivalPath, scan.fingerprint, primary.Tokens, primary.Name, primary.IsExact, res.FileCount)
	return primary.Tokens
}

// CountFestivals returns a map from festival path to token count for each
// festival in festivalPaths. The whole call is bounded by tokenCountBudget and
// each festival by perFestivalBudget. Festivals that cannot be counted, that
// exceed a budget, or that trip the size guard get 0.
func (c *Counter) CountFestivals(ctx context.Context, festivalPaths []string) map[string]int {
	result := make(map[string]int, len(festivalPaths))
	if c == nil || !c.enabled || c.counter == nil {
		for _, p := range festivalPaths {
			result[p] = 0
		}
		return result
	}
	budgetCtx, cancel := context.WithTimeout(ctx, tokenCountBudget)
	defer cancel()
	for _, p := range festivalPaths {
		if budgetCtx.Err() != nil {
			// Budget spent or caller cancelled: the rest render without counts.
			result[p] = 0
			continue
		}
		result[p] = c.countFestivalBounded(budgetCtx, p)
	}
	return result
}

// countFestivalBounded counts one festival under its own deadline so a single
// slow directory cannot consume the whole list budget.
func (c *Counter) countFestivalBounded(ctx context.Context, festivalPath string) int {
	festCtx, cancel := context.WithTimeout(ctx, perFestivalBudget)
	defer cancel()
	return c.CountFestival(festCtx, festivalPath)
}

// dirScan is the result of one stat-only walk of a festival directory: a
// fingerprint that detects content changes, plus the size facts the tokenizing
// guard needs.
type dirScan struct {
	fingerprint string
	totalBytes  int64
	largestFile int64
}

// withinSizeLimits reports whether a festival is small enough to tokenize.
//
// maxFileBytes is also handed to tcount as CountDirectoryOptions.MaxFileSize,
// so oversized files are skipped inside the walk. This guard stays in fest as
// the aggregate ceiling and as defense in depth: a single multi-hundred-MB
// file that slips past detection is read and tokenized in one uninterruptible
// step, and no deadline can cut that short.
//
// The scan behind this guard applies no .gitignore rules, so an ignored
// artifact can still suppress a festival's count. That is deliberate: a
// missing decoration costs nothing, and a hung `fest list` costs the session.
func withinSizeLimits(scan dirScan) bool {
	return scan.largestFile <= maxFileBytes && scan.totalBytes <= maxFestivalBytes
}

// scanDir walks a directory once, stat-only, producing a deterministic
// fingerprint (sorted relative path + modtime + size) and the size facts used
// by withinSizeLimits. The walk reads no file contents, so it stays cheap even
// for directories the tokenizer must refuse.
func scanDir(ctx context.Context, dir string) (dirScan, error) {
	type fileInfo struct {
		relPath string
		modTime time.Time
		size    int64
	}
	var files []fileInfo
	var scan dirScan

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, fileInfo{relPath: rel, modTime: info.ModTime(), size: info.Size()})
		size := effectiveSize(path, info)
		scan.totalBytes += size
		if size > scan.largestFile {
			scan.largestFile = size
		}
		return nil
	})
	if err != nil {
		return dirScan{}, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].relPath < files[j].relPath
	})
	h := sha256.New()
	for _, f := range files {
		_, _ = fmt.Fprintf(h, "%s\t%d\t%d\n", f.relPath, f.modTime.UnixNano(), f.size)
	}
	scan.fingerprint = hex.EncodeToString(h.Sum(nil))
	return scan, nil
}

// effectiveSize returns the number of bytes the tokenizer would read for an
// entry. Symlinks are resolved because filepath.Walk reports the link's own
// size while a reader follows it to the target; links to directories and
// broken links contribute nothing.
func effectiveSize(path string, info os.FileInfo) int64 {
	if info.Mode().IsRegular() {
		return info.Size()
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return 0
	}
	target, err := os.Stat(path)
	if err != nil || !target.Mode().IsRegular() {
		return 0
	}
	return target.Size()
}

// FormatCompact renders a token count in a compact human-readable form:
// 0-999 stays as-is, 1000-9999 shows one decimal (e.g. 1.2k), and larger
// values round to the nearest k (e.g. 12k, 130k). This matches the visual
// density of the list output where it appears next to a progress percentage.
func FormatCompact(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", (n+500)/1000)
}

// cachePath returns the on-disk path for a festival's cache entry.
func (c *Counter) cachePath(festivalPath string) string {
	h := sha256.Sum256([]byte(festivalPath))
	return filepath.Join(c.cacheDir, hex.EncodeToString(h[:])+".json")
}

// loadCache returns the cached entry if the fingerprint matches.
func (c *Counter) loadCache(festivalPath, fp string) (CacheEntry, bool) {
	data, err := os.ReadFile(c.cachePath(festivalPath))
	if err != nil {
		return CacheEntry{}, false
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return CacheEntry{}, false
	}
	if entry.Fingerprint != fp {
		return CacheEntry{}, false
	}
	return entry, true
}

// saveCache writes the cache entry, creating the cache directory if needed.
// Failures are silently ignored: a missing cache only means the next run
// recounts, which is correct behavior.
func (c *Counter) saveCache(festivalPath, fp string, tokens int, method string, isExact bool, fileCount int) {
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return
	}
	entry := CacheEntry{
		Tokens:      tokens,
		Method:      method,
		IsExact:     isExact,
		FileCount:   fileCount,
		Fingerprint: fp,
		CountedAt:   time.Now().UTC(),
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(c.cachePath(festivalPath), data, 0o644)
}
