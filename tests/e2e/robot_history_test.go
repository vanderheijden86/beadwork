package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// createHistoryRepo seeds a git repo with a bead lifecycle (open -> in_progress -> closed)
// and co-committed code changes so robot-history can correlate events and commits.
func createHistoryRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	write := func(content string) {
		if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(content), 0o644); err != nil {
			t.Fatalf("write beads.jsonl: %v", err)
		}
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

	// Commit 1: creation (open)
	write(`{"id":"HIST-1","title":"History bead","status":"open","priority":1,"issue_type":"task"}`)
	git("add", ".beads/beads.jsonl")
	git("commit", "-m", "seed HIST-1")

	// Commit 2: claim + code change
	write(`{"id":"HIST-1","title":"History bead","status":"in_progress","priority":1,"issue_type":"task"}`)
	if err := os.MkdirAll(filepath.Join(repoDir, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "pkg", "work.go"), []byte("package pkg\n\n// work in progress\n"), 0o644); err != nil {
		t.Fatalf("write work.go: %v", err)
	}
	git("add", ".beads/beads.jsonl", "pkg/work.go")
	git("commit", "-m", "claim HIST-1 with code")

	// Commit 3: close + code tweak
	write(`{"id":"HIST-1","title":"History bead","status":"closed","priority":1,"issue_type":"task"}`)
	if err := os.WriteFile(filepath.Join(repoDir, "pkg", "work.go"), []byte("package pkg\n\n// finished work\nfunc Done() {}\n"), 0o644); err != nil {
		t.Fatalf("update work.go: %v", err)
	}
	git("add", ".beads/beads.jsonl", "pkg/work.go")
	git("commit", "-m", "close HIST-1")

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repoDir
	out, err := revCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v\n%s", err, out)
	}

	return repoDir, strings.TrimSpace(string(out))
}

func TestRobotHistoryIncludesEventsAndCommitIndex(t *testing.T) {
	bv := buildBvBinary(t)
	repoDir, head := createHistoryRepo(t)

	cmd := exec.Command(bv, "--robot-history")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--robot-history failed: %v\n%s", err, out)
	}

	var payload struct {
		GeneratedAt     string `json:"generated_at"`
		DataHash        string `json:"data_hash"`
		GitRange        string `json:"git_range"`
		LatestCommitSHA string `json:"latest_commit_sha"`
		Stats           struct {
			TotalBeads         int            `json:"total_beads"`
			BeadsWithCommits   int            `json:"beads_with_commits"`
			MethodDistribution map[string]int `json:"method_distribution"`
		} `json:"stats"`
		Histories map[string]struct {
			Events []struct {
				EventType string `json:"event_type"`
				CommitSHA string `json:"commit_sha"`
			} `json:"events"`
			Commits []struct {
				Method string `json:"method"`
				Files  []struct {
					Path string `json:"path"`
				} `json:"files"`
			} `json:"commits"`
			Milestones struct {
				Closed interface{} `json:"closed"`
			} `json:"milestones"`
		} `json:"histories"`
		CommitIndex map[string][]string `json:"commit_index"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("json decode: %v\nout=%s", err, out)
	}

	if payload.DataHash == "" {
		t.Fatal("missing data_hash")
	}
	if payload.GitRange == "" {
		t.Fatal("missing git_range")
	}
	if payload.LatestCommitSHA == "" {
		t.Fatalf("latest_commit_sha missing")
	}
	if payload.Stats.TotalBeads != 1 {
		t.Fatalf("expected total_beads=1, got %d", payload.Stats.TotalBeads)
	}
	if payload.Stats.BeadsWithCommits != 1 {
		t.Fatalf("expected beads_with_commits=1, got %d", payload.Stats.BeadsWithCommits)
	}
	if payload.Stats.MethodDistribution["co_committed"] == 0 {
		t.Fatalf("expected co_committed entries in method_distribution, got %v", payload.Stats.MethodDistribution)
	}

	hist, ok := payload.Histories["HIST-1"]
	if !ok {
		t.Fatalf("history for HIST-1 missing: keys=%v", keys(payload.Histories))
	}
	if len(hist.Events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(hist.Events))
	}
	if len(hist.Commits) == 0 {
		t.Fatalf("expected commits correlated, got 0")
	}
	if hist.Milestones.Closed == nil {
		t.Fatalf("expected closed milestone populated")
	}

	// Commit index should map at least one commit to HIST-1
	found := false
	for sha, beads := range payload.CommitIndex {
		if len(beads) > 0 && beads[0] == "HIST-1" {
			found = true
			if sha == payload.LatestCommitSHA || sha == head {
				break
			}
		}
	}
	if !found {
		t.Fatalf("commit_index missing mapping to HIST-1: %v", payload.CommitIndex)
	}
}

// keys returns map keys for debugging in assertions.
func keys(m map[string]struct {
	Events []struct {
		EventType string `json:"event_type"`
		CommitSHA string `json:"commit_sha"`
	} `json:"events"`
	Commits []struct {
		Method string `json:"method"`
		Files  []struct {
			Path string `json:"path"`
		} `json:"files"`
	} `json:"commits"`
	Milestones struct {
		Closed interface{} `json:"closed"`
	} `json:"milestones"`
}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
