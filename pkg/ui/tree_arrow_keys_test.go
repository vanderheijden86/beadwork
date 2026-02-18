package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// createTreeTestIssues creates a hierarchy for tree view key testing.
//
//	epic-1 (P1, epic)
//	  task-1 (P2, task, child of epic-1)
//	  task-2 (P2, task, child of epic-1)
//	standalone-1 (P2, task, no parent)
func createTreeTestIssues() []model.Issue {
	now := time.Now()
	// CreatedAt values are set so that default sort (Created desc) produces
	// the expected display order: epic-1, task-1, task-2, standalone-1 (bd-2ty).
	return []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now.Add(3 * time.Second)},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(2 * time.Second),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
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

// enterTreeView ensures the model is in tree view. If already in tree view
// (bd-dxc: tree is now the default), it returns immediately. Otherwise it
// presses "E" to switch and verifies focus changed.
func enterTreeView(t *testing.T, m ui.Model) ui.Model {
	t.Helper()
	if m.FocusState() == "tree" {
		return m // Already in tree view (bd-dxc default)
	}
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
	m := ui.NewModel(issues, "")
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
	m := ui.NewModel(issues, "")
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
	m1 := ui.NewModel(issues, "")
	m1 = enterTreeView(t, m1)
	m1 = sendKey(t, m1, "j")
	jID := m1.TreeSelectedID()

	// Path 2: Use Down arrow
	m2 := ui.NewModel(issues, "")
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
	m := ui.NewModel(issues, "")

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
	m := ui.NewModel(issues, "")

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
		// CreatedAt descends so that default sort (Created desc) preserves
		// the expected display order: t-0000 first, t-NNNN last (bd-2ty).
		issues[i] = model.Issue{
			ID:        fmt.Sprintf("t-%04d", i),
			Title:     fmt.Sprintf("Task %04d", i),
			Status:    model.StatusOpen,
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
		}
	}
	return issues
}

// TestTreeViewPageIndicator verifies that the tree view renders a page indicator
// in the same format as the list view: "Page X/Y (start-end of total)".
func TestTreeViewPageIndicator(t *testing.T) {
	issues := createManyTreeIssues(100)
	m := ui.NewModel(issues, "")

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
	m := ui.NewModel(issues, "")

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
	m := ui.NewModel(issues, "")

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
	m := ui.NewModel(issues, "")
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
	m := ui.NewModel(issues, "")
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
	m := ui.NewModel(issues, "")
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
	m := ui.NewModel(issues, "")
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
// Tests: Sort popup menu integration (bd-u81)
// ============================================================================

// TestTreeViewSortPopupOpensOnS verifies that 's' opens the sort popup overlay.
func TestTreeViewSortPopupOpensOnS(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Popup should be closed initially
	if m.TreeSortPopupOpen() {
		t.Fatal("sort popup should be closed initially")
	}

	// Press 's' to open popup
	m = sendKey(t, m, "s")
	if !m.TreeSortPopupOpen() {
		t.Error("sort popup should be open after pressing 's'")
	}
}

// TestTreeViewSortPopupEscCloses verifies that Esc closes the sort popup without changing sort.
func TestTreeViewSortPopupEscCloses(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	initialField := m.TreeSortField()
	initialDir := m.TreeSortDirection()

	// Open popup then close with Esc
	m = sendKey(t, m, "s")
	m = sendSpecialKey(t, m, tea.KeyEsc)

	if m.TreeSortPopupOpen() {
		t.Error("sort popup should be closed after Esc")
	}
	if m.TreeSortField() != initialField {
		t.Errorf("sort field should be unchanged after Esc close, got %v", m.TreeSortField())
	}
	if m.TreeSortDirection() != initialDir {
		t.Errorf("sort direction should be unchanged after Esc close, got %v", m.TreeSortDirection())
	}
}

// TestTreeViewSortPopupSKeyCloses verifies that pressing 's' again closes the popup.
func TestTreeViewSortPopupSKeyCloses(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Open popup
	m = sendKey(t, m, "s")
	if !m.TreeSortPopupOpen() {
		t.Fatal("sort popup should be open after 's'")
	}

	// Press 's' again to close
	m = sendKey(t, m, "s")
	if m.TreeSortPopupOpen() {
		t.Error("sort popup should be closed after pressing 's' again")
	}
}

// TestTreeViewSortPopupSelectChangesSort verifies that navigating and pressing Enter
// in the popup changes the sort field.
func TestTreeViewSortPopupSelectChangesSort(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Default sort: Created descending (bd-ctu)
	if m.TreeSortField() != ui.SortFieldCreated {
		t.Fatalf("expected initial sort Created, got %v", m.TreeSortField())
	}

	// Open popup, navigate down to Title, press Enter
	m = sendKey(t, m, "s")
	m = sendKey(t, m, "j") // Updated
	m = sendKey(t, m, "j") // Title
	m = sendSpecialKey(t, m, tea.KeyEnter)

	if m.TreeSortPopupOpen() {
		t.Error("popup should close after Enter")
	}
	if m.TreeSortField() != ui.SortFieldTitle {
		t.Errorf("expected Title after selecting, got %v", m.TreeSortField())
	}
}

// TestTreeViewSortPopupToggleDirection verifies that selecting the current sort field
// toggles its direction (asc/desc).
func TestTreeViewSortPopupToggleDirection(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	initialDir := m.TreeSortDirection()

	// Open popup and select current field (Created) to toggle direction
	m = sendKey(t, m, "s")
	m = sendSpecialKey(t, m, tea.KeyEnter)

	if m.TreeSortField() != ui.SortFieldCreated {
		t.Errorf("sort field should remain Created, got %v", m.TreeSortField())
	}
	if m.TreeSortDirection() == initialDir {
		t.Error("sort direction should have toggled after selecting the current field")
	}
}

// TestTreeViewSortPopupDoesNotAffectNavigation verifies that j/k are consumed by the
// popup when open (not passed to tree navigation), and after closing, navigation works.
func TestTreeViewSortPopupDoesNotAffectNavigation(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	idBefore := m.TreeSelectedID()

	// Open popup, press j (should move popup cursor, not tree cursor)
	m = sendKey(t, m, "s")
	m = sendKey(t, m, "j")

	if m.TreeSelectedID() != idBefore {
		t.Error("j inside popup should not move tree cursor")
	}

	// Close popup with Esc
	m = sendSpecialKey(t, m, tea.KeyEsc)

	// Now j should move the tree cursor
	m = sendKey(t, m, "j")
	if m.TreeSelectedID() == idBefore {
		t.Error("j after popup close should move tree cursor")
	}
}

// ============================================================================
// Tests: Backtick key toggles flat/tree mode (bd-39v)
// ============================================================================

// TestTreeViewBacktickTogglesFlatMode verifies that the backtick key toggles
// between flat and tree mode within the tree view.
func TestTreeViewBacktickTogglesFlatMode(t *testing.T) {
	// Use flat issues (no hierarchy) to avoid stale tree-state.json affecting results
	issues := createManyTreeIssues(10)
	m := ui.NewModel(issues, "")
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

// ============================================================================
// Tests: Bookmark key bindings (bd-k4n)
// ============================================================================

// TestTreeViewBKeyOpensBoard verifies 'b' from tree toggles board view (bd-8hw.4)
func TestTreeViewBKeyOpensBoard(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Press 'b' to enter board view
	m = sendKey(t, m, "b")

	if !m.IsBoardView() {
		t.Error("Expected board view after 'b' in tree")
	}
	if m.FocusState() != "board" {
		t.Errorf("Expected focus 'board', got %q", m.FocusState())
	}

	// Press 'b' again to return to tree
	m = sendKey(t, m, "b")
	if m.IsBoardView() {
		t.Error("Expected board off after second 'b'")
	}
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus 'tree' after board toggle, got %q", m.FocusState())
	}
}

// ============================================================================
// Tests: Follow mode key binding (bd-c0c)
// ============================================================================

// TestTreeViewFollowModeToggleKey verifies 'F' key toggles follow mode
func TestTreeViewFollowModeToggleKey(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	if m.TreeFollowMode() {
		t.Error("Expected follow mode off initially")
	}

	// Press 'F' to enable follow mode
	m = sendKey(t, m, "F")
	if !m.TreeFollowMode() {
		t.Error("Expected follow mode on after 'F'")
	}

	// Press 'F' again to disable
	m = sendKey(t, m, "F")
	if m.TreeFollowMode() {
		t.Error("Expected follow mode off after second 'F'")
	}
}

// ============================================================================
// Detail panel toggle tests (bd-80u)
// ============================================================================

// TestTreeViewDetailToggle verifies 'd' key toggles treeDetailHidden.
func TestTreeViewDetailToggle(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	if m.TreeDetailHidden() {
		t.Error("Expected detail visible by default")
	}

	// Press 'd' to hide detail
	m = sendKey(t, m, "d")
	if !m.TreeDetailHidden() {
		t.Error("Expected detail hidden after 'd'")
	}

	// Press 'd' again to show detail
	m = sendKey(t, m, "d")
	if m.TreeDetailHidden() {
		t.Error("Expected detail visible after second 'd'")
	}
}

// TestTreeViewSpaceInTreeOnlyShowsDetail verifies Space in tree-only mode
// switches focus to detail (detail-only full-screen view, bd-8zc).
func TestTreeViewSpaceInTreeOnlyShowsDetail(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Hide detail panel
	m = sendKey(t, m, "d")
	if !m.TreeDetailHidden() {
		t.Fatal("Expected detail hidden after 'd'")
	}

	// Press Space - should switch to detail focus (bd-8zc)
	m = sendKey(t, m, " ")
	if m.FocusState() != "detail" {
		t.Errorf("Expected focus 'detail' after Space in tree-only mode, got %q", m.FocusState())
	}
}

// TestTreeViewEscFromDetailOnlyReturnsToTree verifies ESC from detail-only
// mode (entered via Space in tree-only) returns focus to tree.
func TestTreeViewEscFromDetailOnlyReturnsToTree(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Hide detail, enter detail-only via Space (bd-8zc)
	m = sendKey(t, m, "d")
	m = sendKey(t, m, " ")
	if m.FocusState() != "detail" {
		t.Fatalf("Expected focus 'detail', got %q", m.FocusState())
	}

	// Press ESC - should return to tree focus
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(ui.Model)
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus 'tree' after ESC from detail-only, got %q", m.FocusState())
	}
	// Detail should still be hidden (tree-only mode preserved)
	if !m.TreeDetailHidden() {
		t.Error("Expected detail still hidden after ESC from detail-only")
	}
}

// TestTreeViewEnterExpandsCollapseInSplitMode verifies Enter toggles
// expand/collapse in split mode (bd-8zc: Enter always expands/collapses).
func TestTreeViewEnterExpandsCollapseInSplitMode(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Set wide terminal for split view
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// Ensure detail is visible (split mode default)
	if m.TreeDetailHidden() {
		t.Fatal("Expected detail visible in split mode")
	}

	// Enter should toggle expand/collapse, NOT switch focus (bd-8zc).
	m = sendSpecialKey(t, m, tea.KeyEnter)
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus to remain 'tree' after Enter in split mode, got %q", m.FocusState())
	}
	if m.TreeDetailHidden() {
		t.Error("Expected detail to remain visible after Enter in split mode")
	}
}

// TestTreeViewSpaceOpenDetailInSplitMode verifies Space switches focus to detail
// in split mode (bd-8zc).
func TestTreeViewSpaceOpenDetailInSplitMode(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Set wide terminal for split view
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	if m.TreeDetailHidden() {
		t.Fatal("Expected detail visible in split mode")
	}

	// Space should switch focus to detail pane (bd-8zc)
	m = sendKey(t, m, " ")
	if m.FocusState() != "detail" {
		t.Errorf("Expected focus 'detail' after Space in split mode, got %q", m.FocusState())
	}
}

// TestTreeViewTabSkipsDetailWhenHidden verifies Tab is no-op when detail
// panel is hidden in tree-only mode.
func TestTreeViewTabSkipsDetailWhenHidden(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Make it a split view so Tab normally works
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// Hide detail
	m = sendKey(t, m, "d")

	// Press Tab - should stay on tree (no detail to switch to)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus to remain 'tree' after Tab with detail hidden, got %q", m.FocusState())
	}
}

// TestTreeViewDetailToggleResetsFromDetail verifies that pressing 'd' when
// focused on detail snaps focus back to tree.
func TestTreeViewDetailToggleResetsFromDetail(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Make it a split view so Tab works
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	// Tab to detail
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)
	if m.FocusState() != "detail" {
		t.Fatalf("Expected focus 'detail' after Tab, got %q", m.FocusState())
	}

	// Press 'd' to hide detail - focus should snap to tree
	// Note: 'd' in detail panel is handled in handleTreeKeys when treeViewActive
	// We need to verify this works from the global handler
	m = sendKey(t, m, "d")
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus 'tree' after 'd' from detail, got %q", m.FocusState())
	}
	if !m.TreeDetailHidden() {
		t.Error("Expected detail hidden after 'd' from detail")
	}
}

// ============================================================================
// Tests: Auto-hide detail panel on narrow terminal (bd-dy7)
// ============================================================================

// TestTreeDetailAutoHideNarrowTerminal verifies that the detail panel is
// automatically hidden when the terminal is too narrow for readable content.
func TestTreeDetailAutoHideNarrowTerminal(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Enter tree view
	m = enterTreeView(t, m)

	// Wide terminal with default ratio (0.4) — detail should be visible.
	// At width 200: availWidth=192, detail=192*0.6=115 (> 40)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	if m.TreeDetailHidden() {
		t.Error("Expected detail visible at width 200 with default ratio")
	}

	// Increase split pane ratio to 0.8 using '>' key (8 presses from 0.4).
	// Each '>' adds 0.05 to the ratio.
	for i := 0; i < 8; i++ {
		m = sendKey(t, m, ">")
	}

	// Re-send WindowSizeMsg to trigger auto-hide check.
	// With ratio 0.8 at width 200: availWidth=192, detail=192*0.2=38 (< 40 -> hidden)
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	if !m.TreeDetailHidden() {
		t.Error("Expected detail auto-hidden at width 200 with ratio 0.8 (detail pane ~38 chars)")
	}
}

// TestTreeDetailAutoShowOnWiderTerminal verifies that widening the terminal
// re-shows the detail panel after it was auto-hidden.
func TestTreeDetailAutoShowOnWiderTerminal(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Wide enough with default ratio
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	// Increase split ratio to 0.8
	for i := 0; i < 8; i++ {
		m = sendKey(t, m, ">")
	}

	// At width 200 with ratio 0.8: detail=38 -> auto-hide
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)
	if !m.TreeDetailHidden() {
		t.Fatal("Expected detail auto-hidden at width 200 with ratio 0.8")
	}

	// Widen to 260: availWidth=252, detail=252*0.2=50 (> 40 -> visible)
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 260, Height: 40})
	m = newM.(ui.Model)

	if m.TreeDetailHidden() {
		t.Error("Expected detail visible at width 260 with ratio 0.8 (detail pane ~50 chars)")
	}
}

// TestTreeDetailAutoHideManualToggleStillWorks verifies that after auto-hide,
// the user can still manually show/hide the detail with 'd'.
func TestTreeDetailAutoHideManualToggleStillWorks(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Wide terminal -- detail visible
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	if m.TreeDetailHidden() {
		t.Fatal("Expected detail visible at width 200")
	}

	// User manually hides with 'd'
	m = sendKey(t, m, "d")
	if !m.TreeDetailHidden() {
		t.Error("Expected detail hidden after manual 'd' toggle")
	}

	// User manually shows with 'd' again
	m = sendKey(t, m, "d")
	if m.TreeDetailHidden() {
		t.Error("Expected detail visible after second 'd' toggle")
	}
}

// TestTreeDetailAutoHideFocusSnap verifies that when the detail panel is
// auto-hidden while the user is focused on the detail pane, focus snaps to tree.
func TestTreeDetailAutoHideFocusSnap(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Wide terminal, enter tree view
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)
	m = enterTreeView(t, m)

	// Tab to detail
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)
	if m.FocusState() != "detail" {
		t.Fatalf("Expected focus 'detail' after Tab, got %q", m.FocusState())
	}

	// Tab back to tree so we can adjust ratio
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)

	// Increase ratio to 0.8
	for i := 0; i < 8; i++ {
		m = sendKey(t, m, ">")
	}

	// Tab to detail
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)

	// Resize to trigger auto-hide while focused on detail.
	// With ratio 0.8 at width 200: detail=38 -> auto-hide
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m = newM.(ui.Model)

	// Focus should have snapped to tree
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus snapped to 'tree' after auto-hide, got %q", m.FocusState())
	}
	if !m.TreeDetailHidden() {
		t.Error("Expected detail auto-hidden")
	}
}

// ============================================================================
// Tests: Key remapping — Enter=expand, Space=detail, no TAB/1-9 in tree (bd-8zc)
// ============================================================================

// TestTreeViewEnterExpandsCollapses verifies Enter toggles expand/collapse
// on the current tree node (CycleNodeVisibility), NOT opening detail view.
func TestTreeViewEnterExpandsCollapses(t *testing.T) {
	issues := createTreeTestIssues() // epic-1 with children task-1, task-2
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Cursor should be on epic-1 (root, expanded by default)
	if m.TreeSelectedID() != "epic-1" {
		t.Fatalf("expected epic-1 selected, got %q", m.TreeSelectedID())
	}

	initialCount := m.TreeNodeCount()
	if initialCount < 3 {
		t.Fatalf("expected at least 3 visible nodes (epic + 2 tasks + standalone), got %d", initialCount)
	}

	// Press Enter — should collapse epic-1 (hide children)
	m = sendSpecialKey(t, m, tea.KeyEnter)

	afterCollapse := m.TreeNodeCount()
	if afterCollapse >= initialCount {
		t.Errorf("expected fewer nodes after Enter (collapse), got %d (was %d)", afterCollapse, initialCount)
	}

	// Focus should still be tree, NOT detail
	if m.FocusState() != "tree" {
		t.Errorf("Enter should NOT switch focus, expected 'tree', got %q", m.FocusState())
	}

	// Press Enter again — should expand epic-1 (show children again)
	m = sendSpecialKey(t, m, tea.KeyEnter)

	afterExpand := m.TreeNodeCount()
	if afterExpand != initialCount {
		t.Errorf("expected %d nodes after second Enter (expand), got %d", initialCount, afterExpand)
	}
}

// TestTreeViewSpaceOpensDetailView verifies Space opens the detail view
// of the selected ticket when in tree-only mode (treeDetailHidden).
func TestTreeViewSpaceOpensDetailView(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Use narrow width (< SplitViewThreshold=100) to get tree-only mode
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	if !m.TreeDetailHidden() {
		t.Fatal("expected treeDetailHidden=true in narrow mode")
	}

	// Press Space — should open detail view
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	m = newM.(ui.Model)

	if m.FocusState() != "detail" {
		t.Errorf("expected focus 'detail' after Space, got %q", m.FocusState())
	}
}

// TestTreeViewEnterDoesNotOpenDetail verifies Enter does NOT open detail view
// even when treeDetailHidden is true (Enter is for expand/collapse only).
func TestTreeViewEnterDoesNotOpenDetail(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")

	// Use narrow width to get tree-only mode
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	m = newM.(ui.Model)

	m = enterTreeView(t, m)

	if !m.TreeDetailHidden() {
		t.Fatal("expected treeDetailHidden=true in narrow mode")
	}

	// Press Enter — should NOT open detail, should expand/collapse
	m = sendSpecialKey(t, m, tea.KeyEnter)

	if m.FocusState() != "tree" {
		t.Errorf("Enter should NOT switch to detail, expected 'tree', got %q", m.FocusState())
	}
}

// TestTreeViewNumberKeysDontExpandLevels verifies that 1-9 keys in tree do NOT
// trigger ExpandToLevel anymore (reserved for project switching).
func TestTreeViewNumberKeysDontExpandLevels(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	initialCount := m.TreeNodeCount()

	// Press "1" — should NOT change tree structure (no ExpandToLevel)
	m = sendKey(t, m, "1")

	afterOne := m.TreeNodeCount()
	if afterOne != initialCount {
		t.Errorf("pressing '1' should not change tree (no ExpandToLevel), count went from %d to %d", initialCount, afterOne)
	}
}

// TestTreeViewTabDoesNotCycleVisibility verifies that TAB in tree does NOT
// call CycleNodeVisibility (it's reserved for tree/detail focus switching).
func TestTreeViewTabDoesNotCycleVisibility(t *testing.T) {
	issues := createTreeTestIssues()
	m := ui.NewModel(issues, "")
	m = enterTreeView(t, m)

	// Cursor on epic-1 (expanded)
	initialCount := m.TreeNodeCount()

	// Press TAB — should NOT change visibility
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(ui.Model)

	afterTab := m.TreeNodeCount()
	if afterTab != initialCount {
		t.Errorf("TAB should not cycle visibility, count went from %d to %d", initialCount, afterTab)
	}
}
