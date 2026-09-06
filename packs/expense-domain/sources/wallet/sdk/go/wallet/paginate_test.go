package wallet

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// pageServer serves a fixed number of items in pages of the given size,
// emitting nextOffset until the items run out.
func pageServer(t *testing.T, key string, total, pageSize int) *mockServer {
	t.Helper()
	return newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			_, _ = fmt.Sscanf(v, "%d", &offset)
		}
		end := offset + pageSize
		if end > total {
			end = total
		}
		items := ""
		for i := offset; i < end; i++ {
			if items != "" {
				items += ","
			}
			items += fmt.Sprintf(`{"id":"item-%d"}`, i)
		}
		next := ""
		if end < total {
			next = fmt.Sprintf(`,"nextOffset":%d`, end)
		}
		writeMock(w, mockResponse{Body: fmt.Sprintf(
			`{"limit":%d,"offset":%d%s,"%s":[%s]}`, pageSize, offset, next, key, items)})
	})
}

func TestAllRecordsWalksEveryPage(t *testing.T) {
	m := pageServer(t, "records", 25, 10)
	c := newTestClient(t, m)
	var seen []string
	if err := c.AllRecords(testCtx(t), &ListRecordsParams{Limit: Int(10)}, func(r Record) bool {
		seen = append(seen, r.ID)
		return true
	}); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if len(seen) != 25 {
		t.Fatalf("saw %d records, want 25", len(seen))
	}
	if seen[0] != "item-0" || seen[24] != "item-24" {
		t.Errorf("walk order = %q … %q", seen[0], seen[24])
	}
	if m.Count() != 3 {
		t.Errorf("requests = %d, want 3 pages", m.Count())
	}
	// Each page must carry the advancing offset while keeping the other params.
	// The walk sets offset explicitly from the first page on, so page one
	// sends offset=0 rather than omitting it.
	offsets := []string{"0", "10", "20"}
	for i, req := range m.Requests() {
		if got := req.Query.Get("offset"); got != offsets[i] {
			t.Errorf("request %d offset = %q, want %q", i, got, offsets[i])
		}
		if got := req.Query.Get("limit"); got != "10" {
			t.Errorf("request %d lost the limit: %q", i, got)
		}
	}
}

func TestAllRecordsStopsWhenYieldReturnsFalse(t *testing.T) {
	m := pageServer(t, "records", 100, 10)
	c := newTestClient(t, m)
	count := 0
	if err := c.AllRecords(testCtx(t), nil, func(Record) bool {
		count++
		return count < 15
	}); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if count != 15 {
		t.Errorf("yielded %d records, want it to stop at 15", count)
	}
	if m.Count() != 2 {
		t.Errorf("requests = %d, want the walk to stop after the second page", m.Count())
	}
}

func TestWalkStopsOnASinglePage(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"limit":30,"offset":0,"records":[{"id":"r1"}]}`})
	c := newTestClient(t, m)
	var seen int
	if err := c.AllRecords(testCtx(t), nil, func(Record) bool { seen++; return true }); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if seen != 1 || m.Count() != 1 {
		t.Errorf("seen=%d requests=%d, want 1 and 1 when nextOffset is absent", seen, m.Count())
	}
}

func TestWalkStopsOnAnEmptyResult(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"limit":30,"offset":0,"records":[]}`})
	c := newTestClient(t, m)
	seen := 0
	if err := c.AllRecords(testCtx(t), nil, func(Record) bool { seen++; return true }); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if seen != 0 || m.Count() != 1 {
		t.Errorf("seen=%d requests=%d, want 0 and 1", seen, m.Count())
	}
}

func TestWalkRefusesToLoopOnANonAdvancingNextOffset(t *testing.T) {
	// A server that keeps answering with the same offset would spin forever.
	m := newMockServer(t, mockResponse{Body: `{"limit":10,"offset":0,"nextOffset":0,"records":[{"id":"r1"}]}`})
	c := newTestClient(t, m)
	seen := 0
	if err := c.AllRecords(testCtx(t), nil, func(Record) bool { seen++; return true }); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if m.Count() != 1 {
		t.Errorf("requests = %d, want the walk to stop rather than loop", m.Count())
	}
	if seen != 1 {
		t.Errorf("seen = %d, want the one record from the single page", seen)
	}
}

func TestWalkStopsOnAnEmptyPageThatStillAdvertisesMore(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"limit":10,"offset":0,"nextOffset":10,"records":[]}`})
	c := newTestClient(t, m)
	if err := c.AllRecords(testCtx(t), nil, func(Record) bool { return true }); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if m.Count() != 1 {
		t.Errorf("requests = %d, want the walk to stop on an empty page", m.Count())
	}
}

func TestWalkPropagatesAnErrorMidWalk(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 0 {
			writeMock(w, mockResponse{Body: `{"limit":1,"offset":0,"nextOffset":1,"records":[{"id":"r1"}]}`})
			return
		}
		writeMock(w, mockResponse{Status: 500, Body: `{"error":"boom"}`})
	})
	c := newTestClient(t, m)
	seen := 0
	err := c.AllRecords(testCtx(t), nil, func(Record) bool { seen++; return true })
	mustAPIError(t, err, KindServer)
	if seen != 1 {
		t.Errorf("seen = %d, want the records from the first page before the failure", seen)
	}
}

func TestWalkHonoursContextCancellation(t *testing.T) {
	m := pageServer(t, "records", 1000, 1)
	c := newTestClient(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	seen := 0
	err := c.AllRecords(ctx, nil, func(Record) bool {
		seen++
		if seen == 3 {
			cancel()
		}
		return true
	})
	if err == nil {
		t.Fatal("expected the walk to end with the cancellation error")
	}
	if seen < 3 {
		t.Errorf("seen = %d, want at least the three records yielded before cancelling", seen)
	}
}

func TestWalkDoesNotMutateTheCallersParams(t *testing.T) {
	m := pageServer(t, "records", 20, 10)
	c := newTestClient(t, m)
	params := &ListRecordsParams{Limit: Int(10), Offset: Int(0)}
	if err := c.AllRecords(testCtx(t), params, func(Record) bool { return true }); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if params.Offset == nil || *params.Offset != 0 {
		t.Errorf("the caller's Offset was mutated to %v; the walk must work on a copy", params.Offset)
	}
}

func TestWalkStartsFromTheGivenOffset(t *testing.T) {
	m := pageServer(t, "records", 25, 10)
	c := newTestClient(t, m)
	var seen []string
	if err := c.AllRecords(testCtx(t), &ListRecordsParams{Limit: Int(10), Offset: Int(20)}, func(r Record) bool {
		seen = append(seen, r.ID)
		return true
	}); err != nil {
		t.Fatalf("AllRecords: %v", err)
	}
	if len(seen) != 5 || seen[0] != "item-20" {
		t.Errorf("seen = %v, want the tail from offset 20", seen)
	}
}

func TestEveryListEndpointHasAWalker(t *testing.T) {
	tests := []struct {
		name string
		key  string
		walk func(*Client, *int) error
	}{
		{"accounts", "accounts", func(c *Client, n *int) error {
			return c.AllAccounts(context.Background(), nil, func(Account) bool { *n++; return true })
		}},
		{"budgets", "budgets", func(c *Client, n *int) error {
			return c.AllBudgets(context.Background(), nil, func(Budget) bool { *n++; return true })
		}},
		{"categories", "categories", func(c *Client, n *int) error {
			return c.AllCategories(context.Background(), nil, func(Category) bool { *n++; return true })
		}},
		{"labels", "labels", func(c *Client, n *int) error {
			return c.AllLabels(context.Background(), nil, func(Label) bool { *n++; return true })
		}},
		{"goals", "goals", func(c *Client, n *int) error {
			return c.AllGoals(context.Background(), nil, func(Goal) bool { *n++; return true })
		}},
		{"recordRules", "recordRules", func(c *Client, n *int) error {
			return c.AllRecordRules(context.Background(), nil, func(RecordRule) bool { *n++; return true })
		}},
		{"standingOrders", "standingOrders", func(c *Client, n *int) error {
			return c.AllStandingOrders(context.Background(), nil, func(StandingOrder) bool { *n++; return true })
		}},
		{"standingOrderItems", "standingOrderItems", func(c *Client, n *int) error {
			return c.AllStandingOrderItems(context.Background(), nil, func(StandingOrderItem) bool { *n++; return true })
		}},
		{"records", "records", func(c *Client, n *int) error {
			return c.AllRecords(context.Background(), nil, func(Record) bool { *n++; return true })
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := pageServer(t, tc.key, 7, 3)
			c := newTestClient(t, m)
			seen := 0
			if err := tc.walk(c, &seen); err != nil {
				t.Fatalf("walk: %v", err)
			}
			if seen != 7 {
				t.Errorf("seen = %d, want 7 across 3 pages", seen)
			}
			if m.Count() != 3 {
				t.Errorf("requests = %d, want 3", m.Count())
			}
		})
	}
}

func TestCollectRecordsRespectsTheLimit(t *testing.T) {
	m := pageServer(t, "records", 50, 10)
	c := newTestClient(t, m)
	got, err := c.CollectRecords(testCtx(t), nil, 12)
	if err != nil {
		t.Fatalf("CollectRecords: %v", err)
	}
	if len(got) != 12 {
		t.Errorf("collected %d, want 12", len(got))
	}
}

func TestCollectRecordsUnlimited(t *testing.T) {
	m := pageServer(t, "records", 23, 10)
	c := newTestClient(t, m)
	got, err := c.CollectRecords(testCtx(t), nil, 0)
	if err != nil {
		t.Fatalf("CollectRecords: %v", err)
	}
	if len(got) != 23 {
		t.Errorf("collected %d, want all 23", len(got))
	}
}

func TestCollectAccounts(t *testing.T) {
	m := pageServer(t, "accounts", 5, 2)
	c := newTestClient(t, m)
	got, err := c.CollectAccounts(testCtx(t), nil, 0)
	if err != nil {
		t.Fatalf("CollectAccounts: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("collected %d, want 5", len(got))
	}
}

func TestCollectPropagatesErrors(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 401, Body: `{"error":"nope"}`})
	c := newTestClient(t, m)
	got, err := c.CollectRecords(testCtx(t), nil, 0)
	mustAPIError(t, err, KindUnauthorized)
	if got != nil {
		t.Errorf("a failed collect should return nil, got %v", got)
	}
}
