package logx_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/imlukasd/Bastion/internal/platform/logx"
)

// AD-21, E35: one JSON object per record, with the service on every record.
func TestNew_WritesJSON(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo, "pay-api")
	log.Debug("below the level")
	log.Info("intent confirmed", "intent_id", "pi_1", "attempt", 3)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (Debug dropped): %q", len(lines), buf.String())
	}
	dec := json.NewDecoder(strings.NewReader(lines[0]))
	dec.UseNumber()
	var rec map[string]any
	if err := dec.Decode(&rec); err != nil {
		t.Fatalf("not JSON: %v: %q", err, lines[0])
	}
	want := map[string]any{
		"level":     "INFO",
		"msg":       "intent confirmed",
		"service":   "pay-api",
		"intent_id": "pi_1",
		"attempt":   json.Number("3"),
	}
	for k, v := range want {
		if rec[k] != v {
			t.Errorf("%s = %#v, want %#v", k, rec[k], v)
		}
	}
	if ts, ok := rec["time"].(string); !ok || ts == "" {
		t.Errorf("time = %#v, want a timestamp", rec["time"])
	}
}
