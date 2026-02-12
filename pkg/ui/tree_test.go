package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/charmbracelet/lipgloss"
)

func newTreeTestTheme() Theme {
	return DefaultTheme(lipgloss.NewRenderer(nil))
}

// TestTreeBuildEmpty verifies Build() handles empty issues slice
func TestTreeBuildEmpty(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(nil)

	if !tree.IsBuilt() {
		t.Error("expected tree to be marked as built")
	}
	if tree.RootCount() != 0 {
		t.Errorf("expected 0 roots, got %d", tree.RootCount())
	}
	if tree.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", tree.NodeCount())
	}
}

// TestTreeBuildNoHierarchy verifies all issues become roots when no parent-child deps
func TestTreeBuildNoHierarchy(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Title: "Task 1", Priority: 1, IssueType: model.TypeTask},
		{ID: "bv-2", Title: "Task 2", Priority: 2, IssueType: model.TypeTask},
		{ID: "bv-3", Title: "Task 3", Priority: 0, IssueType: model.TypeBug},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	if tree.RootCount() != 3 {
		t.Errorf("expected 3 roots (no hierarchy), got %d", tree.RootCount())
	}
	if tree.NodeCount() != 3 {
		t.Errorf("expected 3 visible nodes, got %d", tree.NodeCount())
	}
}

// TestTreeBuildParentChild verifies proper nesting with parent-child deps
func TestTreeBuildParentChild(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task under Epic", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "subtask-1", Title: "Subtask", Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "subtask-1", DependsOnID: "task-1", Type: model.DepParentChild},
			},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Should have 1 root (epic-1)
	if tree.RootCount() != 1 {
		t.Errorf("expected 1 root, got %d", tree.RootCount())
	}

	// With depth < 2 auto-expand, all 3 should be visible
	if tree.NodeCount() != 3 {
		t.Errorf("expected 3 visible nodes (auto-expanded), got %d", tree.NodeCount())
	}

	// Verify hierarchy structure
	root := tree.roots[0]
	if root.Issue.ID != "epic-1" {
		t.Errorf("expected root to be epic-1, got %s", root.Issue.ID)
	}
	if len(root.Children) != 1 {
		t.Errorf("expected epic to have 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Issue.ID != "task-1" {
		t.Errorf("expected child to be task-1, got %s", root.Children[0].Issue.ID)
	}
	if len(root.Children[0].Children) != 1 {
		t.Errorf("expected task to have 1 child, got %d", len(root.Children[0].Children))
	}
	if root.Children[0].Children[0].Issue.ID != "subtask-1" {
		t.Errorf("expected grandchild to be subtask-1, got %s", root.Children[0].Children[0].Issue.ID)
	}
}

// TestTreeBuildOrphanParent verifies issues with non-existent parent become roots
func TestTreeBuildOrphanParent(t *testing.T) {
	issues := []model.Issue{
		{ID: "root-1", Title: "Root", Priority: 1, IssueType: model.TypeTask},
		{
			ID: "orphan-1", Title: "Orphan with missing parent", Priority: 2, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "orphan-1", DependsOnID: "nonexistent-parent", Type: model.DepParentChild},
			},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// orphan-1 declares a parent that doesn't exist in the issue set.
	// Rather than disappearing from the tree entirely (bad UX), orphan-1
	// should be treated as a root - its parent reference is dangling.

	if tree.RootCount() != 2 {
		t.Errorf("expected 2 roots (orphan with missing parent becomes root), got %d", tree.RootCount())
	}
	// Both issues should be visible as roots
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes, got %d", tree.NodeCount())
	}
}

// TestTreeBuildCycleDetection verifies cycles are handled gracefully
func TestTreeBuildCycleDetection(t *testing.T) {
	// Create a cycle: A -> B -> A (A is parent of B, B is parent of A)
	// This shouldn't cause infinite recursion
	issues := []model.Issue{
		{
			ID: "cycle-a", Title: "Cycle A", Priority: 1, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "cycle-a", DependsOnID: "cycle-b", Type: model.DepParentChild},
			},
		},
		{
			ID: "cycle-b", Title: "Cycle B", Priority: 1, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "cycle-b", DependsOnID: "cycle-a", Type: model.DepParentChild},
			},
		},
	}

	// This should not hang or panic
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Both issues have parents, so neither is a root in the normal sense
	// But they form a cycle, which the algorithm handles
	if !tree.IsBuilt() {
		t.Error("expected tree to be built despite cycle")
	}
	// With the cycle, both have parents, so there are no roots
	// This is correct behavior - a pure cycle has no entry point
}

// TestTreeBuildChildSorting verifies children are sorted by priority, type, date
func TestTreeBuildChildSorting(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "parent", Title: "Parent", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child-p2-task", Title: "P2 Task", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child-p2-task", DependsOnID: "parent", Type: model.DepParentChild}},
		},
		{
			ID: "child-p1-bug", Title: "P1 Bug", Priority: 1, IssueType: model.TypeBug, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child-p1-bug", DependsOnID: "parent", Type: model.DepParentChild}},
		},
		{
			ID: "child-p1-task", Title: "P1 Task", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(3 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child-p1-task", DependsOnID: "parent", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	if tree.RootCount() != 1 {
		t.Fatalf("expected 1 root, got %d", tree.RootCount())
	}

	children := tree.roots[0].Children
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}

	// Expected order: P1 Task (priority 1, task before bug), P1 Bug, P2 Task
	expectedOrder := []string{"child-p1-task", "child-p1-bug", "child-p2-task"}
	for i, expected := range expectedOrder {
		if children[i].Issue.ID != expected {
			t.Errorf("child[%d]: expected %s, got %s", i, expected, children[i].Issue.ID)
		}
	}
}

// TestTreeBuildBlockingDepsIgnored verifies blocking deps don't create hierarchy
func TestTreeBuildBlockingDepsIgnored(t *testing.T) {
	issues := []model.Issue{
		{ID: "blocker", Title: "Blocker", Priority: 1, IssueType: model.TypeTask},
		{
			ID: "blocked", Title: "Blocked task", Priority: 2, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "blocked", DependsOnID: "blocker", Type: model.DepBlocks},
			},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Blocking deps shouldn't create hierarchy - both should be roots
	if tree.RootCount() != 2 {
		t.Errorf("expected 2 roots (blocking deps ignored), got %d", tree.RootCount())
	}
}

// TestTreeBuildRelatedDepsIgnored verifies related deps don't create hierarchy
func TestTreeBuildRelatedDepsIgnored(t *testing.T) {
	issues := []model.Issue{
		{ID: "main", Title: "Main task", Priority: 1, IssueType: model.TypeTask},
		{
			ID: "related", Title: "Related task", Priority: 2, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{
				{IssueID: "related", DependsOnID: "main", Type: model.DepRelated},
			},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Related deps shouldn't create hierarchy - both should be roots
	if tree.RootCount() != 2 {
		t.Errorf("expected 2 roots (related deps ignored), got %d", tree.RootCount())
	}
}

// TestTreeNavigation verifies cursor movement through the tree
func TestTreeNavigation(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root-1", Title: "Root 1", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child-1", Title: "Child 1", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child-1", DependsOnID: "root-1", Type: model.DepParentChild}},
		},
		{ID: "root-2", Title: "Root 2", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour)},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Initial selection should be first node (root-1)
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "root-1" {
		t.Errorf("expected initial selection root-1, got %v", sel)
	}

	// Move down to child-1 (auto-expanded)
	tree.MoveDown()
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "child-1" {
		t.Errorf("expected selection child-1 after MoveDown, got %v", sel)
	}

	// Move down to root-2
	tree.MoveDown()
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "root-2" {
		t.Errorf("expected selection root-2 after second MoveDown, got %v", sel)
	}

	// Move up back to child-1
	tree.MoveUp()
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "child-1" {
		t.Errorf("expected selection child-1 after MoveUp, got %v", sel)
	}

	// Jump to bottom
	tree.JumpToBottom()
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "root-2" {
		t.Errorf("expected selection root-2 after JumpToBottom, got %v", sel)
	}

	// Jump to top
	tree.JumpToTop()
	if sel := tree.SelectedIssue(); sel == nil || sel.ID != "root-1" {
		t.Errorf("expected selection root-1 after JumpToTop, got %v", sel)
	}
}

// TestTreeExpandCollapse verifies expand/collapse functionality
func TestTreeExpandCollapse(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child", Title: "Child", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Initially auto-expanded (depth < 2)
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes (auto-expanded), got %d", tree.NodeCount())
	}

	// Collapse root
	tree.ToggleExpand() // cursor is on root
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 visible node after collapse, got %d", tree.NodeCount())
	}

	// Expand root
	tree.ToggleExpand()
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes after expand, got %d", tree.NodeCount())
	}

	// Collapse all
	tree.CollapseAll()
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 visible node after CollapseAll, got %d", tree.NodeCount())
	}

	// Expand all
	tree.ExpandAll()
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes after ExpandAll, got %d", tree.NodeCount())
	}
}

// TestTreeIssueMap verifies the issueMap lookup is populated
func TestTreeIssueMap(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Test 1", Priority: 1, IssueType: model.TypeTask},
		{ID: "test-2", Title: "Test 2", Priority: 2, IssueType: model.TypeTask},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Verify issueMap contains all nodes
	if len(tree.issueMap) != 2 {
		t.Errorf("expected issueMap to have 2 entries, got %d", len(tree.issueMap))
	}

	if _, ok := tree.issueMap["test-1"]; !ok {
		t.Error("expected test-1 in issueMap")
	}
	if _, ok := tree.issueMap["test-2"]; !ok {
		t.Error("expected test-2 in issueMap")
	}
}

// TestIssueTypeOrder verifies the ordering of issue types
func TestIssueTypeOrder(t *testing.T) {
	tests := []struct {
		issueType model.IssueType
		expected  int
	}{
		{model.TypeEpic, 0},
		{model.TypeFeature, 1},
		{model.TypeTask, 2},
		{model.TypeBug, 3},
		{model.TypeChore, 4},
		{"unknown", 5},
	}

	for _, tt := range tests {
		got := issueTypeOrder(tt.issueType)
		if got != tt.expected {
			t.Errorf("issueTypeOrder(%s) = %d, want %d", tt.issueType, got, tt.expected)
		}
	}
}

// TestTreeViewEmpty verifies View() output for empty tree
func TestTreeViewEmpty(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(nil)
	tree.SetSize(80, 20)

	view := tree.View()
	if !strings.Contains(view, "No issues to display") {
		t.Errorf("expected empty state message, got:\n%s", view)
	}
	if !strings.Contains(view, "Press E to return") {
		t.Errorf("expected return hint in empty state, got:\n%s", view)
	}
}

// TestTreeViewRendering verifies View() renders tree structure correctly
func TestTreeViewRendering(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic Issue", Priority: 1, IssueType: model.TypeEpic, Status: model.StatusOpen, CreatedAt: now},
		{
			ID: "task-1", Title: "Task under Epic", Priority: 2, IssueType: model.TypeTask, Status: model.StatusInProgress, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)
	tree.SetSize(100, 30)

	view := tree.View()

	// Should contain both issue IDs
	if !strings.Contains(view, "epic-1") {
		t.Errorf("expected epic-1 in view, got:\n%s", view)
	}
	if !strings.Contains(view, "task-1") {
		t.Errorf("expected task-1 in view, got:\n%s", view)
	}

	// Should contain titles
	if !strings.Contains(view, "Epic Issue") {
		t.Errorf("expected 'Epic Issue' in view, got:\n%s", view)
	}

	// Should contain tree characters (for child node)
	if !strings.Contains(view, "└") && !strings.Contains(view, "├") {
		t.Errorf("expected tree branch characters in view, got:\n%s", view)
	}

	// Should contain expand/collapse indicators
	if !strings.Contains(view, "▾") && !strings.Contains(view, "▸") && !strings.Contains(view, "•") {
		t.Errorf("expected expand/collapse indicators in view, got:\n%s", view)
	}
}

// TestTreeViewIndicators verifies expand/collapse indicators
func TestTreeViewIndicators(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())

	// Test leaf node indicator
	leafNode := &IssueTreeNode{
		Issue:    &model.Issue{ID: "leaf"},
		Children: nil,
	}
	if got := tree.getExpandIndicator(leafNode); got != "•" {
		t.Errorf("leaf indicator = %q, want %q", got, "•")
	}

	// Test expanded node indicator
	expandedNode := &IssueTreeNode{
		Issue:    &model.Issue{ID: "expanded"},
		Children: []*IssueTreeNode{{Issue: &model.Issue{ID: "child"}}},
		Expanded: true,
	}
	if got := tree.getExpandIndicator(expandedNode); got != "▾" {
		t.Errorf("expanded indicator = %q, want %q", got, "▾")
	}

	// Test collapsed node indicator
	collapsedNode := &IssueTreeNode{
		Issue:    &model.Issue{ID: "collapsed"},
		Children: []*IssueTreeNode{{Issue: &model.Issue{ID: "child"}}},
		Expanded: false,
	}
	if got := tree.getExpandIndicator(collapsedNode); got != "▸" {
		t.Errorf("collapsed indicator = %q, want %q", got, "▸")
	}
}

// TestTreeTruncateTitle verifies title truncation
func TestTreeTruncateTitle(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())

	tests := []struct {
		title  string
		maxLen int
		want   string
	}{
		{"Short", 20, "Short"},
		{"This is a very long title that should be truncated", 20, "This is a very long…"},
		{"ABC", 3, "..."},
		{"A", 10, "A"},
	}

	for _, tt := range tests {
		got := tree.truncateTitle(tt.title, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.title, tt.maxLen, got, tt.want)
		}
	}
}

// TestTreeJumpToParent verifies JumpToParent navigation
func TestTreeJumpToParent(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child", Title: "Child", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Move to child
	tree.MoveDown()
	if tree.GetSelectedID() != "child" {
		t.Fatalf("expected child selected, got %s", tree.GetSelectedID())
	}

	// Jump to parent
	tree.JumpToParent()
	if tree.GetSelectedID() != "root" {
		t.Errorf("expected root after JumpToParent, got %s", tree.GetSelectedID())
	}

	// Jump to parent at root should do nothing
	tree.JumpToParent()
	if tree.GetSelectedID() != "root" {
		t.Errorf("expected root to stay selected, got %s", tree.GetSelectedID())
	}
}

// TestTreeExpandOrMoveToChild verifies → key behavior
func TestTreeExpandOrMoveToChild(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child", Title: "Child", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Root is initially expanded (auto-expand depth < 2)
	// ExpandOrMoveToChild should move to first child
	tree.ExpandOrMoveToChild()
	if tree.GetSelectedID() != "child" {
		t.Errorf("expected child after ExpandOrMoveToChild on expanded node, got %s", tree.GetSelectedID())
	}

	// Go back to root
	tree.JumpToTop()

	// Collapse root first
	tree.ToggleExpand()
	if tree.NodeCount() != 1 {
		t.Fatalf("expected 1 node after collapse, got %d", tree.NodeCount())
	}

	// Now ExpandOrMoveToChild should expand
	tree.ExpandOrMoveToChild()
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 nodes after expand, got %d", tree.NodeCount())
	}
	// Cursor should still be on root
	if tree.GetSelectedID() != "root" {
		t.Errorf("expected cursor on root after expand, got %s", tree.GetSelectedID())
	}
}

// TestTreeCollapseOrJumpToParent verifies ← key behavior
func TestTreeCollapseOrJumpToParent(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "child", Title: "Child", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads")) // Isolate from CWD state
	tree.Build(issues)

	// Root is expanded - CollapseOrJumpToParent should collapse
	tree.CollapseOrJumpToParent()
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 node after collapse, got %d", tree.NodeCount())
	}

	// Now root is collapsed - CollapseOrJumpToParent should do nothing (already at root)
	tree.CollapseOrJumpToParent()
	if tree.GetSelectedID() != "root" {
		t.Errorf("expected cursor on root, got %s", tree.GetSelectedID())
	}

	// Expand and move to child
	tree.ExpandOrMoveToChild() // expand
	tree.ExpandOrMoveToChild() // move to child
	if tree.GetSelectedID() != "child" {
		t.Fatalf("expected child selected, got %s", tree.GetSelectedID())
	}

	// CollapseOrJumpToParent on leaf should jump to parent
	tree.CollapseOrJumpToParent()
	if tree.GetSelectedID() != "root" {
		t.Errorf("expected root after jump to parent from leaf, got %s", tree.GetSelectedID())
	}
}

// TestTreePageNavigation verifies PageUp/PageDown
func TestTreePageNavigation(t *testing.T) {
	// Create many issues for pagination testing
	var issues []model.Issue
	for i := 0; i < 20; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.SetSize(80, 10) // Height of 10 -> page size of 5

	// PageDown
	tree.PageDown()
	if tree.cursor != 5 {
		t.Errorf("expected cursor at 5 after PageDown, got %d", tree.cursor)
	}

	// PageDown again
	tree.PageDown()
	if tree.cursor != 10 {
		t.Errorf("expected cursor at 10 after 2nd PageDown, got %d", tree.cursor)
	}

	// PageUp
	tree.PageUp()
	if tree.cursor != 5 {
		t.Errorf("expected cursor at 5 after PageUp, got %d", tree.cursor)
	}

	// Jump to bottom and PageDown should stay at end
	tree.JumpToBottom()
	tree.PageDown()
	if tree.cursor != 19 {
		t.Errorf("expected cursor at 19 (end), got %d", tree.cursor)
	}
}

// TestTreeSelectByID verifies cursor preservation by ID
func TestTreeSelectByID(t *testing.T) {
	issues := []model.Issue{
		{ID: "first", Title: "First", Priority: 1, IssueType: model.TypeTask},
		{ID: "second", Title: "Second", Priority: 2, IssueType: model.TypeTask},
		{ID: "third", Title: "Third", Priority: 3, IssueType: model.TypeTask},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Select middle issue
	if !tree.SelectByID("second") {
		t.Fatal("SelectByID failed to find 'second'")
	}
	if tree.GetSelectedID() != "second" {
		t.Errorf("expected 'second' selected, got %s", tree.GetSelectedID())
	}

	// Try to select non-existent
	if tree.SelectByID("nonexistent") {
		t.Error("SelectByID should return false for non-existent ID")
	}
	// Cursor should remain unchanged
	if tree.GetSelectedID() != "second" {
		t.Errorf("cursor should not change after failed SelectByID, got %s", tree.GetSelectedID())
	}
}

// =============================================================================
// TreeState persistence tests (bv-zv7p)
// =============================================================================

func TestDefaultTreeState(t *testing.T) {
	state := DefaultTreeState()

	if state.Version != TreeStateVersion {
		t.Errorf("expected version %d, got %d", TreeStateVersion, state.Version)
	}
	if state.Expanded == nil {
		t.Error("Expanded map should not be nil")
	}
	if len(state.Expanded) != 0 {
		t.Errorf("expected empty Expanded map, got %d entries", len(state.Expanded))
	}
}

func TestTreeStatePath(t *testing.T) {
	tests := []struct {
		name     string
		beadsDir string
		want     string
	}{
		{
			name:     "default empty beads dir",
			beadsDir: "",
			want:     ".beads/tree-state.json",
		},
		{
			name:     "custom beads dir",
			beadsDir: "/path/to/beads",
			want:     "/path/to/beads/tree-state.json",
		},
		{
			name:     "relative beads dir",
			beadsDir: "custom/.beads",
			want:     "custom/.beads/tree-state.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TreeStatePath(tt.beadsDir)
			if got != tt.want {
				t.Errorf("TreeStatePath(%q) = %q, want %q", tt.beadsDir, got, tt.want)
			}
		})
	}
}

func TestTreeStateVersion(t *testing.T) {
	// Ensure version constant is reasonable
	if TreeStateVersion < 1 {
		t.Errorf("TreeStateVersion should be >= 1, got %d", TreeStateVersion)
	}
}

// =============================================================================
// visibleRange tests (bv-r4ng)
// =============================================================================

func TestVisibleRange(t *testing.T) {
	// effectiveVisibleCount logic:
	//   visibleCount = height - 1 (for header); default 19 when height <= 0
	//   if flatList > visibleCount: visibleCount-- (for position indicator)
	//   minimum 1
	//
	// height=10 => 9 nodes; with indicator (100 > 9) => 8
	// height=10, 5 nodes => 9 nodes capacity, 5 < 9 so no indicator => 9 (but clamped to 5)
	// height=10, 10 nodes => 9, 10 > 9 so indicator => 8
	// height=0 => default 19; with indicator (100 > 19) => 18
	tests := []struct {
		name      string
		nodeCount int
		height    int
		offset    int
		wantStart int
		wantEnd   int
	}{
		{"empty tree", 0, 10, 0, 0, 0},
		{"fewer nodes than viewport", 5, 10, 0, 0, 5},                          // 5 < 9, no indicator
		{"exact fit", 10, 10, 0, 0, 8},                                         // 10 > 9 => indicator => 8
		{"offset at start", 100, 10, 0, 0, 8},                                  // 100 > 9 => indicator => 8
		{"offset in middle", 100, 10, 45, 45, 53},                              // 8 nodes visible
		{"offset near end", 100, 10, 92, 92, 100},                              // last 8
		{"offset past end clamps", 100, 10, 95, 92, 100},                       // clamped to 92
		{"zero height uses default 19 minus indicator", 100, 0, 0, 0, 18},      // 19-1=18
		{"negative height uses default 19 minus indicator", 100, -5, 0, 0, 18}, // 19-1=18
		{"single node", 1, 10, 0, 0, 1},
		{"negative offset clamps to start", 100, 10, -5, 0, 8}, // 8 nodes visible
		{"negative offset small list", 5, 10, -5, 0, 5},        // no indicator
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tree model with test nodes
			tree := NewTreeModel(testTheme())
			tree.height = tt.height
			tree.viewportOffset = tt.offset

			// Create fake flat list with the specified number of nodes
			tree.flatList = make([]*IssueTreeNode, tt.nodeCount)
			for i := 0; i < tt.nodeCount; i++ {
				tree.flatList[i] = &IssueTreeNode{
					Issue: &model.Issue{ID: fmt.Sprintf("test-%d", i)},
				}
			}

			gotStart, gotEnd := tree.visibleRange()

			if gotStart != tt.wantStart {
				t.Errorf("visibleRange() start = %d, want %d", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("visibleRange() end = %d, want %d", gotEnd, tt.wantEnd)
			}

			// Verify the range is valid
			if gotEnd < gotStart {
				t.Errorf("visibleRange() end (%d) < start (%d)", gotEnd, gotStart)
			}
			if gotStart < 0 {
				t.Errorf("visibleRange() start (%d) is negative", gotStart)
			}
			if gotEnd > tt.nodeCount {
				t.Errorf("visibleRange() end (%d) exceeds node count (%d)", gotEnd, tt.nodeCount)
			}
		})
	}
}

func TestVisibleRangePerformance(t *testing.T) {
	// Verify O(1) behavior - should complete quickly regardless of tree size
	tree := NewTreeModel(testTheme())
	tree.height = 20

	// Large tree
	tree.flatList = make([]*IssueTreeNode, 100000)
	tree.viewportOffset = 50000

	// Should complete instantly (O(1))
	// height=20 => 19 - 1 (indicator for 100000 nodes) = 18
	start, end := tree.visibleRange()

	if start != 50000 || end != 50018 {
		t.Errorf("visibleRange() = (%d, %d), want (50000, 50018)", start, end)
	}
}

// =============================================================================
// saveState tests (bv-19vz)
// =============================================================================

// TestSaveState tests the saveState method
func TestSaveState(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")

	// Create a tree with test data
	issues := []model.Issue{
		{ID: "root-1", Title: "Root 1", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "child-1", Title: "Child 1", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "child-1", DependsOnID: "root-1", Type: model.DepParentChild}}},
		{ID: "grandchild-1", Title: "Grandchild 1", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "grandchild-1", DependsOnID: "child-1", Type: model.DepParentChild}}},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)
	tree.Build(issues)

	// Initially, root-1 (depth=0) and child-1 (depth=1) are expanded by default
	// grandchild-1 (depth=2) is collapsed by default

	// Collapse child-1 (non-default state)
	for i, node := range tree.flatList {
		if node.Issue != nil && node.Issue.ID == "child-1" {
			tree.cursor = i
			break
		}
	}
	tree.ToggleExpand() // This should save state

	// Verify the file was created
	statePath := filepath.Join(beadsDir, "tree-state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	// Parse and verify content
	var state TreeState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Failed to parse state file: %v", err)
	}

	if state.Version != TreeStateVersion {
		t.Errorf("State version = %d, want %d", state.Version, TreeStateVersion)
	}

	// child-1 at depth=1 was expanded by default (depth < 2), now collapsed
	// So it should be in the Expanded map as false
	if expanded, ok := state.Expanded["child-1"]; !ok || expanded {
		t.Errorf("Expected child-1 to be in Expanded map as false, got %v (ok=%v)", expanded, ok)
	}
}

// TestSaveStateOnlyNonDefault verifies that only non-default states are saved
func TestSaveStateOnlyNonDefault(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")

	// Create deep tree
	issues := []model.Issue{
		{ID: "root", Title: "Root", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "d1", Title: "Depth 1", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "d1", DependsOnID: "root", Type: model.DepParentChild}}},
		{ID: "d2", Title: "Depth 2", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "d2", DependsOnID: "d1", Type: model.DepParentChild}}},
		{ID: "d3", Title: "Depth 3", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "d3", DependsOnID: "d2", Type: model.DepParentChild}}},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)
	tree.Build(issues)

	// Default state:
	// - root (depth=0): expanded
	// - d1 (depth=1): expanded
	// - d2 (depth=2): collapsed
	// - d3 (depth=3): collapsed

	// Expand all - this makes d2 and d3 non-default (expanded when default is collapsed)
	tree.ExpandAll()

	// Read state
	statePath := filepath.Join(beadsDir, "tree-state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var state TreeState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Failed to parse state file: %v", err)
	}

	// root and d1 should NOT be in the map (they're in default expanded state)
	if _, ok := state.Expanded["root"]; ok {
		t.Error("root should not be in Expanded map (already default expanded)")
	}
	if _, ok := state.Expanded["d1"]; ok {
		t.Error("d1 should not be in Expanded map (already default expanded)")
	}

	// d2 and d3 SHOULD be in the map as true (expanded is non-default for depth >= 2)
	if expanded, ok := state.Expanded["d2"]; !ok || !expanded {
		t.Errorf("d2 should be in Expanded map as true, got %v (ok=%v)", expanded, ok)
	}
	if expanded, ok := state.Expanded["d3"]; !ok || !expanded {
		t.Errorf("d3 should be in Expanded map as true, got %v (ok=%v)", expanded, ok)
	}

	// Now collapse all
	tree.CollapseAll()

	// Read state again
	data, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file after collapse: %v", err)
	}

	// Re-initialize state to ensure clean unmarshal
	state = TreeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Failed to parse state file after collapse: %v", err)
	}

	// root and d1 should be in the map as false (collapsed is non-default for depth < 2)
	if expanded, ok := state.Expanded["root"]; !ok || expanded {
		t.Errorf("root should be in Expanded map as false after CollapseAll, got %v (ok=%v)", expanded, ok)
	}
	if expanded, ok := state.Expanded["d1"]; !ok || expanded {
		t.Errorf("d1 should be in Expanded map as false after CollapseAll, got %v (ok=%v)", expanded, ok)
	}

	// d2 and d3 should NOT be in the map (collapsed is default for depth >= 2)
	if _, ok := state.Expanded["d2"]; ok {
		t.Error("d2 should not be in Expanded map after CollapseAll (collapsed is default)")
	}
	if _, ok := state.Expanded["d3"]; ok {
		t.Error("d3 should not be in Expanded map after CollapseAll (collapsed is default)")
	}
}

// TestSetBeadsDir tests the SetBeadsDir method
func TestSetBeadsDir(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)

	// Default should be empty
	if tree.beadsDir != "" {
		t.Errorf("Expected empty beadsDir initially, got %q", tree.beadsDir)
	}

	// Set custom directory
	tree.SetBeadsDir("/custom/path")
	if tree.beadsDir != "/custom/path" {
		t.Errorf("Expected beadsDir to be /custom/path, got %q", tree.beadsDir)
	}
}

// =============================================================================
// loadState tests (bv-afcm)
// =============================================================================

// TestLoadState tests that loadState correctly restores expand/collapse state
func TestLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Create a state file that makes child-1 collapsed (non-default for depth 1)
	// and grandchild-1 expanded (non-default for depth 2)
	state := TreeState{
		Version: TreeStateVersion,
		Expanded: map[string]bool{
			"child-1":      false, // collapsed (non-default for depth 1)
			"grandchild-1": true,  // expanded (non-default for depth 2)
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(beadsDir, "tree-state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	// Create issues
	issues := []model.Issue{
		{ID: "root-1", Title: "Root 1", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "child-1", Title: "Child 1", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "child-1", DependsOnID: "root-1", Type: model.DepParentChild}}},
		{ID: "grandchild-1", Title: "Grandchild 1", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "grandchild-1", DependsOnID: "child-1", Type: model.DepParentChild}}},
	}

	// Build tree with beadsDir set
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)
	tree.Build(issues)

	// Verify state was loaded correctly
	child1 := tree.issueMap["child-1"]
	if child1 == nil {
		t.Fatal("child-1 not found in issueMap")
	}
	if child1.Expanded {
		t.Error("Expected child-1 to be collapsed (from state file)")
	}

	grandchild1 := tree.issueMap["grandchild-1"]
	if grandchild1 == nil {
		t.Fatal("grandchild-1 not found in issueMap")
	}
	if !grandchild1.Expanded {
		t.Error("Expected grandchild-1 to be expanded (from state file)")
	}

	// root-1 should still be expanded (default for depth 0, no override in state)
	root1 := tree.issueMap["root-1"]
	if root1 == nil {
		t.Fatal("root-1 not found in issueMap")
	}
	if !root1.Expanded {
		t.Error("Expected root-1 to be expanded (default behavior)")
	}
}

// TestLoadStateNoFile tests that missing state file uses defaults
func TestLoadStateNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	// Intentionally NOT creating state file

	issues := []model.Issue{
		{ID: "root", Title: "Root", Status: model.StatusOpen, IssueType: model.TypeEpic},
		{ID: "child", Title: "Child", Status: model.StatusOpen, IssueType: model.TypeTask,
			Dependencies: []*model.Dependency{{IssueID: "child", DependsOnID: "root", Type: model.DepParentChild}}},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)
	tree.Build(issues)

	// Should use defaults without error
	if !tree.IsBuilt() {
		t.Error("Tree should be built even without state file")
	}

	// Default: root (depth 0) and child (depth 1) should be expanded
	if !tree.issueMap["root"].Expanded {
		t.Error("Expected root to be expanded (default for depth 0)")
	}
	if !tree.issueMap["child"].Expanded {
		t.Error("Expected child to be expanded (default for depth 1)")
	}
}

// TestLoadStateCorrupted tests that corrupted state file uses defaults
func TestLoadStateCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Write invalid JSON
	statePath := filepath.Join(beadsDir, "tree-state.json")
	if err := os.WriteFile(statePath, []byte("not valid json {"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted state file: %v", err)
	}

	issues := []model.Issue{
		{ID: "root", Title: "Root", Status: model.StatusOpen, IssueType: model.TypeEpic},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)

	// Should not panic
	tree.Build(issues)

	// Should use defaults
	if !tree.IsBuilt() {
		t.Error("Tree should be built despite corrupted state file")
	}
	if !tree.issueMap["root"].Expanded {
		t.Error("Expected root to be expanded (default) after corrupted state file")
	}
}

// TestLoadStateStaleIDs tests that stale IDs in state file are ignored
func TestLoadStateStaleIDs(t *testing.T) {
	tmpDir := t.TempDir()
	beadsDir := filepath.Join(tmpDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("Failed to create beads dir: %v", err)
	}

	// Create state file with IDs that don't exist
	state := TreeState{
		Version: TreeStateVersion,
		Expanded: map[string]bool{
			"nonexistent-1": true,
			"nonexistent-2": false,
			"root":          false, // This one exists
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(beadsDir, "tree-state.json")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	issues := []model.Issue{
		{ID: "root", Title: "Root", Status: model.StatusOpen, IssueType: model.TypeEpic},
	}

	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	tree := NewTreeModel(theme)
	tree.SetBeadsDir(beadsDir)
	tree.Build(issues)

	// Should not panic, should build successfully
	if !tree.IsBuilt() {
		t.Error("Tree should be built despite stale IDs in state file")
	}

	// The existing ID should have its state applied
	if tree.issueMap["root"].Expanded {
		t.Error("Expected root to be collapsed (from state file)")
	}
}

// =============================================================================
// ensureCursorVisible tests (bv-lnc4)
// =============================================================================

// TestEnsureCursorVisible verifies the cursor-follows-viewport behavior
// Note: effectiveVisibleCount = height-1 (header), then -1 more if flatList > that
// For height=10, 100 nodes: effective = 10-1-1 = 8
// For height=0, 100 nodes: effective = 19-1 = 18
func TestEnsureCursorVisible(t *testing.T) {
	tests := []struct {
		name          string
		nodeCount     int
		height        int
		initialCursor int
		initialOffset int
		wantOffset    int
	}{
		{"cursor at start, offset at start", 100, 10, 0, 0, 0},
		{"cursor in visible range", 100, 10, 5, 0, 0},
		{"cursor below viewport - scroll down", 100, 10, 15, 0, 8},
		{"cursor above viewport - scroll up", 100, 10, 0, 10, 0},
		{"cursor at viewport bottom edge", 100, 10, 9, 0, 2},
		{"cursor just past viewport bottom", 100, 10, 10, 0, 3},
		{"cursor at end of list", 100, 10, 99, 0, 92},
		{"offset already correct", 100, 10, 50, 45, 45},
		{"empty list", 0, 10, 0, 0, 0},
		{"single node", 1, 10, 0, 0, 0},
		{"fewer nodes than viewport", 5, 10, 3, 0, 0},
		{"zero height uses default", 100, 0, 25, 0, 8},
		{"large cursor with max offset clamping", 100, 10, 95, 0, 88},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewTreeModel(testTheme())
			tree.height = tt.height
			tree.cursor = tt.initialCursor
			tree.viewportOffset = tt.initialOffset

			// Create fake flat list
			tree.flatList = make([]*IssueTreeNode, tt.nodeCount)
			for i := 0; i < tt.nodeCount; i++ {
				tree.flatList[i] = &IssueTreeNode{
					Issue: &model.Issue{ID: fmt.Sprintf("test-%d", i)},
				}
			}

			tree.ensureCursorVisible()

			if tree.viewportOffset != tt.wantOffset {
				t.Errorf("ensureCursorVisible() offset = %d, want %d", tree.viewportOffset, tt.wantOffset)
			}

			// Verify cursor is now within visible range (unless empty)
			if tt.nodeCount > 0 {
				visibleCount := tree.effectiveVisibleCount()
				if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+visibleCount {
					// This check needs adjustment for edge case where list is smaller than viewport
					if tt.nodeCount > visibleCount && tree.cursor >= tree.viewportOffset+visibleCount {
						t.Errorf("cursor %d not visible with offset %d and effectiveVisible %d",
							tree.cursor, tree.viewportOffset, visibleCount)
					}
				}
			}
		})
	}
}

// TestEnsureCursorVisibleNegativeOffset tests clamping of negative offset
func TestEnsureCursorVisibleNegativeOffset(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.height = 10
	tree.cursor = 0
	tree.viewportOffset = -5 // Invalid negative offset

	tree.flatList = make([]*IssueTreeNode, 100)
	for i := 0; i < 100; i++ {
		tree.flatList[i] = &IssueTreeNode{
			Issue: &model.Issue{ID: fmt.Sprintf("test-%d", i)},
		}
	}

	tree.ensureCursorVisible()

	if tree.viewportOffset < 0 {
		t.Errorf("viewportOffset should be >= 0, got %d", tree.viewportOffset)
	}
}

// TestNavigationCallsEnsureCursorVisible verifies navigation methods maintain visibility
func TestNavigationCallsEnsureCursorVisible(t *testing.T) {
	// Create many issues for testing scroll
	var issues []model.Issue
	for i := 0; i < 50; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10) // Viewport of 10 lines

	// Test MoveDown past viewport
	for i := 0; i < 15; i++ {
		tree.MoveDown()
	}
	// Cursor at 15, viewport should have scrolled
	if tree.cursor != 15 {
		t.Errorf("cursor should be 15, got %d", tree.cursor)
	}
	if tree.viewportOffset == 0 {
		t.Error("viewportOffset should have scrolled down")
	}
	// Cursor should be visible (using effectiveVisibleCount)
	effVis := tree.effectiveVisibleCount()
	if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+effVis {
		t.Errorf("cursor %d not visible with offset %d (effective %d)", tree.cursor, tree.viewportOffset, effVis)
	}

	// Test JumpToBottom
	tree.JumpToBottom()
	if tree.cursor != 49 {
		t.Errorf("cursor should be 49 after JumpToBottom, got %d", tree.cursor)
	}
	// Cursor should still be visible
	if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+effVis {
		t.Errorf("cursor %d not visible after JumpToBottom with offset %d", tree.cursor, tree.viewportOffset)
	}

	// Test JumpToTop
	tree.JumpToTop()
	if tree.cursor != 0 {
		t.Errorf("cursor should be 0 after JumpToTop, got %d", tree.cursor)
	}
	if tree.viewportOffset != 0 {
		t.Errorf("viewportOffset should be 0 after JumpToTop, got %d", tree.viewportOffset)
	}

	// Test PageDown
	tree.PageDown()
	// Cursor should be visible
	if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+effVis {
		t.Errorf("cursor %d not visible after PageDown with offset %d", tree.cursor, tree.viewportOffset)
	}
}

// TestGetViewportOffset tests the accessor method
func TestGetViewportOffset(t *testing.T) {
	tree := NewTreeModel(testTheme())
	tree.viewportOffset = 42

	if got := tree.GetViewportOffset(); got != 42 {
		t.Errorf("GetViewportOffset() = %d, want 42", got)
	}
}

// =============================================================================
// Windowed rendering tests (bv-db02)
// =============================================================================

// TestViewRendersOnlyVisible verifies that View() only renders visible nodes
func TestViewRendersOnlyVisible(t *testing.T) {
	// Create many issues for testing
	var issues []model.Issue
	for i := 0; i < 100; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10) // Viewport of 10 lines

	// Scroll to middle
	tree.viewportOffset = 50

	output := tree.View()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	// With height=10, effectiveVisibleCount = 10-1(header)-1(indicator) = 8 nodes visible.
	// Output: 1 header + 8 node lines + 1 position indicator = 10 lines total.
	if len(lines) != 10 {
		t.Errorf("expected 10 lines (1 header + 8 content + 1 indicator), got %d", len(lines))
	}

	// Should contain node 50's content (issue-50)
	if !strings.Contains(output, "issue-50") {
		t.Error("first visible node (issue-50) not rendered")
	}

	// Should contain node 57's content (last visible: 50+8-1=57)
	if !strings.Contains(output, "issue-57") {
		t.Error("last visible node (issue-57) not rendered")
	}

	// Should NOT contain node 0's content
	if strings.Contains(output, "issue-00") {
		t.Error("non-visible node (issue-00) incorrectly rendered")
	}

	// Should NOT contain node 49's content (just before viewport)
	if strings.Contains(output, "issue-49") {
		t.Error("non-visible node (issue-49) incorrectly rendered")
	}

	// Should NOT contain node 58's content (just after viewport)
	if strings.Contains(output, "issue-58") {
		t.Error("non-visible node (issue-58) incorrectly rendered")
	}
}

// TestViewRendersSmallTree verifies small trees still work correctly
func TestViewRendersSmallTree(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "Issue 1", Priority: 1, IssueType: model.TypeTask},
		{ID: "issue-2", Title: "Issue 2", Priority: 2, IssueType: model.TypeTask},
		{ID: "issue-3", Title: "Issue 3", Priority: 3, IssueType: model.TypeTask},
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 20) // Viewport larger than tree

	output := tree.View()

	// Should contain all 3 issues
	if !strings.Contains(output, "issue-1") {
		t.Error("issue-1 not rendered")
	}
	if !strings.Contains(output, "issue-2") {
		t.Error("issue-2 not rendered")
	}
	if !strings.Contains(output, "issue-3") {
		t.Error("issue-3 not rendered")
	}
}

// TestViewSelectionHighlightWithOffset verifies selection works with offset
func TestViewSelectionHighlightWithOffset(t *testing.T) {
	// Create issues
	var issues []model.Issue
	for i := 0; i < 50; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10)

	// Move cursor to position 25 and ensure it's visible
	tree.cursor = 25
	tree.ensureCursorVisible()

	output := tree.View()

	// The selected issue should be in the output
	if !strings.Contains(output, "issue-25") {
		t.Error("selected issue (issue-25) not in output")
	}

	// Cursor should be visible (use effectiveVisibleCount)
	eff := tree.effectiveVisibleCount()
	if tree.cursor < tree.viewportOffset || tree.cursor >= tree.viewportOffset+eff {
		t.Errorf("cursor %d not visible with offset %d (effective %d)", tree.cursor, tree.viewportOffset, eff)
	}
}

// TestViewAtEndOfList verifies rendering at the end of a long list
func TestViewAtEndOfList(t *testing.T) {
	var issues []model.Issue
	for i := 0; i < 100; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10)

	// Jump to bottom
	tree.JumpToBottom()

	output := tree.View()

	// Should contain the last issue
	if !strings.Contains(output, "issue-99") {
		t.Error("last issue (issue-99) not rendered")
	}

	// height=10, 100 nodes -> effective=8. Last 8 items: 92-99
	if !strings.Contains(output, "issue-92") {
		t.Error("issue-92 not rendered (first in last window)")
	}

	// Should NOT contain issue-91 (just before the visible window)
	if strings.Contains(output, "issue-91") {
		t.Error("issue-91 incorrectly rendered")
	}
}

// =============================================================================
// Position indicator tests (bv-2nax)
// =============================================================================

// TestPositionIndicatorShown verifies indicator appears for large trees
func TestPositionIndicatorShown(t *testing.T) {
	var issues []model.Issue
	for i := 0; i < 100; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10) // Viewport of 10, 100 nodes

	output := tree.View()

	// height=10, 100 nodes -> effective=8. Position indicator: "Page 1/13 (1-8 of 100)"
	if !strings.Contains(output, "Page 1/13 (1-8 of 100)") {
		t.Errorf("position indicator not found in output, got:\n%s", output)
	}
}

// TestPositionIndicatorMiddle verifies indicator at middle of list
func TestPositionIndicatorMiddle(t *testing.T) {
	var issues []model.Issue
	for i := 0; i < 100; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10)

	// Move cursor to 50 and scroll
	tree.cursor = 50
	tree.ensureCursorVisible()

	output := tree.View()

	// Should contain updated position indicator with Page format
	// The offset should be adjusted so cursor 50 is visible
	if !strings.Contains(output, "of 100)") {
		t.Errorf("position indicator with 'of 100)' not found, got:\n%s", output)
	}
}

// TestPositionIndicatorNotShownSmallTree verifies no indicator for small trees
func TestPositionIndicatorNotShownSmallTree(t *testing.T) {
	issues := []model.Issue{
		{ID: "issue-1", Title: "Issue 1", Priority: 1, IssueType: model.TypeTask},
		{ID: "issue-2", Title: "Issue 2", Priority: 2, IssueType: model.TypeTask},
		{ID: "issue-3", Title: "Issue 3", Priority: 3, IssueType: model.TypeTask},
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 20) // Viewport larger than tree

	output := tree.View()

	// Should NOT contain position indicator (all nodes fit)
	if strings.Contains(output, " of ") {
		t.Errorf("position indicator should not show for small tree, got:\n%s", output)
	}
}

// TestPositionIndicatorAtEnd verifies indicator at end of list
func TestPositionIndicatorAtEnd(t *testing.T) {
	var issues []model.Issue
	for i := 0; i < 100; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("issue-%02d", i),
			Title:     fmt.Sprintf("Issue %d", i),
			Priority:  2,
			IssueType: model.TypeTask,
		})
	}

	tree := NewTreeModel(testTheme())
	tree.Build(issues)
	tree.SetSize(80, 10)

	// Jump to bottom
	tree.JumpToBottom()

	output := tree.View()

	// height=10, 100 nodes -> effective=8. Last window shows "93-100 of 100" with page info
	if !strings.Contains(output, "93-100 of 100)") {
		t.Errorf("position indicator at end not found, got:\n%s", output)
	}
}

// =============================================================================
// Filter tests (bd-e3w, bd-05v)
// =============================================================================

// TestTreeApplyFilterAll verifies "all" filter shows everything
func TestTreeApplyFilterAll(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Apply "all" filter - nothing should change
	tree.ApplyFilter("all")
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes with 'all' filter, got %d", tree.NodeCount())
	}
	if tree.GetFilter() != "all" {
		t.Errorf("expected filter 'all', got %q", tree.GetFilter())
	}
}

// TestTreeApplyFilterOpen verifies "open" filter hides closed issues
func TestTreeApplyFilterOpen(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "in-progress-1", Title: "In Progress", Priority: 1, IssueType: model.TypeTask, Status: model.StatusInProgress},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	tree.ApplyFilter("open")
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes with 'open' filter, got %d", tree.NodeCount())
	}
	if tree.GetFilter() != "open" {
		t.Errorf("expected filter 'open', got %q", tree.GetFilter())
	}
}

// TestTreeApplyFilterClosed verifies "closed" filter shows only closed issues
func TestTreeApplyFilterClosed(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
		{ID: "closed-2", Title: "Tombstone", Priority: 2, IssueType: model.TypeTask, Status: model.StatusTombstone},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	tree.ApplyFilter("closed")
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes with 'closed' filter, got %d", tree.NodeCount())
	}
}

// TestTreeApplyFilterReady verifies "ready" filter excludes blocked issues
func TestTreeApplyFilterReady(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open Ready", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "blocked-1", Title: "Blocked", Priority: 1, IssueType: model.TypeTask, Status: model.StatusBlocked},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
		{
			ID: "has-blocker", Title: "Has Open Blocker", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen,
			Dependencies: []*model.Dependency{
				{IssueID: "has-blocker", DependsOnID: "open-1", Type: model.DepBlocks},
			},
		},
	}

	// Provide global issue map for blocker resolution
	globalMap := make(map[string]*model.Issue)
	for i := range issues {
		globalMap[issues[i].ID] = &issues[i]
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	tree.SetGlobalIssueMap(globalMap)

	tree.ApplyFilter("ready")

	// Only "open-1" should pass: blocked-1 has status=blocked, closed-1 is closed,
	// has-blocker has an open blocker
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 visible node with 'ready' filter, got %d", tree.NodeCount())
		for i := 0; i < tree.NodeCount(); i++ {
			t.Logf("  visible[%d]: %s (%s)", i, tree.flatList[i].Issue.ID, tree.flatList[i].Issue.Status)
		}
	}
}

// TestTreeFilterWithHierarchy verifies context ancestors are shown dimmed
func TestTreeFilterWithHierarchy(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, Status: model.StatusOpen, CreatedAt: now},
		{
			ID: "task-open", Title: "Open Task", Priority: 2, IssueType: model.TypeTask, Status: model.StatusOpen, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "task-open", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
		{
			ID: "task-closed", Title: "Closed Task", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "task-closed", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Filter to "closed" - only task-closed matches, epic-1 is context ancestor
	tree.ApplyFilter("closed")

	// Should show 2 nodes: epic-1 (context) + task-closed (match)
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 visible nodes (1 context + 1 match), got %d", tree.NodeCount())
		for i := 0; i < tree.NodeCount(); i++ {
			t.Logf("  visible[%d]: %s", i, tree.flatList[i].Issue.ID)
		}
	}

	// Verify epic-1 is dimmed (context ancestor, not match)
	if epic := tree.issueMap["epic-1"]; epic != nil {
		if !tree.IsFilterDimmed(epic) {
			t.Error("expected epic-1 to be dimmed (context ancestor)")
		}
	}

	// Verify task-closed is NOT dimmed (direct match)
	if task := tree.issueMap["task-closed"]; task != nil {
		if tree.IsFilterDimmed(task) {
			t.Error("expected task-closed to NOT be dimmed (direct match)")
		}
	}
}

// TestTreeFilterResetToAll verifies clearing filter restores full tree
func TestTreeFilterResetToAll(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Apply filter then clear
	tree.ApplyFilter("open")
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 node with 'open' filter, got %d", tree.NodeCount())
	}

	tree.ApplyFilter("all")
	if tree.NodeCount() != 2 {
		t.Errorf("expected 2 nodes after clearing filter, got %d", tree.NodeCount())
	}

	// filterMatches and contextAncestors should be nil
	if tree.filterMatches != nil {
		t.Error("expected filterMatches to be nil after 'all' filter")
	}
	if tree.contextAncestors != nil {
		t.Error("expected contextAncestors to be nil after 'all' filter")
	}
}

// TestTreeFilterEmptyResult verifies filter with no matches shows nothing
func TestTreeFilterEmptyResult(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	tree.ApplyFilter("closed")
	if tree.NodeCount() != 0 {
		t.Errorf("expected 0 nodes with 'closed' filter (no closed issues), got %d", tree.NodeCount())
	}
}

// TestTreeFilterDeepHierarchy verifies ancestor chain is preserved for deep matches
func TestTreeFilterDeepHierarchy(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "root", Title: "Root", Priority: 1, IssueType: model.TypeEpic, Status: model.StatusOpen, CreatedAt: now},
		{
			ID: "mid", Title: "Mid", Priority: 2, IssueType: model.TypeFeature, Status: model.StatusOpen, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "mid", DependsOnID: "root", Type: model.DepParentChild}},
		},
		{
			ID: "leaf-closed", Title: "Leaf Closed", Priority: 3, IssueType: model.TypeTask, Status: model.StatusClosed, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "leaf-closed", DependsOnID: "mid", Type: model.DepParentChild}},
		},
		{
			ID: "leaf-open", Title: "Leaf Open", Priority: 3, IssueType: model.TypeTask, Status: model.StatusOpen, CreatedAt: now.Add(3 * time.Hour),
			Dependencies: []*model.Dependency{{IssueID: "leaf-open", DependsOnID: "mid", Type: model.DepParentChild}},
		},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	// Filter to "closed" - only leaf-closed matches
	// But root and mid should appear as context ancestors
	tree.ApplyFilter("closed")

	if tree.NodeCount() != 3 {
		t.Errorf("expected 3 visible nodes (root + mid as context, leaf-closed as match), got %d", tree.NodeCount())
		for i := 0; i < tree.NodeCount(); i++ {
			t.Logf("  visible[%d]: %s", i, tree.flatList[i].Issue.ID)
		}
	}

	// Verify dimming
	if !tree.IsFilterDimmed(tree.issueMap["root"]) {
		t.Error("expected root to be dimmed")
	}
	if !tree.IsFilterDimmed(tree.issueMap["mid"]) {
		t.Error("expected mid to be dimmed")
	}
	if tree.IsFilterDimmed(tree.issueMap["leaf-closed"]) {
		t.Error("expected leaf-closed to NOT be dimmed")
	}
}

// TestTreeFilterNoDimmedWithoutFilter verifies IsFilterDimmed returns false when no filter
func TestTreeFilterNoDimmedWithoutFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Test", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
	}

	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)

	if tree.IsFilterDimmed(tree.issueMap["test-1"]) {
		t.Error("expected no dimming when no filter is active")
	}
}

// TestTreeSetGlobalIssueMap verifies the global issue map is stored
func TestTreeSetGlobalIssueMap(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	m := map[string]*model.Issue{
		"test-1": {ID: "test-1"},
	}
	tree.SetGlobalIssueMap(m)
	if tree.globalIssueMap == nil {
		t.Error("expected globalIssueMap to be set")
	}
	if tree.globalIssueMap["test-1"] == nil {
		t.Error("expected test-1 in globalIssueMap")
	}
}

// =============================================================================
// Flat mode toggle tests (bd-39v)
// =============================================================================

// newIsolatedTree creates a TreeModel with an isolated beadsDir to prevent
// stale tree-state.json files from affecting test results.
func newIsolatedTree(t *testing.T) TreeModel {
	t.Helper()
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetBeadsDir(filepath.Join(t.TempDir(), ".beads"))
	return tree
}

// TestFlatModeDefault verifies flat mode is off by default
func TestFlatModeDefault(t *testing.T) {
	tree := newIsolatedTree(t)
	if tree.IsFlatMode() {
		t.Error("expected flat mode to be off by default")
	}
}

// TestToggleFlatMode verifies ToggleFlatMode flips the flag
func TestToggleFlatMode(t *testing.T) {
	tree := newIsolatedTree(t)
	tree.ToggleFlatMode()
	if !tree.IsFlatMode() {
		t.Error("expected flat mode to be on after toggle")
	}
	tree.ToggleFlatMode()
	if tree.IsFlatMode() {
		t.Error("expected flat mode to be off after second toggle")
	}
}

// TestFlatModeShowsAllNodesWithoutHierarchy verifies flat mode lists all nodes
// at depth 0 without parent-child nesting, even when tree has hierarchy
func TestFlatModeShowsAllNodesWithoutHierarchy(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task under Epic", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
		{
			ID: "subtask-1", Title: "Subtask", Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "subtask-1", DependsOnID: "task-1", Type: model.DepParentChild},
			},
		},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)

	// In tree mode, initially all 3 are visible (auto-expanded for depth < 2)
	if tree.NodeCount() != 3 {
		t.Fatalf("expected 3 visible nodes in tree mode, got %d", tree.NodeCount())
	}

	// Now collapse the root — only epic-1 visible
	tree.ToggleExpand() // cursor is on epic-1
	if tree.NodeCount() != 1 {
		t.Fatalf("expected 1 visible node after collapse, got %d", tree.NodeCount())
	}

	// Toggle flat mode — all 3 issues should be visible regardless of expand state
	tree.ToggleFlatMode()
	if tree.NodeCount() != 3 {
		t.Errorf("expected 3 visible nodes in flat mode (ignoring hierarchy), got %d", tree.NodeCount())
	}

	// Verify nodes in flat mode have no tree prefix (depth 0 behavior)
	for i := 0; i < tree.NodeCount(); i++ {
		node := tree.flatList[i]
		if node.Depth != 0 {
			t.Errorf("flat mode node[%d] %s has depth %d, expected 0", i, node.Issue.ID, node.Depth)
		}
	}
}

// TestFlatModePreservesSortMode verifies flat mode respects the current sort order
func TestFlatModePreservesSortMode(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "a", Title: "A", Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour)},
		{ID: "b", Title: "B", Priority: 1, IssueType: model.TypeTask, CreatedAt: now},
		{ID: "c", Title: "C", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour)},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)

	// In default sort mode (priority), order should be b(P1), c(P2), a(P3)
	tree.ToggleFlatMode()
	if tree.NodeCount() != 3 {
		t.Fatalf("expected 3 nodes, got %d", tree.NodeCount())
	}

	expectedOrder := []string{"b", "c", "a"}
	for i, expected := range expectedOrder {
		got := tree.flatList[i].Issue.ID
		if got != expected {
			t.Errorf("flat mode node[%d]: expected %s, got %s", i, expected, got)
		}
	}
}

// TestFlatModePreservesFilter verifies flat mode respects the current filter
func TestFlatModePreservesFilter(t *testing.T) {
	issues := []model.Issue{
		{ID: "open-1", Title: "Open", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
		{ID: "closed-1", Title: "Closed", Priority: 2, IssueType: model.TypeTask, Status: model.StatusClosed},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)

	// Apply "open" filter first, then toggle flat mode
	tree.ApplyFilter("open")
	tree.ToggleFlatMode()

	// Should only show open issue
	if tree.NodeCount() != 1 {
		t.Errorf("expected 1 node with open filter in flat mode, got %d", tree.NodeCount())
	}
	if tree.flatList[0].Issue.ID != "open-1" {
		t.Errorf("expected open-1, got %s", tree.flatList[0].Issue.ID)
	}
}

// TestFlatModeViewIndicator verifies the view shows a [FLAT] or [TREE] indicator
func TestFlatModeViewIndicator(t *testing.T) {
	issues := []model.Issue{
		{ID: "test-1", Title: "Test", Priority: 1, IssueType: model.TypeTask, Status: model.StatusOpen},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 20)

	// Tree mode should show TREE indicator
	view := tree.View()
	if !strings.Contains(view, "TREE") {
		t.Errorf("expected TREE indicator in tree mode view")
	}

	// Flat mode should show FLAT indicator
	tree.ToggleFlatMode()
	view = tree.View()
	if !strings.Contains(view, "FLAT") {
		t.Errorf("expected FLAT indicator in flat mode view")
	}
}

// TestFlatModeToggleBackRestoresTree verifies toggling back restores hierarchy
func TestFlatModeToggleBackRestoresTree(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)

	// Record tree mode state
	treeCount := tree.NodeCount()

	// Toggle to flat, then back to tree
	tree.ToggleFlatMode()
	tree.ToggleFlatMode()

	// Should restore tree hierarchy
	if tree.NodeCount() != treeCount {
		t.Errorf("expected %d nodes after restoring tree mode, got %d", treeCount, tree.NodeCount())
	}

	// Verify hierarchy is restored (task-1 should have depth > 0)
	for _, node := range tree.flatList {
		if node.Issue.ID == "task-1" && node.Depth == 0 {
			t.Error("expected task-1 to have depth > 0 in restored tree mode")
		}
	}
}

// =============================================================================
// Sticky scroll tests (bd-2z9)
// =============================================================================

// TestStickyScrollNoParentOffScreen verifies no sticky header when parent is visible
func TestStickyScrollNoParentOffScreen(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic One", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task One", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 20) // Large viewport - everything visible

	// Select task-1 (child) — parent epic-1 is visible on screen
	tree.MoveDown()
	if tree.GetSelectedID() != "task-1" {
		t.Fatalf("expected task-1 selected, got %s", tree.GetSelectedID())
	}

	// No sticky lines should be generated since parent is visible
	lines := tree.StickyScrollLines()
	if len(lines) != 0 {
		t.Errorf("expected 0 sticky lines when parent is visible, got %d", len(lines))
	}
}

// TestStickyScrollParentOffScreen verifies sticky header appears when parent scrolls off
func TestStickyScrollParentOffScreen(t *testing.T) {
	now := time.Now()

	// Create a deep tree with enough nodes to force scrolling
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic One", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
	}
	// Add 20 children under epic-1 so the parent scrolls off screen
	for i := 0; i < 20; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("task-%02d", i),
			Title:     fmt.Sprintf("Task %02d", i),
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Duration(i+1) * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: fmt.Sprintf("task-%02d", i), DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		})
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 10) // Small viewport — will need scrolling

	// Move cursor down until parent scrolls off screen
	for i := 0; i < 15; i++ {
		tree.MoveDown()
	}

	// Parent epic-1 should now be off screen
	start, _ := tree.visibleRange()
	if start == 0 {
		t.Fatal("expected viewport to have scrolled past the parent")
	}

	// Sticky lines should contain parent info
	lines := tree.StickyScrollLines()
	if len(lines) == 0 {
		t.Error("expected sticky scroll lines when parent is off screen")
	}

	// The sticky line should reference the parent issue
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Epic One") || strings.Contains(line, "epic-1") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sticky lines should reference parent 'Epic One', got: %v", lines)
	}
}

// TestStickyScrollMaxLines verifies sticky scroll is limited to 2 lines max
func TestStickyScrollMaxLines(t *testing.T) {
	now := time.Now()

	// Create a deeply nested tree: epic -> feature -> task -> many subtasks
	issues := []model.Issue{
		{ID: "epic", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "feature", Title: "Feature", Priority: 1, IssueType: model.TypeFeature, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "feature", DependsOnID: "epic", Type: model.DepParentChild},
			},
		},
		{
			ID: "task", Title: "Task", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task", DependsOnID: "feature", Type: model.DepParentChild},
			},
		},
	}
	// Add subtasks under task
	for i := 0; i < 20; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("sub-%02d", i),
			Title:     fmt.Sprintf("Subtask %02d", i),
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Duration(i+3) * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: fmt.Sprintf("sub-%02d", i), DependsOnID: "task", Type: model.DepParentChild},
			},
		})
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 10) // Small viewport

	// Expand all nodes to make subtasks visible
	tree.ExpandAll()

	// Move cursor far down so all ancestors scroll off
	for i := 0; i < 18; i++ {
		tree.MoveDown()
	}

	lines := tree.StickyScrollLines()

	// Should be at most 2 lines (don't consume too much viewport space)
	if len(lines) > 2 {
		t.Errorf("expected at most 2 sticky scroll lines, got %d", len(lines))
	}
}

// TestStickyScrollInFlatMode verifies no sticky scroll in flat mode
func TestStickyScrollInFlatMode(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		},
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 10)

	// Switch to flat mode
	tree.ToggleFlatMode()

	lines := tree.StickyScrollLines()
	if len(lines) != 0 {
		t.Errorf("expected 0 sticky lines in flat mode, got %d", len(lines))
	}
}

// TestStickyScrollRenderedInView verifies sticky scroll appears in the View output
func TestStickyScrollRenderedInView(t *testing.T) {
	now := time.Now()

	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic One", Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
	}
	for i := 0; i < 20; i++ {
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("task-%02d", i),
			Title:     fmt.Sprintf("Task %02d", i),
			Priority:  2,
			IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Duration(i+1) * time.Hour),
			Dependencies: []*model.Dependency{
				{IssueID: fmt.Sprintf("task-%02d", i), DependsOnID: "epic-1", Type: model.DepParentChild},
			},
		})
	}

	tree := newIsolatedTree(t)
	tree.Build(issues)
	tree.SetSize(100, 10)

	// Scroll down past parent
	for i := 0; i < 15; i++ {
		tree.MoveDown()
	}

	view := tree.View()
	// The view should contain some indicator of the sticky parent
	// The sticky parent line contains the parent's title in a muted style
	if !strings.Contains(view, "Epic One") {
		t.Errorf("expected sticky parent 'Epic One' in view output when parent is scrolled off-screen")
	}
}
