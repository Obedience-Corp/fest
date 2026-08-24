package runloop

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// Record is one append-only row in fest-run.jsonl.
type Record struct {
	SchemaVersion string `json:"schema_version"`
	Timestamp     string `json:"timestamp"`
	Iteration     int    `json:"iteration"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason,omitempty"`
	Label         string `json:"label,omitempty"`
	Path          string `json:"path,omitempty"`
	Commit        string `json:"commit,omitempty"`
	Exec          string `json:"exec,omitempty"`
	Error         string `json:"error,omitempty"`
}

const ledgerSchema = "fest.run.record/v1"

// AppendLedger writes one record to path, creating parent dirs.
func AppendLedger(ctx context.Context, path string, rec Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if rec.SchemaVersion == "" {
		rec.SchemaVersion = ledgerSchema
	}
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errors.IO("creating fest-run ledger dir", err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return errors.Parse("encoding fest-run record", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.IO("opening fest-run ledger", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return errors.IO("writing fest-run ledger", err)
	}
	return nil
}

// ReadLedger returns records in order. A missing file is an empty run.
func ReadLedger(ctx context.Context, path string) ([]Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.IO("reading fest-run ledger", err)
	}
	defer func() { _ = f.Close() }()
	var out []Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, errors.Parse("parsing fest-run ledger", err)
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, errors.IO("scanning fest-run ledger", err)
	}
	return out, nil
}

// TaskCount counts successful drive iterations in a ledger.
func TaskCount(recs []Record) int {
	n := 0
	for _, rec := range recs {
		if rec.Outcome == "ok" {
			n++
		}
	}
	return n
}
