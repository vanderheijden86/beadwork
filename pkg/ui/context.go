package ui

import (
	"github.com/charmbracelet/bubbles/list"
)

// Context represents the current UI context for context-sensitive help
type Context string

const (
	// Overlays (highest priority)
	ContextLabelPicker     Context = "label-picker"
	ContextHelp            Context = "help"
	ContextQuitConfirm     Context = "quit-confirm"
	ContextTimeTravelInput Context = "time-travel-input"
	ContextRepoPicker      Context = "repo-picker"

	// Views
	ContextBoard Context = "board"
	ContextTree  Context = "tree"

	// Detail states
	ContextSplit      Context = "split"
	ContextDetail     Context = "detail"
	ContextTimeTravel Context = "time-travel"

	// Filter state
	ContextFilter Context = "filter"

	// Default
	ContextList Context = "list"
)

// CurrentContext returns the current UI context identifier.
// This is used for context-sensitive help (e.g., double-tap CapsLock).
// Priority order: overlays → views → detail states → filter → default
func (m Model) CurrentContext() Context {
	// === Overlays (most specific - check first) ===

	// Help overlay
	if m.showHelp {
		return ContextHelp
	}

	// Quit confirmation
	if m.showQuitConfirm {
		return ContextQuitConfirm
	}

	// Label picker overlay
	if m.showLabelPicker {
		return ContextLabelPicker
	}

	// Time-travel input prompt
	if m.showTimeTravelPrompt {
		return ContextTimeTravelInput
	}

	// Repo picker overlay (workspace mode)
	if m.showRepoPicker {
		return ContextRepoPicker
	}

	// === Views (based on focus or view flags) ===

	// Board view
	if m.isBoardView {
		return ContextBoard
	}

	// Tree view
	if m.treeViewActive || m.focused == focusTree {
		return ContextTree
	}

	// === Detail states ===

	// Time-travel mode (comparing snapshots)
	if m.timeTravelMode {
		return ContextTimeTravel
	}

	// Split view (list + detail side by side)
	if m.isSplitView {
		return ContextSplit
	}

	// Detail view (single issue detail)
	if m.showDetails {
		return ContextDetail
	}

	// === Filter state ===

	// Active filtering/search
	if m.list.FilterState() != list.Unfiltered {
		return ContextFilter
	}

	// === Default ===
	return ContextList
}

// ContextDescription returns a human-readable description of the context.
// Useful for status messages or debugging.
func (c Context) Description() string {
	descriptions := map[Context]string{
		ContextLabelPicker:     "Label picker",
		ContextHelp:            "Help overlay",
		ContextQuitConfirm:     "Quit confirmation",
		ContextTimeTravelInput: "Time-travel input",
		ContextRepoPicker:      "Repo picker",
		ContextBoard:           "Kanban board",
		ContextTree:            "Tree view",
		ContextSplit:           "Split view",
		ContextDetail:          "Issue detail",
		ContextTimeTravel:      "Time-travel mode",
		ContextFilter:          "Filter/search mode",
		ContextList:            "Issue list",
	}
	if desc, ok := descriptions[c]; ok {
		return desc
	}
	return string(c)
}

// IsOverlay returns true if the context is an overlay (modal/popup)
func (c Context) IsOverlay() bool {
	switch c {
	case ContextLabelPicker, ContextHelp, ContextQuitConfirm,
		ContextTimeTravelInput, ContextRepoPicker:
		return true
	}
	return false
}

// IsView returns true if the context is a full view (not overlay or default list)
func (c Context) IsView() bool {
	switch c {
	case ContextBoard, ContextTree, ContextSplit, ContextDetail,
		ContextTimeTravel:
		return true
	}
	return false
}

// TutorialPages returns the recommended tutorial page IDs for this context.
// Used to provide context-sensitive help.
func (c Context) TutorialPages() []int {
	pageMap := map[Context][]int{
		ContextList:            {0, 1, 2},
		ContextFilter:          {2, 3},
		ContextDetail:          {4},
		ContextSplit:           {4, 2},
		ContextBoard:           {5},
		ContextTree:            {2, 3},
		ContextTimeTravel:      {10},
		ContextHelp:            {13},
		ContextLabelPicker:     {11, 3},
		ContextRepoPicker:      {12},
		ContextTimeTravelInput: {10},
		ContextQuitConfirm:     {1},
	}
	if pages, ok := pageMap[c]; ok {
		return pages
	}
	return []int{0}
}
