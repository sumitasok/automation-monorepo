package event

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newFixtureRepo(t *testing.T, initialRules string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "Test")

	rulesDir := filepath.Join(root, "data", "config")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(rulesDir, "expense-rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(initialRules), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "data/config/expense-rules.yaml")
	gitRun(t, root, "commit", "-q", "-m", "initial rules")
	return rulesPath
}

func TestWorkspaceRootDerivedFromRulesFile(t *testing.T) {
	got := workspaceRoot("/home/sumit/workspace/data/config/expense-rules.yaml")
	want := "/home/sumit/workspace"
	if got != want {
		t.Errorf("workspaceRoot = %q, want %q", got, want)
	}
}

func TestCaptureRuleAppendsAndCommits(t *testing.T) {
	const initial = `# header comment — must survive untouched
rules:
  - name: existing-rule
    applies_to: [event]
    match:
      merchant_contains: ["existing"]
    outcome:
      event_relevance: routine
`
	rulesPath := newFixtureRepo(t, initial)
	root := workspaceRoot(rulesPath)

	before, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := captureRule(rulesPath, "HungerBox"); err != nil {
		t.Fatalf("captureRule failed: %v", err)
	}

	after, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatalf("existing file content was not preserved byte-for-byte:\nbefore=%q\nafter=%q", before, after)
	}
	if !strings.Contains(string(after), "name: hungerbox-routine") {
		t.Errorf("appended rule name not found in file:\n%s", after)
	}

	rs, err := LoadExpenseRules(rulesPath)
	if err != nil {
		t.Fatalf("resulting file did not parse: %v", err)
	}
	if len(rs.Rules) != 2 {
		t.Fatalf("expected 2 rules after capture, got %d", len(rs.Rules))
	}

	clean, err := gitClean(root, "data/config/expense-rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Error("working tree should be clean after a successful capture commit")
	}
}

func TestCaptureRuleAbortsOnDirtyFile(t *testing.T) {
	rulesPath := newFixtureRepo(t, "rules: []\n")
	if err := os.WriteFile(rulesPath, []byte("rules: []\n# uncommitted edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := captureRule(rulesPath, "HungerBox")
	if err == nil {
		t.Fatal("expected an error for a dirty rules file, got nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("error = %v, want mention of uncommitted changes", err)
	}
}

func TestUniqueRuleNameCollisionSuffix(t *testing.T) {
	existing := ExpenseRules{Rules: []ExpenseRule{{Name: "hungerbox-routine"}, {Name: "hungerbox-routine-2"}}}
	got := uniqueRuleName(existing, "HungerBox")
	if got != "hungerbox-routine-3" {
		t.Errorf("uniqueRuleName = %q, want hungerbox-routine-3", got)
	}
}
