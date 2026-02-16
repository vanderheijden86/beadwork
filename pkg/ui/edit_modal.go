package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditFieldType defines the type of edit field
type EditFieldType int

const (
	EditFieldText EditFieldType = iota
	EditFieldTextArea
	EditFieldSelect
)

// EditField represents a single editable field
type EditField struct {
	Label    string
	Key      string // field name for bd CLI (title, status, priority, etc.)
	Type     EditFieldType
	Input    textinput.Model // for text fields
	TextArea textarea.Model  // for textarea fields
	Options  []string        // for select fields
	Selected int             // current selection index for select fields
	Original string          // original value for dirty detection
}

// EditModal provides field-by-field issue editing
type EditModal struct {
	fields       []EditField
	focusedField int
	width        int
	height       int
	theme        Theme
	issueID      string // empty for create mode
	isCreateMode bool
	dirty        bool   // true if any field changed from original
	saveRequested   bool
	cancelRequested bool
}

// NewEditModal creates an edit modal pre-populated from an existing issue
func NewEditModal(issue *model.Issue, theme Theme) EditModal {
	fields := []EditField{
		makeTextField("Title", "title", issue.Title, theme),
		makeSelectField("Status", "status", string(issue.Status), getStatusOptions(), theme),
		makeSelectField("Priority", "priority", formatPriority(issue.Priority), getPriorityOptions(), theme),
		makeSelectField("Type", "type", string(issue.IssueType), getTypeOptions(), theme),
		makeTextField("Assignee", "assignee", issue.Assignee, theme),
		makeTextField("Labels", "labels", strings.Join(issue.Labels, ", "), theme),
		makeTextAreaField("Description", "description", issue.Description, theme),
		makeTextAreaField("Notes", "notes", issue.Notes, theme),
	}

	return EditModal{
		fields:       fields,
		focusedField: 0,
		theme:        theme,
		issueID:      issue.ID,
		isCreateMode: false,
	}
}

// NewCreateModal creates an edit modal with defaults for creating a new issue
func NewCreateModal(theme Theme) EditModal {
	fields := []EditField{
		makeTextField("Title", "title", "", theme),
		makeSelectField("Status", "status", "open", getStatusOptions(), theme),
		makeSelectField("Priority", "priority", "P2", getPriorityOptions(), theme),
		makeSelectField("Type", "type", "task", getTypeOptions(), theme),
		makeTextField("Assignee", "assignee", "", theme),
		makeTextField("Labels", "labels", "", theme),
		makeTextAreaField("Description", "description", "", theme),
		makeTextAreaField("Notes", "notes", "", theme),
	}

	// Focus the title field's input
	fields[0].Input.Focus()

	return EditModal{
		fields:       fields,
		focusedField: 0,
		theme:        theme,
		isCreateMode: true,
	}
}

// makeTextField creates a text input field
func makeTextField(label, key, value string, theme Theme) EditField {
	ti := textinput.New()
	ti.SetValue(value)
	ti.CharLimit = 200
	ti.Width = 50

	return EditField{
		Label:    label,
		Key:      key,
		Type:     EditFieldText,
		Input:    ti,
		Original: value,
	}
}

// makeTextAreaField creates a textarea field
func makeTextAreaField(label, key, value string, theme Theme) EditField {
	ta := textarea.New()
	ta.SetValue(value)
	ta.SetWidth(50)
	ta.SetHeight(3)
	ta.CharLimit = 5000

	return EditField{
		Label:    label,
		Key:      key,
		Type:     EditFieldTextArea,
		TextArea: ta,
		Original: value,
	}
}

// makeSelectField creates a select field
func makeSelectField(label, key, value string, options []string, theme Theme) EditField {
	selected := 0
	for i, opt := range options {
		if opt == value {
			selected = i
			break
		}
	}

	return EditField{
		Label:    label,
		Key:      key,
		Type:     EditFieldSelect,
		Options:  options,
		Selected: selected,
		Original: value,
	}
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

// Update handles input for the edit modal
func (m EditModal) Update(msg tea.Msg) (EditModal, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			m.saveRequested = true
			return m, nil

		case "esc":
			m.cancelRequested = true
			return m, nil

		case "tab":
			// Move to next field
			m.fields[m.focusedField] = m.blurField(m.fields[m.focusedField])
			m.focusedField = (m.focusedField + 1) % len(m.fields)
			m.fields[m.focusedField] = m.focusField(m.fields[m.focusedField])
			return m, nil

		case "shift+tab":
			// Move to previous field
			m.fields[m.focusedField] = m.blurField(m.fields[m.focusedField])
			m.focusedField = (m.focusedField - 1 + len(m.fields)) % len(m.fields)
			m.fields[m.focusedField] = m.focusField(m.fields[m.focusedField])
			return m, nil

		case "left", "h":
			// For select fields, cycle left
			if m.fields[m.focusedField].Type == EditFieldSelect {
				field := &m.fields[m.focusedField]
				field.Selected = (field.Selected - 1 + len(field.Options)) % len(field.Options)
				m.updateDirtyFlag()
				return m, nil
			}

		case "right", "l":
			// For select fields, cycle right
			if m.fields[m.focusedField].Type == EditFieldSelect {
				field := &m.fields[m.focusedField]
				field.Selected = (field.Selected + 1) % len(field.Options)
				m.updateDirtyFlag()
				return m, nil
			}
		}

		// Pass key to focused field
		field := &m.fields[m.focusedField]
		switch field.Type {
		case EditFieldText:
			field.Input, cmd = field.Input.Update(msg)
			cmds = append(cmds, cmd)
		case EditFieldTextArea:
			field.TextArea, cmd = field.TextArea.Update(msg)
			cmds = append(cmds, cmd)
		}
		m.updateDirtyFlag()
	}

	return m, tea.Batch(cmds...)
}

// focusField sets focus on the given field
func (m EditModal) focusField(field EditField) EditField {
	switch field.Type {
	case EditFieldText:
		field.Input.Focus()
	case EditFieldTextArea:
		field.TextArea.Focus()
	}
	return field
}

// blurField removes focus from the given field
func (m EditModal) blurField(field EditField) EditField {
	switch field.Type {
	case EditFieldText:
		field.Input.Blur()
	case EditFieldTextArea:
		field.TextArea.Blur()
	}
	return field
}

// updateDirtyFlag checks if any field differs from its original value
func (m *EditModal) updateDirtyFlag() {
	m.dirty = false
	for _, field := range m.fields {
		if m.getCurrentValue(field) != field.Original {
			m.dirty = true
			break
		}
	}
}

// getCurrentValue returns the current value of a field as a string
func (m EditModal) getCurrentValue(field EditField) string {
	switch field.Type {
	case EditFieldText:
		return field.Input.Value()
	case EditFieldTextArea:
		return field.TextArea.Value()
	case EditFieldSelect:
		if field.Selected >= 0 && field.Selected < len(field.Options) {
			return field.Options[field.Selected]
		}
		return ""
	}
	return ""
}

// View renders the edit modal
func (m EditModal) View() string {
	r := m.theme.Renderer

	// Calculate box width based on terminal width
	boxWidth := m.width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}

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

	var content strings.Builder
	content.WriteString(headerStyle.Render(title))
	content.WriteString("\n\n")

	// Render each field
	labelStyle := r.NewStyle().
		Foreground(m.theme.Secondary).
		Width(12).
		Align(lipgloss.Right)

	focusedLabelStyle := r.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Width(12).
		Align(lipgloss.Right)

	selectStyle := r.NewStyle().
		Foreground(m.theme.Primary)

	for i, field := range m.fields {
		isFocused := i == m.focusedField

		// Render label
		var labelStr string
		if isFocused {
			labelStr = focusedLabelStyle.Render(field.Label + ":")
		} else {
			labelStr = labelStyle.Render(field.Label + ":")
		}
		content.WriteString(labelStr)
		content.WriteString(" ")

		// Render field value
		switch field.Type {
		case EditFieldText:
			content.WriteString(field.Input.View())

		case EditFieldTextArea:
			taView := field.TextArea.View()
			// Indent textarea lines
			lines := strings.Split(taView, "\n")
			for idx, line := range lines {
				if idx > 0 {
					content.WriteString(strings.Repeat(" ", 13)) // indent to match label width
				}
				content.WriteString(line)
				if idx < len(lines)-1 {
					content.WriteString("\n")
				}
			}

		case EditFieldSelect:
			val := field.Options[field.Selected]
			if isFocused {
				content.WriteString(selectStyle.Render(fmt.Sprintf("< %s >", val)))
			} else {
				content.WriteString(val)
			}
		}

		content.WriteString("\n")
		if field.Type == EditFieldTextArea {
			content.WriteString("\n") // Extra spacing after textarea
		}
	}

	// Instructions
	content.WriteString("\n")
	subtextStyle := r.NewStyle().
		Foreground(m.theme.Subtext).
		Italic(true)

	instructions := "[Tab] Next field   [Ctrl+S] Save   [Esc] Cancel"
	if m.fields[m.focusedField].Type == EditFieldSelect {
		instructions = "[←/→] Change   [Tab] Next field   [Ctrl+S] Save   [Esc] Cancel"
	}
	content.WriteString(subtextStyle.Render(instructions))

	// Render modal with border
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
}

// IsSaveRequested returns true if ctrl+s was pressed
func (m EditModal) IsSaveRequested() bool {
	return m.saveRequested
}

// IsCancelRequested returns true if esc was pressed
func (m EditModal) IsCancelRequested() bool {
	return m.cancelRequested
}

// BuildUpdateArgs returns only changed fields for bd update
func (m EditModal) BuildUpdateArgs() map[string]string {
	args := make(map[string]string)

	for _, field := range m.fields {
		current := m.getCurrentValue(field)
		if current != field.Original {
			key := field.Key
			// Convert priority display format to bd format
			if key == "priority" {
				args[key] = fmt.Sprintf("%d", parsePriority(current))
			} else {
				// bd update uses --set-labels (not --labels)
				if key == "labels" {
					key = "set-labels"
				}
				args[key] = current
			}
		}
	}

	return args
}

// BuildCreateArgs returns all non-empty fields for bd create
func (m EditModal) BuildCreateArgs() map[string]string {
	args := make(map[string]string)

	for _, field := range m.fields {
		current := m.getCurrentValue(field)
		if current != "" {
			key := field.Key
			// bd create has no --status flag (issues are always created as open)
			if key == "status" {
				continue
			}
			// Convert priority display format to bd format
			if key == "priority" {
				args[key] = fmt.Sprintf("%d", parsePriority(current))
			} else {
				args[key] = current
			}
		}
	}

	return args
}
