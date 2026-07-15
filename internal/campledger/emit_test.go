package campledger

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"
)

func TestNewFromFestival_NoCampaignIsNoop(t *testing.T) {
	// Temp dir with no .campaign marker.
	root := t.TempDir()
	fest := filepath.Join(root, "my-fest")
	if err := os.MkdirAll(fest, 0o755); err != nil {
		t.Fatal(err)
	}
	var warned error
	e := NewFromFestival(context.Background(), fest, func(err error) { warned = err })
	if !e.disabled {
		t.Fatal("expected disabled emitter outside campaign")
	}
	// Emit must not panic or write.
	e.Emit(context.Background(), ledgerkit.KindCompleted, FestivalScope(fest, "p/s/t.md"))
	if warned != nil {
		t.Fatalf("standalone no-op should be silent, got warn %v", warned)
	}
}

func TestNewFromFestival_EmitsIntoCampaignLedger(t *testing.T) {
	camp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(camp, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-id\nname: test\n"
	if err := os.WriteFile(filepath.Join(camp, ".campaign", "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	fest := filepath.Join(camp, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(fest, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pin writer id so we do not depend on global config mutation in unit tests.
	// ResolveWriterID may still touch ~/.obey; if it fails we skip.
	e := NewFromFestival(context.Background(), fest, func(error) {})
	if e.disabled {
		t.Skip("writer id resolution disabled emitter in this environment")
	}
	e.Emit(context.Background(), ledgerkit.KindCompleted, FestivalScope(fest, "003_IMPLEMENT/01_seq/01_task.md"),
		WithWhy("done"))

	// Find a shard file under .campaign/events
	eventsRoot := filepath.Join(camp, ".campaign", "events")
	var found string
	_ = filepath.Walk(eventsRoot, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(path, ".jsonl") {
			found = path
		}
		return nil
	})
	if found == "" {
		t.Fatal("expected ledger shard to be written")
	}
	data, err := os.ReadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"kind":"completed"`) {
		t.Fatalf("shard missing completed event: %s", data)
	}
	if !strings.Contains(string(data), "DF0001") && !strings.Contains(string(data), "demo-fest") {
		t.Fatalf("shard missing festival scope: %s", data)
	}
	if !strings.Contains(string(data), "test-campaign-id") {
		t.Fatalf("shard missing campaign id: %s", data)
	}
}

func TestFestivalScopeParsesTaskPath(t *testing.T) {
	s := FestivalScope("/camp/festivals/active/x-CA0002", "003_IMPLEMENT/05_views/02_graph_ingestion.md")
	if s.Festival != "CA0002" {
		t.Fatalf("festival=%q", s.Festival)
	}
	if s.Phase != "003_IMPLEMENT" || s.Sequence != "05_views" || s.Task != "02_graph_ingestion.md" {
		t.Fatalf("scope=%+v", s)
	}
}

func TestWarnMsgMatchesCampWording(t *testing.T) {
	if !strings.Contains(warnMsg, "campaign ledger not updated") {
		t.Fatalf("warn wording drift: %q", warnMsg)
	}
}
