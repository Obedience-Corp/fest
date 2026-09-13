package festival

import (
	"bytes"
	"context"
	"errors"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/pkg/festgif"
)

func TestEmbedMarkdownPreservesDocument(t *testing.T) {
	original := "---\nfest_type: festival\ncustom: keep\n---\n# My overview\n\nUser content without a final newline"
	first, managed, err := embedMarkdown([]byte(original))
	if err != nil || managed || !bytes.HasPrefix(first, []byte(original)) {
		t.Fatalf("first embed: managed=%v err=%v document=%s", managed, err, first)
	}
	first = append(first, []byte("\n## Notes after replay\nKeep these too.\n")...)
	second, managed, err := embedMarkdown(first)
	if err != nil || !managed || !bytes.Equal(first, second) {
		t.Fatalf("repeat changed document: managed=%v err=%v document=%s", managed, err, second)
	}
	for _, malformed := range []string{replayStart, replayEnd, replayEnd + replayStart, replayBlock + replayBlock} {
		if _, _, err := embedMarkdown([]byte(malformed)); err == nil {
			t.Errorf("accepted malformed markers: %q", malformed)
		}
	}
}

// Filesystem tests must run in a container, along with the command tests.
func TestEmbedCreatesAndRefreshesReplay(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No event log and no overview: older festivals still get a final-state GIF.
	result, size, err := Embed(t.Context(), dir, 1)
	if err != nil || result.Frames == 0 || size == 0 {
		t.Fatalf("Embed: result=%+v size=%d err=%v", result, size, err)
	}
	path := filepath.Join(dir, ReplayFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gif.DecodeAll(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	overview := filepath.Join(dir, OverviewFilename)
	first, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}
	first = append(first, []byte("\n## My notes\nKeep this.\n")...)
	if err := os.WriteFile(overview, first, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(overview, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Embed(t.Context(), dir, 2); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(overview)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || strings.Count(string(second), "![Festival execution replay]") != 1 {
		t.Fatalf("overview changed unexpectedly: %s", second)
	}
	info, err := os.Stat(overview)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("mode changed: %v %v", info, err)
	}
}

func TestEmbedFailurePreservesFiles(t *testing.T) {
	for _, cause := range []string{"cancelled", "collision", "bad-markers", "symlink", "render-failure"} {
		t.Run(cause, func(t *testing.T) {
			dir := t.TempDir()
			overview := filepath.Join(dir, OverviewFilename)
			original := []byte("# Overview\nKeep me.\n")
			if cause == "bad-markers" {
				original = append(original, []byte(replayStart)...)
			}
			if cause == "render-failure" {
				original = append(original, []byte(replayBlock)...)
			}
			if err := os.WriteFile(overview, original, 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, ReplayFilename)
			if cause == "collision" {
				if err := os.WriteFile(out, []byte("user asset"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if cause == "symlink" {
				if err := os.Rename(overview, overview+".source"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(overview+".source", overview); err != nil {
					t.Fatal(err)
				}
			}
			if cause == "render-failure" {
				if err := os.Mkdir(out, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if cause == "cancelled" {
				cancel()
			}
			_, _, err := Embed(ctx, dir, 1)
			if err == nil {
				t.Fatal("expected failure")
			}
			if cause == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(overview)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatalf("overview changed: %s (%v)", after, err)
			}
			if cause == "collision" {
				data, err := os.ReadFile(out)
				if err != nil || string(data) != "user asset" {
					t.Fatalf("asset changed: %s (%v)", data, err)
				}
			}
			leftovers, _ := filepath.Glob(filepath.Join(dir, ".fest-*.tmp"))
			if len(leftovers) != 0 {
				t.Fatalf("temporary files left: %v", leftovers)
			}
		})
	}
}

func TestWriteGIFCancelledPreservesExistingAsset(t *testing.T) {
	out := filepath.Join(t.TempDir(), "existing.gif")
	if err := os.WriteFile(out, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err := WriteGIF(ctx, out, festgif.Plan(festgif.Input{}, festgif.DefaultTiming))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "original" {
		t.Fatalf("asset changed: %s (%v)", data, err)
	}
}
