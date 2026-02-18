package ui_test

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// cleanTreeState removes any .beads/tree-state.json that may have been
// left behind by other tests, preventing state pollution.
func cleanTreeState(t *testing.T) {
	t.Helper()
	os.Remove(".beads/tree-state.json")
	t.Cleanup(func() {
		os.Remove(".beads/tree-state.json")
	})
}

// ============================================================================
// Helper: create a deeper tree for sibling/parent navigation tests.
//
//   epic-1 (P1, epic)
//     task-1 (P2, task, child of epic-1)
//     task-2 (P2, task, child of epic-1)
//     task-3 (P2, task, child of epic-1)
//   standalone-1 (P2, task, no parent)
//   standalone-2 (P3, task, no parent)
// ============================================================================

func createNavTestIssues() []model.Issue {
	now := time.Now()
	// CreatedAt values are set so that default sort (Created desc) produces
	// the expected display order: epic-1, task-1, task-2, task-3,
	// standalone-1, standalone-2 (bd-2ty).
	return []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now.Add(5 * time.Second)},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(4 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "task-2", Title: "Task Two", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(3 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-2", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "task-3", Title: "Task Three", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(2 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-3", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{ID: "standalone-1", Title: "Standalone One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Second)},
		{ID: "standalone-2", Title: "Standalone Two", Status: model.StatusOpen, Priority: 3, IssueType: model.TypeTask, CreatedAt: now},
	}
}

// ============================================================================
// Feature 1: Structural tree navigation (bd-ryu)
// Keys: p/P = jump to parent, [ / ] = prev/next sibling, { / } = first/last sibling
// ============================================================================

// TestTreeNavJumpToParentP verifies that 'p' jumps to the parent node.
func TestTreeNavJumpToParentP(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-1 (child of epic-1)
	m = sendKey(t, m, "j") // epic-1 -> task-1
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1 selected, got %q", m.TreeSelectedID())
	}

	// Press 'p' to jump to parent
	m = sendKey(t, m, "p")
	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("expected epic-1 after 'p', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavPUppercaseTogglesPickerNotParent verifies 'P' toggles picker (bd-ey3),
// not parent-jump. Lowercase 'p' still does parent-jump (tested in TestTreeNavJumpToParentP).
func TestTreeNavPUppercaseTogglesPickerNotParent(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-2
	m = sendKey(t, m, "j") // task-1
	m = sendKey(t, m, "j") // task-2
	if m.TreeSelectedID() != "task-2" {
		t.Fatalf("expected task-2 selected, got %q", m.TreeSelectedID())
	}

	// 'P' should NOT jump to parent (it toggles picker now)
	m = sendKey(t, m, "P")
	if m.TreeSelectedID() == "epic-1" {
		t.Error("'P' should not jump to parent (use lowercase 'p' for that)")
	}
	// Should stay on task-2
	if m.TreeSelectedID() != "task-2" {
		t.Errorf("expected to stay on task-2, got %q", m.TreeSelectedID())
	}
}

// TestTreeNavJumpToParentAtRoot verifies that 'p' does nothing at root.
func TestTreeNavJumpToParentAtRoot(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// We're on epic-1 (a root node)
	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1 selected, got %q", m.TreeSelectedID())
	}

	// Press 'p' - should stay at epic-1
	m = sendKey(t, m, "p")
	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("expected epic-1 to remain selected at root, got %q", m.TreeSelectedID())
	}
}

// TestTreeNavNextSibling verifies that ']' jumps to the next sibling.
func TestTreeNavNextSibling(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-1 (first child of epic-1)
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1, got %q", m.TreeSelectedID())
	}

	// Press ']' to go to next sibling (task-2)
	m = sendKey(t, m, "]")
	if m.TreeSelectedID() != "task-2" {
		t.Errorf("expected task-2 after ']', got %q", m.TreeSelectedID())
	}

	// Press ']' again to go to task-3
	m = sendKey(t, m, "]")
	if m.TreeSelectedID() != "task-3" {
		t.Errorf("expected task-3 after second ']', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavNextSiblingAtLast verifies that ']' does nothing at the last sibling.
func TestTreeNavNextSiblingAtLast(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-3 (last child of epic-1)
	m = sendKey(t, m, "j") // task-1
	m = sendKey(t, m, "j") // task-2
	m = sendKey(t, m, "j") // task-3
	if m.TreeSelectedID() != "task-3" {
		t.Fatalf("expected task-3, got %q", m.TreeSelectedID())
	}

	// Press ']' - should stay at task-3
	m = sendKey(t, m, "]")
	if m.TreeSelectedID() != "task-3" {
		t.Errorf("expected task-3 to remain selected at last sibling, got %q", m.TreeSelectedID())
	}
}

// TestTreeNavPrevSibling verifies that '[' jumps to the previous sibling.
func TestTreeNavPrevSibling(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-2
	m = sendKey(t, m, "j") // task-1
	m = sendKey(t, m, "j") // task-2
	if m.TreeSelectedID() != "task-2" {
		t.Fatalf("expected task-2, got %q", m.TreeSelectedID())
	}

	// Press '[' to go to previous sibling (task-1)
	m = sendKey(t, m, "[")
	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 after '[', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavPrevSiblingAtFirst verifies that '[' does nothing at the first sibling.
func TestTreeNavPrevSiblingAtFirst(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-1 (first child)
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1, got %q", m.TreeSelectedID())
	}

	// Press '[' - should stay at task-1
	m = sendKey(t, m, "[")
	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 to remain selected at first sibling, got %q", m.TreeSelectedID())
	}
}

// TestTreeNavFirstSibling verifies that '{' jumps to the first sibling.
func TestTreeNavFirstSibling(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-3 (last child)
	m = sendKey(t, m, "j") // task-1
	m = sendKey(t, m, "j") // task-2
	m = sendKey(t, m, "j") // task-3
	if m.TreeSelectedID() != "task-3" {
		t.Fatalf("expected task-3, got %q", m.TreeSelectedID())
	}

	// Press '{' to jump to first sibling (task-1)
	m = sendKey(t, m, "{")
	if m.TreeSelectedID() != "task-1" {
		t.Errorf("expected task-1 after '{', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavLastSibling verifies that '}' jumps to the last sibling.
func TestTreeNavLastSibling(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Move to task-1 (first child)
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1, got %q", m.TreeSelectedID())
	}

	// Press '}' to jump to last sibling (task-3)
	m = sendKey(t, m, "}")
	if m.TreeSelectedID() != "task-3" {
		t.Errorf("expected task-3 after '}', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavSiblingAtRootLevel verifies sibling navigation works for root nodes.
func TestTreeNavSiblingAtRootLevel(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// We're at epic-1 (root)
	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1, got %q", m.TreeSelectedID())
	}

	// ']' should go to next root sibling (standalone-1)
	m = sendKey(t, m, "]")
	if m.TreeSelectedID() != "standalone-1" {
		t.Errorf("expected standalone-1 after ']' at root, got %q", m.TreeSelectedID())
	}

	// ']' again should go to standalone-2
	m = sendKey(t, m, "]")
	if m.TreeSelectedID() != "standalone-2" {
		t.Errorf("expected standalone-2 after second ']', got %q", m.TreeSelectedID())
	}

	// '[' should go back to standalone-1
	m = sendKey(t, m, "[")
	if m.TreeSelectedID() != "standalone-1" {
		t.Errorf("expected standalone-1 after '[', got %q", m.TreeSelectedID())
	}
}

// ============================================================================
// Feature 2: Org-mode visibility cycling (bd-8of)
// TAB on a node: folded -> children visible -> subtree visible -> folded
// Shift+TAB: global cycling: all folded -> top-level only -> all expanded
// ============================================================================

// createDeepTreeIssues creates a 3-level hierarchy for visibility cycling tests.
//
//   epic-1 (depth 0)
//     task-1 (depth 1, child of epic-1)
//       subtask-1 (depth 2, child of task-1)
//     task-2 (depth 1, child of epic-1)
//   standalone-1 (depth 0)
func createDeepTreeIssues() []model.Issue {
	now := time.Now()
	// CreatedAt values are set so that default sort (Created desc) produces
	// the expected display order: epic-1, task-1, subtask-1, task-2,
	// standalone-1 (bd-2ty).
	return []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now.Add(4 * time.Second)},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(3 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "subtask-1", Title: "Subtask One", Status: model.StatusOpen, Priority: 3, IssueType: model.TypeTask,
			CreatedAt: now.Add(2 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "subtask-1", DependsOnID: "task-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "task-2", Title: "Task Two", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-2", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{ID: "standalone-1", Title: "Standalone", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now},
	}
}

// ============================================================================
// Tests: Enter cycles node visibility, TAB/Shift+TAB/1-9 removed (bd-8zc)
// TAB is now tree↔detail focus switching only.
// 1-9 are now project switching only.
// Enter does CycleNodeVisibility (expand/collapse cycling).
// ============================================================================

// TestTreeNavEnterCycleFolded verifies Enter cycles from folded to children-visible (bd-8zc).
func TestTreeNavEnterCycleFolded(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Collapse epic-1 first
	m = sendKey(t, m, "h") // collapse epic-1
	countAfterCollapse := m.TreeNodeCount()

	// Now Enter should expand to show direct children only
	m = sendSpecialKey(t, m, tea.KeyEnter)
	countAfterEnter := m.TreeNodeCount()

	if countAfterEnter <= countAfterCollapse {
		t.Errorf("Enter should expand folded node: had %d nodes, got %d", countAfterCollapse, countAfterEnter)
	}
}

// TestTreeNavEnterCycleChildrenToSubtree verifies Enter cycles from children-visible to subtree-visible (bd-8zc).
func TestTreeNavEnterCycleChildrenToSubtree(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1, got %q", m.TreeSelectedID())
	}

	countBefore := m.TreeNodeCount()

	// Enter on epic-1 which already has children visible should expand full subtree
	m = sendSpecialKey(t, m, tea.KeyEnter)
	countAfterFirstEnter := m.TreeNodeCount()

	if countAfterFirstEnter <= countBefore {
		t.Errorf("Enter should expand subtree: had %d nodes, got %d", countBefore, countAfterFirstEnter)
	}
}

// TestTreeNavEnterCycleBackToFolded verifies Enter eventually cycles back to folded state (bd-8zc).
func TestTreeNavEnterCycleBackToFolded(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// First collapse epic-1
	m = sendKey(t, m, "h")
	collapsedCount := m.TreeNodeCount()

	// Cycle through: folded -> children -> subtree -> folded
	m = sendSpecialKey(t, m, tea.KeyEnter)
	m = sendSpecialKey(t, m, tea.KeyEnter)
	m = sendSpecialKey(t, m, tea.KeyEnter)

	finalCount := m.TreeNodeCount()
	if finalCount != collapsedCount {
		t.Errorf("Enter cycle should return to folded state: had %d, got %d", collapsedCount, finalCount)
	}
}

// TestTreeNavEnterOnLeafDoesNothing verifies Enter on a leaf node does nothing (bd-8zc).
func TestTreeNavEnterOnLeafDoesNothing(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Navigate to task-1 (a leaf node)
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1, got %q", m.TreeSelectedID())
	}

	countBefore := m.TreeNodeCount()
	m = sendSpecialKey(t, m, tea.KeyEnter)
	countAfter := m.TreeNodeCount()

	if countBefore != countAfter {
		t.Errorf("Enter on leaf node should not change node count: had %d, got %d", countBefore, countAfter)
	}
}

// TestTreeNavNumberKeysNoLongerExpandLevels verifies 1-9 don't expand tree levels (bd-8zc).
// These keys are now reserved for project switching.
func TestTreeNavNumberKeysNoLongerExpandLevels(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	initialCount := m.TreeNodeCount()

	// Press '1' — should NOT collapse to roots only
	m = sendKey(t, m, "1")
	if m.TreeNodeCount() != initialCount {
		t.Errorf("pressing '1' should not change tree, count went from %d to %d", initialCount, m.TreeNodeCount())
	}

	// Press '9' — should NOT expand all
	m = sendKey(t, m, "Z") // collapse everything first
	collapsedCount := m.TreeNodeCount()
	m = sendKey(t, m, "9")
	if m.TreeNodeCount() != collapsedCount {
		t.Errorf("pressing '9' should not change tree, count went from %d to %d", collapsedCount, m.TreeNodeCount())
	}
}
