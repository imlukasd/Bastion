package ids_test

import (
	"errors"
	"testing"

	"github.com/imlukasd/Bastion/internal/platform/ids"
)

// AD-14: internal IDs are UUIDv7 (RFC 9562: version nibble 7, variant 10)
// and sort in creation order.
func TestNew_IsVersion7(t *testing.T) {
	prev := ids.New()
	for range 1_000 {
		id := ids.New()
		if v := id[6] >> 4; v != 7 {
			t.Fatalf("%s: version %d, want 7", id, v)
		}
		if variant := id[8] >> 6; variant != 0b10 {
			t.Fatalf("%s: variant %02b, want 10", id, variant)
		}
		if id.Compare(prev) <= 0 {
			t.Fatalf("%s does not sort after %s", id, prev)
		}
		prev = id
	}
}

func TestParse(t *testing.T) {
	id := ids.New()
	got, err := ids.Parse(id.String())
	if err != nil || got != id {
		t.Fatalf("Parse(%q) = %s, %v; want %s", id.String(), got, err, id)
	}
	const valid = "01a1bf4d-d0e5-75d8-90e9-fe04dd4685c9"
	if _, err := ids.Parse(valid); err != nil {
		t.Fatalf("Parse(%q): %v", valid, err)
	}
	bad := map[string]string{
		"version 4":     "f81d4fae-7dec-41d0-a765-00a0c91e6bf6",
		"wrong variant": "01a1bf4d-d0e5-75d8-c0e9-fe04dd4685c9",
		"braces":        "{" + valid + "}",
		"urn":           "urn:uuid:" + valid,
		"32 hex":        "01a1bf4dd0e575d890e9fe04dd4685c9",
		"35 characters": valid[:35],
		"37 characters": valid + "0",
		"not hex":       "01a1bf4d-d0e5-75d8-90e9-fe04dd4685cz",
		"empty":         "",
	}
	for name, s := range bad {
		if _, err := ids.Parse(s); !errors.Is(err, ids.ErrInvalid) {
			t.Errorf("%s: Parse(%q) error = %v, want ErrInvalid", name, s, err)
		}
	}
}
