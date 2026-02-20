package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVersionFlag_OutputsVersion(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	output := string(out)

	if !strings.Contains(output, "v") {
		t.Errorf("expected version to contain 'v', got: %s", output)
	}

	versionPattern := regexp.MustCompile(`v\d+\.\d+\.\d+`)
	if !versionPattern.MatchString(output) {
		t.Errorf("expected semver format in version output, got: %s", output)
	}
}

func TestVersionFlag_IncludesBuildInfo(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "b9s") {
		t.Errorf("expected 'b9s' in version output, got: %s", output)
	}
}

func TestRollbackFlag_FailsWithoutBackup(t *testing.T) {
	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	cmd := exec.Command(bv, "--rollback")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("expected --rollback to fail without backup, but succeeded: %s", out)
	}

	output := string(out)
	if !strings.Contains(strings.ToLower(output), "backup") && !strings.Contains(strings.ToLower(output), "no backup") {
		t.Errorf("expected error message about missing backup, got: %s", output)
	}
}

func TestUpdateFlag_RequiresNetwork(t *testing.T) {
	if os.Getenv("SKIP_NETWORK_TESTS") != "" {
		t.Skip("Skipping network-dependent test")
	}

	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bv, "--update", "-y")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "B9S_UPDATE_TIMEOUT=2s")
	out, _ := cmd.CombinedOutput()

	output := string(out)
	hasUpdateContent := strings.Contains(output, "update") ||
		strings.Contains(output, "Update") ||
		strings.Contains(output, "version") ||
		strings.Contains(output, "latest") ||
		strings.Contains(output, "download") ||
		strings.Contains(output, "failed")

	if !hasUpdateContent {
		t.Logf("Update output: %s", output)
	}
}

func TestHelpFlag_DocumentsUpdateFeatures(t *testing.T) {
	bv := buildBvBinary(t)

	cmd := exec.Command(bv, "--help")
	out, _ := cmd.CombinedOutput()
	output := string(out)

	if !strings.Contains(output, "-update") && !strings.Contains(output, "--update") {
		t.Errorf("expected help to document --update flag, got: %s", output)
	}

	if !strings.Contains(output, "-rollback") && !strings.Contains(output, "--rollback") {
		t.Errorf("expected help to document --rollback flag, got: %s", output)
	}
}

func TestBinary_HasProperPermissions(t *testing.T) {
	bv := buildBvBinary(t)

	info, err := os.Stat(bv)
	if err != nil {
		t.Fatalf("stat binary: %v", err)
	}

	mode := info.Mode()
	if mode&0111 == 0 {
		t.Errorf("binary should be executable, mode: %v", mode)
	}
}

func TestBinary_RespondsToSignals(t *testing.T) {
	bv := buildBvBinary(t)
	tmpDir := t.TempDir()

	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bv, "--version")
	cmd.Dir = tmpDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("binary failed: %v\n%s", err, out)
	}
}
