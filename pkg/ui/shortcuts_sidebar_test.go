package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewShortcutsSidebar(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.width != 34 {
		t.Errorf("Expected width 34, got %d", sidebar.width)
	}
	if sidebar.context != "list" {
		t.Errorf("Expected context 'list', got %q", sidebar.context)
	}
}

func TestShortcutsSidebarSetContext(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	sidebar.SetContext("graph")
	if sidebar.context != "graph" {
		t.Errorf("Expected context 'graph', got %q", sidebar.context)
	}

	sidebar.SetContext("insights")
	if sidebar.context != "insights" {
		t.Errorf("Expected context 'insights', got %q", sidebar.context)
	}
}

func TestShortcutsSidebarScrolling(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	// Initial scroll offset should be 0
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected initial scroll 0, got %d", sidebar.scrollOffset)
	}

	// Scroll down
	sidebar.ScrollDown()
	if sidebar.scrollOffset != 1 {
		t.Errorf("Expected scroll 1 after ScrollDown, got %d", sidebar.scrollOffset)
	}

	// Scroll up
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after ScrollUp, got %d", sidebar.scrollOffset)
	}

	// Scroll up at top should stay at 0
	sidebar.ScrollUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 at top, got %d", sidebar.scrollOffset)
	}

	// Page down
	sidebar.ScrollPageDown()
	if sidebar.scrollOffset != 10 {
		t.Errorf("Expected scroll 10 after PageDown, got %d", sidebar.scrollOffset)
	}

	// Page up
	sidebar.ScrollPageUp()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after PageUp, got %d", sidebar.scrollOffset)
	}

	// Reset
	sidebar.scrollOffset = 5
	sidebar.ResetScroll()
	if sidebar.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after Reset, got %d", sidebar.scrollOffset)
	}
}

func TestShortcutsSidebarView(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetSize(28, 30)

	view := sidebar.View()
	if view == "" {
		t.Error("Expected non-empty view")
	}

	// Should contain title
	if !strings.Contains(view, "Shortcuts") {
		t.Error("Expected view to contain 'Shortcuts'")
	}

	// Should contain Navigation section
	if !strings.Contains(view, "Navigation") {
		t.Error("Expected view to contain 'Navigation'")
	}
}

func TestShortcutsSidebarContextFiltering(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}

	// Test graph context
	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetSize(28, 50)
	sidebar.SetContext("graph")
	view := sidebar.View()

	if !strings.Contains(view, "Graph") {
		t.Error("Expected graph context to show Graph section")
	}

	// Test insights context
	sidebar.SetContext("insights")
	view = sidebar.View()

	if !strings.Contains(view, "Insights") {
		t.Error("Expected insights context to show Insights section")
	}
}

func TestContextFromFocus(t *testing.T) {
	tests := []struct {
		focus    focus
		expected string
	}{
		{focusList, "list"},
		{focusDetail, "detail"},
		{focusBoard, "board"},
		{focusTree, "tree"},
		{focusHelp, "list"}, // Default fallback
	}

	for _, tt := range tests {
		got := ContextFromFocus(tt.focus)
		if got != tt.expected {
			t.Errorf("ContextFromFocus(%d) = %q, want %q", tt.focus, got, tt.expected)
		}
	}
}

func TestShortcutsSidebarWidth(t *testing.T) {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	sidebar := NewShortcutsSidebar(theme)

	if sidebar.Width() != 34 {
		t.Errorf("Expected Width() = 34, got %d", sidebar.Width())
	}
}

func TestShortcutsSidebarScrollClampOnView(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	// Set a very small height so content overflows
	sidebar.SetSize(34, 10)
	sidebar.SetContext("list") // list context has many sections

	// Scroll way past the end
	for i := 0; i < 200; i++ {
		sidebar.ScrollDown()
	}

	// View() should clamp scrollOffset to maxScroll
	sidebar.View()

	// After rendering, scrollOffset should be clamped (not 200)
	if sidebar.scrollOffset >= 200 {
		t.Errorf("Expected scrollOffset to be clamped, got %d", sidebar.scrollOffset)
	}
}

func TestShortcutsSidebarScrollIndicators(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	// Use a small height to force overflow
	sidebar.SetSize(34, 10)
	sidebar.SetContext("list")

	// At top: should show down indicator but not up
	sidebar.ResetScroll()
	view := sidebar.View()
	if !strings.Contains(view, "\u25bc") { // ▼
		t.Error("Expected down arrow indicator at top of scrollable content")
	}

	// Scroll down a bit: should show both indicators
	for i := 0; i < 5; i++ {
		sidebar.ScrollDown()
	}
	view = sidebar.View()
	if !strings.Contains(view, "\u25b2") { // ▲
		t.Error("Expected up arrow indicator when scrolled down")
	}
	if !strings.Contains(view, "\u25bc") { // ▼
		t.Error("Expected down arrow indicator when not at bottom")
	}

	// Scroll to bottom: should show up indicator but not down
	for i := 0; i < 200; i++ {
		sidebar.ScrollDown()
	}
	view = sidebar.View()
	if !strings.Contains(view, "\u25b2") { // ▲
		t.Error("Expected up arrow indicator at bottom")
	}
	// At the bottom, no down arrow
	// Count occurrences of ▼ - there should be none
	if strings.Contains(view, "\u25bc") {
		t.Error("Expected no down arrow indicator at bottom of scrollable content")
	}
}

func TestShortcutsSidebarTreeContextIncludesEmacsShortcuts(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetSize(34, 50)
	sidebar.SetContext("tree")

	view := sidebar.View()

	// New emacs-inspired shortcuts should be listed in tree context
	for _, expected := range []string{"Occur"} {
		if !strings.Contains(view, expected) {
			t.Errorf("Expected tree context to include %q shortcut", expected)
		}
	}
}

func TestShortcutsSidebarNeedsScroll(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}

	sidebar := NewShortcutsSidebar(theme)
	sidebar.SetContext("graph") // Few items

	// Large height - content should fit
	sidebar.SetSize(34, 100)
	if sidebar.NeedsScroll() {
		t.Error("Expected NeedsScroll=false when content fits in large height")
	}

	// Small height - content should overflow
	sidebar.SetSize(34, 10)
	if !sidebar.NeedsScroll() {
		t.Error("Expected NeedsScroll=true when height is small")
	}
}

func TestShortcutsSidebarNoIndicatorsWhenFits(t *testing.T) {
	theme := Theme{
		Renderer:  lipgloss.DefaultRenderer(),
		Primary:   lipgloss.AdaptiveColor{Light: "#00ff00", Dark: "#00ff00"},
		Secondary: lipgloss.AdaptiveColor{Light: "#888888", Dark: "#888888"},
		Base:      lipgloss.NewStyle(),
	}
	sidebar := NewShortcutsSidebar(theme)
	// Set a large height so everything fits
	sidebar.SetSize(34, 100)
	sidebar.SetContext("graph") // graph context has few items

	view := sidebar.View()
	// Should NOT contain scroll indicators since content fits
	if strings.Contains(view, "\u25b2") || strings.Contains(view, "\u25bc") {
		t.Error("Expected no scroll indicators when content fits in view")
	}
}
