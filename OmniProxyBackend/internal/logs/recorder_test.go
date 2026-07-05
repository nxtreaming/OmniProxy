package logs

import (
	"strings"
	"testing"
	"time"
)

func TestRecorderCapsSortsAndSanitizesEntries(t *testing.T) {
	recorder := NewRecorder(2)

	recorder.Add(Entry{Path: "/v1?api_key=first-secret", Message: "old"})
	second := recorder.Add(Entry{
		Time:    time.Unix(2, 0),
		Path:    "/v1?token=second-secret",
		Message: "Authorization: Bearer abcdefghijklmnop",
	})
	third := recorder.Add(Entry{Time: time.Unix(3, 0), Message: "new"})

	entries := recorder.List()
	if len(entries) != 2 {
		t.Fatalf("expected capped recorder to keep 2 entries, got %#v", entries)
	}
	if entries[0].ID != third.ID || entries[1].ID != second.ID {
		t.Fatalf("expected newest entries in descending id order, got %#v", entries)
	}
	if strings.Contains(entries[1].Path, "second-secret") || !strings.Contains(entries[1].Path, "token=***") {
		t.Fatalf("expected sensitive query value to be redacted, got %q", entries[1].Path)
	}
	if strings.Contains(entries[1].Message, "abcdefghijklmnop") || !strings.Contains(entries[1].Message, "Bearer ***") {
		t.Fatalf("expected bearer token to be redacted, got %q", entries[1].Message)
	}
}

func TestRecorderUsesDefaultCapacityAndTimestamp(t *testing.T) {
	recorder := NewRecorder(0)
	entry := recorder.Add(Entry{Message: "ready"})

	if entry.ID != 1 {
		t.Fatalf("expected first entry id 1, got %d", entry.ID)
	}
	if entry.Time.IsZero() {
		t.Fatal("expected recorder to set timestamp")
	}
	if len(recorder.List()) != 1 {
		t.Fatalf("expected one entry, got %#v", recorder.List())
	}
}
