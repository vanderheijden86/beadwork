package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBvBinary builds the bv binary into a temp directory and returns the path.
func buildBvBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "bv")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bv/main.go")
	cmd.Dir = "../../"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bv failed: %v\n%s", err, out)
	}
	return binPath
}

// initGitRepo creates a git repo with an initial beads commit and a follow-up change.
// It returns the repository directory and the hash of the first commit (HEAD~1).
func initGitRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := t.TempDir()
	beadsDir := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Commit 1: single issue
	first := `{"id":"A","title":"Alpha","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(first), 0o644); err != nil {
		t.Fatalf("write beads v1: %v", err)
	}

	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	git("init")
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "initial")

	// Commit 2: add new issue B
	second := first + "\n" + `{"id":"B","title":"Beta","status":"open","priority":2,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(second), 0o644); err != nil {
		t.Fatalf("write beads v2: %v", err)
	}
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "add B")

	revCmd := exec.Command("git", "rev-parse", "HEAD~1")
	revCmd.Dir = repoDir
	out, err := revCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v\n%s", err, out)
	}
	return repoDir, strings.TrimSpace(string(out))
}

func TestRobotDiffIncludesHashesAndNewIssues(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, priorRev := initGitRepo(t)

	cmd := exec.Command(bv, "--robot-diff", "--diff-since", "HEAD~1")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-diff failed: %v\n%s", err, out)
	}

	var payload struct {
		GeneratedAt      string `json:"generated_at"`
		ResolvedRevision string `json:"resolved_revision"`
		FromDataHash     string `json:"from_data_hash"`
		ToDataHash       string `json:"to_data_hash"`
		Diff             struct {
			NewIssues []struct {
				ID string `json:"id"`
			} `json:"new_issues"`
			Summary struct {
				IssuesAdded int `json:"issues_added"`
			} `json:"summary"`
		} `json:"diff"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}

	if payload.GeneratedAt == "" {
		t.Fatal("generated_at missing")
	}
	if payload.FromDataHash == "" || payload.ToDataHash == "" {
		t.Fatalf("expected both data hashes, got from=%q to=%q", payload.FromDataHash, payload.ToDataHash)
	}
	if payload.FromDataHash == payload.ToDataHash {
		t.Fatalf("data hashes should differ when issues change")
	}
	if payload.ResolvedRevision != priorRev {
		t.Fatalf("resolved_revision mismatch: want %s got %s", priorRev, payload.ResolvedRevision)
	}
	if len(payload.Diff.NewIssues) != 1 || payload.Diff.NewIssues[0].ID != "B" {
		t.Fatalf("expected new issue B, got %+v", payload.Diff.NewIssues)
	}
	if payload.Diff.Summary.IssuesAdded != 1 {
		t.Fatalf("expected issues_added=1, got %d", payload.Diff.Summary.IssuesAdded)
	}
}

func TestRobotOutputsShareDataHashAndStatus(t *testing.T) {
	bv := buildBvBinary(t)

	envDir := t.TempDir()
	beadsDir := filepath.Join(envDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	beads := `{"id":"X","title":"Node X","status":"open","priority":1,"issue_type":"task"}
{"id":"Y","title":"Node Y","status":"open","priority":2,"issue_type":"task","dependencies":[{"issue_id":"Y","depends_on_id":"X","type":"blocks"}]}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	flags := []string{"--robot-insights", "--robot-plan", "--robot-priority"}
	hashes := make([]string, 0, len(flags))
	for _, flag := range flags {
		cmd := exec.Command(bv, flag)
		cmd.Dir = envDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", flag, err, out)
		}
		var payload struct {
			DataHash string                 `json:"data_hash"`
			Status   map[string]interface{} `json:"status"`
		}
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("%s json decode: %v\nout=%s", flag, err, out)
		}
		if payload.DataHash == "" {
			t.Fatalf("%s missing data_hash", flag)
		}
		if payload.Status == nil || len(payload.Status) == 0 {
			t.Fatalf("%s missing status map", flag)
		}
		hashes = append(hashes, payload.DataHash)
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[i] != hashes[0] {
			t.Fatalf("data_hash mismatch across robot outputs: %v", hashes)
		}
	}
}
