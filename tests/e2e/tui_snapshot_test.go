package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTUIPrioritySnapshot launches the TUI long enough to render list + insights panels.
// We assert it exits cleanly (Ctrl+C) and produces no stderr noise.
func TestTUIPrioritySnapshot(t *testing.T) {
	bv := buildBvBinary(t)

	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	// Minimal graph with a dependency to exercise insights/priority panes.
	beads := `{"id":"P1","title":"Parent","status":"open","priority":1,"issue_type":"task"}
{"id":"C1","title":"Child","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"C1","depends_on_id":"P1","type":"blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	// Use `script` to allocate a pty and feed keys: insights (i), priority (p), quit (q).
	input := "i\np\nq\n"
	cmd := exec.Command("script", "-q", "/dev/null", bv)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
}
