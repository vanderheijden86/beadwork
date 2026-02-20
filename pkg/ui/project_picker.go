package ui

import (
	"fmt"
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
// It renders as a multi-column panel: project table (# NAME O P R) | shortcuts | B9s logo.
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

// panelRows is the fixed number of content rows in the picker panel.
// Matches the B9s logo height (6 lines). Title bar adds 1 more.
const panelRows = 6

// maxVisibleProjects is the max number of projects shown in the table.
// Row 0 = column headers, so 5 project rows fit in 6 panel rows.
const maxVisibleProjects = 5

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

// b9sLogo returns the ASCII art logo lines.
func b9sLogo() []string {
	return []string{
		`__________  ________`,
		`\______   \/   __   \______`,
		` |    |  _/\____    /  ___/`,
		` |    |   \   /    /\___ \`,
		` |______  /  /____//____  >`,
		`        \/              \/`,
	}
}

// pickerShortcuts returns the shortcut definitions for the picker panel (bd-2me).
// Two columns of real keybindings, 6 rows matching panelRows.
func pickerShortcuts() [panelRows][2]struct{ key, desc string } {
	return [panelRows][2]struct{ key, desc string }{
		{{"o", "Open"}, {"b", "Board"}},
		{{"c", "Closed"}, {"g", "Graph"}},
		{{"r", "Ready"}, {"h", "History"}},
		{{"a", "All"}, {"i", "Insights"}},
		{{"/", "Search"}, {"?", "Help"}},
		{{"", ""}, {"", ""}},
	}
}

// View renders the k9s-style multi-column project picker panel (bd-b4u, bd-qyr).
// Layout: [project table with O P R columns] [shortcuts] [B9s logo]
// Bottom: title bar divider.
func (m *ProjectPickerModel) View() string {
	if m.width == 0 {
		m.width = 80
	}

	w := m.width

	// --- Build each column as []string of panelRows lines ---

	// Column 1: Project table (# NAME ... O P R)
	tableLines := m.renderProjectTable()

	// Column 2: Shortcuts
	shortcutLines := m.renderShortcutsColumn()

	// Column 3: Type legend (bd-5im0)
	legendLines := m.renderTypeLegendColumn()

	// Column 4: B9s logo
	logoLines := m.renderLogoColumn()

	// --- Determine column widths, progressively drop columns on narrow terminals (bd-ecmm) ---
	shortcutsWidth := m.maxLineWidth(shortcutLines)
	if shortcutsWidth < 16 {
		shortcutsWidth = 16
	}
	legendWidth := m.maxLineWidth(legendLines)
	if legendWidth < 10 {
		legendWidth = 10
	}
	logoWidth := m.maxLineWidth(logoLines)
	gap := 2
	minTableWidth := 30

	// Decide which optional columns fit: drop logo first, then legend, then shortcuts
	showLogo := true
	showLegend := true
	showShortcuts := true

	needed := minTableWidth + gap + shortcutsWidth + gap + legendWidth + gap + logoWidth
	if needed > w {
		showLogo = false // drop logo first
		needed = minTableWidth + gap + shortcutsWidth + gap + legendWidth
	}
	if needed > w {
		showLegend = false // then legend
		needed = minTableWidth + gap + shortcutsWidth
	}
	if needed > w {
		showShortcuts = false // then shortcuts
	}

	// Table gets remaining space
	tableWidth := w
	if showShortcuts {
		tableWidth -= shortcutsWidth + gap
	}
	if showLegend {
		tableWidth -= legendWidth + gap
	}
	if showLogo {
		tableWidth -= logoWidth + gap
	}
	if tableWidth < minTableWidth {
		tableWidth = minTableWidth
	}

	// --- Join columns row by row using padRight for alignment (bd-qyr) ---
	gapStr := strings.Repeat(" ", gap)
	var rows []string
	for i := 0; i < panelRows; i++ {
		row := padRight(safeIndex(tableLines, i), tableWidth)
		if showShortcuts {
			row += gapStr + padRight(safeIndex(shortcutLines, i), shortcutsWidth)
		}
		if showLegend {
			row += gapStr + padRight(safeIndex(legendLines, i), legendWidth)
		}
		if showLogo {
			row += gapStr + safeIndex(logoLines, i)
		}
		rows = append(rows, row)
	}

	// --- Title bar at bottom ---
	rows = append(rows, m.renderTitleBar(w))

	return strings.Join(rows, "\n")
}

// Height returns the number of terminal lines the picker panel uses.
func (m *ProjectPickerModel) Height() int {
	return panelRows + 1 // content rows + title bar
}

// ViewMinimized renders a single-line title bar showing the active project.
func (m *ProjectPickerModel) ViewMinimized() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	return m.renderTitleBar(w)
}

// renderProjectTable renders the project list with # NAME and O P R columns.
func (m *ProjectPickerModel) renderProjectTable() []string {
	t := m.theme

	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	numStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	activeStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	normalStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())
	cursorStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	dimStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	// Find max name width for alignment
	nameW := 12 // minimum
	for _, idx := range m.filtered {
		entry := m.entries[idx]
		if len(entry.Project.Name) > nameW {
			nameW = len(entry.Project.Name)
		}
	}
	if nameW > 20 {
		nameW = 20
	}

	lines := make([]string, panelRows)

	// Row 0: column headers or filter input
	if m.filtering {
		filterStyle := t.Renderer.NewStyle().Foreground(t.Primary)
		lines[0] = headerStyle.Render(" > ") + filterStyle.Render(m.filterInput.View())
	} else {
		lines[0] = headerStyle.Render(fmt.Sprintf("     %-*s  %3s %3s %3s", nameW, "", "O", "P", "R"))
	}

	if len(m.filtered) == 0 {
		lines[1] = dimStyle.Render(" No projects found")
		return lines
	}

	// Rows 1-5: project entries
	visible := len(m.filtered)
	if visible > maxVisibleProjects {
		visible = maxVisibleProjects
	}

	for i := 0; i < visible; i++ {
		entry := m.entries[m.filtered[i]]
		isCursor := m.filtering && i == m.cursor

		// Number
		numStr := " "
		if entry.FavoriteNum > 0 {
			numStr = fmt.Sprintf("%d", entry.FavoriteNum)
		}

		// Name (truncated if needed)
		name := entry.Project.Name
		if len(name) > nameW {
			name = name[:nameW-3] + "..."
		}

		// Build the row text with fixed-width columns
		rowText := fmt.Sprintf(" <%s> %-*s  %3d %3d %3d",
			numStr, nameW, name,
			entry.OpenCount, entry.InProgressCount, entry.ReadyCount)

		switch {
		case isCursor:
			lines[i+1] = cursorStyle.Render(rowText)
		case entry.IsActive:
			lines[i+1] = activeStyle.Render(rowText)
		default:
			// Style number separately for color
			numPart := numStyle.Render(fmt.Sprintf(" <%s>", numStr))
			restText := fmt.Sprintf(" %-*s  %3d %3d %3d",
				nameW, name,
				entry.OpenCount, entry.InProgressCount, entry.ReadyCount)
			lines[i+1] = numPart + normalStyle.Render(restText)
		}
	}

	if len(m.filtered) > maxVisibleProjects {
		remaining := len(m.filtered) - maxVisibleProjects
		if visible < panelRows-1 {
			lines[visible+1] = dimStyle.Render(fmt.Sprintf("      ... +%d more", remaining))
		}
	}

	return lines
}

// renderShortcutsColumn renders two columns of real keybindings (bd-2me).
func (m *ProjectPickerModel) renderShortcutsColumn() []string {
	t := m.theme

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#BD93F9"}).
		Bold(true)
	descStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	shortcuts := pickerShortcuts()
	lines := make([]string, panelRows)

	for i := 0; i < panelRows; i++ {
		left := keyStyle.Render(shortcuts[i][0].key) + " " + descStyle.Render(fmt.Sprintf("%-8s", shortcuts[i][0].desc))
		right := keyStyle.Render(shortcuts[i][1].key) + " " + descStyle.Render(shortcuts[i][1].desc)
		lines[i] = left + " " + right
	}

	return lines
}

// renderTypeLegendColumn renders a legend of issue type icons and labels (bd-5im0).
func (m *ProjectPickerModel) renderTypeLegendColumn() []string {
	t := m.theme
	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground()).
		Bold(true)
	labelStyle := t.Renderer.NewStyle().
		Foreground(t.MutedText.GetForeground())

	types := []struct {
		typ   string
		label string
	}{
		{"bug", "Bug"},
		{"feature", "Feature"},
		{"task", "Task"},
		{"epic", "Epic"},
		{"chore", "Chore"},
	}

	lines := make([]string, panelRows)
	lines[0] = headerStyle.Render("Types")
	for i, tp := range types {
		icon, color := t.GetTypeIcon(tp.typ)
		iconStyled := t.Renderer.NewStyle().Foreground(color).Render(icon)
		lines[i+1] = iconStyled + " " + labelStyle.Render(tp.label)
	}
	return lines
}

// renderLogoColumn renders the B9s ASCII art logo.
func (m *ProjectPickerModel) renderLogoColumn() []string {
	t := m.theme
	logoStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"})

	logo := b9sLogo()
	lines := make([]string, panelRows)
	for i := 0; i < len(logo) && i < panelRows; i++ {
		lines[i] = logoStyle.Render(logo[i])
	}

	return lines
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
	activeNum := 0
	if m.filtering && m.filterInput.Value() != "" {
		label = fmt.Sprintf("projects(%s)", m.filterInput.Value())
	} else {
		for _, entry := range m.entries {
			if entry.IsActive {
				label = fmt.Sprintf("projects(%s)", entry.Project.Name)
				activeNum = entry.FavoriteNum
				break
			}
		}
	}

	title := titleText.Render(label) + countText.Render(fmt.Sprintf("[%d]", activeNum))

	sepChar := "\u2500"
	sepStyle := t.Renderer.NewStyle().Foreground(t.Border)

	titleLen := len(label) + len(fmt.Sprintf("[%d]", activeNum))
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

// maxLineWidth returns the max visible width across a set of pre-rendered lines.
// Uses lipgloss.Width to account for ANSI escape codes.
func (m *ProjectPickerModel) maxLineWidth(lines []string) int {
	maxW := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

// safeIndex returns lines[i] or empty string if out of bounds.
func safeIndex(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
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
