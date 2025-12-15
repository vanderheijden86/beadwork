package ui

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/correlation"
	"github.com/charmbracelet/lipgloss"
)

func createTestHistoryReport() *correlation.HistoryReport {
	now := time.Now()

	return &correlation.HistoryReport{
		GeneratedAt: now,
		Stats: correlation.HistoryStats{
			TotalBeads:       3,
			BeadsWithCommits: 3,
			TotalCommits:     5,
			UniqueAuthors:    2,
		},
		Histories: map[string]correlation.BeadHistory{
			"bv-1": {
				BeadID: "bv-1",
				Title:  "Fix authentication bug",
				Status: "closed",
				Commits: []correlation.CorrelatedCommit{
					{
						SHA:        "abc123def456",
						ShortSHA:   "abc123d",
						Message:    "fix: auth bug",
						Author:     "Dev One",
						Timestamp:  now,
						Method:     correlation.MethodCoCommitted,
						Confidence: 0.95,
					},
					{
						SHA:        "def456ghi789",
						ShortSHA:   "def456g",
						Message:    "test: add auth tests",
						Author:     "Dev One",
						Timestamp:  now.Add(-time.Hour),
						Method:     correlation.MethodExplicitID,
						Confidence: 0.90,
					},
				},
			},
			"bv-2": {
				BeadID: "bv-2",
				Title:  "Add logging",
				Status: "open",
				Commits: []correlation.CorrelatedCommit{
					{
						SHA:        "abc123def456",
						ShortSHA:   "abc123d",
						Message:    "fix: auth bug",
						Author:     "Dev Two",
						Timestamp:  now,
						Method:     correlation.MethodTemporalAuthor,
						Confidence: 0.60,
					},
				},
			},
			"bv-3": {
				BeadID: "bv-3",
				Title:  "Refactor database",
				Status: "in_progress",
				Commits: []correlation.CorrelatedCommit{
					{
						SHA:        "ghi789abc123",
						ShortSHA:   "ghi789a",
						Message:    "refactor: db layer",
						Author:     "Dev Two",
						Timestamp:  now.Add(-2 * time.Hour),
						Method:     correlation.MethodCoCommitted,
						Confidence: 0.92,
					},
					{
						SHA:        "jkl012mno345",
						ShortSHA:   "jkl012m",
						Message:    "refactor: db indexes",
						Author:     "Dev Two",
						Timestamp:  now.Add(-3 * time.Hour),
						Method:     correlation.MethodCoCommitted,
						Confidence: 0.88,
					},
				},
			},
		},
		CommitIndex: correlation.CommitIndex{
			"abc123def456": {"bv-1", "bv-2"},
			"def456ghi789": {"bv-1"},
			"ghi789abc123": {"bv-3"},
			"jkl012mno345": {"bv-3"},
		},
	}
}

func testTheme() Theme {
	return DefaultTheme(lipgloss.NewRenderer(nil))
}

func TestNewHistoryModel(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()

	h := NewHistoryModel(report, theme)

	if h.report != report {
		t.Error("report not set correctly")
	}

	// Should have 3 histories with commits
	if len(h.histories) != 3 {
		t.Errorf("histories count = %d, want 3", len(h.histories))
	}

	if len(h.beadIDs) != 3 {
		t.Errorf("beadIDs count = %d, want 3", len(h.beadIDs))
	}
}

func TestHistoryModel_SetReport(t *testing.T) {
	theme := testTheme()
	h := NewHistoryModel(nil, theme)

	if len(h.histories) != 0 {
		t.Error("should have no histories with nil report")
	}

	report := createTestHistoryReport()
	h.SetReport(report)

	if len(h.histories) != 3 {
		t.Errorf("histories count after SetReport = %d, want 3", len(h.histories))
	}
}

func TestHistoryModel_Navigation(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)
	h.SetSize(100, 40)

	// Start at first bead
	if h.selectedBead != 0 {
		t.Errorf("initial selectedBead = %d, want 0", h.selectedBead)
	}

	// Move down
	h.MoveDown()
	if h.selectedBead != 1 {
		t.Errorf("selectedBead after MoveDown = %d, want 1", h.selectedBead)
	}

	// Move up
	h.MoveUp()
	if h.selectedBead != 0 {
		t.Errorf("selectedBead after MoveUp = %d, want 0", h.selectedBead)
	}

	// Can't move up past 0
	h.MoveUp()
	if h.selectedBead != 0 {
		t.Errorf("selectedBead should stay at 0, got %d", h.selectedBead)
	}
}

func TestHistoryModel_ToggleFocus(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Start with list focus
	if h.focused != historyFocusList {
		t.Errorf("initial focus = %v, want historyFocusList", h.focused)
	}

	h.ToggleFocus()
	if h.focused != historyFocusDetail {
		t.Errorf("focus after toggle = %v, want historyFocusDetail", h.focused)
	}

	h.ToggleFocus()
	if h.focused != historyFocusList {
		t.Errorf("focus after second toggle = %v, want historyFocusList", h.focused)
	}
}

func TestHistoryModel_Selection(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Get selected bead ID
	beadID := h.SelectedBeadID()
	if beadID == "" {
		t.Error("SelectedBeadID() returned empty string")
	}

	// Get selected history
	hist := h.SelectedHistory()
	if hist == nil {
		t.Fatal("SelectedHistory() returned nil")
	}
	if hist.BeadID != beadID {
		t.Errorf("SelectedHistory().BeadID = %s, want %s", hist.BeadID, beadID)
	}

	// Get selected commit
	commit := h.SelectedCommit()
	if commit == nil {
		t.Fatal("SelectedCommit() returned nil")
	}
	if commit.SHA == "" {
		t.Error("SelectedCommit().SHA is empty")
	}
}

func TestHistoryModel_AuthorFilter(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Initially all 3 beads
	if len(h.histories) != 3 {
		t.Errorf("initial histories count = %d, want 3", len(h.histories))
	}

	// Filter by "Dev One"
	h.SetAuthorFilter("Dev One")
	if len(h.histories) != 1 {
		t.Errorf("histories after 'Dev One' filter = %d, want 1", len(h.histories))
	}

	// Check the remaining bead is bv-1
	if len(h.beadIDs) != 1 || h.beadIDs[0] != "bv-1" {
		t.Errorf("filtered beadID = %v, want [bv-1]", h.beadIDs)
	}

	// Clear filter
	h.SetAuthorFilter("")
	if len(h.histories) != 3 {
		t.Errorf("histories after clearing filter = %d, want 3", len(h.histories))
	}
}

func TestHistoryModel_ConfidenceFilter(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Filter by high confidence (>=0.85)
	h.SetMinConfidence(0.85)

	// bv-1 has commits at 0.95 and 0.90 - should be included
	// bv-2 has only 0.60 - should be excluded
	// bv-3 has 0.92 and 0.88 - should be included
	if len(h.histories) != 2 {
		t.Errorf("histories after confidence filter = %d, want 2", len(h.histories))
	}

	// Reset filter
	h.SetMinConfidence(0)
	if len(h.histories) != 3 {
		t.Errorf("histories after clearing confidence filter = %d, want 3", len(h.histories))
	}
}

func TestHistoryModel_EmptyReport(t *testing.T) {
	theme := testTheme()

	// Test with nil report
	h := NewHistoryModel(nil, theme)
	if h.SelectedBeadID() != "" {
		t.Error("SelectedBeadID() should return empty for nil report")
	}
	if h.SelectedHistory() != nil {
		t.Error("SelectedHistory() should return nil for nil report")
	}
	if h.SelectedCommit() != nil {
		t.Error("SelectedCommit() should return nil for nil report")
	}

	// Test with empty histories
	emptyReport := &correlation.HistoryReport{
		Histories: map[string]correlation.BeadHistory{},
	}
	h2 := NewHistoryModel(emptyReport, theme)
	if len(h2.histories) != 0 {
		t.Errorf("histories count = %d, want 0", len(h2.histories))
	}
}

func TestHistoryModel_DetailNavigation(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Find the bead with 2 commits (bv-1 or bv-3)
	var beadWithTwoCommits int
	for i, hist := range h.histories {
		if len(hist.Commits) >= 2 {
			beadWithTwoCommits = i
			break
		}
	}
	h.selectedBead = beadWithTwoCommits

	// Switch to detail focus
	h.ToggleFocus()
	if h.focused != historyFocusDetail {
		t.Fatal("should be in detail focus")
	}

	// Initial commit selection
	if h.selectedCommit != 0 {
		t.Errorf("initial selectedCommit = %d, want 0", h.selectedCommit)
	}

	// Move down in commits
	h.MoveDown()
	if h.selectedCommit != 1 {
		t.Errorf("selectedCommit after MoveDown = %d, want 1", h.selectedCommit)
	}

	// Move up in commits
	h.MoveUp()
	if h.selectedCommit != 0 {
		t.Errorf("selectedCommit after MoveUp = %d, want 0", h.selectedCommit)
	}
}

func TestHistoryModel_View(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)
	h.SetSize(120, 40)

	// Should render without panic
	view := h.View()
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Should contain header
	if len(view) < 100 {
		t.Errorf("View() seems too short: %d chars", len(view))
	}
}

func TestHistoryModel_ViewEmpty(t *testing.T) {
	theme := testTheme()

	// Test with nil report
	h := NewHistoryModel(nil, theme)
	h.SetSize(80, 24)

	view := h.View()
	if view == "" {
		t.Error("View() for nil report returned empty")
	}
}

func TestHistoryModel_ViewSmallWidth(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Test various small widths - should not panic
	smallWidths := []int{5, 6, 7, 10, 15, 20}
	for _, w := range smallWidths {
		h.SetSize(w, 10)
		// This should not panic
		view := h.View()
		if view == "" {
			t.Errorf("View() with width %d returned empty", w)
		}
	}
}

func TestHistoryModel_SortByCommitCount(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Verify histories are sorted by commit count (descending)
	// bv-1 has 2 commits, bv-3 has 2 commits, bv-2 has 1 commit
	// Ties are broken by bead ID
	if len(h.histories) < 2 {
		t.Fatal("not enough histories for sort test")
	}

	// The last bead should have fewest commits
	lastHist := h.histories[len(h.histories)-1]
	if len(lastHist.Commits) > 1 {
		t.Errorf("last bead has %d commits, expected 1 (bv-2)", len(lastHist.Commits))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a…"},
		{"abc", 5, "abc"},
		{"hello", 5, "hello"},
		{"hello!", 5, "hell…"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestMethodLabel(t *testing.T) {
	tests := []struct {
		method correlation.CorrelationMethod
		want   string
	}{
		{correlation.MethodCoCommitted, "(co-committed)"},
		{correlation.MethodExplicitID, "(explicit ID)"},
		{correlation.MethodTemporalAuthor, "(temporal)"},
		{correlation.CorrelationMethod("unknown"), ""},
	}

	for _, tt := range tests {
		got := methodLabel(tt.method)
		if got != tt.want {
			t.Errorf("methodLabel(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}

func TestHistoryModel_GetHistoryForBead(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Get existing bead history
	hist := h.GetHistoryForBead("bv-1")
	if hist == nil {
		t.Fatal("GetHistoryForBead(bv-1) returned nil")
	}
	if hist.BeadID != "bv-1" {
		t.Errorf("GetHistoryForBead(bv-1).BeadID = %s, want bv-1", hist.BeadID)
	}

	// Get non-existent bead history
	histNone := h.GetHistoryForBead("bv-nonexistent")
	if histNone != nil {
		t.Error("GetHistoryForBead(bv-nonexistent) should return nil")
	}
}

func TestHistoryModel_HasReport(t *testing.T) {
	theme := testTheme()

	// Without report
	h := NewHistoryModel(nil, theme)
	if h.HasReport() {
		t.Error("HasReport() should return false with nil report")
	}

	// With report
	report := createTestHistoryReport()
	h2 := NewHistoryModel(report, theme)
	if !h2.HasReport() {
		t.Error("HasReport() should return true with report")
	}
}

func TestHistoryModel_CommitNavigation(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Find a bead with 2 commits
	for i, hist := range h.histories {
		if len(hist.Commits) >= 2 {
			h.selectedBead = i
			break
		}
	}

	// Start at first commit
	if h.selectedCommit != 0 {
		t.Errorf("initial selectedCommit = %d, want 0", h.selectedCommit)
	}

	// NextCommit moves to next
	h.NextCommit()
	if h.selectedCommit != 1 {
		t.Errorf("selectedCommit after NextCommit = %d, want 1", h.selectedCommit)
	}

	// PrevCommit moves back
	h.PrevCommit()
	if h.selectedCommit != 0 {
		t.Errorf("selectedCommit after PrevCommit = %d, want 0", h.selectedCommit)
	}

	// PrevCommit at 0 stays at 0
	h.PrevCommit()
	if h.selectedCommit != 0 {
		t.Errorf("selectedCommit after PrevCommit at 0 = %d, want 0", h.selectedCommit)
	}
}

func TestHistoryModel_CycleConfidence(t *testing.T) {
	report := createTestHistoryReport()
	theme := testTheme()
	h := NewHistoryModel(report, theme)

	// Initial confidence is 0
	if h.GetMinConfidence() != 0 {
		t.Errorf("initial confidence = %f, want 0", h.GetMinConfidence())
	}

	// Cycle through thresholds
	h.CycleConfidence()
	if h.GetMinConfidence() != 0.5 {
		t.Errorf("confidence after first cycle = %f, want 0.5", h.GetMinConfidence())
	}

	h.CycleConfidence()
	if h.GetMinConfidence() != 0.75 {
		t.Errorf("confidence after second cycle = %f, want 0.75", h.GetMinConfidence())
	}

	h.CycleConfidence()
	if h.GetMinConfidence() != 0.9 {
		t.Errorf("confidence after third cycle = %f, want 0.9", h.GetMinConfidence())
	}

	// Wrap around to 0
	h.CycleConfidence()
	if h.GetMinConfidence() != 0 {
		t.Errorf("confidence after fourth cycle = %f, want 0", h.GetMinConfidence())
	}
}
