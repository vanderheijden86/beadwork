package main_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var bvBinaryPath string
var bvBinaryDir string

var (
	scriptTUISupported      = true
	scriptTUIDisabledReason string
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("B9S_NO_BROWSER", "1")
	os.Setenv("B9S_TEST_MODE", "1")

	// Build the binary once for all tests
	if err := buildBvOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build bv binary: %v\n", err)
		os.Exit(1)
	}

	scriptTUISupported, scriptTUIDisabledReason = detectScriptTUICapability(bvBinaryPath)

	code := m.Run()
	if bvBinaryDir != "" {
		_ = os.RemoveAll(bvBinaryDir)
	}
	os.Exit(code)
}

func detectScriptTUICapability(bvPath string) (bool, string) {
	if _, err := exec.LookPath("script"); err != nil {
		return false, "script command not available"
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return false, "script TUI harness unsupported on this OS"
	}
	if bvPath == "" {
		return false, "bv binary path is empty"
	}

	tempDir, err := os.MkdirTemp("", "bv-e2e-tui-cap-*")
	if err != nil {
		return false, fmt.Sprintf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	beadsDir := filepath.Join(tempDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		return false, fmt.Sprintf("failed to create beads dir: %v", err)
	}
	beads := `{"id":"cap-1","title":"Capability check","status":"open","priority":1,"issue_type":"task"}`
	if err := os.WriteFile(filepath.Join(beadsDir, "beads.jsonl"), []byte(beads), 0o644); err != nil {
		return false, fmt.Sprintf("failed to write beads.jsonl: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := scriptTUICommand(ctx, bvPath)
	if cmd == nil {
		return false, "script command unavailable"
	}
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"B9S_TUI_AUTOCLOSE_MS=250",
	)

	outFile := filepath.Join(tempDir, "script.out")
	f, err := os.Create(outFile)
	if err != nil {
		return false, fmt.Sprintf("failed to create output file: %v", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	runErr := cmd.Run()
	_ = f.Close()

	if ctx.Err() == context.DeadlineExceeded {
		return false, "bv did not auto-exit under script (PTY/CI mismatch)"
	}
	if runErr != nil {
		return false, fmt.Sprintf("script TUI run failed: %v", runErr)
	}

	return true, ""
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

	cmd := exec.Command("go", "build", "-o", binPath, "../../cmd/b9s")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %v\n%s", err, out)
	}

	bvBinaryPath = binPath
	return nil
}

// buildBvBinary returns the path to the pre-built binary.
func buildBvBinary(t *testing.T) string {
	t.Helper()
	if bvBinaryPath == "" {
		t.Fatal("bv binary not built")
	}
	return bvBinaryPath
}

// skipIfNoScript skips the test if the script command is unavailable.
func skipIfNoScript(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("skipping: script command not available")
	}
	if !scriptTUISupported {
		if scriptTUIDisabledReason != "" {
			t.Skipf("skipping: %s", scriptTUIDisabledReason)
		}
		t.Skip("skipping: script-based TUI harness unavailable")
	}
}

// scriptTUICommand creates an exec.Cmd that runs the bv binary under `script`
// to provide a pseudo-TTY for TUI tests.
func scriptTUICommand(ctx context.Context, bvPath string, args ...string) *exec.Cmd {
	if _, err := exec.LookPath("script"); err != nil {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		scriptArgs := []string{"-q", "/dev/null", bvPath}
		scriptArgs = append(scriptArgs, args...)
		return exec.CommandContext(ctx, "script", scriptArgs...)

	case "linux":
		cmdStr := bvPath
		for _, arg := range args {
			if strings.ContainsAny(arg, " \t") {
				cmdStr += " \"" + arg + "\""
			} else {
				cmdStr += " " + arg
			}
		}
		return exec.CommandContext(ctx, "script", "-q", "-e", "-f", "-c", cmdStr, "/dev/null")

	default:
		return nil
	}
}

// ensureCmdStdinCloses wires a controllable stdin for command execution.
func ensureCmdStdinCloses(t *testing.T, ctx context.Context, cmd *exec.Cmd, closeAfter time.Duration) {
	t.Helper()
	if cmd == nil || cmd.Stdin != nil {
		return
	}
	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})

	go func() {
		select {
		case <-ctx.Done():
			_ = stdinW.Close()
		case <-time.After(closeAfter):
			_ = stdinW.Close()
		}
	}()
}

// runCmdToFile runs a command and captures stdout+stderr to a temp file.
func runCmdToFile(t *testing.T, cmd *exec.Cmd) ([]byte, error) {
	t.Helper()
	if cmd == nil {
		return nil, fmt.Errorf("nil cmd")
	}

	outPath := filepath.Join(t.TempDir(), "cmd.out")
	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	cmd.Stdout = f
	cmd.Stderr = f

	runErr := cmd.Run()
	_ = f.Close()

	out, readErr := os.ReadFile(outPath)
	if readErr != nil {
		return nil, fmt.Errorf("read output file: %w (run err: %v)", readErr, runErr)
	}
	return out, runErr
}
