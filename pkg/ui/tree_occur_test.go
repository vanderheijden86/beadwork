// tree_occur_test.go - Tests for occur mode (bd-sjs.2)
package ui

import (
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestTreeOccurModeToggle(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	if tree.IsOccurMode() {
		t.Error("expected occur mode off initially")
	}
	issues := []model.Issue{
		{ID: "a", Title: "Fix bug", Priority: 1, IssueType: model.TypeBug},
		{ID: "b", Title: "Add feature", Priority: 2, IssueType: model.TypeFeature},
	}
	tree.Build(issues)
	tree.SetSize(120, 40)
	tree.EnterOccurMode("bug")
	if !tree.IsOccurMode() {
		t.Error("expected occur mode on")
	}
	if tree.OccurPattern() != "bug" {
		t.Errorf("expected pattern 'bug', got %q", tree.OccurPattern())
	}
	tree.ExitOccurMode()
	if tree.IsOccurMode() {
		t.Error("expected occur mode off after exit")
	}
}

func TestTreeOccurModeFilters(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-1", Title: "Fix login bug", Priority: 1, IssueType: model.TypeBug},
		{ID: "bd-2", Title: "Add dashboard", Priority: 2, IssueType: model.TypeFeature},
		{ID: "bd-3", Title: "Fix logout bug", Priority: 1, IssueType: model.TypeBug},
	}
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	tree.Build(issues)
	tree.SetSize(120, 40)

	tree.EnterOccurMode("bug")
	if len(tree.flatList) != 2 {
		t.Errorf("expected 2 matches for 'bug', got %d", len(tree.flatList))
	}
}

func TestTreeOccurModeEmptyPattern(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.EnterOccurMode("")
	if tree.IsOccurMode() {
		t.Error("expected occur mode not activated with empty pattern")
	}
}
