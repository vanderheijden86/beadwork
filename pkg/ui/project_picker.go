package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// ProjectEntry holds display data for one project in the picker.
type ProjectEntry struct {
	Project         config.Project
	FavoriteNum     int  // 0 = not favorited, 1-9 = key
	IsActive        bool // Currently loaded project
	OpenCount       int
	InProgressCount int
	ReadyCount      int
	BlockedCount    int
}

// SwitchProjectMsg is sent when the user selects a project to switch to.
type SwitchProjectMsg struct {
	Project config.Project
}

// ToggleFavoriteMsg is sent when the user toggles a project's favorite slot.
type ToggleFavoriteMsg struct {
	ProjectName string
	SlotNumber  int // 0 = remove, 1-9 = assign
}

// ProjectPickerModel is an always-visible k9s-style header for selecting projects.
// It has two display modes: expanded (full table) and minimized (single summary line).
// In expanded mode, the project list is display-only (no cursor navigation).
// Project switching is done via number keys 1-9 or filter mode.
type ProjectPickerModel struct {
	entries     []ProjectEntry
	filtered    []int // indices into entries
	cursor      int   // only used during filter mode for selecting results
	width       int
	height      int
	filterInput textinput.Model
	filtering   bool
	theme       Theme
}

// NewProjectPicker creates a new project picker.
func NewProjectPicker(entries []ProjectEntry, theme Theme) ProjectPickerModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 50
	ti.Width = 30

	indices := make([]int, len(entries))
	for i := range entries {
		indices[i] = i
	}

	return ProjectPickerModel{
		entries:     entries,
		filtered:    indices,
		cursor:      0,
		filterInput: ti,
		filtering:   false,
		theme:       theme,
	}
}

// SetSize updates the picker dimensions.
func (m *ProjectPickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles keyboard input for the project picker.
func (m ProjectPickerModel) Update(msg tea.Msg) (ProjectPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			return m.updateFiltering(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

// updateNormal handles keys in display-only mode.
// Only filter entry (/) and number-key quick-switch are active.
// Navigation (j/k/enter) is intentionally omitted — the picker is display-only.
// Project switching is done via number keys 1-9, handled at top priority in Model.Update.
func (m ProjectPickerModel) updateNormal(msg tea.KeyMsg) (ProjectPickerModel, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.filtering = true
		m.cursor = 0
		m.filterInput.SetValue("")
		m.filterInput.Focus()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		n := int(msg.String()[0] - '0')
		for _, entry := range m.entries {
			if entry.FavoriteNum == n {
				return m, func() tea.Msg {
					return SwitchProjectMsg{Project: entry.Project}
				}
			}
		}
	}
	return m, nil
}

// updateFiltering handles keys when in filter mode.
func (m ProjectPickerModel) updateFiltering(msg tea.KeyMsg) (ProjectPickerModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.applyFilter()
		return m, nil
	case "enter":
		m.filtering = false
		m.filterInput.Blur()
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			entry := m.entries[m.filtered[m.cursor]]
			return m, func() tea.Msg {
				return SwitchProjectMsg{Project: entry.Project}
			}
		}
		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

// applyFilter updates the filtered indices based on the current filter input.
func (m *ProjectPickerModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.filtered = make([]int, len(m.entries))
		for i := range m.entries {
			m.filtered[i] = i
		}
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		return
	}

	type scored struct {
		index int
		score int
	}
	var matches []scored
	for i, entry := range m.entries {
		name := strings.ToLower(entry.Project.Name)
		path := strings.ToLower(entry.Project.Path)
		nameScore := fuzzyScore(name, query)
		pathScore := fuzzyScore(path, query)
		best := nameScore
		if pathScore > best {
			best = pathScore
		}
		if best > 0 {
			matches = append(matches, scored{i, best})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	m.filtered = make([]int, len(matches))
	for i, match := range matches {
		m.filtered[i] = match.index
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// nextAvailableFavoriteSlot cycles through favorite slots for the given entry.
func (m *ProjectPickerModel) nextAvailableFavoriteSlot(entry ProjectEntry) int {
	if entry.FavoriteNum > 0 {
		return 0
	}
	used := make(map[int]bool)
	for _, e := range m.entries {
		if e.FavoriteNum > 0 {
			used[e.FavoriteNum] = true
		}
	}
	for n := 1; n <= 9; n++ {
		if !used[n] {
			return n
		}
	}
	return 0
}

// maxExpandedRows is the maximum number of project rows shown in expanded mode.
const maxExpandedRows = 10

// View renders the full-screen k9s-style project picker (legacy, delegates to ViewExpanded).
func (m *ProjectPickerModel) View() string {
	return m.ViewExpanded()
}

// ViewExpanded renders the expanded project picker header (display-only).
// Active project is marked with ►, favorite numbers shown explicitly.
func (m *ProjectPickerModel) ViewExpanded() string {
	if m.width == 0 {
		m.width = 80
	}

	w := m.width

	var sections []string

	// --- Shortcut hints (k9s style) ---
	sections = append(sections, m.renderExpandedShortcutBar(w))

	// --- Title bar: " projects(filtered)[count] " ---
	sections = append(sections, m.renderTitleBar(w))

	// --- Column headers ---
	sections = append(sections, m.renderColumnHeaders(w))

	// --- Filter input (shown inline when filtering) ---
	if m.filtering {
		t := m.theme
		filterStyle := t.Renderer.NewStyle().
			Foreground(t.Primary).
			Width(w)
		sections = append(sections, filterStyle.Render("  / "+m.filterInput.View()))
	}

	// --- Project rows (display-only, no cursor) ---
	if len(m.filtered) == 0 {
		t := m.theme
		dimStyle := t.Renderer.NewStyle().
			Foreground(t.Secondary).
			Italic(true)
		sections = append(sections, dimStyle.Render("  No projects found. Configure scan_paths in ~/.config/bw/config.yaml"))
	} else {
		visible := len(m.filtered)
		if visible > maxExpandedRows {
			visible = maxExpandedRows
		}
		for i := 0; i < visible; i++ {
			entry := m.entries[m.filtered[i]]
			isCursor := m.filtering && i == m.cursor
			sections = append(sections, m.renderRow(entry, isCursor, w))
		}
		if len(m.filtered) > maxExpandedRows {
			t := m.theme
			moreStyle := t.Renderer.NewStyle().
				Foreground(t.Secondary).
				Italic(true)
			sections = append(sections, moreStyle.Render(fmt.Sprintf("  ... and %d more", len(m.filtered)-maxExpandedRows)))
		}
	}

	return strings.Join(sections, "\n")
}

// ViewMinimized renders a single-line summary: current project + favorite shortcuts.
func (m *ProjectPickerModel) ViewMinimized() string {
	if m.width == 0 {
		m.width = 80
	}

	t := m.theme

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	descStyle := t.Renderer.NewStyle().
		Foreground(t.Subtext)
	sepStyle := t.Renderer.NewStyle().
		Foreground(t.Border)

	// Find the active project
	var activeName string
	var activeOpen, activeInProg, activeReady, activeBlocked int
	for _, entry := range m.entries {
		if entry.IsActive {
			activeName = entry.Project.Name
			activeOpen = entry.OpenCount
			activeInProg = entry.InProgressCount
			activeReady = entry.ReadyCount
			activeBlocked = entry.BlockedCount
			break
		}
	}
	if activeName == "" {
		activeName = "untitled"
	}

	// Active project info (bd-aa6: no "Project:" prefix, no expand hint)
	projectInfo := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render(activeName) +
		descStyle.Render(fmt.Sprintf(" (%d/%d/%d/%d)", activeOpen, activeInProg, activeReady, activeBlocked))

	// Favorite shortcuts
	var favParts []string
	for _, entry := range m.entries {
		if entry.FavoriteNum > 0 {
			favParts = append(favParts, keyStyle.Render(fmt.Sprintf("<%d>", entry.FavoriteNum))+" "+descStyle.Render(entry.Project.Name))
		}
	}
	// Sort by favorite number
	sort.Slice(favParts, func(i, j int) bool { return favParts[i] < favParts[j] })
	favSection := strings.Join(favParts, "  ")

	sep := sepStyle.Render(" \u2502 ")
	line := "  " + projectInfo + sep + favSection

	return line
}

// ExpandedHeight returns the number of terminal lines the expanded view uses.
func (m *ProjectPickerModel) ExpandedHeight() int {
	lines := 3 // shortcut bar + title bar + column headers
	if m.filtering {
		lines++ // filter input line
	}
	if len(m.filtered) == 0 {
		lines++ // "No projects found" message
	} else {
		visible := len(m.filtered)
		if visible > maxExpandedRows {
			visible = maxExpandedRows
			lines++ // "... and N more" line
		}
		lines += visible
	}
	return lines
}

// MinimizedHeight returns the number of terminal lines the minimized view uses.
func (m *ProjectPickerModel) MinimizedHeight() int {
	return 1
}

// renderShortcutBar renders the k9s-style shortcut hints at the top (legacy, delegates to expanded).
func (m *ProjectPickerModel) renderShortcutBar(w int) string {
	return m.renderExpandedShortcutBar(w)
}

// renderExpandedShortcutBar renders shortcut hints for the expanded picker header.
func (m *ProjectPickerModel) renderExpandedShortcutBar(w int) string {
	t := m.theme

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	descStyle := t.Renderer.NewStyle().
		Foreground(t.Subtext)

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"<1-9>", "Quick Switch"},
		{"</>", "Filter"},
		{"<P>", "Minimize"},
	}

	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, keyStyle.Render(s.key)+" "+descStyle.Render(s.desc))
	}

	line := strings.Join(parts, "  ")

	return " " + line
}

// renderTitleBar renders the k9s-style title bar with resource type and count.
func (m *ProjectPickerModel) renderTitleBar(w int) string {
	t := m.theme

	titleText := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	countText := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"})

	label := "projects"
	if m.filtering && m.filterInput.Value() != "" {
		label = fmt.Sprintf("projects(%s)", m.filterInput.Value())
	}

	title := titleText.Render(label) + countText.Render(fmt.Sprintf("[%d]", len(m.filtered)))

	// Center the title with separator lines
	sepChar := "\u2500"
	sepStyle := t.Renderer.NewStyle().Foreground(t.Border)

	// Calculate padding (approximate since styled text has zero-width codes)
	titleLen := len(label) + len(fmt.Sprintf("[%d]", len(m.filtered)))
	leftPad := (w - titleLen - 4) / 2
	rightPad := w - titleLen - 4 - leftPad
	if leftPad < 1 {
		leftPad = 1
	}
	if rightPad < 1 {
		rightPad = 1
	}

	return sepStyle.Render(strings.Repeat(sepChar, leftPad)) + " " + title + " " + sepStyle.Render(strings.Repeat(sepChar, rightPad))
}

// renderColumnHeaders renders the table column header row.
func (m *ProjectPickerModel) renderColumnHeaders(w int) string {
	t := m.theme
	nameW, pathW := m.columnWidths(w)

	header := fmt.Sprintf("  %-2s %-*s %-*s %6s %8s %6s %8s",
		"#", nameW, "NAME", pathW, "PATH", "OPEN", "IN_PROG", "READY", "BLOCKED")

	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true)

	return headerStyle.Render(header)
}

// renderRow renders a single project entry row in k9s table style.
// Active project is marked with ► indicator; isCursor is used for filter-mode selection.
func (m *ProjectPickerModel) renderRow(entry ProjectEntry, isCursor bool, w int) string {
	t := m.theme
	nameW, pathW := m.columnWidths(w)

	// Row indicator: ► for active project, space otherwise
	indicator := " "
	if entry.IsActive {
		indicator = "\u25ba"
	}

	// Favorite number
	favStr := " "
	if entry.FavoriteNum > 0 {
		favStr = fmt.Sprintf("%d", entry.FavoriteNum)
	}

	// Project name
	name := entry.Project.Name
	name = truncateRunesHelper(name, nameW, "...")

	// Path (abbreviated)
	path := abbreviatePath(entry.Project.Path)
	path = truncateRunesHelper(path, pathW, "...")

	// Counts
	openStr := fmt.Sprintf("%d", entry.OpenCount)
	inProgStr := fmt.Sprintf("%d", entry.InProgressCount)
	readyStr := fmt.Sprintf("%d", entry.ReadyCount)
	blockedStr := fmt.Sprintf("%d", entry.BlockedCount)

	line := fmt.Sprintf("%s %-2s %-*s %-*s %6s %8s %6s %8s",
		indicator, favStr, nameW, name, pathW, path, openStr, inProgStr, readyStr, blockedStr)

	if isCursor {
		// Cursor highlight during filter mode
		return t.Renderer.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render(line)
	}

	if entry.IsActive {
		// Active project: cyan text
		return t.Renderer.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
			Bold(true).
			Render(line)
	}

	// Normal row
	return t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground()).
		Render(line)
}

// columnWidths calculates name and path column widths based on terminal width.
func (m *ProjectPickerModel) columnWidths(totalWidth int) (nameWidth, pathWidth int) {
	// Fixed columns: "► # " (4) + " " (gaps) + "  OPEN" (7) + " IN_PROG" (9) + " READY" (7) + " BLOCKED" (9) = ~38 fixed
	available := totalWidth - 39
	if available < 20 {
		available = 20
	}
	// Split 35/65 between name and path
	nameWidth = available * 35 / 100
	pathWidth = available - nameWidth
	if nameWidth < 10 {
		nameWidth = 10
	}
	if pathWidth < 15 {
		pathWidth = 15
	}
	return nameWidth, pathWidth
}

// abbreviatePath replaces the user's home directory with ~ in a path.
func abbreviatePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// Filtering returns whether the picker is in filter mode.
func (m *ProjectPickerModel) Filtering() bool {
	return m.filtering
}

// Cursor returns the current cursor position.
func (m *ProjectPickerModel) Cursor() int {
	return m.cursor
}

// FilteredCount returns the number of entries matching the current filter.
func (m *ProjectPickerModel) FilteredCount() int {
	return len(m.filtered)
}

// SelectedEntry returns the currently highlighted project entry, or nil if none.
func (m *ProjectPickerModel) SelectedEntry() *ProjectEntry {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	entry := m.entries[m.filtered[m.cursor]]
	return &entry
}
