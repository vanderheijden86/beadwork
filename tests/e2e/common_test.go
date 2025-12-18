package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var bvBinaryPath string
var bvBinaryDir string

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("BV_NO_BROWSER", "1")
	os.Setenv("BV_TEST_MODE", "1")

	// Build the binary once for all tests
	if err := buildBvOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build bv binary: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	if bvBinaryDir != "" {
		_ = os.RemoveAll(bvBinaryDir)
	}
	os.Exit(code)
}

func buildBvOnce() error {
	tempDir, err := os.MkdirTemp("", "bv-e2e-build-*")
	if err != nil {
		return err
	}
	bvBinaryDir = tempDir

	binName := "bv"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tempDir, binName)

	// Determine project root (../../) relative to this file
	// We assume tests are run from project root or package dir.
	// `go test ./tests/e2e/...` -> CWD is project root?
	// Actually `go test` sets CWD to the package directory.
	// So `../../` is correct for `tests/e2e`.

	cmd := exec.Command("go", "build", "-o", binPath, "../../cmd/bv")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %v\n%s", err, out)
	}

	bvBinaryPath = binPath
	return nil
}

// buildBvBinary returns the path to the pre-built binary.
// It acts as a helper to ensure tests use the shared binary.
func buildBvBinary(t *testing.T) string {
	t.Helper()
	if bvBinaryPath == "" {
		t.Fatal("bv binary not built")
	}
	return bvBinaryPath
}
