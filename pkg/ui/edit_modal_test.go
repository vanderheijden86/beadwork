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
	if modal.title != "Test Issue" {
		t.Errorf("Expected title %q, got %q", "Test Issue", modal.title)
	}
	if modal.status != "in_progress" {
		t.Errorf("Expected status %q, got %q", "in_progress", modal.status)
	}
	if modal.priority != "P1" {
		t.Errorf("Expected priority %q, got %q", "P1", modal.priority)
	}
	if modal.issueType != "bug" {
		t.Errorf("Expected type %q, got %q", "bug", modal.issueType)
	}
	if modal.assignee != "alice" {
		t.Errorf("Expected assignee %q, got %q", "alice", modal.assignee)
	}
	if modal.labels != "frontend, urgent" {
		t.Errorf("Expected labels %q, got %q", "frontend, urgent", modal.labels)
	}
	if modal.description != "A test description" {
		t.Errorf("Expected description %q, got %q", "A test description", modal.description)
	}
	if modal.notes != "Some notes" {
		t.Errorf("Expected notes %q, got %q", "Some notes", modal.notes)
	}
}

func TestNewEditModal_StoresOriginals(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:        "test-123",
		Title:     "Original Title",
		Status:    model.StatusOpen,
		Priority:  2,
		IssueType: model.TypeTask,
		Assignee:  "alice",
		Labels:    []string{"frontend"},
	}

	modal := NewEditModal(issue, theme)

	expected := map[string]string{
		"title":       "Original Title",
		"status":      "open",
		"priority":    "P2",
		"type":        "task",
		"assignee":    "alice",
		"labels":      "frontend",
		"description": "",
		"notes":       "",
	}

	for key, want := range expected {
		got, ok := modal.originals[key]
		if !ok {
			t.Errorf("Missing original for key %q", key)
			continue
		}
		if got != want {
			t.Errorf("Original %q: expected %q, got %q", key, want, got)
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
	if modal.title != "" {
		t.Errorf("Expected empty title, got %q", modal.title)
	}
	if modal.priority != "P2" {
		t.Errorf("Expected priority %q, got %q", "P2", modal.priority)
	}
	if modal.issueType != "task" {
		t.Errorf("Expected type %q, got %q", "task", modal.issueType)
	}
	if modal.assignee != "" {
		t.Errorf("Expected empty assignee, got %q", modal.assignee)
	}
	if modal.labels != "" {
		t.Errorf("Expected empty labels, got %q", modal.labels)
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

	// Change only title and priority via bound fields
	modal.title = "New Title"
	modal.priority = "P0"

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

	// Change labels via bound field
	modal.labels = "frontend, backend"

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

func TestEditModal_BuildCreateArgs_AllNonEmpty(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	// Set fields via bound values
	modal.title = "New Issue"
	modal.assignee = "bob"
	modal.labels = "urgent"

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

func TestEditModal_CtrlC_ReturnsQuit(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	modal := NewCreateModal(theme)

	_, cmd := modal.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	// ctrl+c should return tea.Quit command
	if cmd == nil {
		t.Error("Ctrl+C should return a non-nil command (tea.Quit)")
	}
}

func TestEditModal_BuildUpdateArgs_NoChanges(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	issue := &model.Issue{
		ID:        "test-123",
		Title:     "Original Title",
		Status:    model.StatusOpen,
		Priority:  2,
		IssueType: model.TypeTask,
	}

	modal := NewEditModal(issue, theme)

	// No changes made
	args := modal.BuildUpdateArgs()

	if len(args) != 0 {
		t.Errorf("Expected no changed fields, got %d: %v", len(args), args)
	}
}
