package ui

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewEditModal_PopulatesFromIssue(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:          "test-123",
		Title:       "Test Issue",
		Status:      model.StatusInProgress,
		Priority:    1,
		IssueType:   model.TypeBug,
		Assignee:    "alice",
		Labels:      []string{"frontend", "urgent"},
		Description: "A test description",
		Notes:       "Some notes",
	}

	modal := NewEditModal(issue, theme)

	if modal.issueID != "test-123" {
		t.Errorf("Expected issueID test-123, got %s", modal.issueID)
	}
	if modal.isCreateMode {
		t.Error("Expected edit mode, got create mode")
	}

	// Check field values
	expectedValues := map[string]string{
		"title":       "Test Issue",
		"status":      "in_progress",
		"priority":    "P1",
		"type":        "bug",
		"assignee":    "alice",
		"labels":      "frontend, urgent",
		"description": "A test description",
		"notes":       "Some notes",
	}

	for _, field := range modal.fields {
		expected := expectedValues[field.Key]
		current := modal.getCurrentValue(field)
		if current != expected {
			t.Errorf("Field %s: expected %q, got %q", field.Key, expected, current)
		}
		if field.Original != expected {
			t.Errorf("Field %s original: expected %q, got %q", field.Key, expected, field.Original)
		}
	}
}

func TestNewCreateModal_HasDefaults(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	if !modal.isCreateMode {
		t.Error("Expected create mode")
	}
	if modal.issueID != "" {
		t.Errorf("Expected empty issueID, got %s", modal.issueID)
	}

	// Check defaults
	expectedDefaults := map[string]string{
		"title":       "",
		"status":      "open",
		"priority":    "P2",
		"type":        "task",
		"assignee":    "",
		"labels":      "",
		"description": "",
		"notes":       "",
	}

	for _, field := range modal.fields {
		expected := expectedDefaults[field.Key]
		current := modal.getCurrentValue(field)
		if current != expected {
			t.Errorf("Field %s: expected %q, got %q", field.Key, expected, current)
		}
	}
}

func TestEditModal_TabNavigation(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	if modal.focusedField != 0 {
		t.Errorf("Expected initial focus on field 0, got %d", modal.focusedField)
	}

	// Tab forward
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	if modal.focusedField != 1 {
		t.Errorf("After tab: expected field 1, got %d", modal.focusedField)
	}

	// Tab forward again
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	if modal.focusedField != 2 {
		t.Errorf("After 2nd tab: expected field 2, got %d", modal.focusedField)
	}

	// Shift+Tab backward
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if modal.focusedField != 1 {
		t.Errorf("After shift+tab: expected field 1, got %d", modal.focusedField)
	}

	// Tab wraps around
	for i := 0; i < len(modal.fields); i++ {
		modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	if modal.focusedField != 1 {
		t.Errorf("After full cycle: expected field 1, got %d", modal.focusedField)
	}
}

func TestEditModal_SelectFieldNavigation(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	// Navigate to status field (index 1)
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyTab})

	statusField := &modal.fields[modal.focusedField]
	if statusField.Key != "status" {
		t.Fatalf("Expected to focus status field, got %s", statusField.Key)
	}

	initialSelected := statusField.Selected
	if modal.getCurrentValue(*statusField) != "open" {
		t.Errorf("Expected initial status 'open', got %s", modal.getCurrentValue(*statusField))
	}

	// Right arrow should change selection
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRight})
	statusField = &modal.fields[modal.focusedField]
	if statusField.Selected == initialSelected {
		t.Error("Right arrow should change selection")
	}

	// Left arrow should change back
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyLeft})
	statusField = &modal.fields[modal.focusedField]
	if statusField.Selected != initialSelected {
		t.Error("Left arrow should change selection back")
	}

	// h/l keys should work too
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	statusField = &modal.fields[modal.focusedField]
	if statusField.Selected == initialSelected {
		t.Error("'l' key should change selection")
	}
}

func TestEditModal_BuildUpdateArgs_OnlyChanged(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:          "test-123",
		Title:       "Original Title",
		Status:      model.StatusOpen,
		Priority:    2,
		IssueType:   model.TypeTask,
		Assignee:    "alice",
		Labels:      []string{"frontend"},
		Description: "Original description",
		Notes:       "Original notes",
	}

	modal := NewEditModal(issue, theme)

	// Change only title and priority
	modal.fields[0].Input.SetValue("New Title")
	modal.fields[2].Selected = 0 // Change priority to P0

	args := modal.BuildUpdateArgs()

	if len(args) != 2 {
		t.Errorf("Expected 2 changed fields, got %d: %v", len(args), args)
	}

	if args["title"] != "New Title" {
		t.Errorf("Expected title 'New Title', got %s", args["title"])
	}

	// Priority should be converted from display format (P0) to bd format (0)
	if args["priority"] != "0" {
		t.Errorf("Expected priority '0', got %s", args["priority"])
	}

	// Unchanged fields should not be in args
	if _, exists := args["status"]; exists {
		t.Error("Unchanged status should not be in args")
	}
	if _, exists := args["assignee"]; exists {
		t.Error("Unchanged assignee should not be in args")
	}
}

func TestEditModal_BuildCreateArgs_AllNonEmpty(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	// Set some fields
	modal.fields[0].Input.SetValue("New Issue")
	modal.fields[4].Input.SetValue("bob")   // assignee
	modal.fields[5].Input.SetValue("urgent") // labels

	args := modal.BuildCreateArgs()

	// Should include title, priority, type, assignee, labels
	// Should NOT include status (bd create has no --status flag) or empty fields
	if args["title"] != "New Issue" {
		t.Errorf("Expected title 'New Issue', got %s", args["title"])
	}
	if _, exists := args["status"]; exists {
		t.Error("Status should not be in create args (bd create has no --status flag)")
	}
	if args["priority"] != "2" { // P2 -> 2
		t.Errorf("Expected priority '2', got %s", args["priority"])
	}
	if args["type"] != "task" {
		t.Errorf("Expected type 'task', got %s", args["type"])
	}
	if args["assignee"] != "bob" {
		t.Errorf("Expected assignee 'bob', got %s", args["assignee"])
	}
	if args["labels"] != "urgent" {
		t.Errorf("Expected labels 'urgent', got %s", args["labels"])
	}

	// Empty fields should not be included
	if _, exists := args["description"]; exists {
		t.Error("Empty description should not be in create args")
	}
	if _, exists := args["notes"]; exists {
		t.Error("Empty notes should not be in create args")
	}
}

func TestEditModal_SaveCancelRequests(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	if modal.IsSaveRequested() {
		t.Error("Initially should not have save request")
	}
	if modal.IsCancelRequested() {
		t.Error("Initially should not have cancel request")
	}

	// Ctrl+S should set save request
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !modal.IsSaveRequested() {
		t.Error("Ctrl+S should set save request")
	}

	// Reset for cancel test
	modal = NewCreateModal(theme)

	// Esc should set cancel request
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !modal.IsCancelRequested() {
		t.Error("Esc should set cancel request")
	}
}

func TestEditModal_PriorityConversion(t *testing.T) {
	tests := []struct {
		display string
		bd      int
	}{
		{"P0", 0},
		{"P1", 1},
		{"P2", 2},
		{"P3", 3},
		{"P4", 4},
	}

	for _, tt := range tests {
		// Test formatPriority
		if got := formatPriority(tt.bd); got != tt.display {
			t.Errorf("formatPriority(%d) = %s, want %s", tt.bd, got, tt.display)
		}

		// Test parsePriority
		if got := parsePriority(tt.display); got != tt.bd {
			t.Errorf("parsePriority(%s) = %d, want %d", tt.display, got, tt.bd)
		}
	}
}

func TestEditModal_ViewContainsTitle(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:        "test-xyz",
		Title:     "Test Issue",
		Status:    model.StatusOpen,
		Priority:  2,
		IssueType: model.TypeTask,
	}

	modal := NewEditModal(issue, theme)
	modal.SetSize(100, 40)

	view := modal.View()

	if !strings.Contains(view, "Edit Issue") {
		t.Error("View should contain 'Edit Issue' header")
	}
	if !strings.Contains(view, "test-xyz") {
		t.Error("View should contain issue ID")
	}

	// Create mode
	createModal := NewCreateModal(theme)
	createModal.SetSize(100, 40)
	createView := createModal.View()

	if !strings.Contains(createView, "Create Issue") {
		t.Error("Create view should contain 'Create Issue' header")
	}
}

func TestEditModal_BuildUpdateArgs_LabelsUsesSetLabels(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:        "test-123",
		Title:     "Test",
		Status:    model.StatusOpen,
		Priority:  2,
		IssueType: model.TypeTask,
		Labels:    []string{"frontend"},
	}

	modal := NewEditModal(issue, theme)

	// Change labels
	for i := range modal.fields {
		if modal.fields[i].Key == "labels" {
			modal.fields[i].Input.SetValue("frontend, backend")
			break
		}
	}

	args := modal.BuildUpdateArgs()

	// Should use "set-labels" key (not "labels") for bd update
	if _, exists := args["labels"]; exists {
		t.Error("BuildUpdateArgs should not use 'labels' key, should use 'set-labels'")
	}
	if val, exists := args["set-labels"]; !exists {
		t.Error("BuildUpdateArgs should use 'set-labels' key for label changes")
	} else if val != "frontend, backend" {
		t.Errorf("Expected 'frontend, backend', got %q", val)
	}
}

func TestEditModal_TextInputUpdates(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	// Focus is on title field initially
	if modal.fields[0].Key != "title" {
		t.Fatalf("Expected first field to be title, got %s", modal.fields[0].Key)
	}

	// Simulate typing
	for _, r := range "Test" {
		modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	titleValue := modal.fields[0].Input.Value()
	if titleValue != "Test" {
		t.Errorf("Expected title 'Test', got %s", titleValue)
	}
}
