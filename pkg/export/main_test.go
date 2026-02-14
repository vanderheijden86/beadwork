package export

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Prevent any test from accidentally opening a browser
	os.Setenv("BW_NO_BROWSER", "1")
	os.Setenv("BW_TEST_MODE", "1")

	os.Exit(m.Run())
}
