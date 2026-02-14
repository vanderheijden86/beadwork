package ui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

func TestModelFiltering(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Open Issue", Status: model.StatusOpen, Priority: 1},
		{ID: "2", Title: "Closed Issue", Status: model.StatusClosed, Priority: 2},
		{ID: "3", Title: "Blocked Issue", Status: model.StatusBlocked, Priority: 1},
		{
			ID: "4", Title: "Ready Issue", Status: model.StatusOpen, Priority: 1,
			Dependencies: []*model.Dependency{},
		},
		{
			ID: "5", Title: "Blocked by Open", Status: model.StatusOpen, Priority: 1,
			Dependencies: []*model.Dependency{
				{DependsOnID: "3", Type: model.DepBlocks},
			},
		},
		{ID: "6", Title: "Tombstone Issue", Status: model.StatusTombstone, Priority: 1},
		{
			ID: "7", Title: "Blocked by Tombstone", Status: model.StatusOpen, Priority: 1,
			Dependencies: []*model.Dependency{
				{DependsOnID: "6", Type: model.DepBlocks},
			},
		},
		{
			ID: "8", Title: "Blocked by Open (legacy)", Status: model.StatusOpen, Priority: 1,
			Dependencies: []*model.Dependency{
				{DependsOnID: "3", Type: ""},
			},
		},
	}

	m := ui.NewModel(issues, nil, "")

	// Test "All"
	if len(m.FilteredIssues()) != 8 {
		t.Errorf("Expected 8 issues for 'all', got %d", len(m.FilteredIssues()))
	}

	// Test "Open" (includes Open, InProgress, Blocked)
	m.SetFilter("open")
	if len(m.FilteredIssues()) != 6 {
		t.Errorf("Expected 6 issues for 'open', got %d", len(m.FilteredIssues()))
	}

	// Test "Closed"
	m.SetFilter("closed")
	closedIssues := m.FilteredIssues()
	if len(closedIssues) != 2 {
		t.Errorf("Expected 2 issues for 'closed', got %d", len(closedIssues))
	} else {
		got := map[string]bool{
			closedIssues[0].ID: true,
			closedIssues[1].ID: true,
		}
		if !got["2"] || !got["6"] {
			t.Errorf("Expected closed issues to include IDs 2 and 6, got %#v", got)
		}
	}

	// Test "Ready"
	m.SetFilter("ready")
	readyIssues := m.FilteredIssues()
	if len(readyIssues) != 3 {
		t.Errorf("Expected 3 issues for 'ready', got %d", len(readyIssues))
		for _, i := range readyIssues {
			t.Logf("Got issue: %s", i.Title)
		}
	} else {
		got := map[string]bool{
			readyIssues[0].ID: true,
			readyIssues[1].ID: true,
			readyIssues[2].ID: true,
		}
		if !got["1"] || !got["4"] || !got["7"] {
			t.Errorf("Expected ready issues to include IDs 1, 4, and 7, got %#v", got)
		}
	}
}

func TestFormatTimeRel(t *testing.T) {
	now := time.Now()
	tests := []struct {
		t        time.Time
		expected string
	}{
		{now.Add(-30 * time.Minute), "30m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-25 * time.Hour), "1d ago"},
		{now.Add(-48 * time.Hour), "2d ago"},
		{time.Time{}, "unknown"},
	}

	for _, tt := range tests {
		got := ui.FormatTimeRel(tt.t)
		if got != tt.expected {
			t.Errorf("FormatTimeRel(%v) = %s; want %s", tt.t, got, tt.expected)
		}
	}
}

func TestTimeTravelMode(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}

	m := ui.NewModel(issues, nil, "")

	if m.IsTimeTravelMode() {
		t.Error("Expected not to be in time-travel mode initially")
	}

	if m.TimeTravelDiff() != nil {
		t.Error("Expected TimeTravelDiff to be nil initially")
	}
}

func TestGetTypeIconMD(t *testing.T) {
	tests := []struct {
		issueType string
		expected  string
	}{
		{"bug", "🐛"},
		{"feature", "✨"},
		{"task", "📋"},
		{"epic", "🚀"},
		{"chore", "🧹"},
		{"unknown", "•"},
		{"", "•"},
	}

	for _, tt := range tests {
		got := ui.GetTypeIconMD(tt.issueType)
		if got != tt.expected {
			t.Errorf("GetTypeIconMD(%q) = %s; want %s", tt.issueType, got, tt.expected)
		}
	}
}

func TestModelCreationWithEmptyIssues(t *testing.T) {
	m := ui.NewModel([]model.Issue{}, nil, "")

	if len(m.FilteredIssues()) != 0 {
		t.Errorf("Expected 0 issues for empty input, got %d", len(m.FilteredIssues()))
	}

	m.SetFilter("open")
	m.SetFilter("closed")
	m.SetFilter("ready")
}

func TestIssueItemDiffStatus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test", Status: model.StatusOpen},
	}

	m := ui.NewModel(issues, nil, "")

	filtered := m.FilteredIssues()
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(filtered))
	}
}

// =============================================================================
// Focus Transition Tests (bv-5e5q)
// =============================================================================

// TestFocusStateInitial verifies initial focus state is "tree" (bd-dxc: tree view is default)
func TestFocusStateInitial(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")

	if m.FocusState() != "tree" {
		t.Errorf("Initial focus state = %q, want %q", m.FocusState(), "tree")
	}
}

// TestFocusTransitionBoard verifies 'b' toggles board view
func TestFocusTransitionBoard(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	if m.FocusState() != "list" {
		t.Fatalf("Initial focus = %q, want 'list'", m.FocusState())
	}
	if m.IsBoardView() {
		t.Fatal("IsBoardView should be false initially")
	}

	newM, _ := m.Update(keyMsg("b"))
	m = newM.(ui.Model)

	if m.FocusState() != "board" {
		t.Errorf("After 'b', focus = %q, want 'board'", m.FocusState())
	}
	if !m.IsBoardView() {
		t.Error("IsBoardView should be true after 'b'")
	}

	newM, _ = m.Update(keyMsg("b"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After second 'b', focus = %q, want 'list'", m.FocusState())
	}
	if m.IsBoardView() {
		t.Error("IsBoardView should be false after second 'b'")
	}
}

// TestFocusTransitionGraph verifies 'g' toggles graph view
func TestFocusTransitionGraph(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	newM, _ := m.Update(keyMsg("g"))
	m = newM.(ui.Model)

	if m.FocusState() != "graph" {
		t.Errorf("After 'g', focus = %q, want 'graph'", m.FocusState())
	}
	if !m.IsGraphView() {
		t.Error("IsGraphView should be true after 'g'")
	}

	newM, _ = m.Update(keyMsg("g"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After second 'g', focus = %q, want 'list'", m.FocusState())
	}
	if m.IsGraphView() {
		t.Error("IsGraphView should be false after second 'g'")
	}
}

// TestFocusTransitionActionable verifies 'a' toggles actionable view
func TestFocusTransitionActionable(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	newM, _ := m.Update(keyMsg("a"))
	m = newM.(ui.Model)

	if m.FocusState() != "actionable" {
		t.Errorf("After 'a', focus = %q, want 'actionable'", m.FocusState())
	}
	if !m.IsActionableView() {
		t.Error("IsActionableView should be true after 'a'")
	}

	newM, _ = m.Update(keyMsg("a"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After second 'a', focus = %q, want 'list'", m.FocusState())
	}
	if m.IsActionableView() {
		t.Error("IsActionableView should be false after second 'a'")
	}
}

// TestFocusTransitionInsights verifies 'i' toggles insights view
func TestFocusTransitionInsights(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	newM, _ := m.Update(keyMsg("i"))
	m = newM.(ui.Model)

	if m.FocusState() != "insights" {
		t.Errorf("After 'i', focus = %q, want 'insights'", m.FocusState())
	}

	newM, _ = m.Update(keyMsg("i"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After second 'i', focus = %q, want 'list'", m.FocusState())
	}
}

// TestFocusTransitionTree verifies 'E' toggles tree view (bd-dxc: starts in tree)
func TestFocusTransitionTree(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic},
	}
	m := ui.NewModel(issues, nil, "")

	// Default is tree view (bd-dxc)
	if m.FocusState() != "tree" {
		t.Fatalf("Initial focus = %q, want 'tree'", m.FocusState())
	}

	// Press 'E' to exit tree view
	newM, _ := m.Update(keyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After 'E', focus = %q, want 'list'", m.FocusState())
	}

	// Press 'E' again to re-enter tree view
	newM, _ = m.Update(keyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "tree" {
		t.Errorf("After second 'E', focus = %q, want 'tree'", m.FocusState())
	}
}

// TestFocusTransitionHelp verifies '?' opens help view
func TestFocusTransitionHelp(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")

	newM, _ := m.Update(keyMsg("?"))
	m = newM.(ui.Model)

	if m.FocusState() != "help" {
		t.Errorf("After '?', focus = %q, want 'help'", m.FocusState())
	}
}

// TestFocusTransitionHistory verifies 'h' toggles history view
func TestFocusTransitionHistory(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	newM, _ := m.Update(keyMsg("h"))
	m = newM.(ui.Model)

	if m.FocusState() != "history" {
		t.Errorf("After 'h', focus = %q, want 'history'", m.FocusState())
	}
	if !m.IsHistoryView() {
		t.Error("IsHistoryView should be true after 'h'")
	}

	newM, _ = m.Update(keyMsg("h"))
	m = newM.(ui.Model)

	if m.FocusState() != "list" {
		t.Errorf("After second 'h', focus = %q, want 'list'", m.FocusState())
	}
	if m.IsHistoryView() {
		t.Error("IsHistoryView should be false after second 'h'")
	}
}

// TestViewSwitchClearsOthers verifies switching views clears other view states
func TestViewSwitchClearsOthers(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")
	m = switchToList(t, m) // Exit default tree view (bd-dxc)

	newM, _ := m.Update(keyMsg("b"))
	m = newM.(ui.Model)

	if !m.IsBoardView() {
		t.Fatal("IsBoardView should be true after 'b'")
	}

	newM, _ = m.Update(keyMsg("g"))
	m = newM.(ui.Model)

	if !m.IsGraphView() {
		t.Error("IsGraphView should be true after 'g'")
	}
	if m.IsBoardView() {
		t.Error("IsBoardView should be false after switching to graph")
	}

	newM, _ = m.Update(keyMsg("a"))
	m = newM.(ui.Model)

	if !m.IsActionableView() {
		t.Error("IsActionableView should be true after 'a'")
	}
	if m.IsGraphView() {
		t.Error("IsGraphView should be false after switching to actionable")
	}
}

// TestEscClosesViews verifies Esc returns to list from various views
func TestEscClosesViews(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}

	tests := []struct {
		name       string
		enterKey   string
		expectView string
	}{
		{"board", "b", "board"},
		{"graph", "g", "graph"},
		{"actionable", "a", "actionable"},
		{"insights", "i", "insights"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ui.NewModel(issues, nil, "")
			m = switchToList(t, m) // Exit default tree view (bd-dxc)

			newM, _ := m.Update(keyMsg(tt.enterKey))
			m = newM.(ui.Model)

			if m.FocusState() != tt.expectView {
				t.Fatalf("After %q, focus = %q, want %q", tt.enterKey, m.FocusState(), tt.expectView)
			}

			newM, _ = m.Update(keyMsg("esc"))
			m = newM.(ui.Model)

			if m.FocusState() != "list" {
				t.Errorf("After Esc from %s, focus = %q, want 'list'", tt.name, m.FocusState())
			}
		})
	}
}

// TestQuitClosesViews verifies 'q' returns to list from various views
func TestQuitClosesViews(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test Issue", Status: model.StatusOpen, Priority: 1},
	}

	tests := []struct {
		name       string
		enterKey   string
		expectView string
	}{
		{"board", "b", "board"},
		{"graph", "g", "graph"},
		{"insights", "i", "insights"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ui.NewModel(issues, nil, "")
			m = switchToList(t, m) // Exit default tree view (bd-dxc)

			newM, _ := m.Update(keyMsg(tt.enterKey))
			m = newM.(ui.Model)

			if m.FocusState() != tt.expectView {
				t.Fatalf("After %q, focus = %q, want %q", tt.enterKey, m.FocusState(), tt.expectView)
			}

			newM, _ = m.Update(keyMsg("q"))
			m = newM.(ui.Model)

			if m.FocusState() != "list" {
				t.Errorf("After 'q' from %s, focus = %q, want 'list'", tt.name, m.FocusState())
			}
		})
	}
}

// TestEmptyIssuesDoesNotPanic verifies state machine handles empty issues
func TestEmptyIssuesDoesNotPanic(t *testing.T) {
	m := ui.NewModel([]model.Issue{}, nil, "")

	keys := []string{"b", "g", "a", "i", "E", "H", "?", "j", "k", "enter", "esc"}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic on key %q: %v", key, r)
				}
			}()

			newM, _ := m.Update(keyMsg(key))
			m = newM.(ui.Model)
		})
	}
}

// TestFocusStateString verifies all focus states have valid strings
func TestFocusStateString(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test", Status: model.StatusOpen, Priority: 1},
	}
	m := ui.NewModel(issues, nil, "")

	state := m.FocusState()
	if state == "unknown" {
		t.Error("Initial focus state should not be 'unknown'")
	}
	if state == "" {
		t.Error("Focus state should not be empty string")
	}
}

// switchToList exits the default tree view to reach list view (bd-dxc).
// Since tree view is the default on launch, tests that need list focus
// must press 'E' first to toggle out of tree view.
func switchToList(t *testing.T, m ui.Model) ui.Model {
	t.Helper()
	if m.FocusState() != "tree" {
		return m
	}
	newM, _ := m.Update(keyMsg("E"))
	m = newM.(ui.Model)
	if m.FocusState() != "list" {
		t.Fatalf("switchToList: after 'E', focus = %q, want 'list'", m.FocusState())
	}
	return m
}

// Helper to create a KeyMsg
func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	}
}
