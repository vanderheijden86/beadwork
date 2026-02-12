package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
)

// createTreeTestIssues creates a hierarchy for tree view key testing.
//
//	epic-1 (P1, epic)
//	  task-1 (P2, task, child of epic-1)
//	  task-2 (P2, task, child of epic-1)
//	standalone-1 (P2, task, no parent)
func createTreeTestIssues() []model.Issue {
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
		{ID: "standalone-1", Title: "Standalone", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(3 * time.Second)},
	}
}

// enterTreeView presses "E" to enter tree view and verifies focus changed.
func enterTreeView(t *testing.T, m ui.Model) ui.Model {
	t.Helper()
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = newM.(ui.Model)
	if m.FocusState() != "tree" {
		t.Fatalf("Expected focus 'tree' after E, got %q", m.FocusState())
	}
	return m
}

// sendKey sends a rune key message through Update.
func sendKey(t *testing.T, m ui.Model, key string) ui.Model {
	t.Helper()
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return newM.(ui.Model)
}

// sendSpecialKey sends a special key (arrow, etc.) through Update.
func sendSpecialKey(t *testing.T, m ui.Model, keyType tea.KeyType) ui.Model {
	t.Helper()
	newM, _ := m.Update(tea.KeyMsg{Type: keyType})
	return newM.(ui.Model)
}

// TestTreeViewArrowDownMovesSelection verifies that the Down arrow key
// moves the tree cursor down, just like 'j'.
func TestTreeViewArrowDownMovesSelection(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Record initial selection
	initialID := m.TreeSelectedID()
	if initialID == "" {
		t.Fatal("Expected non-empty initial tree selection")
	}

	// Press Down arrow
	m = sendSpecialKey(t, m, tea.KeyDown)

	afterDownID := m.TreeSelectedID()
	if afterDownID == initialID {
		t.Errorf("Down arrow did not change selection: still %q", afterDownID)
	}
	if afterDownID == "" {
		t.Error("Down arrow resulted in empty selection")
	}
}

// TestTreeViewArrowUpMovesSelection verifies that the Up arrow key
// moves the tree cursor up, just like 'k'.
func TestTreeViewArrowUpMovesSelection(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Move down first with 'j' (known working)
	m = sendKey(t, m, "j")
	afterJID := m.TreeSelectedID()

	// Move down again
	m = sendKey(t, m, "j")
	afterJJID := m.TreeSelectedID()
	if afterJJID == afterJID {
		t.Fatal("j key didn't move cursor, can't test Up arrow")
	}

	// Press Up arrow
	m = sendSpecialKey(t, m, tea.KeyUp)
	afterUpID := m.TreeSelectedID()

	if afterUpID == afterJJID {
		t.Errorf("Up arrow did not change selection: still %q", afterUpID)
	}
	if afterUpID != afterJID {
		t.Errorf("Up arrow should return to %q, got %q", afterJID, afterUpID)
	}
}

// TestTreeViewArrowKeysParity verifies that arrow keys produce the same
// cursor movement as j/k vim keys.
func TestTreeViewArrowKeysParity(t *testing.T) {
	issues := createTreeTestIssues()

	// Path 1: Use j key
	m1 := ui.NewModel(issues, nil, "")
	m1 = enterTreeView(t, m1)
	m1 = sendKey(t, m1, "j")
	jID := m1.TreeSelectedID()

	// Path 2: Use Down arrow
	m2 := ui.NewModel(issues, nil, "")
	m2 = enterTreeView(t, m2)
	m2 = sendSpecialKey(t, m2, tea.KeyDown)
	downID := m2.TreeSelectedID()

	if jID != downID {
		t.Errorf("j key selected %q but Down arrow selected %q (should be identical)", jID, downID)
	}

	// Continue: k vs Up
	m1 = sendKey(t, m1, "k")
	kID := m1.TreeSelectedID()

	m2 = sendSpecialKey(t, m2, tea.KeyUp)
	upID := m2.TreeSelectedID()

	if kID != upID {
		t.Errorf("k key selected %q but Up arrow selected %q (should be identical)", kID, upID)
	}
}

// TestTreeViewArrowLeftPageBack verifies that Left arrow does page-backward
// navigation (not collapse) in tree view.
func TestTreeViewArrowLeftPageBack(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, nil, "")

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// Go to page 2 first
	m = sendSpecialKey(t, m, tea.KeyRight)
	page2ID := m.TreeSelectedID()

	// Left arrow should go back to page 1
	m = sendSpecialKey(t, m, tea.KeyLeft)
	afterLeftID := m.TreeSelectedID()

	if afterLeftID == page2ID {
		t.Errorf("Left arrow should page backward, but selection didn't change from %q", page2ID)
	}
	if afterLeftID != "t-0000" {
		t.Errorf("Left arrow from page 2 should return to page 1 start (t-0000), got %q", afterLeftID)
	}
}

// TestTreeViewArrowRightPageForward verifies that Right arrow does page-forward
// navigation (not expand) in tree view.
func TestTreeViewArrowRightPageForward(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, nil, "")

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	initialID := m.TreeSelectedID()
	if initialID != "t-0000" {
		t.Fatalf("Expected initial selection t-0000, got %q", initialID)
	}

	// Right arrow should advance a full page
	m = sendSpecialKey(t, m, tea.KeyRight)
	afterRightID := m.TreeSelectedID()

	if afterRightID == initialID {
		t.Errorf("Right arrow should page forward, but selection didn't change")
	}
	if afterRightID == "t-0001" {
		t.Errorf("Right arrow should jump a full page, not just one item (got t-0001)")
	}
}

// ============================================================================
// Tests: Page-based pagination (matching list view behavior)
// ============================================================================

// createManyTreeIssues creates n root-level issues (no hierarchy) for pagination tests.
func createManyTreeIssues(n int) []model.Issue {
	now := time.Now()
	issues := make([]model.Issue, n)
	for i := 0; i < n; i++ {
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("t-%04d", i),
			Title:     fmt.Sprintf("Task %04d", i),
			Status:    model.StatusOpen,
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
	}
	return issues
}

// TestTreeViewPageIndicator verifies that the tree view renders a page indicator
// in the same format as the list view: "Page X/Y (start-end of total)".
func TestTreeViewPageIndicator(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, nil, "")

	// Set a realistic terminal size so the tree has a known page size
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	view := m.View()
	if !strings.Contains(view, "Page") {
		t.Errorf("Tree view should show 'Page X/Y' indicator, but it was not found in view output")
	}
	// Should show page 1 of something
	if !strings.Contains(view, "Page 1/") {
		t.Errorf("Tree view should show 'Page 1/' when at top, but not found.\nLooking for page indicator in output (last 500 chars):\n%s", lastN(view, 500))
	}
}

// TestTreeViewPageForward verifies that Right arrow in tree view moves to the next page.
func TestTreeViewPageForward(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, nil, "")

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// Verify we start on page 1
	initialID := m.TreeSelectedID()
	if initialID != "t-0000" {
		t.Fatalf("Expected initial selection t-0000, got %q", initialID)
	}

	// Press Right arrow to go to next page
	m = sendSpecialKey(t, m, tea.KeyRight)

	afterRightID := m.TreeSelectedID()
	// Should have jumped by a full page (not just one item)
	if afterRightID == initialID {
		t.Error("Right arrow should advance to next page, but selection didn't change")
	}
	// The selection should be far from the initial position (a full page jump)
	if afterRightID == "t-0001" {
		t.Error("Right arrow should jump a full page, not just one item (got t-0001)")
	}
}

// TestTreeViewPageBackward verifies that Left arrow in tree view moves to the previous page.
func TestTreeViewPageBackward(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, nil, "")

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// First go to page 2 with Right
	m = sendSpecialKey(t, m, tea.KeyRight)
	page2ID := m.TreeSelectedID()

	// Now go back with Left
	m = sendSpecialKey(t, m, tea.KeyLeft)
	afterLeftID := m.TreeSelectedID()

	if afterLeftID == page2ID {
		t.Error("Left arrow should go back to previous page, but selection didn't change")
	}
	// Should be back at (or near) the initial position
	if afterLeftID != "t-0000" {
		t.Errorf("Left arrow from page 2 should return to page 1 start (t-0000), got %q", afterLeftID)
	}
}

// TestTreeViewHLStillCollapseExpand verifies that h/l keys still collapse/expand
// (not pagination) after remapping left/right to pagination.
func TestTreeViewHLStillCollapseExpand(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	initialCount := m.TreeNodeCount()
	if initialCount <= 1 {
		t.Fatalf("Expected multiple visible nodes, got %d", initialCount)
	}

	// 'h' should still collapse
	m = sendKey(t, m, "h")
	afterH := m.TreeNodeCount()
	if afterH >= initialCount {
		t.Errorf("'h' should collapse, reducing count from %d, got %d", initialCount, afterH)
	}

	// 'l' should still expand
	m = sendKey(t, m, "l")
	afterL := m.TreeNodeCount()
	if afterL <= afterH {
		t.Errorf("'l' should expand, increasing count from %d, got %d", afterH, afterL)
	}
}

// sendCtrlKey sends a ctrl+key message through Update.
func sendCtrlKey(t *testing.T, m ui.Model, keyType tea.KeyType) ui.Model {
	t.Helper()
	newM, _ := m.Update(tea.KeyMsg{Type: keyType})
	return newM.(ui.Model)
}

// ============================================================================
// Tests: Ctrl+A expand all / collapse all toggle
// ============================================================================

// TestTreeViewCtrlAExpandsAll verifies that Ctrl+A expands all nodes
// when some nodes are collapsed.
func TestTreeViewCtrlAExpandsAll(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// First collapse all with 'Z'
	m = sendKey(t, m, "Z")
	collapsedCount := m.TreeNodeCount()

	// Now press Ctrl+A to toggle — should expand all
	m = sendCtrlKey(t, m, tea.KeyCtrlA)
	afterToggle := m.TreeNodeCount()

	if afterToggle <= collapsedCount {
		t.Errorf("Ctrl+A should expand all nodes when collapsed: had %d, got %d", collapsedCount, afterToggle)
	}
}

// TestTreeViewCtrlACollapsesAll verifies that Ctrl+A collapses all nodes
// when all nodes are already expanded.
func TestTreeViewCtrlACollapsesAll(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// First ensure all expanded with 'X'
	m = sendKey(t, m, "X")
	expandedCount := m.TreeNodeCount()
	if expandedCount <= 2 {
		t.Fatalf("Expected more than 2 nodes after expand all, got %d", expandedCount)
	}

	// Now press Ctrl+A to toggle — should collapse all
	m = sendCtrlKey(t, m, tea.KeyCtrlA)
	afterToggle := m.TreeNodeCount()

	if afterToggle >= expandedCount {
		t.Errorf("Ctrl+A should collapse all nodes when expanded: had %d, got %d", expandedCount, afterToggle)
	}
}

// TestTreeViewCtrlATogglesCycle verifies that Ctrl+A toggles:
// collapsed → expanded → collapsed
func TestTreeViewCtrlATogglesCycle(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	// Collapse all first
	m = sendKey(t, m, "Z")
	collapsedCount := m.TreeNodeCount()

	// First Ctrl+A: expand all
	m = sendCtrlKey(t, m, tea.KeyCtrlA)
	expandedCount := m.TreeNodeCount()
	if expandedCount <= collapsedCount {
		t.Fatalf("First Ctrl+A should expand: had %d, got %d", collapsedCount, expandedCount)
	}

	// Second Ctrl+A: collapse all
	m = sendCtrlKey(t, m, tea.KeyCtrlA)
	afterSecond := m.TreeNodeCount()
	if afterSecond >= expandedCount {
		t.Errorf("Second Ctrl+A should collapse: had %d, got %d", expandedCount, afterSecond)
	}
	if afterSecond != collapsedCount {
		t.Errorf("Second Ctrl+A should return to collapsed count %d, got %d", collapsedCount, afterSecond)
	}
}

// lastN returns the last n characters of s.
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ============================================================================
// Tests: Backtick key toggles flat/tree mode (bd-39v)
// ============================================================================

// TestTreeViewBacktickTogglesFlatMode verifies that the backtick key toggles
// between flat and tree mode within the tree view.
func TestTreeViewBacktickTogglesFlatMode(t *testing.T) {
	// Use flat issues (no hierarchy) to avoid stale tree-state.json affecting results
	issues := createManyTreeIssues(10)
	m := ui.NewModel(issues, nil, "")
	m = enterTreeView(t, m)

	initialCount := m.TreeNodeCount()
	if initialCount == 0 {
		t.Fatal("expected non-zero initial tree node count")
	}

	// Press backtick to toggle flat mode
	m = sendKey(t, m, "`")

	// In flat mode, all issues should be visible
	flatCount := m.TreeNodeCount()
	if flatCount != len(issues) {
		t.Errorf("expected %d nodes in flat mode, got %d", len(issues), flatCount)
	}

	// Press backtick again to toggle back to tree mode
	m = sendKey(t, m, "`")

	afterToggleBack := m.TreeNodeCount()
	if afterToggleBack != initialCount {
		t.Errorf("expected %d nodes after toggling back to tree, got %d", initialCount, afterToggleBack)
	}
}
