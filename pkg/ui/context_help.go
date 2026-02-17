package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ContextHelpContent contains compact help content for each context.
// This is used when user triggers context-specific help (e.g., double-tap backtick).
// Content should fit on one screen (~20 lines) without scrolling.
var ContextHelpContent = map[Context]string{
	ContextList:        contextHelpList,
	ContextTree:        contextHelpTree,
	ContextBoard:       contextHelpBoard,
	ContextDetail:      contextHelpDetail,
	ContextSplit:       contextHelpSplit,
	ContextFilter:      contextHelpFilter,
	ContextLabelPicker: contextHelpLabelPicker,
	ContextHelp:        contextHelpHelp,
	ContextTimeTravel:  contextHelpTimeTravel,
}

// GetContextHelp returns the help content for a given context.
// Falls back to generic help if the context has no specific content.
func GetContextHelp(ctx Context) string {
	if content, ok := ContextHelpContent[ctx]; ok {
		return content
	}
	return contextHelpGeneric
}

// RenderContextHelp renders the context-specific help modal.
// This is a compact modal (~60 chars wide) that shows quick reference info.
func RenderContextHelp(ctx Context, theme Theme, width, height int) string {
	content := GetContextHelp(ctx)

	r := theme.Renderer

	// Modal dimensions - compact
	modalWidth := 60
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	// Title
	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(theme.Primary)

	// Content style
	contentStyle := r.NewStyle().
		Foreground(theme.Subtext)

	// Footer hint
	footerStyle := r.NewStyle().
		Foreground(theme.Muted).
		Italic(true)

	// Build content
	var b strings.Builder
	b.WriteString(titleStyle.Render("Quick Reference"))
	b.WriteString("\n")
	b.WriteString(r.NewStyle().Foreground(theme.Border).Render(strings.Repeat("─", modalWidth-4)))
	b.WriteString("\n\n")
	b.WriteString(contentStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(footerStyle.Render("Press ` for full tutorial │ Esc to close"))

	// Wrap in modal style
	modalStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Secondary).
		Padding(1, 2).
		Width(modalWidth)

	return modalStyle.Render(b.String())
}

// =============================================================================
// CONTEXT-SPECIFIC HELP CONTENT (bv-4swd)
// =============================================================================

const contextHelpList = `## List View

**Navigation**
  j/k       Move up/down
  Enter     View issue details
  G         Jump to bottom
  ^d/^u     Page down/up

**Filtering**
  o/c/r/a   Open/closed/ready/all
  /         Fuzzy search
  s         Cycle sort mode
  l         Label picker

**Switch Views**
  E         Tree view
  b         Board view

**Actions**
  y         Copy issue ID
  t/T       Time-travel
  U         Self-update bv`

const contextHelpTree = `## Tree View

**Navigation**
  j/k       Move up/down
  h         Collapse or go to parent
  l         Expand or go to child
  ←/→       Page backward/forward
  Enter/Spc Toggle expand/collapse
  g/G       Jump to top/bottom
  p         Jump to parent node

**Structure**
  X/Z       Expand/collapse all
  Tab       Cycle node visibility
  S-Tab     Cycle global visibility
  1-9       Expand to level N
  d         Toggle detail panel

**Filtering**
  o/c/r/a   Open/closed/ready/all
  s         Sort popup · /  Search
  n/N       Next/prev match

**Modes**
  O  Occur   x  XRay
  ` + "`" + `  Flat    F  Follow
  b/B  Bookmark/cycle   m/M  Mark`

const contextHelpBoard = `## Board View

**Navigation**
  h/l       Move between columns
  j/k       Move within column
  1-4       Jump to column by number
  H/L       Jump to first/last column
  gg/G      Go to top/bottom of column

**Filtering**
  o/c/r     Filter: open/closed/ready

**Search**
  /         Start search
  n/N       Next/prev match

**Grouping**
  s         Cycle: Status/Priority/Type

**Visual Indicators** (card borders)
  🔴 Red     Has blockers
  🟡 Yellow  High-impact (blocks others)
  🟢 Green   Ready to work

**Actions**
  Tab       Toggle detail panel
  Ctrl+j/k  Scroll detail panel
  y         Copy issue ID
  Enter     View issue details
  Esc       Return to List view`

const contextHelpDetail = `## Detail View

**Navigation**
  j/k       Scroll content
  Esc       Return to list
  Tab       Switch to split view

**Actions (from list view)**
  O         Open in editor
  C         Copy issue ID

**Info Shown**
• Full description (markdown)
• Dependencies
• Labels and metadata`

const contextHelpSplit = `## Split View

**Focus**
  Tab       Switch panes
  <         Shrink list pane
  >         Expand list pane

**Left Pane (List)**
  j/k       Navigate issues

**Right Pane (Detail)**
  j/k       Scroll content

**Exit**
  Esc       Return to list view
  Enter     Open full detail

Tip: Detail updates as you navigate`

const contextHelpFilter = `## Filter Mode

**Status Filters**
  o         Open only
  c         Closed only
  r         Ready (no blockers)
  a         All (clear filter)

**Search**
  /         Start fuzzy search
  n/N       Next/prev match
  Esc       Clear search

**Label Filters**
  l         Open label picker`

const contextHelpLabelPicker = `## Label Picker

**Navigation**
  j/k       Move selection
  Enter     Apply label
  Space     Toggle multi-select
  Esc       Cancel

**Search**
  /         Filter labels

**Actions**
  n         Create new label
  d         Delete label
  e         Edit label`

const contextHelpHelp = `## Help Overlay

You're looking at the help overlay!

**Navigation**
  j/k       Scroll help content
  Space     Open full tutorial
  Esc/?     Close this overlay

**Other Help**
  ` + "`" + `         Full tutorial (any time)
  ;         Toggle shortcuts sidebar`

const contextHelpTimeTravel = `## Time Travel Mode

**Currently Viewing**: Past state

This is read-only - you're viewing
how the project looked at a specific
point in history.

**Navigation**
  j/k       Navigate issues
  Enter     View issue detail

**Exit**
  Esc       Return to present

Tip: Use History view (h) to pick
different points in time`

const contextHelpGeneric = `## Quick Reference

**Global Keys**
  ?         Help overlay
  ` + "`" + `         Full tutorial
  Esc       Close/back
  q         Quit

**Navigation**
  j/k       Move up/down
  h/l       Move left/right
  Enter     Select/open

**Views**
  b         Board view
  E         Tree view`

