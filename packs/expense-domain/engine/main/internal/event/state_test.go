package event

import "testing"

// TestNeedsReprocessing covers spec 003's dirty-tracking cases (data-model.md).
func TestNeedsReprocessing(t *testing.T) {
	st := &State{Assigned: map[string]AssignmentEntry{
		"never-assigned-doesnt-matter": {}, // placeholder so the map isn't nil-ambiguous
	}}
	delete(st.Assigned, "never-assigned-doesnt-matter")

	st.Mark("done-no-comment", "trip-a", 1.0, "ai:stub", "")
	st.Mark("done-same-comment", "trip-a", 1.0, "ai:stub+comment", "goa trip")
	st.Mark("done-old-comment", "trip-a", 1.0, "ai:stub+comment", "old note")

	cases := []struct {
		name           string
		id             string
		currentComment string
		want           bool
	}{
		{"never assigned", "never-seen", "", true},
		{"never assigned, with comment", "never-seen", "a comment", true},
		{"assigned, no comment", "done-no-comment", "", false},
		{"assigned, comment now added", "done-no-comment", "a new comment", true},
		{"assigned, comment unchanged", "done-same-comment", "goa trip", false},
		{"assigned, comment changed", "done-old-comment", "new note", true},
		{"assigned, comment cleared after being considered", "done-same-comment", "", false},
	}
	for _, c := range cases {
		if got := st.needsReprocessing(c.id, c.currentComment); got != c.want {
			t.Errorf("%s: needsReprocessing(%q, %q) = %v, want %v", c.name, c.id, c.currentComment, got, c.want)
		}
	}
}
