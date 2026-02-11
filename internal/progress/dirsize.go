package progress

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
)

// ComputeDirectorySize walks a directory and sums the sizes of all .md files.
// This provides a proxy for content volume (approximately 4 bytes per token).
func ComputeDirectorySize(ctx context.Context, dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// ContentDelta represents the change in content size for a festival.
type ContentDelta struct {
	InitialBytes int64 `json:"initial_bytes"`
	CurrentBytes int64 `json:"current_bytes"`
	DeltaBytes   int64 `json:"delta_bytes"`
	DeltaTokens  int64 `json:"delta_tokens"` // Approximate: 4 bytes per token
}

// ComputeContentDelta calculates the content delta for a festival directory.
func ComputeContentDelta(ctx context.Context, festivalPath string, initialBytes int64) (*ContentDelta, error) {
	currentBytes, err := ComputeDirectorySize(ctx, festivalPath)
	if err != nil {
		return nil, err
	}

	delta := currentBytes - initialBytes
	return &ContentDelta{
		InitialBytes: initialBytes,
		CurrentBytes: currentBytes,
		DeltaBytes:   delta,
		DeltaTokens:  delta / 4,
	}, nil
}
