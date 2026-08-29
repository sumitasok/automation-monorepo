package main

import (
	"testing"
	"time"

	"github.com/sumitasok/sa.automation.wallet/internal/wallet"
)

func TestMergeRecords_NewIDsAppended(t *testing.T) {
	existing := []wallet.Record{{"id": "a", "note": "old-a"}}
	incoming := []wallet.Record{{"id": "b", "note": "new-b"}}
	merged := mergeRecords(existing, incoming)
	if len(merged) != 2 {
		t.Fatalf("expected 2 records, got %d: %+v", len(merged), merged)
	}
	if merged[0]["id"] != "a" || merged[1]["id"] != "b" {
		t.Fatalf("unexpected order: %+v", merged)
	}
}

func TestMergeRecords_ExistingIDReplacedByIncoming(t *testing.T) {
	existing := []wallet.Record{{"id": "a", "note": "stale"}, {"id": "b", "note": "unchanged"}}
	incoming := []wallet.Record{{"id": "a", "note": "fresh"}}
	merged := mergeRecords(existing, incoming)
	if len(merged) != 2 {
		t.Fatalf("expected 2 records (a replaced, b kept), got %d: %+v", len(merged), merged)
	}
	if merged[0]["id"] != "a" || merged[0]["note"] != "fresh" {
		t.Fatalf("expected a's note to be replaced with the fresher copy, got %+v", merged[0])
	}
	if merged[1]["id"] != "b" || merged[1]["note"] != "unchanged" {
		t.Fatalf("expected b untouched, got %+v", merged[1])
	}
}

func TestMergeRecords_RecordWithoutIDIsDropped(t *testing.T) {
	existing := []wallet.Record{{"note": "no id here"}}
	incoming := []wallet.Record{{"id": "a", "note": "has id"}}
	merged := mergeRecords(existing, incoming)
	if len(merged) != 1 || merged[0]["id"] != "a" {
		t.Fatalf("expected only the id-bearing record to survive, got %+v", merged)
	}
}

func TestMergeRecords_EmptyBothSides(t *testing.T) {
	merged := mergeRecords(nil, nil)
	if len(merged) != 0 {
		t.Fatalf("expected empty merge, got %+v", merged)
	}
}

func TestMaxUpdatedAt_PicksLatest(t *testing.T) {
	records := []wallet.Record{
		{"id": "a", "updatedAt": "2026-08-01T00:00:00Z"},
		{"id": "b", "updatedAt": "2026-08-20T12:30:00Z"},
		{"id": "c", "updatedAt": "2026-08-10T00:00:00Z"},
	}
	got, ok := maxUpdatedAt(records)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	want := time.Date(2026, 8, 20, 12, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestMaxUpdatedAt_IgnoresMissingOrUnparseable(t *testing.T) {
	records := []wallet.Record{
		{"id": "a"},                                 // no updatedAt at all
		{"id": "b", "updatedAt": 12345},             // wrong type
		{"id": "c", "updatedAt": "not-a-timestamp"}, // unparseable
		{"id": "d", "updatedAt": "2026-08-05T00:00:00Z"},
	}
	got, ok := maxUpdatedAt(records)
	if !ok {
		t.Fatalf("expected ok=true (record d has a valid timestamp)")
	}
	want := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestMaxUpdatedAt_EmptyOrAllInvalid(t *testing.T) {
	if _, ok := maxUpdatedAt(nil); ok {
		t.Fatalf("expected ok=false for no records")
	}
	if _, ok := maxUpdatedAt([]wallet.Record{{"id": "a", "note": "no timestamp field"}}); ok {
		t.Fatalf("expected ok=false when no record has a parseable updatedAt")
	}
}
