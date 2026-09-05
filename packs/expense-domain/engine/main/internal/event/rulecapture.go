// Story 6 (spec 003): after an approved correction that lands a transaction
// on "no event" (routine), offer to capture the merchant pattern as a new,
// git-committed event_relevance: routine rule in the shared
// data/config/expense-rules.yaml (spec 002). Only runs on an interactive run
// (isInteractive()) — a scheduled/cron run never prompts. Independent copy
// of the gmail pack's identical shape (spec 002 Decision 3 precedent); the
// event side only ever captures the "routine" outcome — the rules engine has
// no per-event outcome field (data-model.md).
package event

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var ruleCaptureSlugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func ruleCaptureSlugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = ruleCaptureSlugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "rule"
	}
	return s
}

// workspaceRoot derives the workspace root from an already-resolved
// --rules-file path (research.md Decision 11): data/config/expense-rules.yaml
// is always three path components below the workspace root.
func workspaceRoot(rulesFilePath string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(rulesFilePath)))
}

// offerRuleCapture prompts the user (only ever called from an interactive
// run) to capture a "not event-worthy" correction for merchant as a new
// expense-rules.yaml rule. rulesFile is the resolved --rules-file path; ""
// disables the offer entirely.
func offerRuleCapture(rulesFile, merchant string) {
	if rulesFile == "" || strings.TrimSpace(merchant) == "" {
		return
	}

	fmt.Printf("\n[capture] Turn this into a lasting rule for future %q transactions (-> not event-worthy)? [y/N]: ", merchant)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return
	}

	if err := captureRule(rulesFile, merchant); err != nil {
		fmt.Printf("[capture] not captured: %v\n", err)
	}
}

// captureRule performs the write-then-commit sequence in
// contracts/rule-capture.md: git-clean precondition, name-collision-safe
// append, git add + commit.
func captureRule(rulesFile, merchant string) error {
	root := workspaceRoot(rulesFile)
	relPath := filepath.Join("data", "config", filepath.Base(rulesFile))

	clean, err := gitClean(root, relPath)
	if err != nil {
		return fmt.Errorf("checking git status: %w", err)
	}
	if !clean {
		return fmt.Errorf("%s has uncommitted changes — commit or stash them first, then re-run", relPath)
	}

	existing, err := LoadExpenseRules(rulesFile)
	if err != nil {
		return fmt.Errorf("loading existing rules: %w", err)
	}
	name := uniqueRuleName(existing, merchant)

	if err := appendRule(rulesFile, name, merchant); err != nil {
		return fmt.Errorf("writing rule: %w", err)
	}

	// Sanity check: the file we just hand-appended to must still parse.
	if _, err := LoadExpenseRules(rulesFile); err != nil {
		return fmt.Errorf("captured rule produced an invalid rules file: %w", err)
	}

	msg := fmt.Sprintf("Capture rule: %s -> not event-worthy", merchant)
	hash, err := gitCommit(root, relPath, msg)
	if err != nil {
		return fmt.Errorf("rule written to %s but git commit failed (working tree now dirty): %w", relPath, err)
	}
	fmt.Printf("[capture] rule %q written to %s and committed (%s)\n", name, relPath, hash)
	return nil
}

// uniqueRuleName derives a kebab-case rule name from merchant, appending a
// numeric suffix on collision with an existing rule name.
func uniqueRuleName(existing ExpenseRules, merchant string) string {
	seen := make(map[string]bool, len(existing.Rules))
	for _, r := range existing.Rules {
		seen[r.Name] = true
	}
	base := ruleCaptureSlugify(merchant) + "-routine"
	name := base
	for n := 2; seen[name]; n++ {
		name = fmt.Sprintf("%s-%d", base, n)
		if n > 20 {
			break // pathological collision streak — give up growing the suffix
		}
	}
	return name
}

// appendRule hand-appends one new rule entry to rulesFile's existing text,
// preserving every other byte in the file (contracts/rule-capture.md) —
// deliberately not a full load-then-remarshal round trip, which would strip
// the file's header comments and any per-rule comments.
func appendRule(rulesFile, name, merchant string) error {
	raw, err := os.ReadFile(rulesFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = []byte("rules:\n")
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		raw = append(raw, '\n')
	}

	entry := fmt.Sprintf(`  - name: %s
    description: >
      Captured from a comment-driven correction on %s.
    applies_to: [event]
    match:
      merchant_contains: [%q]
    outcome:
      event_relevance: routine
`, name, time.Now().UTC().Format("2006-01-02"), merchant)

	raw = append(raw, []byte(entry)...)
	return os.WriteFile(rulesFile, raw, 0o644)
}

// gitClean reports whether relPath has no uncommitted changes in the git
// repository rooted at root (FR-020).
func gitClean(root, relPath string) (bool, error) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--", relPath).Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// gitCommit stages and commits relPath in the git repository rooted at root,
// returning the short commit hash (FR-021).
func gitCommit(root, relPath, message string) (string, error) {
	if out, err := exec.Command("git", "-C", root, "add", "--", relPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %w: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "commit", "-m", message, "--", relPath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %w: %s", err, out)
	}
	hashOut, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(hashOut)), nil
}
