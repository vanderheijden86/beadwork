package ui

import (
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
	tea "github.com/charmbracelet/bubbletea"
)

// Cover additional branches in Model.Update for quit/help/tab handling and update notices.
func TestUpdateHelpQuitAndTabFocus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Make model ready and split view
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Help toggle via ? then dismiss with another key
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp || m.focused != focusHelp {
		t.Fatalf("expected help overlay shown")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.showHelp || m.focused != focusTree {
		t.Fatalf("expected help overlay dismissed back to tree, got focus %v", m.focused)
	}

	// Exit tree view to test Tab toggling between list and detail
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = updated.(Model)
	if m.focused != focusList {
		t.Fatalf("expected list focus after exiting tree, got %v", m.focused)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusDetail {
		t.Fatalf("expected detail focus after tab")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusList {
		t.Fatalf("expected list focus after second tab")
	}

	// Escape should show quit confirm, 'y' should issue tea.Quit
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if !m.showQuitConfirm {
		t.Fatalf("expected quit confirm after esc")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatalf("expected quit command on confirm quit")
	}
}

func TestUpdateMsgSetsUpdateAvailable(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, "")
	updated, _ := m.Update(UpdateMsg{TagName: "v9.9.9", URL: "https://example"})
	m = updated.(Model)
	if !m.updateAvailable || m.updateTag != "v9.9.9" {
		t.Fatalf("update flag not set")
	}
}

// TestNarrowWindowTreeDetailHidden verifies that in a narrow window (width <= SplitViewThreshold),
// treeDetailHidden is true so Enter opens full-screen detail instead of toggling expand (bd-6eg).
func TestNarrowWindowTreeDetailHidden(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Narrow window: below SplitViewThreshold (100)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)

	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden=true in narrow window")
	}
	if m.focused != focusTree {
		t.Fatalf("expected focusTree, got %v", m.focused)
	}

	// Enter should open detail-only view (not toggle expand)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.focused != focusDetail {
		t.Fatalf("expected Enter to open detail view in narrow window, got focus %v", m.focused)
	}

	// Esc should return to tree
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.focused != focusTree {
		t.Fatalf("expected Esc to return to tree, got focus %v", m.focused)
	}
}

// TestResizeNarrowToWideStaysManual verifies that resizing from narrow to wide
// does NOT auto-show the detail panel (bd-6eg).
func TestResizeNarrowToWideStaysManual(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Start narrow
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)

	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden=true in narrow window")
	}

	// Resize to wide
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Should stay hidden - user must press d to restore
	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden to stay true after resize to wide (manual mode)")
	}
}
