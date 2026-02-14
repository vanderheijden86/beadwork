package ui

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("BW_NO_BROWSER", "1")
	os.Setenv("BW_TEST_MODE", "1")

	// Clean up any tree-state.json that non-isolated tree tests leave behind
	// in the CWD. Go tests run from the package directory, so expand/collapse
	// operations via ui.NewModel can pollute .beads/tree-state.json here,
	// causing cross-test ordering failures.
	os.RemoveAll(".beads")

	code := m.Run()

	// Post-test cleanup
	os.RemoveAll(".beads")

	os.Exit(code)
}
