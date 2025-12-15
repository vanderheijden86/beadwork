// Package ui provides the history view for displaying bead-to-commit correlations.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/correlation"
	"github.com/charmbracelet/lipgloss"
)

// historyFocus tracks which pane has focus in the history view
type historyFocus int

const (
	historyFocusList historyFocus = iota
	historyFocusDetail
)

// HistoryModel represents the TUI view for bead history and code correlations
type HistoryModel struct {
	// Data
	report    *correlation.HistoryReport
	histories []correlation.BeadHistory // Filtered and sorted list
	beadIDs   []string                  // Sorted bead IDs for navigation

	// Navigation state
	selectedBead   int // Index into beadIDs
	selectedCommit int // Index into selected bead's commits
	scrollOffset   int // For scrolling the bead list
	focused        historyFocus

	// Filters
	authorFilter  string  // Filter by author (empty = all)
	minConfidence float64 // Minimum confidence threshold (0-1)

	// Display state
	width  int
	height int
	theme  Theme

	// Expanded state tracking
	expandedBeads map[string]bool // Track which beads have commits expanded
}

// NewHistoryModel creates a new history view from a correlation report
func NewHistoryModel(report *correlation.HistoryReport, theme Theme) HistoryModel {
	h := HistoryModel{
		report:        report,
		theme:         theme,
		focused:       historyFocusList,
		minConfidence: 0.0, // Show all by default
		expandedBeads: make(map[string]bool),
	}
	h.rebuildFilteredList()
	return h
}

// SetReport updates the history data
func (h *HistoryModel) SetReport(report *correlation.HistoryReport) {
	h.report = report
	h.rebuildFilteredList()
}

// rebuildFilteredList rebuilds the filtered and sorted list of histories
func (h *HistoryModel) rebuildFilteredList() {
	h.histories = nil
	h.beadIDs = nil

	if h.report == nil {
		return
	}

	// Filter and collect histories
	for beadID, history := range h.report.Histories {
		// Skip beads with no commits
		if len(history.Commits) == 0 {
			continue
		}

		// Apply author filter
		if h.authorFilter != "" {
			authorMatch := false
			for _, c := range history.Commits {
				if strings.Contains(strings.ToLower(c.Author), strings.ToLower(h.authorFilter)) ||
					strings.Contains(strings.ToLower(c.AuthorEmail), strings.ToLower(h.authorFilter)) {
					authorMatch = true
					break
				}
			}
			if !authorMatch {
				continue
			}
		}

		// Apply confidence filter - keep only commits meeting threshold
		if h.minConfidence > 0 {
			var filtered []correlation.CorrelatedCommit
			for _, c := range history.Commits {
				if c.Confidence >= h.minConfidence {
					filtered = append(filtered, c)
				}
			}
			if len(filtered) == 0 {
				continue
			}
			history.Commits = filtered
		}

		h.histories = append(h.histories, history)
		h.beadIDs = append(h.beadIDs, beadID)
	}

	// Sort by most commits first
	sort.Slice(h.histories, func(i, j int) bool {
		if len(h.histories[i].Commits) != len(h.histories[j].Commits) {
			return len(h.histories[i].Commits) > len(h.histories[j].Commits)
		}
		return h.histories[i].BeadID < h.histories[j].BeadID
	})

	// Rebuild beadIDs to match sorted order
	h.beadIDs = make([]string, len(h.histories))
	for i, hist := range h.histories {
		h.beadIDs[i] = hist.BeadID
	}

	// Reset selection if out of bounds
	if h.selectedBead >= len(h.histories) {
		h.selectedBead = 0
		h.selectedCommit = 0
	}
}

// SetSize updates the view dimensions
func (h *HistoryModel) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// SetAuthorFilter sets the author filter and rebuilds the list
func (h *HistoryModel) SetAuthorFilter(author string) {
	h.authorFilter = author
	h.rebuildFilteredList()
}

// SetMinConfidence sets the minimum confidence threshold and rebuilds the list
func (h *HistoryModel) SetMinConfidence(conf float64) {
	h.minConfidence = conf
	h.rebuildFilteredList()
}

// Navigation methods

// MoveUp moves selection up in the current focus pane
func (h *HistoryModel) MoveUp() {
	if h.focused == historyFocusList {
		if h.selectedBead > 0 {
			h.selectedBead--
			h.selectedCommit = 0
			h.ensureBeadVisible()
		}
	} else {
		// In detail pane, move to previous commit
		if h.selectedCommit > 0 {
			h.selectedCommit--
		}
	}
}

// MoveDown moves selection down in the current focus pane
func (h *HistoryModel) MoveDown() {
	if h.focused == historyFocusList {
		if h.selectedBead < len(h.histories)-1 {
			h.selectedBead++
			h.selectedCommit = 0
			h.ensureBeadVisible()
		}
	} else {
		// In detail pane, move to next commit
		if h.selectedBead < len(h.histories) {
			commits := h.histories[h.selectedBead].Commits
			if h.selectedCommit < len(commits)-1 {
				h.selectedCommit++
			}
		}
	}
}

// ToggleFocus switches between list and detail panes
func (h *HistoryModel) ToggleFocus() {
	if h.focused == historyFocusList {
		h.focused = historyFocusDetail
	} else {
		h.focused = historyFocusList
	}
}

// NextCommit moves to the next commit within the selected bead (J key)
func (h *HistoryModel) NextCommit() {
	if h.selectedBead >= len(h.histories) {
		return
	}
	commits := h.histories[h.selectedBead].Commits
	if h.selectedCommit < len(commits)-1 {
		h.selectedCommit++
	}
}

// PrevCommit moves to the previous commit within the selected bead (K key)
func (h *HistoryModel) PrevCommit() {
	if h.selectedCommit > 0 {
		h.selectedCommit--
	}
}

// CycleConfidence cycles through common confidence thresholds (0, 0.5, 0.75, 0.9)
func (h *HistoryModel) CycleConfidence() {
	thresholds := []float64{0, 0.5, 0.75, 0.9}
	// Find current threshold index
	currentIdx := 0
	for i, t := range thresholds {
		if h.minConfidence >= t-0.01 && h.minConfidence <= t+0.01 {
			currentIdx = i
			break
		}
	}
	// Move to next threshold (wrap around)
	nextIdx := (currentIdx + 1) % len(thresholds)
	h.SetMinConfidence(thresholds[nextIdx])
}

// GetMinConfidence returns the current minimum confidence threshold
func (h *HistoryModel) GetMinConfidence() float64 {
	return h.minConfidence
}

// ToggleExpand expands/collapses the commits for the selected bead
func (h *HistoryModel) ToggleExpand() {
	if h.selectedBead < len(h.beadIDs) {
		beadID := h.beadIDs[h.selectedBead]
		h.expandedBeads[beadID] = !h.expandedBeads[beadID]
	}
}

// ensureBeadVisible adjusts scroll offset to keep selected bead visible
func (h *HistoryModel) ensureBeadVisible() {
	visibleItems := h.listHeight()
	if visibleItems < 1 {
		visibleItems = 1
	}

	if h.selectedBead < h.scrollOffset {
		h.scrollOffset = h.selectedBead
	} else if h.selectedBead >= h.scrollOffset+visibleItems {
		h.scrollOffset = h.selectedBead - visibleItems + 1
	}
}

// listHeight returns the number of visible items in the list
func (h *HistoryModel) listHeight() int {
	// Reserve 3 lines for header/filter bar
	return h.height - 3
}

// SelectedBeadID returns the currently selected bead ID
func (h *HistoryModel) SelectedBeadID() string {
	if h.selectedBead < len(h.beadIDs) {
		return h.beadIDs[h.selectedBead]
	}
	return ""
}

// SelectedHistory returns the currently selected bead history
func (h *HistoryModel) SelectedHistory() *correlation.BeadHistory {
	if h.selectedBead < len(h.histories) {
		return &h.histories[h.selectedBead]
	}
	return nil
}

// SelectedCommit returns the currently selected commit
func (h *HistoryModel) SelectedCommit() *correlation.CorrelatedCommit {
	hist := h.SelectedHistory()
	if hist != nil && h.selectedCommit < len(hist.Commits) {
		return &hist.Commits[h.selectedCommit]
	}
	return nil
}

// GetHistoryForBead returns the history for a specific bead ID
func (h *HistoryModel) GetHistoryForBead(beadID string) *correlation.BeadHistory {
	if h.report == nil {
		return nil
	}
	hist, ok := h.report.Histories[beadID]
	if !ok {
		return nil
	}
	return &hist
}

// HasReport returns true if history data is loaded
func (h *HistoryModel) HasReport() bool {
	return h.report != nil
}

// View renders the history view
func (h *HistoryModel) View() string {
	if h.report == nil {
		return h.renderEmpty("No history data loaded")
	}

	if len(h.histories) == 0 {
		return h.renderEmpty("No beads with commit correlations found")
	}

	// Calculate panel widths (40% list, 60% detail)
	listWidth := int(float64(h.width) * 0.4)
	detailWidth := h.width - listWidth

	// Render header
	header := h.renderHeader()

	// Render list panel
	listPanel := h.renderListPanel(listWidth, h.height-2) // -2 for header

	// Render detail panel
	detailPanel := h.renderDetailPanel(detailWidth, h.height-2)

	// Combine panels
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels)
}

// renderEmpty renders an empty state message
func (h *HistoryModel) renderEmpty(msg string) string {
	t := h.theme
	style := t.Renderer.NewStyle().
		Width(h.width).
		Height(h.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(t.Secondary)

	return style.Render(msg + "\n\nPress H to close")
}

// renderHeader renders the filter bar and title
func (h *HistoryModel) renderHeader() string {
	t := h.theme

	titleStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Padding(0, 1)

	filterStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Padding(0, 1)

	title := titleStyle.Render("BEAD HISTORY")

	// Build filter info
	var filters []string
	filters = append(filters, fmt.Sprintf("%d/%d beads", len(h.histories), len(h.report.Histories)))

	if h.authorFilter != "" {
		filters = append(filters, fmt.Sprintf("Author: %s", h.authorFilter))
	}
	if h.minConfidence > 0 {
		filters = append(filters, fmt.Sprintf("Conf: ≥%.0f%%", h.minConfidence*100))
	}

	filterInfo := filterStyle.Render(strings.Join(filters, " | "))

	// Close hint
	closeHint := t.Renderer.NewStyle().
		Foreground(t.Muted).
		Padding(0, 1).
		Render("[H] to close")

	// Combine with spacing
	spacerWidth := h.width - lipgloss.Width(title) - lipgloss.Width(filterInfo) - lipgloss.Width(closeHint)
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := strings.Repeat(" ", spacerWidth)

	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, title, filterInfo, spacer, closeHint)

	// Add separator line
	separatorWidth := h.width
	if separatorWidth < 1 {
		separatorWidth = 1
	}
	separator := t.Renderer.NewStyle().
		Foreground(t.Muted).
		Width(h.width).
		Render(strings.Repeat("─", separatorWidth))

	return lipgloss.JoinVertical(lipgloss.Left, headerLine, separator)
}

// renderListPanel renders the left panel with bead list
func (h *HistoryModel) renderListPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusList {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2). // Account for border
		Height(height - 2)

	// Column header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Width(width - 4)
	header := headerStyle.Render("BEADS WITH HISTORY")

	// Build list content
	var lines []string
	lines = append(lines, header)
	sepWidth := width - 4
	if sepWidth < 1 {
		sepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", sepWidth))

	visibleItems := height - 5 // Account for header, separator, border
	if visibleItems < 1 {
		visibleItems = 1
	}

	for i := h.scrollOffset; i < len(h.histories) && i < h.scrollOffset+visibleItems; i++ {
		hist := h.histories[i]
		line := h.renderBeadLine(i, hist, width-4)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderBeadLine renders a single bead in the list
func (h *HistoryModel) renderBeadLine(idx int, hist correlation.BeadHistory, width int) string {
	t := h.theme

	selected := idx == h.selectedBead

	// Indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// Status icon
	statusIcon := "○"
	switch hist.Status {
	case "closed":
		statusIcon = "✓"
	case "in_progress":
		statusIcon = "●"
	}

	// Commit count
	commitCount := fmt.Sprintf("%d commits", len(hist.Commits))

	// Truncate title
	maxTitleLen := width - len(indicator) - len(statusIcon) - len(commitCount) - 6
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := hist.Title
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-1] + "…"
	}

	// Build line
	idStyle := t.Renderer.NewStyle().Foreground(t.Secondary).Width(12)
	titleStyle := t.Renderer.NewStyle().Width(maxTitleLen)
	countStyle := t.Renderer.NewStyle().Foreground(t.Muted).Align(lipgloss.Right)

	if selected && h.focused == historyFocusList {
		idStyle = idStyle.Bold(true).Foreground(t.Primary)
		titleStyle = titleStyle.Bold(true)
	}

	line := fmt.Sprintf("%s%s %s %s %s",
		indicator,
		statusIcon,
		idStyle.Render(hist.BeadID),
		titleStyle.Render(title),
		countStyle.Render(commitCount),
	)

	return line
}

// renderDetailPanel renders the right panel with commit details
func (h *HistoryModel) renderDetailPanel(width, height int) string {
	t := h.theme

	// Panel border style based on focus
	borderColor := t.Muted
	if h.focused == historyFocusDetail {
		borderColor = t.Primary
	}

	panelStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	hist := h.SelectedHistory()
	if hist == nil {
		return panelStyle.Render("No bead selected")
	}

	// Header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary)
	header := headerStyle.Render("COMMIT DETAILS")

	// Bead info
	beadInfo := fmt.Sprintf("%s: %s", hist.BeadID, hist.Title)
	if width > 10 && len(beadInfo) > width-6 {
		beadInfo = beadInfo[:width-7] + "…"
	} else if width <= 10 && len(beadInfo) > 5 {
		beadInfo = beadInfo[:4] + "…"
	}
	beadInfoStyle := t.Renderer.NewStyle().Foreground(t.Secondary)

	var lines []string
	lines = append(lines, header)
	lines = append(lines, beadInfoStyle.Render(beadInfo))
	detailSepWidth := width - 4
	if detailSepWidth < 1 {
		detailSepWidth = 1
	}
	lines = append(lines, strings.Repeat("─", detailSepWidth))

	// Render commits
	for i, commit := range hist.Commits {
		isSelected := i == h.selectedCommit && h.focused == historyFocusDetail
		commitLines := h.renderCommitDetail(commit, width-4, isSelected)
		lines = append(lines, commitLines...)
		if i < len(hist.Commits)-1 {
			lines = append(lines, "") // Spacer between commits
		}
	}

	// Pad with empty lines
	for len(lines) < height-2 {
		lines = append(lines, "")
	}

	// Truncate if too many lines
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}

	content := strings.Join(lines, "\n")
	return panelStyle.Render(content)
}

// renderCommitDetail renders details for a single commit
func (h *HistoryModel) renderCommitDetail(commit correlation.CorrelatedCommit, width int, selected bool) []string {
	t := h.theme

	var lines []string

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = "▸ "
	}

	// SHA and message
	shaStyle := t.Renderer.NewStyle().Foreground(t.Primary)
	if selected {
		shaStyle = shaStyle.Bold(true)
	}
	shaLine := fmt.Sprintf("%s%s %s", indicator, shaStyle.Render(commit.ShortSHA), truncate(commit.Message, width-15))
	lines = append(lines, shaLine)

	// Author and date
	authorStyle := t.Renderer.NewStyle().Foreground(t.Secondary)
	authorLine := fmt.Sprintf("    %s • %s", authorStyle.Render(commit.Author), commit.Timestamp.Format("2006-01-02 15:04"))
	lines = append(lines, authorLine)

	// Confidence and method
	confStyle := t.Renderer.NewStyle()
	switch {
	case commit.Confidence >= 0.8:
		confStyle = confStyle.Foreground(t.Open) // Green
	case commit.Confidence >= 0.5:
		confStyle = confStyle.Foreground(t.Secondary) // Yellow/neutral
	default:
		confStyle = confStyle.Foreground(t.Muted) // Gray
	}

	methodStr := methodLabel(commit.Method)
	confLine := fmt.Sprintf("    %s %s",
		confStyle.Render(fmt.Sprintf("%.0f%%", commit.Confidence*100)),
		methodStr,
	)
	lines = append(lines, confLine)

	// Files (abbreviated)
	if len(commit.Files) > 0 {
		fileCount := fmt.Sprintf("    %d files changed", len(commit.Files))
		if len(commit.Files) <= 3 {
			var filenames []string
			for _, f := range commit.Files {
				filenames = append(filenames, f.Path)
			}
			fileCount = fmt.Sprintf("    %s", strings.Join(filenames, ", "))
			if width > 6 && len(fileCount) > width-2 {
				fileCount = fileCount[:width-3] + "…"
			} else if width <= 6 && len(fileCount) > 5 {
				fileCount = fileCount[:4] + "…"
			}
		}
		fileStyle := t.Renderer.NewStyle().Foreground(t.Muted)
		lines = append(lines, fileStyle.Render(fileCount))
	}

	return lines
}

// Helper functions

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func methodLabel(method correlation.CorrelationMethod) string {
	switch method {
	case correlation.MethodCoCommitted:
		return "(co-committed)"
	case correlation.MethodExplicitID:
		return "(explicit ID)"
	case correlation.MethodTemporalAuthor:
		return "(temporal)"
	default:
		return ""
	}
}
