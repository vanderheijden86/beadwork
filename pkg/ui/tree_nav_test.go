package ui_test

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
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
	return []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "task-2", Title: "Task Two", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(2 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-2", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "task-3", Title: "Task Three", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(3 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-3", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{ID: "standalone-1", Title: "Standalone One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(4 * time.Second)},
		{ID: "standalone-2", Title: "Standalone Two", Status: model.StatusOpen, Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(5 * time.Second)},
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
	m := ui.NewModel(issues, nil, "")
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

// TestTreeNavJumpToParentPUppercase verifies that 'P' also jumps to the parent node.
func TestTreeNavJumpToParentPUppercase(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Move to task-2
	m = sendKey(t, m, "j") // task-1
	m = sendKey(t, m, "j") // task-2
	if m.TreeSelectedID() != "task-2" {
		t.Fatalf("expected task-2 selected, got %q", m.TreeSelectedID())
	}

	// Press 'P' to jump to parent
	m = sendKey(t, m, "P")
	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("expected epic-1 after 'P', got %q", m.TreeSelectedID())
	}
}

// TestTreeNavJumpToParentAtRoot verifies that 'p' does nothing at root.
func TestTreeNavJumpToParentAtRoot(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	m := ui.NewModel(issues, nil, "")
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
	return []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Second),
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
			CreatedAt: now.Add(3 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-2", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{ID: "standalone-1", Title: "Standalone", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(4 * time.Second)},
	}
}

// TestTreeNavTabCycleFolded verifies TAB cycles from folded to children-visible.
// Starting state: epic-1 is collapsed (folded).
// After TAB: children should become visible (but not grandchildren).
func TestTreeNavTabCycleFolded(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Collapse epic-1 first
	m = sendKey(t, m, "h") // collapse epic-1
	countAfterCollapse := m.TreeNodeCount()

	// Now TAB should expand to show direct children only
	m = sendSpecialKey(t, m, tea.KeyTab)
	countAfterTab := m.TreeNodeCount()

	if countAfterTab <= countAfterCollapse {
		t.Errorf("TAB should expand folded node: had %d nodes, got %d", countAfterCollapse, countAfterTab)
	}
}

// TestTreeNavTabCycleChildrenToSubtree verifies TAB cycles from children-visible to subtree-visible.
// When children are visible but subtree is not fully expanded, TAB expands the full subtree.
func TestTreeNavTabCycleChildrenToSubtree(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Start state: epic-1 expanded (depth<1 auto-expand), task-1 collapsed by default.
	// subtask-1 not visible (parent task-1 collapsed).
	// Visible: epic-1, task-1, task-2, standalone-1 = 4 nodes

	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1, got %q", m.TreeSelectedID())
	}

	countBefore := m.TreeNodeCount()

	// TAB on epic-1 which already has children visible should expand full subtree
	m = sendSpecialKey(t, m, tea.KeyTab)
	countAfterFirstTab := m.TreeNodeCount()

	// subtask-1 should now appear (subtree fully expanded)
	if countAfterFirstTab <= countBefore {
		t.Errorf("TAB should expand subtree: had %d nodes, got %d", countBefore, countAfterFirstTab)
	}
}

// TestTreeNavTabCycleBackToFolded verifies that TAB eventually cycles back to folded state.
func TestTreeNavTabCycleBackToFolded(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// First collapse epic-1
	m = sendKey(t, m, "h")
	collapsedCount := m.TreeNodeCount()

	// Cycle through: folded -> children -> subtree -> folded
	m = sendSpecialKey(t, m, tea.KeyTab) // folded -> children visible
	m = sendSpecialKey(t, m, tea.KeyTab) // children -> subtree visible
	m = sendSpecialKey(t, m, tea.KeyTab) // subtree -> folded again

	finalCount := m.TreeNodeCount()
	if finalCount != collapsedCount {
		t.Errorf("TAB cycle should return to folded state: had %d, got %d", collapsedCount, finalCount)
	}
}

// TestTreeNavShiftTabGlobalCycle verifies Shift+TAB cycles global visibility.
// all folded -> top-level only -> all expanded -> all folded
func TestTreeNavShiftTabGlobalCycle(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Press Shift+TAB to cycle to "all folded"
	m = sendSpecialKey(t, m, tea.KeyShiftTab)
	allFoldedCount := m.TreeNodeCount()

	// Should show only root nodes when all folded
	// Roots: epic-1, standalone-1 = 2
	if allFoldedCount != 2 {
		t.Errorf("Shift+TAB all-folded should show 2 roots, got %d", allFoldedCount)
	}

	// Press Shift+TAB again for "top-level children visible"
	m = sendSpecialKey(t, m, tea.KeyShiftTab)
	topLevelCount := m.TreeNodeCount()

	if topLevelCount <= allFoldedCount {
		t.Errorf("Shift+TAB top-level should show more than all-folded: %d vs %d", topLevelCount, allFoldedCount)
	}

	// Press Shift+TAB again for "all expanded"
	m = sendSpecialKey(t, m, tea.KeyShiftTab)
	allExpandedCount := m.TreeNodeCount()

	if allExpandedCount < topLevelCount {
		t.Errorf("Shift+TAB all-expanded should show at least as many as top-level: %d vs %d", allExpandedCount, topLevelCount)
	}

	// Press Shift+TAB again - should cycle back to "all folded"
	m = sendSpecialKey(t, m, tea.KeyShiftTab)
	cycledCount := m.TreeNodeCount()

	if cycledCount != allFoldedCount {
		t.Errorf("Shift+TAB should cycle back to all-folded: expected %d, got %d", allFoldedCount, cycledCount)
	}
}

// TestTreeNavTabOnLeafDoesNothing verifies TAB on a leaf node does nothing.
func TestTreeNavTabOnLeafDoesNothing(t *testing.T) {
	cleanTreeState(t)
	issues := createNavTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Navigate to task-1 (a leaf node)
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() != "task-1" {
		t.Fatalf("expected task-1, got %q", m.TreeSelectedID())
	}

	countBefore := m.TreeNodeCount()
	m = sendSpecialKey(t, m, tea.KeyTab)
	countAfter := m.TreeNodeCount()

	if countBefore != countAfter {
		t.Errorf("TAB on leaf node should not change node count: had %d, got %d", countBefore, countAfter)
	}
}

// ============================================================================
// Feature 3: Level-based expand/collapse (bd-9jr)
// Press 1-9 to expand tree to that depth level.
// ============================================================================

// TestTreeNavLevel1ShowsOnlyRoots verifies pressing '1' shows only root nodes.
func TestTreeNavLevel1ShowsOnlyRoots(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Press '1' to show only roots
	m = sendKey(t, m, "1")
	count := m.TreeNodeCount()

	// Should show 2 roots: epic-1, standalone-1
	if count != 2 {
		t.Errorf("pressing '1' should show only 2 roots, got %d", count)
	}
}

// TestTreeNavLevel2ShowsRootsAndChildren verifies pressing '2' shows roots + direct children.
func TestTreeNavLevel2ShowsRootsAndChildren(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Press '2' to show roots + direct children
	m = sendKey(t, m, "2")
	count := m.TreeNodeCount()

	// Should show: epic-1, task-1, task-2, standalone-1 = 4
	// (subtask-1 at depth 2 should be hidden because level 2 means only depth < 2 expanded)
	if count != 4 {
		t.Errorf("pressing '2' should show 4 nodes (roots + children), got %d", count)
	}
}

// TestTreeNavLevel3ShowsThreeLevels verifies pressing '3' shows 3 levels deep.
func TestTreeNavLevel3ShowsThreeLevels(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Press '3' to show 3 levels
	m = sendKey(t, m, "3")
	count := m.TreeNodeCount()

	// Should show all 5 nodes: epic-1, task-1, subtask-1, task-2, standalone-1
	if count != 5 {
		t.Errorf("pressing '3' should show all 5 nodes (3 levels deep), got %d", count)
	}
}

// TestTreeNavLevel9ExpandsAll verifies pressing '9' expands everything.
func TestTreeNavLevel9ExpandsAll(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// First collapse everything
	m = sendKey(t, m, "Z")
	collapsedCount := m.TreeNodeCount()

	// Press '9' to expand all
	m = sendKey(t, m, "9")
	count := m.TreeNodeCount()

	if count <= collapsedCount {
		t.Errorf("pressing '9' should expand all nodes: had %d collapsed, got %d", collapsedCount, count)
	}

	// Should show all 5 nodes
	if count != 5 {
		t.Errorf("pressing '9' should show all 5 nodes, got %d", count)
	}
}

// TestTreeNavLevelPreservesCursor verifies that level-based expand preserves cursor position.
func TestTreeNavLevelPreservesCursor(t *testing.T) {
	cleanTreeState(t)
	issues := createDeepTreeIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Select epic-1
	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1, got %q", m.TreeSelectedID())
	}

	// Press '1' then '3' - cursor should stay on epic-1
	m = sendKey(t, m, "1")
	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("after '1', expected epic-1 still selected, got %q", m.TreeSelectedID())
	}

	m = sendKey(t, m, "3")
	if m.TreeSelectedID() != "epic-1" {
		t.Errorf("after '3', expected epic-1 still selected, got %q", m.TreeSelectedID())
	}
}
