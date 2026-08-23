package runloop

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAppendAndReadLedger(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "runs", "r1", "fest-run.jsonl")
	if err := AppendLedger(ctx, path, Record{Iteration: 1, Outcome: "ok", Label: "ALIGN"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendLedger(ctx, path, Record{Iteration: 2, Outcome: OutcomeWaitingHuman, Reason: "human gate"}); err != nil {
		t.Fatal(err)
	}
	recs, err := ReadLedger(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("len = %d", len(recs))
	}
	if recs[0].SchemaVersion != ledgerSchema {
		t.Fatalf("schema = %s", recs[0].SchemaVersion)
	}
	if TaskCount(recs) != 1 {
		t.Fatalf("task count = %d", TaskCount(recs))
	}
}

func TestReadLedgerMissing(t *testing.T) {
	recs, err := ReadLedger(context.Background(), filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if recs != nil && len(recs) != 0 {
		t.Fatalf("want empty, got %v", recs)
	}
}
