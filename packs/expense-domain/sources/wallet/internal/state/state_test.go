package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Has("x", 0) {
		t.Fatalf("expected empty state")
	}
}

func TestSaveAndReload(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s.Mark("m1", "rec1", "2026-07-01", 100.50)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	s2, err := Load(p)
	if err != nil {
		t.Fatalf("Load2: %v", err)
	}
	if !s2.HasMessageID("m1") {
		t.Fatalf("expected m1 to be present")
	}
	if s2.Pushed["m1"].RecordID != "rec1" {
		t.Fatalf("expected recordId rec1, got %q", s2.Pushed["m1"].RecordID)
	}
	if s2.Pushed["m1"].Amount != 100.50 {
		t.Fatalf("expected amount 100.50, got %v", s2.Pushed["m1"].Amount)
	}
	// Test Has with matching amount
	if !s2.Has("m1", 100.50) {
		t.Fatalf("expected Has to return true for matching amount")
	}
	// Test Has with different amount
	if s2.Has("m1", 200.00) {
		t.Fatalf("expected Has to return false for different amount")
	}
}
