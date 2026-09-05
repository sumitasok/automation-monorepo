package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRecords_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/records" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("expected limit=200, got %q", got)
		}
		if got := r.URL.Query().Get("offset"); got != "0" {
			t.Fatalf("expected offset=0, got %q", got)
		}
		if got := r.URL.Query().Get("withTotal"); got != "true" {
			t.Fatalf("expected withTotal=true on the first page, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"records": []map[string]any{
				{"id": "r1", "note": "first"},
				{"id": "r2", "note": "second"},
			},
			"total": 2,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	records, total, err := c.GetRecords("", "")
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if total != 2 {
		t.Fatalf("expected apiTotal 2, got %d", total)
	}
	if records[0]["id"] != "r1" || records[1]["id"] != "r2" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func TestGetRecords_Pagination(t *testing.T) {
	var offsetsSeen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("offset")
		offsetsSeen = append(offsetsSeen, offset)
		w.Header().Set("Content-Type", "application/json")
		switch offset {
		case "0":
			if got := r.URL.Query().Get("withTotal"); got != "true" {
				t.Fatalf("expected withTotal=true on the first page, got %q", got)
			}
			next := 200
			json.NewEncoder(w).Encode(map[string]any{
				"records":    []map[string]any{{"id": "r1"}},
				"nextOffset": &next,
				"total":      2,
			})
		case "200":
			if got := r.URL.Query().Get("withTotal"); got != "" {
				t.Fatalf("expected withTotal to be omitted on later pages, got %q", got)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{{"id": "r2"}},
				"total":   0,
			})
		default:
			t.Fatalf("unexpected offset %q", offset)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	records, total, err := c.GetRecords("", "")
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records across pages, got %d", len(records))
	}
	if total != 2 {
		t.Fatalf("expected apiTotal 2, got %d", total)
	}
	if len(offsetsSeen) != 2 {
		t.Fatalf("expected 2 requests, got %d: %v", len(offsetsSeen), offsetsSeen)
	}
}

func TestGetRecords_DefaultsToFarPastFloor(t *testing.T) {
	// No recordDateFrom given: GetRecords must still send an explicit
	// recordDate=gte.<floor> rather than omitting the filter — omitting it
	// was observed to make the live API apply an undocumented ~90-day
	// implicit lookback (ADR 0020 correction, 2026-08-29).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("recordDate"); got != "gte."+farPastFloor {
			t.Fatalf("expected recordDate=gte.%s, got %q", farPastFloor, got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, _, err := c.GetRecords("", ""); err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
}

func TestGetRecords_ExplicitSinceFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("recordDate"); got != "gte.2026-08-01" {
			t.Fatalf("expected recordDate=gte.2026-08-01, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, _, err := c.GetRecords("2026-08-01", ""); err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
}

func TestGetRecords_UpdatedAtFilter_KeepsRecordDateWideOpen(t *testing.T) {
	// An incremental (updatedAtFrom-only) call must still pin recordDate to
	// farPastFloor rather than leaving it empty — the API's default 3-month
	// window is tied to recordDate specifically, so an old record that was
	// only just re-categorized (recent updatedAt, old recordDate) would
	// otherwise fall outside the implicit window and be missed even though
	// updatedAt matches.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("recordDate"); got != "gte."+farPastFloor {
			t.Fatalf("expected recordDate=gte.%s, got %q", farPastFloor, got)
		}
		if got := r.URL.Query().Get("updatedAt"); got != "gte.2026-08-20T00:00:00Z" {
			t.Fatalf("expected updatedAt=gte.2026-08-20T00:00:00Z, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, _, err := c.GetRecords("", "2026-08-20T00:00:00Z"); err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
}

func TestGetRecords_NoUpdatedAtFilter_OmitsParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["updatedAt"]; ok {
			t.Fatalf("expected no updatedAt param, got %q", r.URL.Query().Get("updatedAt"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, _, err := c.GetRecords("", ""); err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
}

func TestGetRecords_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	if _, _, err := c.GetRecords("", ""); err == nil {
		t.Fatalf("expected error on 500 response")
	}
}
