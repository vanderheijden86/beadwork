package ui_test

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// View Transition Integration Tests (bv-i3ls)
// Tests verifying state preservation and behavior across view switches

// Helper to create a KeyMsg for a string key
func integrationKeyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	}
}

// Helper to create special key messages
func integrationSpecialKey(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

// createTestIssues creates a set of test issues for integration tests
func createTestIssues(count int) []model.Issue {
	issues := make([]model.Issue, count)
	statuses := []model.Status{model.StatusOpen, model.StatusInProgress, model.StatusBlocked, model.StatusClosed}
	priorities := []int{0, 1, 2, 3}

	for i := 0; i < count; i++ {
		issues[i] = model.Issue{
			ID:        "test-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Title:     "Test Issue",
			Status:    statuses[i%len(statuses)],
			Priority:  priorities[i%len(priorities)],
			IssueType: model.TypeTask,
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return issues
}

// Basic View Switching Tests

// TestViewTransitionListToTree verifies Tree (default) -> List -> Tree transition (bd-dxc)
func TestViewTransitionListToTree(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Should start in tree view (bd-dxc)
	if m.FocusState() != "tree" {
		t.Errorf("Expected initial focus 'tree', got %q", m.FocusState())
	}

	// Press 'E' to toggle to list view
	newM, _ := m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After 'E', expected focus 'list', got %q", m.FocusState())
	}

	// Press 'E' again to toggle back to tree
	newM, _ = m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "tree" {
		t.Errorf("After second 'E', expected focus 'tree', got %q", m.FocusState())
	}
}

// TestViewTransitionListToBoard verifies List -> Board -> List transition
func TestViewTransitionListToBoard(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	// Press 'b' to toggle board view
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if !m.IsBoardView() {
		t.Error("IsBoardView should be true after 'b'")
	}

	// Press 'b' again to toggle back
	newM, _ = m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if m.IsBoardView() {
		t.Error("IsBoardView should be false after second 'b'")
	}
	if m.FocusState() != "list" {
		t.Errorf("Expected focus 'list' after board toggle, got %q", m.FocusState())
	}
}

// TestViewTransitionFullCycle verifies List -> Board -> Tree -> List cycle
func TestViewTransitionFullCycle(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	// Enter board view
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)
	if !m.IsBoardView() {
		t.Error("Should be in board view")
	}

	// Enter tree view (clears board)
	newM, _ = m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)
	if m.FocusState() != "tree" {
		t.Errorf("Should be in tree view, got %q", m.FocusState())
	}

	// Return to list via 'E' toggle (tree specific exit key)
	newM, _ = m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After 'E' from tree, expected 'list', got %q", m.FocusState())
	}
}

// State Preservation Tests

// TestViewTransitionClearsOtherViews verifies entering one view clears others
func TestViewTransitionClearsOtherViews(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	// Enter board view
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if !m.IsBoardView() {
		t.Error("Should be in board view")
	}

	// Enter tree view (should clear board)
	newM, _ = m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)

	if m.IsBoardView() {
		t.Error("Board view should be cleared when entering tree")
	}
	if m.FocusState() != "tree" {
		t.Error("Should be in tree view")
	}
}

// TestViewTransitionFilterPreserved verifies filter state is preserved across views
func TestViewTransitionFilterPreserved(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	// Apply a filter
	m.SetFilter("open")
	initialCount := len(m.FilteredIssues())

	// Switch to board and back
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	newM, _ = m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	// Filter should still be active
	afterCount := len(m.FilteredIssues())
	if afterCount != initialCount {
		t.Errorf("Filter not preserved: before=%d, after=%d", initialCount, afterCount)
	}
}

// Edge Case Tests

// TestViewTransitionEmptyIssues verifies view switching with no issues doesn't panic
func TestViewTransitionEmptyIssues(t *testing.T) {
	m := ui.NewModel([]model.Issue{}, "")

	// Should not panic on any view transition
	keys := []string{"E", "b", "g", "a", "i", "?"}
	for _, k := range keys {
		newM, _ := m.Update(integrationKeyMsg(k))
		m = newM.(ui.Model)
	}
}

// TestViewTransitionEscBehavior verifies Esc behavior varies by view
func TestViewTransitionEscBehavior(t *testing.T) {
	issues := createTestIssues(10)

	t.Run("tree_E_returns_to_list", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		// Already in tree (bd-dxc default)
		if m.FocusState() != "tree" {
			t.Fatalf("Expected tree, got %q", m.FocusState())
		}

		// 'E' from tree should return to list (toggle behavior)
		newM, _ := m.Update(integrationKeyMsg("E"))
		m = newM.(ui.Model)

		if m.FocusState() != "list" {
			t.Errorf("'E' from tree should return to list, got %q", m.FocusState())
		}
	})

	t.Run("board_toggle_exits_board", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		m = switchToList(t, m) // Exit default tree view (bd-dxc)
		newM, _ := m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)

		// Press 'b' again to toggle off board
		newM, _ = m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)

		if m.IsBoardView() {
			t.Error("'b' should toggle off board view")
		}
	})

}

// TestViewToggleExitBehavior verifies toggle keys exit their respective views
func TestViewToggleExitBehavior(t *testing.T) {
	issues := createTestIssues(10)

	// Tree view: already default (bd-dxc), 'E' exits, 'E' re-enters
	t.Run("tree_E_toggle", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		// Already in tree (bd-dxc)
		if m.FocusState() != "tree" {
			t.Errorf("Expected tree, got %q", m.FocusState())
		}
		// Exit with E
		newM, _ := m.Update(integrationKeyMsg("E"))
		m = newM.(ui.Model)
		if m.FocusState() != "list" {
			t.Errorf("'E' should toggle to list, got %q", m.FocusState())
		}
	})

	// Board view uses 'b' to toggle
	t.Run("board_b_toggle", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		m = switchToList(t, m) // Exit default tree view (bd-dxc)
		newM, _ := m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)
		if !m.IsBoardView() {
			t.Error("Should be in board view")
		}
		newM, _ = m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)
		if m.IsBoardView() {
			t.Error("'b' should toggle off board")
		}
	})

}

// Rapid Switching Stress Tests

// TestRapidViewSwitching verifies no panics during rapid view changes
func TestRapidViewSwitching(t *testing.T) {
	issues := createTestIssues(50)
	m := ui.NewModel(issues, "")

	keys := []string{"E", "b", "g", "a", "i", "E", "b", "g"}

	for i := 0; i < 100; i++ {
		for _, k := range keys {
			newM, _ := m.Update(integrationKeyMsg(k))
			m = newM.(ui.Model)
		}
	}
}

// TestRapidViewSwitchingWithNavigation verifies navigation during rapid switches
func TestRapidViewSwitchingWithNavigation(t *testing.T) {
	issues := createTestIssues(50)
	m := ui.NewModel(issues, "")

	actions := []tea.KeyMsg{
		integrationKeyMsg("E"),            // Toggle tree (exits default tree)
		integrationKeyMsg("j"),            // Move down in list
		integrationKeyMsg("j"),            // Move down in list
		integrationKeyMsg("b"),            // Enter board
		integrationKeyMsg("l"),            // Move right in board
		integrationKeyMsg("g"),            // Enter graph
		integrationKeyMsg("j"),            // Move down in graph
		integrationSpecialKey(tea.KeyEsc), // Exit to list
		integrationKeyMsg("j"),            // Move down in list
	}

	for i := 0; i < 50; i++ {
		for _, k := range actions {
			newM, _ := m.Update(k)
			m = newM.(ui.Model)
		}
	}
}

// Performance Tests

// TestViewSwitchingPerformance verifies reasonable performance for view switching
func TestViewSwitchingPerformance(t *testing.T) {
	issues := createTestIssues(100)
	m := ui.NewModel(issues, "")

	keys := []string{"E", "b", "g", "E", "b", "g"}

	start := time.Now()

	for i := 0; i < 100; i++ {
		for _, k := range keys {
			newM, _ := m.Update(integrationKeyMsg(k))
			m = newM.(ui.Model)
		}
	}

	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("View switching too slow: %v for 600 switches", elapsed)
	}
}

// Help View Integration Tests

// TestHelpViewTransition verifies help view can be opened from any view
func TestHelpViewTransition(t *testing.T) {
	issues := createTestIssues(10)

	views := []struct {
		name     string
		enterKey string
	}{
		{"tree", ""},    // Default is tree (bd-dxc)
		{"list", "E"},   // Toggle to list first
		{"board", "b"},  // Need list first, see below
		{"graph", "g"},  // Need list first, see below
	}

	for _, v := range views {
		t.Run(v.name, func(t *testing.T) {
			m := ui.NewModel(issues, "")

			// For board/graph, exit tree first since those keys are intercepted in tree view
			if v.name == "board" || v.name == "graph" {
				m = switchToList(t, m)
			}

			// Enter the base view
			if v.enterKey != "" {
				newM, _ := m.Update(integrationKeyMsg(v.enterKey))
				m = newM.(ui.Model)
			}

			// Open help with '?'
			newM, _ := m.Update(integrationKeyMsg("?"))
			m = newM.(ui.Model)

			if m.FocusState() != "help" {
				t.Errorf("Expected help focus from %s view, got %q", v.name, m.FocusState())
			}

			// Exit help with Esc
			newM, _ = m.Update(integrationSpecialKey(tea.KeyEsc))
			m = newM.(ui.Model)

			if m.FocusState() == "help" {
				t.Error("Should have exited help with Esc")
			}
		})
	}
}

// View Rendering Integration Tests

// TestAllViewsRenderWithoutPanic verifies all views can render without panic
func TestAllViewsRenderWithoutPanic(t *testing.T) {
	issues := createTestIssues(20)

	views := []struct {
		name        string
		enterKey    string
		needsList   bool // Whether we need to exit tree first
	}{
		{"tree", "", false},         // Default is tree (bd-dxc)
		{"list", "E", false},        // Toggle from tree to list
		{"board", "b", true},        // Need list first
		{"graph", "g", true},        // Need list first
		{"actionable", "a", true},   // Need list first
		{"insights", "i", false},    // 'i' works from tree
		{"help", "?", false},        // '?' works from tree
	}

	for _, v := range views {
		t.Run(v.name, func(t *testing.T) {
			m := ui.NewModel(issues, "")

			if v.needsList {
				m = switchToList(t, m)
			}

			// Enter the view
			if v.enterKey != "" {
				newM, _ := m.Update(integrationKeyMsg(v.enterKey))
				m = newM.(ui.Model)
			}

			// Render should not panic
			output := m.View()
			if output == "" {
				t.Errorf("View() returned empty for %s view", v.name)
			}
		})
	}
}

// TestViewRenderingAtDifferentSizes verifies views render at various terminal sizes
func TestViewRenderingAtDifferentSizes(t *testing.T) {
	issues := createTestIssues(20)

	sizes := []struct {
		width, height int
	}{
		{80, 24},
		{120, 30},
		{160, 40},
		{40, 15},  // Narrow
		{200, 50}, // Wide
	}

	views := []struct {
		key       string
		name      string
		needsList bool
	}{
		{"", "tree", false},     // Default is tree
		{"E", "list", false},    // Toggle to list
		{"b", "board", true},    // Need list first
		{"g", "graph", true},    // Need list first
	}

	for _, size := range sizes {
		for _, v := range views {
			t.Run(fmt.Sprintf("%s_%dx%d", v.name, size.width, size.height), func(t *testing.T) {
				m := ui.NewModel(issues, "")

				// Set size
				newM, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
				m = newM.(ui.Model)

				if v.needsList {
					m = switchToList(t, m)
				}

				// Enter view
				if v.key != "" {
					newM, _ = m.Update(integrationKeyMsg(v.key))
					m = newM.(ui.Model)
				}

				// Render should not panic
				output := m.View()
				if output == "" {
					t.Errorf("View() returned empty for %s at %dx%d", v.name, size.width, size.height)
				}
			})
		}
	}
}
