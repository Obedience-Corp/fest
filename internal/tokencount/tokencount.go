// Package tokencount wraps the tcount tokenizer to provide cached token counts
// for festival planning directories. Counts are cached under
// .campaign/cache/tokens/ so repeated `fest list` invocations do not re-walk
// and re-tokenize unchanged festivals.
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
// unchanged. On any error it returns 0 and nil error so list rendering never
// fails because of token counting.
func (c *Counter) CountFestival(ctx context.Context, festivalPath string) int {
	if c == nil || !c.enabled || c.counter == nil {
		return 0
	}
	fp, err := fingerprint(ctx, festivalPath)
	if err != nil {
		return 0
	}
	if entry, ok := c.loadCache(festivalPath, fp); ok {
		return entry.Tokens
	}
	info, err := os.Stat(festivalPath)
	if err != nil || !info.IsDir() {
		return 0
	}
	res, err := c.counter.CountDirectory(ctx, festivalPath, "", false)
	if err != nil || len(res.Methods) == 0 {
		return 0
	}
	primary := res.Methods[0]
	c.saveCache(festivalPath, fp, primary.Tokens, primary.Name, primary.IsExact, res.FileCount)
	return primary.Tokens
}

// CountFestivals returns a map from festival path to token count for each
// festival in festivalPaths. Festivals that cannot be counted get 0.
func (c *Counter) CountFestivals(ctx context.Context, festivalPaths []string) map[string]int {
	result := make(map[string]int, len(festivalPaths))
	for _, p := range festivalPaths {
		if err := ctx.Err(); err != nil {
			return result
		}
		result[p] = c.CountFestival(ctx, p)
	}
	return result
}

// fingerprint produces a deterministic hash of the files inside a directory:
// sorted relative path + modtime + size. This is cheap (stat-only, no reads)
// and changes when any planning file is edited, added, or removed.
func fingerprint(ctx context.Context, dir string) (string, error) {
	type fileInfo struct {
		relPath string
		modTime time.Time
		size    int64
	}
	var files []fileInfo

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
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].relPath < files[j].relPath
	})
	h := sha256.New()
	for _, f := range files {
		_, _ = fmt.Fprintf(h, "%s\t%d\t%d\n", f.relPath, f.modTime.UnixNano(), f.size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
