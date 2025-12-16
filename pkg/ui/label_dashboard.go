package ui

import (
	"fmt"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	tea "github.com/charmbracelet/bubbletea"
)

// LabelDashboardModel renders a lightweight table of label health
type LabelDashboardModel struct {
	labels []analysis.LabelHealth
	cursor int
	width  int
	height int
	theme  Theme
}

func NewLabelDashboardModel(theme Theme) LabelDashboardModel {
	return LabelDashboardModel{theme: theme}
}

func (m *LabelDashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *LabelDashboardModel) SetData(labels []analysis.LabelHealth) {
	m.labels = labels
	if m.cursor >= len(labels) {
		m.cursor = len(labels) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
}

// Update handles navigation keys; returns selected label on enter
func (m *LabelDashboardModel) Update(msg tea.KeyMsg) (string, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.labels)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "home":
		m.cursor = 0
	case "G", "end":
		if len(m.labels) > 0 {
			m.cursor = len(m.labels) - 1
		}
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.labels) {
			return m.labels[m.cursor].Label, nil
		}
	}
	return "", nil
}

func (m LabelDashboardModel) View() string {
	if len(m.labels) == 0 {
		return "No labels found"
	}

	headers := []string{"Label", "Health", "Blocked", "Velocity 7d/30d", "Stale"}
	widths := m.computeColumnWidths(headers)

	var b strings.Builder
	// Header
	headerLine := m.renderRow(headers, widths, true, false)
	b.WriteString(headerLine)
	b.WriteString("\n")

	for i, lh := range m.labels {
		row := []string{
			lh.Label,
			fmt.Sprintf("%3d (%s)", lh.Health, lh.HealthLevel),
			fmt.Sprintf("%d", lh.Blocked),
			fmt.Sprintf("%d/%d", lh.Velocity.ClosedLast7Days, lh.Velocity.ClosedLast30Days),
			fmt.Sprintf("%d", lh.Freshness.StaleCount),
		}
		selected := i == m.cursor
		b.WriteString(m.renderRow(row, widths, false, selected))
		if i != len(m.labels)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m LabelDashboardModel) computeColumnWidths(headers []string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, lh := range m.labels {
		cells := []string{
			lh.Label,
			fmt.Sprintf("%3d (%s)", lh.Health, lh.HealthLevel),
			fmt.Sprintf("%d", lh.Blocked),
			fmt.Sprintf("%d/%d", lh.Velocity.ClosedLast7Days, lh.Velocity.ClosedLast30Days),
			fmt.Sprintf("%d", lh.Freshness.StaleCount),
		}
		for i, c := range cells {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	// Ensure total fits width; if not, truncate label column first
	total := len(headers) - 1 // spaces between columns
	for _, w := range widths {
		total += w
	}
	if m.width > 0 && total > m.width {
		excess := total - m.width
		if excess >= widths[0]-4 {
			widths[0] = 4
		} else {
			widths[0] -= excess
		}
	}
	return widths
}

func (m LabelDashboardModel) renderRow(cells []string, widths []int, header bool, selected bool) string {
	var parts []string
	for i, cell := range cells {
		cell = localTruncate(cell, widths[i])
		cell = padRight(cell, widths[i])
		parts = append(parts, cell)
	}
	row := strings.Join(parts, " ")
	if header {
		return m.theme.Header.Render(row)
	}
	if selected {
		return m.theme.Selected.Render(row)
	}
	return m.theme.Base.Render(row)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// localTruncate avoids clobbering the pkg-level truncate used elsewhere
func localTruncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 1 {
		return s[:width]
	}
	return s[:width-1] + "…"
}
