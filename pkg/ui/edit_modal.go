package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"

	"github.com/charmbracelet/huh"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditModal provides field-by-field issue editing using huh forms
type EditModal struct {
	form         *huh.Form
	theme        Theme
	issueID      string
	isCreateMode bool
	width        int
	height       int

	// Field value pointers (huh binds to these)
	title       string
	status      string
	priority    string
	issueType   string
	assignee    string
	labels      string
	description string
	notes       string

	// Original values for dirty detection (update mode only)
	originals map[string]string

	// initCmd stores the tea.Cmd from form.Init() called during construction.
	// Returned by Init() for callers that can propagate cmds.
	initCmd tea.Cmd

	saveRequested   bool
	cancelRequested bool
}

// NewEditModal creates an edit modal pre-populated from an existing issue
func NewEditModal(issue *model.Issue, theme Theme) EditModal {
	m := EditModal{
		theme:       theme,
		issueID:     issue.ID,
		title:       issue.Title,
		status:      string(issue.Status),
		priority:    formatPriority(issue.Priority),
		issueType:   string(issue.IssueType),
		assignee:    issue.Assignee,
		labels:      strings.Join(issue.Labels, ", "),
		description: issue.Description,
		notes:       issue.Notes,
	}

	m.originals = map[string]string{
		"title":       m.title,
		"status":      m.status,
		"priority":    m.priority,
		"type":        m.issueType,
		"assignee":    m.assignee,
		"labels":      m.labels,
		"description": m.description,
		"notes":       m.notes,
	}

	m.form = buildEditForm(&m)
	m.initCmd = m.form.Init()
	return m
}

// NewCreateModal creates an edit modal with defaults for creating a new issue
func NewCreateModal(theme Theme) EditModal {
	m := EditModal{
		theme:        theme,
		isCreateMode: true,
		priority:     "P2",
		issueType:    "task",
	}

	m.form = buildCreateForm(&m)
	m.initCmd = m.form.Init()
	return m
}

func buildEditForm(m *EditModal) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Title").Value(&m.title),
			huh.NewSelect[string]().Title("Status").
				Options(makeOptions(getStatusOptions())...).
				Value(&m.status),
			huh.NewSelect[string]().Title("Priority").
				Options(makeOptions(getPriorityOptions())...).
				Value(&m.priority),
			huh.NewSelect[string]().Title("Type").
				Options(makeOptions(getTypeOptions())...).
				Value(&m.issueType),
			huh.NewInput().Title("Assignee").Value(&m.assignee),
			huh.NewInput().Title("Labels").Value(&m.labels),
		),
		huh.NewGroup(
			huh.NewText().Title("Description").Value(&m.description).Lines(5),
			huh.NewText().Title("Notes").Value(&m.notes).Lines(3),
		),
	).WithTheme(huh.ThemeDracula()).
		WithShowHelp(true).
		WithShowErrors(true)
}

func buildCreateForm(m *EditModal) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Title").Value(&m.title),
			huh.NewSelect[string]().Title("Priority").
				Options(makeOptions(getPriorityOptions())...).
				Value(&m.priority),
			huh.NewSelect[string]().Title("Type").
				Options(makeOptions(getTypeOptions())...).
				Value(&m.issueType),
			huh.NewInput().Title("Assignee").Value(&m.assignee),
			huh.NewInput().Title("Labels").Value(&m.labels),
		),
		huh.NewGroup(
			huh.NewText().Title("Description").Value(&m.description).Lines(5),
			huh.NewText().Title("Notes").Value(&m.notes).Lines(3),
		),
	).WithTheme(huh.ThemeDracula()).
		WithShowHelp(true).
		WithShowErrors(true)
}

func makeOptions(values []string) []huh.Option[string] {
	opts := make([]huh.Option[string], len(values))
	for i, v := range values {
		opts[i] = huh.NewOption(v, v)
	}
	return opts
}

// getStatusOptions returns the list of valid status values
func getStatusOptions() []string {
	return []string{"open", "in_progress", "blocked", "deferred", "review", "closed"}
}

// getPriorityOptions returns the list of priority display values
func getPriorityOptions() []string {
	return []string{"P0", "P1", "P2", "P3", "P4"}
}

// getTypeOptions returns the list of valid issue types
func getTypeOptions() []string {
	return []string{"bug", "feature", "task", "epic", "chore"}
}

// parsePriority converts display format to int (e.g., "P2" -> 2)
func parsePriority(s string) int {
	if len(s) > 1 && s[0] == 'P' {
		if val, err := strconv.Atoi(s[1:]); err == nil {
			return val
		}
	}
	return 2 // default
}

// Init returns the stored init command from form construction.
// The form is already initialized during NewEditModal/NewCreateModal;
// this method allows callers that can propagate tea.Cmd to do so.
func (m EditModal) Init() tea.Cmd {
	return m.initCmd
}

// Update handles input for the edit modal
func (m EditModal) Update(msg tea.Msg) (EditModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.saveRequested = true
			return m, nil
		case "esc":
			m.cancelRequested = true
			return m, nil
		}
	}

	model, cmd := m.form.Update(msg)
	m.form = model.(*huh.Form)

	// Check if form was completed (user navigated past all fields and submitted)
	if m.form.State == huh.StateCompleted {
		m.saveRequested = true
	}

	return m, cmd
}

// View renders the edit modal
func (m EditModal) View() string {
	r := m.theme.Renderer

	formView := m.form.View()

	// Modal header
	headerStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	var title string
	if m.isCreateMode {
		title = "Create Issue"
	} else {
		title = fmt.Sprintf("Edit Issue: %s", m.issueID)
	}

	boxWidth := m.width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}

	var content strings.Builder
	content.WriteString(headerStyle.Render(title))
	content.WriteString("\n\n")
	content.WriteString(formView)

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(boxWidth)

	box := boxStyle.Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// SetSize sets the modal dimensions
func (m *EditModal) SetSize(width, height int) {
	m.width = width
	m.height = height

	boxWidth := width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}
	// Account for box padding (2 on each side) and border (1 on each side)
	m.form = m.form.WithWidth(boxWidth - 6)
}

// IsSaveRequested returns true if save was requested
func (m EditModal) IsSaveRequested() bool {
	return m.saveRequested
}

// IsCancelRequested returns true if cancel was requested
func (m EditModal) IsCancelRequested() bool {
	return m.cancelRequested
}

// BuildUpdateArgs returns only changed fields for bd update
func (m EditModal) BuildUpdateArgs() map[string]string {
	args := make(map[string]string)

	fields := map[string]string{
		"title":       m.title,
		"status":      m.status,
		"priority":    m.priority,
		"type":        m.issueType,
		"assignee":    m.assignee,
		"labels":      m.labels,
		"description": m.description,
		"notes":       m.notes,
	}

	for key, val := range fields {
		if orig, ok := m.originals[key]; ok && val != orig {
			if key == "priority" {
				val = fmt.Sprintf("%d", parsePriority(val))
			}
			// bd update uses --set-labels (not --labels)
			if key == "labels" {
				key = "set-labels"
			}
			args[key] = val
		}
	}

	return args
}

// BuildCreateArgs returns all non-empty fields for bd create
func (m EditModal) BuildCreateArgs() map[string]string {
	args := make(map[string]string)

	fields := map[string]string{
		"title":       m.title,
		"priority":    m.priority,
		"type":        m.issueType,
		"assignee":    m.assignee,
		"labels":      m.labels,
		"description": m.description,
		"notes":       m.notes,
	}

	for key, val := range fields {
		if val != "" {
			if key == "priority" {
				val = fmt.Sprintf("%d", parsePriority(val))
			}
			args[key] = val
		}
	}

	return args
}
