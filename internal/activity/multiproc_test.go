//go:build container_fs && (unix || windows)

package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	multiprocRoleEnv       = "FEST_ACTIVITY_MULTIPROC_ROLE"
	multiprocDirEnv        = "FEST_ACTIVITY_MULTIPROC_DIR"
	multiprocWriterEnv     = "FEST_ACTIVITY_MULTIPROC_WRITER"
	multiprocThresholdEnv  = "FEST_ACTIVITY_ROTATION_THRESHOLD"
	multiprocTestThreshold = int64(8192)
)

// TestMultiprocessConcurrentEmitAndRotation drives separate OS processes that
// append activity events through the real lock/unlock path (not in-process
// goroutines). A rotation boundary is reached while writers are active.
func TestMultiprocessConcurrentEmitAndRotation(t *testing.T) {
	if os.Getenv(multiprocRoleEnv) != "" {
		runMultiprocChild(t)
		return
	}

	prevThreshold := fileRotationThreshold
	fileRotationThreshold = multiprocTestThreshold
	t.Cleanup(func() { fileRotationThreshold = prevThreshold })

	lockRoot := t.TempDir()
	t.Setenv(activityPathLockDirEnv, lockRoot)

	_, festivalPath := setupCampaign(t, "multiproc-campaign")
	activityPath := filepath.Join(festivalPath, ".fest", FestivalFileName)

	seedLines := appendValidJSONLUntilSize(t, activityPath, multiprocTestThreshold)
	if seedLines < 1 {
		t.Fatal("expected at least one seed JSONL line before concurrent writers")
	}

	const writers = 6
	const eventsPerWriter = 25

	errCh := make(chan error, writers)
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			errCh <- execMultiprocChild(t, "emit", festivalPath, writerID, eventsPerWriter)
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("child writer failed: %v", err)
		}
	}

	allEvents := readAllActivityJSONLEvents(t, festivalPath)
	writerEvents := filterWriterEvents(allEvents)
	if len(writerEvents) != writers*eventsPerWriter {
		t.Fatalf("expected %d writer events across all logs, got %d", writers*eventsPerWriter, len(writerEvents))
	}
	assertUniqueWriterSeq(t, writerEvents, writers, eventsPerWriter)

	if !rotationArchiveExists(t, festivalPath) {
		t.Fatal("expected at least one rotated archive after concurrent writes crossed the threshold")
	}
	if siblingLock := activityPath + ".lock"; fileExists(siblingLock) {
		t.Fatalf("must not leave workspace lock sibling %s", siblingLock)
	}
	for _, ev := range allEvents {
		if ev["event"] == "pad" {
			continue
		}
		data, _ := ev["data"].(map[string]any)
		if data == nil {
			continue
		}
		if _, ok := data["writer"]; ok {
			marker, _ := data["marker"].(string)
			if marker != "concurrent" {
				t.Fatalf("writer event missing concurrent marker: %v", ev)
			}
		}
	}

	if err := execMultiprocChild(t, "cancel", festivalPath, 0, 0); err != nil {
		t.Fatal(err)
	}
	t.Logf("validated %d concurrent writer events across %d parseable JSONL records with a rotation archive", len(writerEvents), len(allEvents))
}

func execMultiprocChild(t *testing.T, role, festivalPath string, writerID, events int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMultiprocessConcurrentEmitAndRotation$", "-test.count=1")
	cmd.Env = append(os.Environ(),
		multiprocRoleEnv+"="+role,
		multiprocDirEnv+"="+festivalPath,
		multiprocWriterEnv+"="+strconv.Itoa(writerID),
		"FEST_ACTIVITY_MULTIPROC_EVENTS="+strconv.Itoa(events),
		multiprocThresholdEnv+"="+strconv.FormatInt(multiprocTestThreshold, 10),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("multiproc child role=%s writer=%d: %w\n%s", role, writerID, err, out)
	}
	return nil
}

func runMultiprocChild(t *testing.T) {
	t.Helper()
	role := os.Getenv(multiprocRoleEnv)
	festivalPath := os.Getenv(multiprocDirEnv)
	writerID, _ := strconv.Atoi(os.Getenv(multiprocWriterEnv))
	events, _ := strconv.Atoi(os.Getenv("FEST_ACTIVITY_MULTIPROC_EVENTS"))

	switch role {
	case "emit":
		if events < 1 {
			fmt.Fprintf(os.Stderr, "invalid FEST_ACTIVITY_MULTIPROC_EVENTS=%q\n", os.Getenv("FEST_ACTIVITY_MULTIPROC_EVENTS"))
			os.Exit(5)
		}
		e := NewFromFestival(context.Background(), festivalPath, func(error) {})
		for i := 0; i < events; i++ {
			e.Emit(context.Background(), "task.completed", Scope{Task: "01_task.md"},
				"fest task completed",
				WithData(map[string]any{"writer": writerID, "seq": i, "marker": "concurrent"}))
		}
	case "cancel":
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var warned bool
		e := NewFromFestival(context.Background(), festivalPath, func(error) { warned = true })
		e.Emit(ctx, "task.created", Scope{}, "fest create task --name setup", nil)
		if !warned {
			os.Exit(3)
		}
	default:
		os.Exit(4)
	}
	os.Exit(0)
}

// appendValidJSONLUntilSize appends compact JSON lines until the file reaches
// minSize. Every byte in the file must remain valid JSONL.
func appendValidJSONLUntilSize(t *testing.T, path string, minSize int64) int {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing valid JSONL seed file: %v", err)
		}
	}()

	n := 0
	for {
		info, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() >= minSize {
			break
		}
		line := fmt.Sprintf(`{"v":1,"event":"pad","result":{"ok":true},"n":%d}`, n) + "\n"
		if _, err := f.WriteString(line); err != nil {
			t.Fatal(err)
		}
		n++
	}
	return n
}

func readAllActivityJSONLEvents(t *testing.T, festivalPath string) []map[string]any {
	t.Helper()
	festDir := filepath.Join(festivalPath, ".fest")
	var paths []string
	paths = append(paths, filepath.Join(festDir, FestivalFileName))
	entries, err := os.ReadDir(festDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, "activity.") && strings.HasSuffix(name, ".jsonl") && name != FestivalFileName {
			paths = append(paths, filepath.Join(festDir, name))
		}
	}
	sort.Strings(paths)

	var all []map[string]any
	for _, path := range paths {
		all = append(all, parseJSONLLinesStrict(t, path)...)
	}
	return all
}

func parseJSONLLinesStrict(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	content := string(data)
	if !strings.HasSuffix(content, "\n") {
		t.Fatalf("%s: JSONL must end with a newline", path)
	}
	lines := strings.Split(content, "\n")
	var events []map[string]any
	for i, line := range lines {
		if line == "" {
			if i == len(lines)-1 {
				continue
			}
			t.Fatalf("%s: empty line at index %d", path, i)
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("invalid JSONL line in %s: %v\n%s", path, err, line)
		}
		events = append(events, ev)
	}
	return events
}

func filterWriterEvents(events []map[string]any) []map[string]any {
	var out []map[string]any
	for _, ev := range events {
		data, _ := ev["data"].(map[string]any)
		if data == nil {
			continue
		}
		if _, ok := data["writer"]; ok {
			out = append(out, ev)
		}
	}
	return out
}

func assertUniqueWriterSeq(t *testing.T, events []map[string]any, writers, perWriter int) {
	t.Helper()
	type key struct{ writer, seq int }
	seen := make(map[key]struct{}, writers*perWriter)
	for _, ev := range events {
		data, ok := ev["data"].(map[string]any)
		if !ok {
			t.Fatalf("writer event missing data map: %v", ev)
		}
		wf, ok := data["writer"].(float64)
		if !ok {
			t.Fatalf("writer id must be numeric: %v", ev)
		}
		sf, ok := data["seq"].(float64)
		if !ok {
			t.Fatalf("seq must be numeric: %v", ev)
		}
		k := key{writer: int(wf), seq: int(sf)}
		if _, dup := seen[k]; dup {
			t.Fatalf("duplicate writer event: writer=%d seq=%d", k.writer, k.seq)
		}
		seen[k] = struct{}{}
	}
	for w := 0; w < writers; w++ {
		for s := 0; s < perWriter; s++ {
			if _, ok := seen[key{w, s}]; !ok {
				t.Fatalf("missing event writer=%d seq=%d", w, s)
			}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func rotationArchiveExists(t *testing.T, festivalPath string) bool {
	t.Helper()
	festDir := filepath.Join(festivalPath, ".fest")
	entries, err := os.ReadDir(festDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, "activity.") && strings.HasSuffix(name, ".jsonl") {
			return true
		}
	}
	return false
}
