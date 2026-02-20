// tree.go - Hierarchical tree view for epic/task/subtask relationships (bv-gllx)
package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// TreeState represents the persistent state of the tree view (bv-zv7p).
// This is saved to .beads/tree-state.json to preserve expand/collapse state
// across sessions.
//
// File format (JSON):
//
//	{
//	  "version": 1,
//	  "expanded": {
//	    "bv-123": true,   // explicitly expanded
//	    "bv-456": false   // explicitly collapsed
//	  }
//	}
//
// Design notes:
//   - Only stores explicit user changes; nodes not in the map use default behavior
//   - Default: expanded for depth < 1, collapsed otherwise
//   - Version field enables future schema migrations
//   - Corrupted/missing file = use defaults (graceful degradation)
type TreeState struct {
	Version       int             `json:"version"`                    // Schema version (currently 1)
	Expanded      map[string]bool `json:"expanded"`                   // Issue ID -> explicitly set state
	Bookmarks     []string        `json:"bookmarks,omitempty"`        // Bookmarked issue IDs (bd-k4n)
	SortField     *int            `json:"sort_field,omitempty"`       // Persisted sort field (bd-2qw)
	SortDirection *int            `json:"sort_direction,omitempty"`   // Persisted sort direction (bd-2qw)
}

// TreeStateVersion is the current schema version for tree persistence
const TreeStateVersion = 1

// DefaultTreeState returns a new TreeState with sensible defaults
func DefaultTreeState() *TreeState {
	return &TreeState{
		Version:  TreeStateVersion,
		Expanded: make(map[string]bool),
	}
}

// treeStateFileName is the filename for persisted tree state
const treeStateFileName = "tree-state.json"

// TreeStatePath returns the path to the tree state file.
// By default this is .beads/tree-state.json in the current directory.
// The beadsDir parameter allows overriding the .beads directory location
// (e.g., from BEADS_DIR environment variable).
func TreeStatePath(beadsDir string) string {
	if beadsDir == "" {
		beadsDir = ".beads"
	}
	return filepath.Join(beadsDir, treeStateFileName)
}

// SetBeadsDir sets the beads directory for persistence (bv-19vz).
// This should be called before any expand/collapse operations if a custom
// beads directory is desired. If not called, defaults to ".beads" in cwd.
func (t *TreeModel) SetBeadsDir(dir string) {
	t.beadsDir = dir
}

// saveState persists the current expand/collapse state to disk (bv-19vz).
// Only stores explicit user changes; nodes not in the map use default behavior.
// Errors are logged but do not interrupt the user experience.
// If beadsDir has not been set (empty string), persistence is skipped entirely
// to avoid reading/writing tree-state.json from the process working directory.
func (t *TreeModel) saveState() {
	if t.beadsDir == "" {
		return // No persistence directory configured
	}
	state := &TreeState{
		Version:  TreeStateVersion,
		Expanded: make(map[string]bool),
	}

	// Walk all nodes and record explicit expand state
	var walk func(node *IssueTreeNode)
	walk = func(node *IssueTreeNode) {
		if node == nil || node.Issue == nil {
			return
		}

		// Default: expanded for depth < 1, collapsed otherwise
		defaultExpanded := node.Depth < 1
		if node.Expanded != defaultExpanded {
			state.Expanded[node.Issue.ID] = node.Expanded
		}

		for _, child := range node.Children {
			walk(child)
		}
	}

	for _, root := range t.roots {
		walk(root)
	}

	// Save bookmarks (bd-k4n)
	for id := range t.bookmarks {
		state.Bookmarks = append(state.Bookmarks, id)
	}
	sort.Strings(state.Bookmarks) // Stable ordering for deterministic output

	// Save sort settings if non-default (bd-2qw)
	defaultField := SortFieldCreated
	defaultDir := SortDescending
	if t.sortField != defaultField || t.sortDirection != defaultDir {
		sf := int(t.sortField)
		sd := int(t.sortDirection)
		state.SortField = &sf
		state.SortDirection = &sd
	}

	// Write to file
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("warning: failed to marshal tree state: %v", err)
		return
	}

	path := TreeStatePath(t.beadsDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("warning: failed to create state directory %s: %v", dir, err)
		return
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("warning: failed to write tree state to %s: %v", path, err)
		return
	}
}

// loadState restores expand/collapse state from disk (bv-afcm).
// If the file doesn't exist or is corrupted, defaults are used silently.
// If beadsDir has not been set (empty string), loading is skipped entirely
// to avoid picking up tree-state.json from the process working directory.
func (t *TreeModel) loadState() {
	if t.beadsDir == "" {
		return // No persistence directory configured
	}
	path := TreeStatePath(t.beadsDir)
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist = first run, use defaults
		return
	}

	var state TreeState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("warning: invalid tree state file, using defaults: %v", err)
		return
	}

	// Apply loaded state to nodes
	t.applyState(&state)
}

// applyState sets expand state on nodes based on loaded state (bv-afcm).
// Unknown IDs in state are silently ignored (stale IDs handled by bv-0jaz).
func (t *TreeModel) applyState(state *TreeState) {
	if state == nil {
		return
	}

	for id, expanded := range state.Expanded {
		if node, ok := t.issueMap[id]; ok {
			node.Expanded = expanded
		}
		// If ID not found, it's stale - ignore
	}

	// Restore bookmarks (bd-k4n)
	if len(state.Bookmarks) > 0 {
		t.bookmarks = make(map[string]bool, len(state.Bookmarks))
		for _, id := range state.Bookmarks {
			t.bookmarks[id] = true
		}
	}

	// Restore sort settings (bd-2qw)
	if state.SortField != nil && state.SortDirection != nil {
		sf := SortField(*state.SortField)
		sd := SortDirection(*state.SortDirection)
		if sf >= 0 && sf < NumSortFields && (sd == SortAscending || sd == SortDescending) {
			t.sortField = sf
			t.sortDirection = sd
		}
	}
}

// TreeViewMode determines what relationships are displayed
type TreeViewMode int

const (
	TreeModeHierarchy TreeViewMode = iota // parent-child deps (default)
	TreeModeBlocking                      // blocking deps (future)
)

// selectionGutterWidth is the width reserved on the left of every tree row
// so the Selected style's left border (1 char) + PaddingLeft(1) doesn't shift
// the tree connectors out of alignment. Non-selected rows get equivalent
// blank padding. Must match theme.Selected border + padding geometry.
const selectionGutterWidth = 2

// IssueTreeNode represents a node in the hierarchical issue tree
type IssueTreeNode struct {
	Issue    *model.Issue     // Reference to the actual issue
	Children []*IssueTreeNode // Child nodes
	Expanded bool             // Is this node expanded?
	Depth    int              // Nesting level (0 = root)
	Parent   *IssueTreeNode   // Back-reference for navigation
}

// TreeModel manages the hierarchical tree view state
type TreeModel struct {
	roots          []*IssueTreeNode          // Root nodes (issues with no parent)
	flatList       []*IssueTreeNode          // Flattened visible nodes for navigation
	cursor         int                       // Current selection index in flatList
	viewport       viewport.Model            // For scrolling
	theme          Theme                     // Visual styling
	mode           TreeViewMode              // Hierarchy vs blocking
	issueMap       map[string]*IssueTreeNode // Quick lookup by issue ID
	width          int                       // Available width
	height         int                       // Available height
	viewportOffset int                       // Index of first visible node (bv-r4ng)
	sortMode       SortMode                  // Current sort mode for tree siblings (bd-adf) — legacy, kept for CycleSortMode compat
	sortField      SortField                 // Current sort field (bd-x3l)
	sortDirection  SortDirection             // Current sort direction (bd-x3l)

	// Build state
	built    bool   // Has tree been built?
	lastHash string // Hash of issues for cache invalidation

	// Persistence state (bv-19vz)
	beadsDir string // Directory containing .beads (for tree-state.json)

	// Filter state (bd-e3w)
	currentFilter    string                  // "all", "open", "closed", "ready"
	filterMatches    map[string]bool         // Issue IDs that match the filter
	contextAncestors map[string]bool         // Ancestor IDs shown for context (dimmed)
	globalIssueMap   map[string]*model.Issue // Reference to global issue map (for blocker checks in "ready" filter)

	// PageRank scores for sort-by-pagerank (bd-x3l)
	pageRankScores map[string]float64 // Issue ID -> PageRank score (set externally)

	// Sort popup state (bd-t4e)
	sortPopupOpen   bool // Is the sort popup overlay visible?
	sortPopupCursor int  // Currently highlighted field index in the popup

	// Search state (bd-uus)
	searchMode       bool             // Is search input active?
	searchQuery      string           // Current search query
	searchMatches    []*IssueTreeNode // Nodes matching search
	searchMatchIndex int              // Current match index for n/N cycling
	searchMatchIDs   map[string]bool  // Quick lookup for highlighting

	// Visibility cycling state (bd-8of)
	cycleStates      map[string]int // Per-node TAB cycle state: 0=folded, 1=children, 2=subtree
	globalCycleState int            // Global Shift+TAB cycle: 0=all-folded, 1=top-level, 2=all-expanded

	// Flat mode state (bd-39v)
	flatMode bool // When true, show all issues in flat list without hierarchy

	// Advanced filter state (bd-08h)
	advancedPredicates []FilterPredicate // Parsed predicates for advanced filtering

	// Mark state (bd-cz0)
	markedIDs map[string]bool // Issue IDs that are marked/selected for batch ops

	// XRay state (bd-0rc)
	xrayRoot *IssueTreeNode // When set, only this subtree is shown

	// Bookmark state (bd-k4n)
	bookmarks map[string]bool // Issue IDs that are bookmarked

	// Follow mode state (bd-c0c)
	followMode   bool     // Whether follow mode is active
	lastIssueIDs []string // Issue IDs from last refresh, for detecting changes

	// Occur mode state (bd-sjs.2)
	occurMode    bool   // Is occur mode active?
	occurPattern string // Current occur pattern


}

// NewTreeModel creates an empty tree model
func NewTreeModel(theme Theme) TreeModel {
	return TreeModel{
		theme:         theme,
		mode:          TreeModeHierarchy,
		issueMap:      make(map[string]*IssueTreeNode),
		sortField:     SortFieldCreated,    // Default: newest first (bd-ctu)
		sortDirection: SortDescending,      // Default: newest first (bd-ctu)
	}
}

// buildIssueTreeNodes constructs the hierarchical parent/child tree and returns:
// - roots: root nodes (sorted)
// - nodeMap: issue ID -> tree node
//
// This does NOT load persisted expand/collapse state or build the visible flat list.
// Those remain view concerns handled by TreeModel (so user state can change without
// requiring a snapshot rebuild).
func buildIssueTreeNodes(issues []model.Issue) ([]*IssueTreeNode, map[string]*IssueTreeNode) {
	t := TreeModel{
		issueMap: make(map[string]*IssueTreeNode),
	}
	if len(issues) == 0 {
		return nil, t.issueMap
	}

	// Step 1: Build parent→children index and track which issues have parents
	childrenOf := make(map[string][]*model.Issue)
	hasParent := make(map[string]bool)
	issueByID := make(map[string]*model.Issue)

	for i := range issues {
		issue := &issues[i]
		issueByID[issue.ID] = issue

		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type == model.DepParentChild {
				parentID := dep.DependsOnID
				childrenOf[parentID] = append(childrenOf[parentID], issue)
				hasParent[issue.ID] = true
			}
		}
	}

	// Step 2: Identify root nodes (issues with no parent OR whose parent doesn't exist)
	var rootIssues []*model.Issue
	for i := range issues {
		issue := &issues[i]
		if !hasParent[issue.ID] {
			rootIssues = append(rootIssues, issue)
			continue
		}

		// Issue declares a parent - verify at least one referenced parent exists
		hasValidParent := false
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type == model.DepParentChild {
				if _, exists := issueByID[dep.DependsOnID]; exists {
					hasValidParent = true
					break
				}
			}
		}
		if !hasValidParent {
			rootIssues = append(rootIssues, issue)
		}
	}

	// Step 3: Build tree recursively with cycle detection
	visited := make(map[string]bool)
	for _, issue := range rootIssues {
		node := t.buildNode(issue, 0, childrenOf, nil, visited)
		if node != nil {
			t.roots = append(t.roots, node)
		}
	}

	// Step 4: Sort roots by priority, type, then created date
	t.sortNodes(t.roots)

	return t.roots, t.issueMap
}

// SetSize updates the available dimensions for the tree view
func (t *TreeModel) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.viewport.Width = width
	t.viewport.Height = height
}

// Build constructs the tree from issues using parent-child dependencies.
// Implementation for bv-j3ck.
func (t *TreeModel) Build(issues []model.Issue) {
	// Reset state
	t.roots = nil
	t.flatList = nil
	t.issueMap = make(map[string]*IssueTreeNode)
	t.cursor = 0

	if len(issues) == 0 {
		t.built = true
		return
	}

	// Build tree structure (no state) and then apply persisted expand/collapse state.
	roots, nodeMap := buildIssueTreeNodes(issues)
	t.roots = roots
	t.issueMap = nodeMap

	// Step 5: Re-sort using the model's configured sortField/sortDirection (bd-2ty).
	// buildIssueTreeNodes uses a hardcoded priority-based sort internally, so we
	// must re-sort all sibling groups to honour the actual default (Created desc).
	t.sortAllSiblings()

	// Step 6: Handle empty tree (no parent-child relationships found)
	// If all issues are roots (no hierarchy), that's fine - show them all
	// The View() will handle displaying a helpful message if needed

	// Step 7: Load persisted state (bv-afcm)
	// This modifies node.Expanded values before we build the flat list
	t.loadState()

	// Step 8: Build the flat list for navigation
	// This must come after loadState so expand states are applied
	t.rebuildFlatList()

	t.built = true
}

// BuildFromSnapshot wires the tree view to precomputed tree data from a DataSnapshot.
// This avoids building the parent/child structure on the UI thread when the snapshot
// already contains it (bv-t435).
func (t *TreeModel) BuildFromSnapshot(snapshot *DataSnapshot) {
	if snapshot == nil {
		return
	}

	// Skip work if we're already built for this snapshot.
	if t.built && snapshot.DataHash != "" && t.lastHash == snapshot.DataHash {
		return
	}

	// Preserve current selection (best-effort) by issue ID.
	prevSelectedID := ""
	if issue := t.SelectedIssue(); issue != nil {
		prevSelectedID = issue.ID
	}

	// Reset view state, but keep dimensions/theme/beadsDir.
	t.roots = snapshot.TreeRoots
	t.issueMap = snapshot.TreeNodeMap

	// If the snapshot didn't include tree data, fall back to building it now.
	if len(t.roots) == 0 || t.issueMap == nil {
		t.Build(snapshot.Issues)
		t.lastHash = snapshot.DataHash
		return
	}

	// Re-sort using the model's configured sortField/sortDirection (bd-2ty).
	// Snapshot tree roots were built with a hardcoded priority-based sort.
	t.sortAllSiblings()

	// Apply persisted expand/collapse state and rebuild visible list.
	t.loadState()
	t.rebuildFlatList()
	t.built = true
	t.lastHash = snapshot.DataHash

	// Restore selection if possible.
	if prevSelectedID != "" {
		for i, node := range t.flatList {
			if node != nil && node.Issue != nil && node.Issue.ID == prevSelectedID {
				t.cursor = i
				t.ensureCursorVisible()
				break
			}
		}
	}
}

// buildNode recursively builds a tree node and its children.
// Uses visited map for cycle detection.
func (t *TreeModel) buildNode(issue *model.Issue, depth int,
	childrenOf map[string][]*model.Issue,
	parent *IssueTreeNode,
	visited map[string]bool) *IssueTreeNode {

	if issue == nil {
		return nil
	}

	// Cycle detection - if we've already visited this node in current path
	if visited[issue.ID] {
		// Return a node marked as part of a cycle (no children to break the loop)
		return &IssueTreeNode{
			Issue:    issue,
			Depth:    depth,
			Parent:   parent,
			Expanded: false,
			// Children intentionally left empty to break cycle
		}
	}

	// Mark as visited for cycle detection
	visited[issue.ID] = true
	defer func() { visited[issue.ID] = false }()

	node := &IssueTreeNode{
		Issue:    issue,
		Depth:    depth,
		Parent:   parent,
		Expanded: depth < 1, // Auto-expand root level only
	}

	// Store in lookup map
	t.issueMap[issue.ID] = node

	// Build children recursively
	children := childrenOf[issue.ID]
	for _, child := range children {
		childNode := t.buildNode(child, depth+1, childrenOf, node, visited)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}

	// Sort children
	t.sortNodes(node.Children)

	return node
}

// sortNodes sorts a slice of tree nodes by priority, issue type, then created date.
func (t *TreeModel) sortNodes(nodes []*IssueTreeNode) {
	if len(nodes) <= 1 {
		return
	}

	sort.Slice(nodes, func(i, j int) bool {
		// Defensive: check for nil nodes first
		if nodes[i] == nil || nodes[j] == nil {
			return nodes[i] != nil // Non-nil nodes first
		}
		a, b := nodes[i].Issue, nodes[j].Issue
		if a == nil || b == nil {
			return a != nil // Non-nil issues first
		}

		// 1. Priority (ascending - P0 first)
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}

		// 2. IssueType order: epic → feature → task → bug → chore
		aTypeOrder := issueTypeOrder(a.IssueType)
		bTypeOrder := issueTypeOrder(b.IssueType)
		if aTypeOrder != bTypeOrder {
			return aTypeOrder < bTypeOrder
		}

		// 3. CreatedAt (oldest first for stable ordering)
		return a.CreatedAt.Before(b.CreatedAt)
	})
}

// issueTypeOrder returns a numeric order for issue types.
// Lower numbers sort first: epic → feature → task → bug → chore
func issueTypeOrder(t model.IssueType) int {
	switch t {
	case model.TypeEpic:
		return 0
	case model.TypeFeature:
		return 1
	case model.TypeTask:
		return 2
	case model.TypeBug:
		return 3
	case model.TypeChore:
		return 4
	default:
		return 5
	}
}

// CycleSortMode advances to the next sort field and re-sorts the tree (bd-adf).
// This preserves the legacy cycling behavior: each press advances to the next field
// with its default direction. Kept for backwards compatibility.
func (t *TreeModel) CycleSortMode() {
	t.sortField = (t.sortField + 1) % NumSortFields
	t.sortDirection = t.sortField.DefaultDirection()
	t.sortAllSiblings()
	t.rebuildFlatList()
}

// GetSortMode returns the current sort mode for legacy callers (bd-adf).
// Maps the new SortField/SortDirection to the old SortMode enum.
func (t *TreeModel) GetSortMode() SortMode {
	switch t.sortField {
	case SortFieldCreated:
		if t.sortDirection == SortAscending {
			return SortCreatedAsc
		}
		return SortCreatedDesc
	case SortFieldPriority:
		return SortPriority
	case SortFieldUpdated:
		return SortUpdated
	default:
		return SortDefault
	}
}

// SetSort sets the sort field and direction, re-sorts the tree, and rebuilds the flat list (bd-x3l).
func (t *TreeModel) SetSort(field SortField, dir SortDirection) {
	t.sortField = field
	t.sortDirection = dir
	t.sortAllSiblings()
	t.rebuildFlatList()
}

// GetSortField returns the current sort field (bd-x3l).
func (t *TreeModel) GetSortField() SortField {
	return t.sortField
}

// GetSortDirection returns the current sort direction (bd-x3l).
func (t *TreeModel) GetSortDirection() SortDirection {
	return t.sortDirection
}

// sortAllSiblings walks the entire tree and sorts children at each level (bd-adf).
func (t *TreeModel) sortAllSiblings() {
	t.sortNodesByFieldDirection(t.roots)
	var walk func(nodes []*IssueTreeNode)
	walk = func(nodes []*IssueTreeNode) {
		for _, node := range nodes {
			if len(node.Children) > 1 {
				t.sortNodesByFieldDirection(node.Children)
			}
			walk(node.Children)
		}
	}
	walk(t.roots)
}

// sortNodesByFieldDirection sorts a slice of sibling nodes using the current
// sortField and sortDirection (bd-x3l). Replaces sortNodesBySortMode.
func (t *TreeModel) sortNodesByFieldDirection(nodes []*IssueTreeNode) {
	if len(nodes) <= 1 {
		return
	}
	asc := t.sortDirection == SortAscending
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i] == nil || nodes[j] == nil {
			return nodes[i] != nil
		}
		a, b := nodes[i].Issue, nodes[j].Issue
		if a == nil || b == nil {
			return a != nil
		}
		less := t.compareByField(a, b)
		if asc {
			return less
		}
		// For descending, reverse the comparison (but handle equality: use stable tiebreak)
		greater := t.compareByField(b, a)
		return greater
	})
}

// compareByField returns true if a should sort before b for the current sortField
// in ascending order. Used by sortNodesByFieldDirection.
func (t *TreeModel) compareByField(a, b *model.Issue) bool {
	switch t.sortField {
	case SortFieldCreated:
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
	case SortFieldUpdated:
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.Before(b.UpdatedAt)
		}
	case SortFieldPriority:
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
	case SortFieldTitle:
		if a.Title != b.Title {
			return a.Title < b.Title
		}
	case SortFieldStatus:
		if a.Status != b.Status {
			return statusOrder(a.Status) < statusOrder(b.Status)
		}
	case SortFieldType:
		aOrder := issueTypeOrder(a.IssueType)
		bOrder := issueTypeOrder(b.IssueType)
		if aOrder != bOrder {
			return aOrder < bOrder
		}
	case SortFieldDepsCount:
		aDeps := len(a.Dependencies)
		bDeps := len(b.Dependencies)
		if aDeps != bDeps {
			return aDeps < bDeps
		}
	case SortFieldPageRank:
		// PageRank is stored externally in graph analysis, not on Issue directly.
		// Use the pageRankScores map if available; otherwise equal (falls to tiebreak).
		aRank := t.getPageRank(a.ID)
		bRank := t.getPageRank(b.ID)
		if aRank != bRank {
			return aRank < bRank
		}
	default:
		// Default sort: priority, then type, then created
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		aOrder := issueTypeOrder(a.IssueType)
		bOrder := issueTypeOrder(b.IssueType)
		if aOrder != bOrder {
			return aOrder < bOrder
		}
		return a.CreatedAt.Before(b.CreatedAt)
	}
	// Tiebreak: fall back to ID for stable ordering
	return a.ID < b.ID
}

// statusOrder returns a numeric order for issue statuses.
// Lower numbers sort first: open → in_progress → blocked → closed → tombstone
func statusOrder(s model.Status) int {
	switch s {
	case model.StatusOpen:
		return 0
	case model.StatusInProgress:
		return 1
	case model.StatusBlocked:
		return 2
	case model.StatusClosed:
		return 3
	case model.StatusTombstone:
		return 4
	default:
		return 5
	}
}

// ── Sort popup methods (bd-t4e) ──

// IsSortPopupOpen returns whether the sort popup overlay is visible.
func (t *TreeModel) IsSortPopupOpen() bool {
	return t.sortPopupOpen
}

// OpenSortPopup opens the sort popup overlay, positioning the cursor on the current sort field.
func (t *TreeModel) OpenSortPopup() {
	t.sortPopupOpen = true
	t.sortPopupCursor = int(t.sortField)
}

// CloseSortPopup closes the sort popup overlay without changing the sort.
func (t *TreeModel) CloseSortPopup() {
	t.sortPopupOpen = false
}

// SortPopupCursor returns the currently highlighted field index in the popup.
func (t *TreeModel) SortPopupCursor() int {
	return t.sortPopupCursor
}

// SortPopupDown moves the popup cursor down one field.
func (t *TreeModel) SortPopupDown() {
	if t.sortPopupCursor < int(NumSortFields)-1 {
		t.sortPopupCursor++
	}
}

// SortPopupUp moves the popup cursor up one field.
func (t *TreeModel) SortPopupUp() {
	if t.sortPopupCursor > 0 {
		t.sortPopupCursor--
	}
}

// SortPopupSelect applies the highlighted sort field. If the selected field is
// already the current sort field, toggle the direction. Otherwise, set the new
// field with its default direction. Closes the popup.
func (t *TreeModel) SortPopupSelect() {
	selectedField := SortField(t.sortPopupCursor)
	if selectedField == t.sortField {
		// Toggle direction
		t.sortDirection = t.sortDirection.Toggle()
	} else {
		// New field with default direction
		t.sortField = selectedField
		t.sortDirection = selectedField.DefaultDirection()
	}
	t.sortAllSiblings()
	t.rebuildFlatList()
	t.sortPopupOpen = false
}

// RenderSortPopup renders the sort popup overlay as a string (bd-t4e).
// Shows all sort fields with the current one marked with a direction indicator.
func (t *TreeModel) RenderSortPopup() string {
	if !t.sortPopupOpen {
		return ""
	}

	r := t.theme.Renderer
	var sb strings.Builder

	for i := SortField(0); i < NumSortFields; i++ {
		isSelected := int(i) == t.sortPopupCursor
		isCurrent := i == t.sortField

		// Build the line
		var line string
		indicator := "  " // 2 chars for alignment
		if isCurrent {
			indicator = t.sortDirection.Indicator() + " "
		}

		label := indicator + i.String()

		if isSelected {
			style := r.NewStyle().
				Foreground(t.theme.Primary).
				Bold(true)
			line = style.Render("▸ " + label)
		} else {
			style := r.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#E8E8E8"})
			line = style.Render("  " + label)
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	content := strings.TrimRight(sb.String(), "\n")

	// Wrap in a bordered box
	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.theme.Primary).
		Padding(0, 1)

	titleStyle := r.NewStyle().
		Foreground(t.theme.Primary).
		Bold(true)

	return boxStyle.Render(titleStyle.Render("Sort by")+"\n"+content) + "\n"
}

// SetPageRankScores sets externally-computed PageRank scores for sort-by-pagerank (bd-x3l).
func (t *TreeModel) SetPageRankScores(scores map[string]float64) {
	t.pageRankScores = scores
}

// getPageRank returns the PageRank score for an issue ID, or 0 if not available.
func (t *TreeModel) getPageRank(id string) float64 {
	if t.pageRankScores == nil {
		return 0
	}
	return t.pageRankScores[id]
}

// SetGlobalIssueMap provides the global issue map for blocker resolution in filters (bd-e3w).
func (t *TreeModel) SetGlobalIssueMap(m map[string]*model.Issue) {
	t.globalIssueMap = m
}

// GetFilter returns the current filter string (bd-e3w).
func (t *TreeModel) GetFilter() string {
	return t.currentFilter
}

// ApplyFilter sets the current filter and rebuilds the visible flat list (bd-e3w).
func (t *TreeModel) ApplyFilter(filter string) {
	t.currentFilter = filter
	if filter == "" || filter == "all" {
		t.currentFilter = "all"
		t.filterMatches = nil
		t.contextAncestors = nil
		t.rebuildFlatList()
		return
	}

	t.filterMatches = make(map[string]bool)
	t.contextAncestors = make(map[string]bool)

	// Mark matching nodes
	for id, node := range t.issueMap {
		if t.nodeMatchesFilter(node) {
			t.filterMatches[id] = true
			// Mark all ancestors as context
			ancestor := node.Parent
			for ancestor != nil {
				if ancestor.Issue != nil {
					t.contextAncestors[ancestor.Issue.ID] = true
				}
				ancestor = ancestor.Parent
			}
		}
	}

	t.rebuildFlatList()
}

// nodeMatchesFilter checks if a single node matches the current filter (bd-e3w).
func (t *TreeModel) nodeMatchesFilter(node *IssueTreeNode) bool {
	if node == nil || node.Issue == nil {
		return false
	}
	issue := node.Issue
	switch t.currentFilter {
	case "open":
		return !isClosedLikeStatus(issue.Status)
	case "closed":
		return isClosedLikeStatus(issue.Status)
	case "ready":
		if isClosedLikeStatus(issue.Status) || issue.Status == model.StatusBlocked {
			return false
		}
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := t.globalIssueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

// IsFilterDimmed returns true if the node is a context ancestor (shown dimmed)
// rather than a direct filter match (bd-05v).
func (t *TreeModel) IsFilterDimmed(node *IssueTreeNode) bool {
	if node == nil || node.Issue == nil || t.filterMatches == nil {
		return false
	}
	id := node.Issue.ID
	return t.contextAncestors[id] && !t.filterMatches[id]
}

// View renders the tree view with a header row and windowed node rendering.
// Implementation for bv-1371, updated for windowed rendering (bv-db02).
// Only renders visible nodes based on viewportOffset and height for O(viewport)
// performance instead of O(n) where n is total nodes.
// The header row is included in the output and accounted for in the visible range.
func (t *TreeModel) View() string {
	if !t.built || len(t.flatList) == 0 {
		return t.renderEmptyState()
	}

	var sb strings.Builder

	// Prepend the column header row (bd-0ex, bd-s2k)
	sb.WriteString(t.RenderHeader())
	// Show [FOLLOW] badge when follow mode is active (bd-c0c)
	if t.followMode {
		followBadge := t.theme.Renderer.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
			Background(lipgloss.AdaptiveColor{Light: "#27AE60", Dark: "#50FA7B"}).
			Bold(true).
			Render(" FOLLOW ")
		sb.WriteString(" ")
		sb.WriteString(followBadge)
	}
	sb.WriteString("\n")

	// Show XRay indicator when in drill-down mode (bd-0rc)
	if t.xrayRoot != nil && t.xrayRoot.Issue != nil {
		xrayStyle := t.theme.Renderer.NewStyle().
			Foreground(t.theme.Highlight).
			Bold(true)
		sb.WriteString(xrayStyle.Render(fmt.Sprintf("[XRAY: %s]", t.xrayRoot.Issue.Title)))
		sb.WriteString("\n")
	}

	// Render sticky scroll lines if parent is off-screen (bd-2z9)
	stickyLines := t.StickyScrollLines()
	gutterPad := strings.Repeat(" ", selectionGutterWidth)
	for _, stickyLine := range stickyLines {
		sb.WriteString(gutterPad)
		sb.WriteString(stickyLine)
		sb.WriteString("\n")
	}

	// Get visible range - O(1) calculation based on viewportOffset and height
	start, end := t.visibleRange()

	// Compute max short ID width across visible nodes for column alignment (bd-uyzc)
	maxIDWidth := 0
	for i := start; i < end; i++ {
		node := t.flatList[i]
		if node == nil || node.Issue == nil {
			continue
		}
		w := lipgloss.Width(shortIDSuffix(node.Issue.ID))
		if w > maxIDWidth {
			maxIDWidth = w
		}
	}

	// Render only visible nodes (bv-db02: windowed rendering)
	for i := start; i < end; i++ {
		node := t.flatList[i]
		if node == nil || node.Issue == nil {
			continue
		}

		isSelected := i == t.cursor
		line := t.renderNode(node, isSelected, maxIDWidth)

		if isSelected {
			// Highlight selected row — the Selected style's left border+padding
			// provides the gutter naturally (bd-6yz)
			line = t.theme.Selected.Render(line)
		} else {
			// Non-selected rows get blank gutter padding to keep tree
			// connectors aligned with the selected row (bd-6yz)
			line = strings.Repeat(" ", selectionGutterWidth) + line
			if t.IsFilterDimmed(node) {
				// Context ancestors shown with muted/faint styling (bd-05v)
				dimStyle := t.theme.Renderer.NewStyle().Foreground(t.theme.Muted).Faint(true)
				line = dimStyle.Render(line)
			}
		}

		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Add position indicator if scrolling is needed (bv-2nax)
	// Only shows when there are more nodes than fit in the viewport
	if len(t.flatList) > t.height && t.height > 0 {
		indicator := t.renderPositionIndicator(start, end)
		sb.WriteString(indicator)
	}

	// Show search bar when search mode is active (bd-wf8)
	if t.searchMode {
		sb.WriteString("\n")
		sb.WriteString(t.renderSearchBar())
	}

	// Overlay sort popup on top of the last rows when open (bd-u81)
	if t.sortPopupOpen {
		output := sb.String()
		popupContent := t.RenderSortPopup()
		if popupContent != "" {
			// Split output into lines, replace last N lines with popup
			lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
			popupLines := strings.Split(strings.TrimRight(popupContent, "\n"), "\n")
			// Replace from the bottom
			replaceStart := len(lines) - len(popupLines)
			if replaceStart < 1 { // keep at least the header
				replaceStart = 1
			}
			lines = lines[:replaceStart]
			lines = append(lines, popupLines...)
			return strings.Join(lines, "\n") + "\n"
		}
	}

	return sb.String()
}

// renderPositionIndicator renders the scroll position indicator (bv-2nax).
// Shows page-based format "Page X/Y (start-end of total)" matching list view.
// Uses 1-indexed numbers for user-friendly display.
func (t *TreeModel) renderPositionIndicator(start, end int) string {
	total := len(t.flatList)
	// Convert to 1-indexed for display
	displayStart := start + 1
	displayEnd := end

	pageSize := t.effectiveVisibleCount()
	currentPage, totalPages := t.pageInfo(pageSize)

	indicator := fmt.Sprintf(" Page %d/%d (%d-%d of %d)", currentPage, totalPages, displayStart, displayEnd, total)
	return t.theme.Renderer.NewStyle().
		Foreground(t.theme.Muted).
		Render(indicator)
}

// pageInfo returns the current page number and total pages based on visible count.
func (t *TreeModel) pageInfo(pageSize int) (currentPage, totalPages int) {
	total := len(t.flatList)
	if pageSize <= 0 {
		pageSize = 1
	}
	totalPages = (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	currentPage = (t.viewportOffset / pageSize) + 1
	if currentPage > totalPages {
		currentPage = totalPages
	}
	return currentPage, totalPages
}

// PageForwardFull moves the viewport forward by a full page of items.
func (t *TreeModel) PageForwardFull() {
	pageSize := t.effectiveVisibleCount()
	if pageSize < 1 {
		pageSize = 1
	}
	total := len(t.flatList)
	t.cursor += pageSize
	t.viewportOffset += pageSize
	if t.cursor >= total {
		t.cursor = total - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	// Clamp viewport offset
	maxOffset := total - pageSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.viewportOffset > maxOffset {
		t.viewportOffset = maxOffset
	}
	t.ensureCursorVisible()
}

// PageBackwardFull moves the viewport backward by a full page of items.
func (t *TreeModel) PageBackwardFull() {
	pageSize := t.effectiveVisibleCount()
	if pageSize < 1 {
		pageSize = 1
	}
	t.cursor -= pageSize
	t.viewportOffset -= pageSize
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.viewportOffset < 0 {
		t.viewportOffset = 0
	}
	t.ensureCursorVisible()
}

// renderEmptyState renders the view when there are no issues.
func (t *TreeModel) renderEmptyState() string {
	r := t.theme.Renderer

	titleStyle := r.NewStyle().
		Foreground(t.theme.Primary).
		Bold(true)

	mutedStyle := r.NewStyle().
		Foreground(t.theme.Muted)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Tree View"))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render("No issues to display."))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render("To create hierarchy, add parent-child dependencies:"))
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render("  br dep add <child> parent-child:<parent>"))
	sb.WriteString("\n\n")
	sb.WriteString(mutedStyle.Render("Press E to return to list view."))

	return sb.String()
}

// RenderHeader returns a styled header row for the tree view, with column
// labels aligned to match the row content below (bd-xhyo).
// Row layout: [gutter 2] [expand 1] [space 1] [icon 1] [space 1] [status 4] [space 1] [title...] ... [age 8] [space 1] [ID maxW]
func (t *TreeModel) RenderHeader() string {
	width := t.width
	if width <= 0 {
		width = 80
	}
	headerStyle := t.theme.Renderer.NewStyle().
		Background(t.theme.Primary).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Bold(true).
		Width(width)

	// Mode/filter badges go before the column labels
	modeBadge := ""
	if t.flatMode {
		modeBadge = "[FLAT] "
	}
	if t.occurMode {
		modeBadge = fmt.Sprintf("[OCCUR[%s](%d)] ", t.occurPattern, len(t.flatList))
	}
	filterBadge := ""
	if t.currentFilter != "" && t.currentFilter != "all" {
		filterBadge = fmt.Sprintf("[%s] ", strings.ToUpper(t.currentFilter))
	}

	// Build left side to match row column positions:
	// gutter(2) + expand(1) + space(1) + icon(1) + space(1) = 6 chars before STATUS
	leftPrefix := "  " + modeBadge + filterBadge
	leftPrefixWidth := lipgloss.Width(leftPrefix)

	// Pad to align STATUS with the status badge column (position 6 from content start)
	statusCol := selectionGutterWidth + 1 + 1 + 1 + 1 // gutter + expand + space + icon + space
	if leftPrefixWidth < statusCol {
		leftPrefix += strings.Repeat(" ", statusCol-leftPrefixWidth)
	}

	// Right side: [sort label] padded to age column width, then ID label right-aligned
	sortBadge := fmt.Sprintf("[%s %s]", t.sortField.String(), t.sortDirection.Indicator())

	// Compute maxIDWidth from visible nodes (same as View does)
	maxIDWidth := 2 // minimum "ID" label width
	start, end := t.visibleRange()
	for i := start; i < end; i++ {
		node := t.flatList[i]
		if node == nil || node.Issue == nil {
			continue
		}
		w := lipgloss.Width(shortIDSuffix(node.Issue.ID))
		if w > maxIDWidth {
			maxIDWidth = w
		}
	}

	// Right side matches row: age(8) + space(1) + ID(maxIDWidth)
	rightID := fmt.Sprintf("%*s", maxIDWidth, "ID")
	rightSide := fmt.Sprintf("%8s %s", sortBadge, rightID)
	rightWidth := lipgloss.Width(rightSide)

	// TITLE fills the space between STATUS and right side
	statusLabel := "STATUS"
	titleStart := lipgloss.Width(leftPrefix) + len(statusLabel) + 1 // +1 for space after STATUS
	titleWidth := width - titleStart - rightWidth - 1               // -1 for trailing padding
	if titleWidth < 5 {
		titleWidth = 5
	}
	titleLabel := "TITLE" + strings.Repeat(" ", titleWidth-5)

	headerText := leftPrefix + statusLabel + " " + titleLabel + " " + rightSide
	return headerStyle.Render(headerText)
}

// renderNode renders a single tree node with column-aligned layout matching the
// main list delegate: [tree-prefix] [expand] [type] [prio-badge] [status-badge] [ID] [title] [age]
func (t *TreeModel) renderNode(node *IssueTreeNode, isSelected bool, maxIDWidth int) string {
	if node == nil || node.Issue == nil {
		return ""
	}

	issue := node.Issue
	r := t.theme.Renderer
	width := t.width
	if width <= 0 {
		width = 80
	}
	// Reduce width by 1 to prevent terminal wrapping on the exact edge,
	// and by selectionGutterWidth so all rows (selected and non-selected)
	// render at the same content width — the gutter is filled by the
	// Selected style's border+padding or by blank padding (bd-6yz).
	width = width - 1 - selectionGutterWidth

	var leftSide strings.Builder

	// ── Mark indicator (bd-cz0) ──
	markWidth := 0
	if t.IsMarked(issue.ID) {
		markStyle := r.NewStyle().Foreground(t.theme.Highlight).Bold(true)
		leftSide.WriteString(markStyle.Render("●"))
		markWidth = 1
	}

	// ── Tree prefix (indentation + branch characters) ──
	prefix := t.buildTreePrefix(node)
	leftSide.WriteString(prefix)
	prefixWidth := lipgloss.Width(prefix) + markWidth

	// ── Expand/collapse indicator ──
	indicator := t.getExpandIndicator(node)
	indicatorStyle := r.NewStyle().Foreground(t.theme.Secondary)
	leftSide.WriteString(indicatorStyle.Render(indicator))
	leftSide.WriteString(" ")

	// ── Type icon (JIRA-style, bd-arxp) ──
	typeIcon, typeIconColor := t.theme.GetTypeIcon(string(issue.IssueType))
	typeIconStyle := r.NewStyle().Foreground(typeIconColor)
	leftSide.WriteString(typeIconStyle.Render(typeIcon))
	leftSide.WriteString(" ")
	typeIconWidth := lipgloss.Width(typeIcon) + 1

	// ── Status badge (polished, matching delegate) ──
	statusBadge := RenderStatusBadge(string(issue.Status))
	statusBadgeWidth := lipgloss.Width(statusBadge)
	leftSide.WriteString(statusBadge)
	leftSide.WriteString(" ")

	// ── Calculate fixed widths (ID moved to right side, bd-03l) ──
	// prefix + indicator(1) + space(1) + typeIcon + status(measured) + space(1)
	fixedWidth := prefixWidth + 1 + 1 + typeIconWidth + statusBadgeWidth + 1

	// ── Right side: age + short ID (bd-03l) ──
	rightWidth := 0
	var rightParts []string

	if width > 60 {
		ageStr := FormatTimeRel(issue.CreatedAt)
		rightParts = append(rightParts, t.theme.MutedText.Render(fmt.Sprintf("%8s", ageStr)))
		rightWidth += 9
	}

	// Short ID suffix at the far right, right-aligned to maxIDWidth for column alignment (bd-03l, bd-uyzc)
	shortID := shortIDSuffix(issue.ID)
	paddedID := fmt.Sprintf("%*s", maxIDWidth, shortID)
	rightParts = append(rightParts, t.theme.SecondaryText.Render(paddedID))
	rightWidth += maxIDWidth + 1

	// ── Bookmark indicator (bd-k4n) ──
	isBookmarked := t.bookmarks[issue.ID]
	if isBookmarked {
		bookmarkStyle := r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#F1FA8C"})
		leftSide.WriteString(bookmarkStyle.Render("\u2605"))
		leftSide.WriteString(" ")
		fixedWidth += 2 // account for the star + space in width calculation
	}

	// ── Title (fills remaining space) ──
	titleWidth := width - fixedWidth - rightWidth - 2
	if titleWidth < 5 {
		titleWidth = 5
	}

	title := truncateRunesHelper(issue.Title, titleWidth, "…")
	// Pad title to fill space
	currentTitleWidth := lipgloss.Width(title)
	if currentTitleWidth < titleWidth {
		title = title + strings.Repeat(" ", titleWidth-currentTitleWidth)
	}

	// ── Search match highlighting (bd-nkt) ──
	isSearchMatch := t.searchMatchIDs != nil && t.searchMatchIDs[node.Issue.ID]
	isCurrentMatch := isSearchMatch && len(t.searchMatches) > 0 &&
		t.searchMatchIndex < len(t.searchMatches) &&
		t.searchMatches[t.searchMatchIndex] == node

	titleStyle := r.NewStyle()
	if isSelected {
		titleStyle = titleStyle.Foreground(t.theme.Primary).Bold(true)
	} else if isCurrentMatch {
		// Current search match: bright yellow foreground + bold
		titleStyle = titleStyle.
			Foreground(lipgloss.AdaptiveColor{Light: "#7A5600", Dark: "#F1FA8C"}).
			Bold(true)
	} else if isSearchMatch {
		// Other search matches: orange foreground
		titleStyle = titleStyle.
			Foreground(lipgloss.AdaptiveColor{Light: "#B06800", Dark: "#FFB86C"})
	} else {
		titleStyle = titleStyle.Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#E8E8E8"})
	}
	leftSide.WriteString(titleStyle.Render(title))

	// ── Right side ──
	rightSide := strings.Join(rightParts, " ")

	// ── Combine: left + padding + right ──
	leftLen := lipgloss.Width(leftSide.String())
	rightLen := lipgloss.Width(rightSide)
	padding := width - leftLen - rightLen
	if padding < 0 {
		padding = 0
	}

	row := leftSide.String() + strings.Repeat(" ", padding) + rightSide

	// Apply row width clamping
	rowStyle := r.NewStyle().Width(width).MaxWidth(width)
	row = rowStyle.Render(row)

	return row
}

// buildTreePrefix builds the indentation and branch characters for a node.
func (t *TreeModel) buildTreePrefix(node *IssueTreeNode) string {
	if node.Depth == 0 {
		return "" // Root nodes have no prefix
	}

	r := t.theme.Renderer
	treeStyle := r.NewStyle().Foreground(t.theme.Muted)

	var prefixParts []string

	// Walk up the tree to build prefix
	ancestors := t.getAncestors(node)

	// For each ancestor level, determine if we need a vertical line
	for i := 0; i < len(ancestors)-1; i++ {
		ancestor := ancestors[i]
		if t.hasSiblingsBelow(ancestor) {
			prefixParts = append(prefixParts, "│   ")
		} else {
			prefixParts = append(prefixParts, "    ")
		}
	}

	// Add the branch character for this node
	if t.isLastChild(node) {
		prefixParts = append(prefixParts, "└── ")
	} else {
		prefixParts = append(prefixParts, "├── ")
	}

	prefix := strings.Join(prefixParts, "")
	return treeStyle.Render(prefix)
}

// getAncestors returns the ancestors of a node from root to parent, with the node itself at the end.
// The last element is the node - used by buildTreePrefix which iterates to len-1.
func (t *TreeModel) getAncestors(node *IssueTreeNode) []*IssueTreeNode {
	var ancestors []*IssueTreeNode
	current := node.Parent
	for current != nil {
		ancestors = append([]*IssueTreeNode{current}, ancestors...)
		current = current.Parent
	}
	ancestors = append(ancestors, node) // Include the node at the end
	return ancestors
}

// hasSiblingsBelow checks if a node has siblings below it in the tree.
func (t *TreeModel) hasSiblingsBelow(node *IssueTreeNode) bool {
	if node.Parent == nil {
		// For root nodes, check if there are more roots after this one
		for i, root := range t.roots {
			if root == node {
				return i < len(t.roots)-1
			}
		}
		return false
	}

	// For non-root nodes, check siblings
	for i, sibling := range node.Parent.Children {
		if sibling == node {
			return i < len(node.Parent.Children)-1
		}
	}
	return false
}

// isLastChild checks if a node is the last child of its parent.
func (t *TreeModel) isLastChild(node *IssueTreeNode) bool {
	if node.Parent == nil {
		// For root nodes, check if it's the last root
		return len(t.roots) > 0 && t.roots[len(t.roots)-1] == node
	}

	parent := node.Parent
	return len(parent.Children) > 0 && parent.Children[len(parent.Children)-1] == node
}

// getExpandIndicator returns the expand/collapse indicator for a node.
func (t *TreeModel) getExpandIndicator(node *IssueTreeNode) string {
	if len(node.Children) == 0 {
		return " " // Leaf node: no indicator
	}
	if node.Expanded {
		return "▾" // Expanded
	}
	return "▸" // Collapsed
}

// truncateTitle truncates a title to the given max length with ellipsis.
func (t *TreeModel) truncateTitle(title string, maxLen int) string {
	if maxLen <= 3 {
		return "..."
	}

	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}

	return string(runes[:maxLen-1]) + "…"
}

// GetPriorityColor returns the color for a priority level.
func (t *TreeModel) GetPriorityColor(priority int) lipgloss.AdaptiveColor {
	switch priority {
	case 0:
		return t.theme.Primary // Critical - red/bright
	case 1:
		return t.theme.Highlight // High - highlighted
	case 2:
		return t.theme.Secondary // Medium - yellow
	default:
		return t.theme.Muted // Low/backlog - gray
	}
}

// SelectedIssue returns the currently selected issue, or nil if none.
func (t *TreeModel) SelectedIssue() *model.Issue {
	if t.cursor >= 0 && t.cursor < len(t.flatList) {
		if node := t.flatList[t.cursor]; node != nil {
			return node.Issue
		}
	}
	return nil
}

// SelectedNode returns the currently selected tree node, or nil if none.
func (t *TreeModel) SelectedNode() *IssueTreeNode {
	if t.cursor >= 0 && t.cursor < len(t.flatList) {
		return t.flatList[t.cursor]
	}
	return nil
}

// MoveDown moves the cursor down in the flat list.
func (t *TreeModel) MoveDown() {
	if t.cursor < len(t.flatList)-1 {
		t.cursor++
		t.ensureCursorVisible()
	}
}

// MoveUp moves the cursor up in the flat list.
func (t *TreeModel) MoveUp() {
	if t.cursor > 0 {
		t.cursor--
		t.ensureCursorVisible()
	}
}

// ToggleExpand expands or collapses the currently selected node.
func (t *TreeModel) ToggleExpand() {
	node := t.SelectedNode()
	if node != nil && len(node.Children) > 0 {
		node.Expanded = !node.Expanded
		t.rebuildFlatList()
		t.saveState() // Persist expand/collapse state (bv-19vz)
		t.ensureCursorVisible()
	}
}

// ExpandAll expands all nodes in the tree.
func (t *TreeModel) ExpandAll() {
	for _, root := range t.roots {
		t.setExpandedRecursive(root, true)
	}
	t.rebuildFlatList()
	t.saveState() // Persist expand/collapse state (bv-19vz)
	t.ensureCursorVisible()
}

// ToggleExpandCollapseAll toggles between expand-all and collapse-all.
// If any expandable node is collapsed, expands all. Otherwise collapses all.
func (t *TreeModel) ToggleExpandCollapseAll() {
	if t.hasAnyCollapsed() {
		t.ExpandAll()
	} else {
		t.CollapseAll()
	}
}

// hasAnyCollapsed returns true if any node with children is collapsed.
func (t *TreeModel) hasAnyCollapsed() bool {
	for _, root := range t.roots {
		if t.nodeHasCollapsed(root) {
			return true
		}
	}
	return false
}

// nodeHasCollapsed recursively checks if a node or its descendants are collapsed.
func (t *TreeModel) nodeHasCollapsed(node *IssueTreeNode) bool {
	if len(node.Children) > 0 && !node.Expanded {
		return true
	}
	for _, child := range node.Children {
		if t.nodeHasCollapsed(child) {
			return true
		}
	}
	return false
}

// CollapseAll collapses all nodes in the tree.
func (t *TreeModel) CollapseAll() {
	for _, root := range t.roots {
		t.setExpandedRecursive(root, false)
	}
	t.rebuildFlatList()
	t.saveState() // Persist expand/collapse state (bv-19vz)
	t.ensureCursorVisible()
}

// JumpToTop moves cursor to the first node.
func (t *TreeModel) JumpToTop() {
	t.cursor = 0
	t.ensureCursorVisible()
}

// JumpToBottom moves cursor to the last node.
func (t *TreeModel) JumpToBottom() {
	if len(t.flatList) > 0 {
		t.cursor = len(t.flatList) - 1
		t.ensureCursorVisible()
	}
}

// JumpToParent moves cursor to the parent of the currently selected node.
// If already at a root node, does nothing.
func (t *TreeModel) JumpToParent() {
	node := t.SelectedNode()
	if node == nil || node.Parent == nil {
		return // No node selected or already at root
	}

	// Find parent in flatList
	for i, n := range t.flatList {
		if n == node.Parent {
			t.cursor = i
			t.ensureCursorVisible()
			return
		}
	}
}

// getSiblings returns the sibling slice for a node (parent's children or roots).
func (t *TreeModel) getSiblings(node *IssueTreeNode) []*IssueTreeNode {
	if node == nil {
		return nil
	}
	if node.Parent == nil {
		return t.roots
	}
	return node.Parent.Children
}

// NextSibling moves cursor to the next sibling at the same depth (bd-ryu).
// If already at the last sibling, does nothing.
func (t *TreeModel) NextSibling() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	siblings := t.getSiblings(node)
	for i, s := range siblings {
		if s == node && i < len(siblings)-1 {
			next := siblings[i+1]
			// Find next sibling in flatList
			for j, n := range t.flatList {
				if n == next {
					t.cursor = j
					t.ensureCursorVisible()
					return
				}
			}
			return
		}
	}
}

// PrevSibling moves cursor to the previous sibling at the same depth (bd-ryu).
// If already at the first sibling, does nothing.
func (t *TreeModel) PrevSibling() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	siblings := t.getSiblings(node)
	for i, s := range siblings {
		if s == node && i > 0 {
			prev := siblings[i-1]
			// Find prev sibling in flatList
			for j, n := range t.flatList {
				if n == prev {
					t.cursor = j
					t.ensureCursorVisible()
					return
				}
			}
			return
		}
	}
}

// FirstSibling moves cursor to the first sibling at the same depth (bd-ryu).
func (t *TreeModel) FirstSibling() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	siblings := t.getSiblings(node)
	if len(siblings) == 0 {
		return
	}
	first := siblings[0]
	for j, n := range t.flatList {
		if n == first {
			t.cursor = j
			t.ensureCursorVisible()
			return
		}
	}
}

// LastSibling moves cursor to the last sibling at the same depth (bd-ryu).
func (t *TreeModel) LastSibling() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	siblings := t.getSiblings(node)
	if len(siblings) == 0 {
		return
	}
	last := siblings[len(siblings)-1]
	for j, n := range t.flatList {
		if n == last {
			t.cursor = j
			t.ensureCursorVisible()
			return
		}
	}
}

// CycleNodeVisibility implements TAB cycling on the current node (bd-8of, bd-g4w7).
// Cycle: folded -> children visible -> folded (2-state toggle).
// On a leaf node, does nothing.
func (t *TreeModel) CycleNodeVisibility() {
	node := t.SelectedNode()
	if node == nil || node.Issue == nil || len(node.Children) == 0 {
		return // Leaf or no node
	}

	if t.cycleStates == nil {
		t.cycleStates = make(map[string]int)
	}

	id := node.Issue.ID

	// Detect current state if not explicitly set
	state, explicit := t.cycleStates[id]
	if !explicit {
		state = t.detectNodeCycleState(node)
		// Map subtree-expanded (2) to children-visible (1) for 2-state cycle
		if state == 2 {
			state = 1
		}
	}

	// Advance to next state (2-state: 0 <-> 1)
	switch state {
	case 0: // folded -> children visible
		node.Expanded = true
		// Collapse all children (show only direct children)
		for _, child := range node.Children {
			t.setExpandedRecursive(child, false)
		}
		t.cycleStates[id] = 1
	default: // children visible -> folded
		node.Expanded = false
		t.setExpandedRecursive(node, false)
		t.cycleStates[id] = 0
	}

	t.rebuildFlatList()
	t.saveState()
	t.ensureCursorVisible()
}

// detectNodeCycleState determines the current visibility state of a node.
// Returns 0 (folded), 1 (children visible), or 2 (subtree visible).
func (t *TreeModel) detectNodeCycleState(node *IssueTreeNode) int {
	if !node.Expanded {
		return 0 // folded
	}
	// Node is expanded - check if all descendants with children are also expanded
	if t.allDescendantsExpanded(node) {
		return 2 // subtree fully visible
	}
	return 1 // children visible but subtree not fully expanded
}

// allDescendantsExpanded checks if all descendant nodes with children are expanded.
func (t *TreeModel) allDescendantsExpanded(node *IssueTreeNode) bool {
	for _, child := range node.Children {
		if len(child.Children) > 0 {
			if !child.Expanded {
				return false
			}
			if !t.allDescendantsExpanded(child) {
				return false
			}
		}
	}
	return true
}

// CycleGlobalVisibility implements Shift+TAB global visibility cycling (bd-8of).
// Cycle: all-folded -> top-level children visible -> all-expanded -> all-folded
func (t *TreeModel) CycleGlobalVisibility() {
	// Advance global state
	switch t.globalCycleState {
	case 0: // -> all folded
		for _, root := range t.roots {
			t.setExpandedRecursive(root, false)
		}
		t.globalCycleState = 1
	case 1: // -> top-level children visible (expand roots only)
		for _, root := range t.roots {
			root.Expanded = true
			for _, child := range root.Children {
				t.setExpandedRecursive(child, false)
			}
		}
		t.globalCycleState = 2
	case 2: // -> all expanded
		for _, root := range t.roots {
			t.setExpandedRecursive(root, true)
		}
		t.globalCycleState = 0
	}

	// Clear per-node cycle states since global cycling overrides them
	t.cycleStates = nil

	t.rebuildFlatList()
	t.saveState()
	t.ensureCursorVisible()
}

// ExpandToLevel expands the tree to show nodes at depths 0..level-1 (bd-9jr).
// Pressing '1' shows only roots, '2' shows roots+children, etc.
// Preserves cursor position (moves to nearest visible ancestor if needed).
func (t *TreeModel) ExpandToLevel(level int) {
	// Remember selected ID for cursor preservation
	selectedID := t.GetSelectedID()

	var setLevel func(node *IssueTreeNode)
	setLevel = func(node *IssueTreeNode) {
		if node == nil {
			return
		}
		if len(node.Children) > 0 {
			node.Expanded = node.Depth < level-1
		}
		for _, child := range node.Children {
			setLevel(child)
		}
	}

	for _, root := range t.roots {
		setLevel(root)
	}

	// Clear cycle states since level-based expand overrides them
	t.cycleStates = nil

	t.rebuildFlatList()
	t.saveState()

	// Restore cursor to same node if still visible, otherwise keep current cursor
	if selectedID != "" {
		t.SelectByID(selectedID)
	}
	t.ensureCursorVisible()
}

// ExpandOrMoveToChild handles the → / l key:
// - If node has children and is collapsed: expand it
// - If node has children and is expanded: move to first child
// - If node is a leaf: do nothing
func (t *TreeModel) ExpandOrMoveToChild() {
	node := t.SelectedNode()
	if node == nil || len(node.Children) == 0 {
		return // No node selected or leaf node
	}

	if !node.Expanded {
		// Expand the node
		node.Expanded = true
		t.rebuildFlatList()
		t.saveState() // Persist expand/collapse state (bv-19vz)
		t.ensureCursorVisible()
	} else {
		// Move to first child
		// Find first child in flatList (should be right after current node)
		for i, n := range t.flatList {
			if n == node.Children[0] {
				t.cursor = i
				t.ensureCursorVisible()
				return
			}
		}
	}
}

// CollapseOrJumpToParent handles the ← / h key:
// - If node has children and is expanded: collapse it
// - If node is collapsed or is a leaf: jump to parent
func (t *TreeModel) CollapseOrJumpToParent() {
	node := t.SelectedNode()
	if node == nil {
		return
	}

	if len(node.Children) > 0 && node.Expanded {
		// Collapse the node
		node.Expanded = false
		t.rebuildFlatList()
		t.saveState() // Persist expand/collapse state (bv-19vz)
		t.ensureCursorVisible()
	} else {
		// Jump to parent (already calls ensureCursorVisible)
		t.JumpToParent()
	}
}

// PageDown moves cursor down by half a viewport.
func (t *TreeModel) PageDown() {
	pageSize := t.height / 2
	if pageSize < 1 {
		pageSize = 5
	}
	t.cursor += pageSize
	if t.cursor >= len(t.flatList) {
		t.cursor = len(t.flatList) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.ensureCursorVisible()
}

// PageUp moves cursor up by half a viewport.
func (t *TreeModel) PageUp() {
	pageSize := t.height / 2
	if pageSize < 1 {
		pageSize = 5
	}
	t.cursor -= pageSize
	if t.cursor < 0 {
		t.cursor = 0
	}
	t.ensureCursorVisible()
}

// visibleRange returns the start and end indices of nodes to render (bv-r4ng).
// The range [start, end) covers nodes visible in the viewport.
// This is an O(1) calculation based on viewportOffset and height.
// Uses effectiveVisibleCount() which reserves space for the header row (bd-s2k)
// and position indicator when scrolling is needed.
func (t *TreeModel) visibleRange() (start, end int) {
	if len(t.flatList) == 0 {
		return 0, 0
	}

	visibleCount := t.effectiveVisibleCount()

	// Start with the viewport offset, clamped to non-negative
	start = t.viewportOffset
	if start < 0 {
		start = 0
	}

	// Calculate end based on clamped start
	end = start + visibleCount

	// If end exceeds list, clamp it and adjust start to maximize visible items
	if end > len(t.flatList) {
		end = len(t.flatList)
		start = end - visibleCount
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

// SelectByID moves cursor to the node with the given issue ID.
// Returns true if found, false otherwise.
// Useful for preserving cursor position after rebuild.
func (t *TreeModel) SelectByID(id string) bool {
	for i, node := range t.flatList {
		if node != nil && node.Issue != nil && node.Issue.ID == id {
			t.cursor = i
			return true
		}
	}
	return false
}

// GetSelectedID returns the ID of the currently selected issue, or empty string.
func (t *TreeModel) GetSelectedID() string {
	if issue := t.SelectedIssue(); issue != nil {
		return issue.ID
	}
	return ""
}

// setExpandedRecursive sets the expanded state for a node and all descendants.
func (t *TreeModel) setExpandedRecursive(node *IssueTreeNode, expanded bool) {
	if node == nil {
		return
	}
	node.Expanded = expanded
	for _, child := range node.Children {
		t.setExpandedRecursive(child, expanded)
	}
}

// rebuildFlatList rebuilds the flattened list of visible nodes.
// When flat mode is active, shows all issues without hierarchy (bd-39v).
// When a filter is active, dispatches to rebuildFilteredFlatList (bd-e3w).
// When XRay mode is active (bd-0rc), only shows the xrayRoot subtree.
func (t *TreeModel) rebuildFlatList() {
	if t.flatMode {
		t.rebuildFlatModeList()
		return
	}
	if t.currentFilter != "" && t.currentFilter != "all" && t.filterMatches != nil {
		t.rebuildFilteredFlatList()
		return
	}
	// Occur mode: filter to matching issues (bd-sjs.2)
	if t.occurMode && t.occurPattern != "" {
		t.flatList = t.flatList[:0]
		if t.xrayRoot != nil {
			t.appendVisible(t.xrayRoot)
		} else {
			for _, root := range t.roots {
				t.appendVisible(root)
			}
		}
		t.rebuildOccurFlatList()
		return
	}
	t.flatList = t.flatList[:0]
	if t.xrayRoot != nil {
		// XRay mode: only show the subtree rooted at xrayRoot (bd-0rc)
		t.appendVisible(t.xrayRoot)
	} else {
		for _, root := range t.roots {
			t.appendVisible(root)
		}
	}
	// Ensure cursor stays in bounds
	if t.cursor >= len(t.flatList) {
		t.cursor = len(t.flatList) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

// rebuildFlatModeList builds the flat list showing all issues without hierarchy (bd-39v).
// Respects current filter settings.
func (t *TreeModel) rebuildFlatModeList() {
	nodes := t.buildFlatNodes()

	// Apply filter if active
	if t.currentFilter != "" && t.currentFilter != "all" && t.filterMatches != nil {
		var filtered []*IssueTreeNode
		for _, node := range nodes {
			if node.Issue != nil && t.filterMatches[node.Issue.ID] {
				filtered = append(filtered, node)
			}
		}
		nodes = filtered
	}

	t.flatList = nodes

	// Ensure cursor stays in bounds
	if t.cursor >= len(t.flatList) {
		t.cursor = len(t.flatList) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

// appendVisible adds a node and its visible descendants to flatList.
func (t *TreeModel) appendVisible(node *IssueTreeNode) {
	if node == nil {
		return
	}
	t.flatList = append(t.flatList, node)
	if node.Expanded {
		for _, child := range node.Children {
			t.appendVisible(child)
		}
	}
}

// rebuildFilteredFlatList builds the flat list showing only matching nodes and
// their context ancestors (bd-e3w).
func (t *TreeModel) rebuildFilteredFlatList() {
	t.flatList = t.flatList[:0]
	for _, root := range t.roots {
		t.appendFilteredVisible(root)
	}
	if t.cursor >= len(t.flatList) {
		t.cursor = len(t.flatList) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

// appendFilteredVisible adds a node to the flat list only if it matches the
// filter or is a context ancestor of a matching node. Context ancestors are
// traversed even if not explicitly expanded to ensure matching descendants
// remain visible (bd-e3w).
func (t *TreeModel) appendFilteredVisible(node *IssueTreeNode) {
	if node == nil || node.Issue == nil {
		return
	}
	id := node.Issue.ID
	isMatch := t.filterMatches[id]
	isContext := t.contextAncestors[id]

	if !isMatch && !isContext {
		return
	}

	t.flatList = append(t.flatList, node)

	// Context ancestors show their children even if not explicitly expanded
	// to ensure matching descendants are visible
	if node.Expanded || isContext {
		for _, child := range node.Children {
			t.appendFilteredVisible(child)
		}
	}
}

// IsBuilt returns whether the tree has been built.
func (t *TreeModel) IsBuilt() bool {
	return t.built
}

// NodeCount returns the total number of visible nodes.
func (t *TreeModel) NodeCount() int {
	return len(t.flatList)
}

// RootCount returns the number of root nodes.
func (t *TreeModel) RootCount() int {
	return len(t.roots)
}

// effectiveVisibleCount returns the number of node lines that can be displayed,
// accounting for the header row and position indicator. This keeps
// ensureCursorVisible and visibleRange in sync (bd-s2k).
func (t *TreeModel) effectiveVisibleCount() int {
	visibleCount := t.height - 1 // subtract 1 for header row
	if visibleCount <= 0 {
		visibleCount = 19 // Default: 20 minus 1 for header
	}
	// Reserve 1 more line for the position indicator when scrolling is needed
	if len(t.flatList) > visibleCount {
		visibleCount--
	}
	if visibleCount < 1 {
		visibleCount = 1
	}
	return visibleCount
}

// ensureCursorVisible adjusts viewportOffset so the cursor is visible (bv-lnc4).
// This method should be called after any cursor movement to maintain
// cursor-follows-viewport behavior. It implements cursor-at-edge scrolling:
// the viewport scrolls just enough to keep the cursor visible.
func (t *TreeModel) ensureCursorVisible() {
	if len(t.flatList) == 0 {
		return
	}

	visibleCount := t.effectiveVisibleCount()

	// Cursor above viewport - scroll up to show cursor at top
	if t.cursor < t.viewportOffset {
		t.viewportOffset = t.cursor
	}

	// Cursor below viewport - scroll down to show cursor at bottom
	if t.cursor >= t.viewportOffset+visibleCount {
		t.viewportOffset = t.cursor - visibleCount + 1
	}

	// Clamp offset to valid range
	maxOffset := len(t.flatList) - visibleCount
	if maxOffset < 0 {
		maxOffset = 0
	}
	if t.viewportOffset > maxOffset {
		t.viewportOffset = maxOffset
	}
	if t.viewportOffset < 0 {
		t.viewportOffset = 0
	}
}

// GetViewportOffset returns the current viewport offset (for testing/debugging).
func (t *TreeModel) GetViewportOffset() int {
	return t.viewportOffset
}

// ── Flat mode methods (bd-39v) ──

// IsFlatMode returns whether flat mode is active.
func (t *TreeModel) IsFlatMode() bool {
	return t.flatMode
}

// ToggleFlatMode toggles between flat-list and tree-hierarchy view.
// In flat mode, all issues are shown at depth 0 without parent-child nesting,
// preserving the current sort and filter settings.
func (t *TreeModel) ToggleFlatMode() {
	t.flatMode = !t.flatMode
	t.rebuildFlatList()
	t.ensureCursorVisible()
}

// buildFlatNodes returns all issues from the tree as flat (depth-0) nodes,
// sorted using the current sort mode. This is used when flat mode is active.
func (t *TreeModel) buildFlatNodes() []*IssueTreeNode {
	var nodes []*IssueTreeNode
	// Collect all nodes from the issue map
	for _, node := range t.issueMap {
		if node == nil || node.Issue == nil {
			continue
		}
		// Create a shallow copy with Depth=0 for flat display
		flatNode := &IssueTreeNode{
			Issue:    node.Issue,
			Children: nil, // No children in flat mode
			Expanded: false,
			Depth:    0,
			Parent:   nil, // No parent in flat mode
		}
		nodes = append(nodes, flatNode)
	}

	// Sort using current sort field/direction
	t.sortNodesByFieldDirection(nodes)
	return nodes
}

// ── Search methods (bd-uus) ──

// EnterSearchMode activates the search input bar.
func (t *TreeModel) EnterSearchMode() {
	t.searchMode = true
	t.searchQuery = ""
	t.searchMatches = nil
	t.searchMatchIDs = nil
	t.searchMatchIndex = 0
}

// ExitSearchMode deactivates the search input bar but keeps matches highlighted.
func (t *TreeModel) ExitSearchMode() {
	t.searchMode = false
}

// ClearSearch deactivates search mode and removes all match state.
func (t *TreeModel) ClearSearch() {
	t.searchMode = false
	t.searchQuery = ""
	t.searchMatches = nil
	t.searchMatchIDs = nil
	t.searchMatchIndex = 0
}

// IsSearchMode returns whether the search input bar is active.
func (t *TreeModel) IsSearchMode() bool { return t.searchMode }

// SearchQuery returns the current search query string.
func (t *TreeModel) SearchQuery() string { return t.searchQuery }

// SearchMatchCount returns the number of nodes matching the current search.
func (t *TreeModel) SearchMatchCount() int { return len(t.searchMatches) }

// SearchMatchIndex returns the 0-based index of the currently focused match.
func (t *TreeModel) SearchMatchIndex() int { return t.searchMatchIndex }

// SearchAddChar appends a character to the search query and re-executes the search.
func (t *TreeModel) SearchAddChar(ch rune) {
	t.searchQuery += string(ch)
	t.executeSearch()
}

// SearchBackspace removes the last character from the search query.
// If the query becomes empty, matches are cleared.
func (t *TreeModel) SearchBackspace() {
	if len(t.searchQuery) > 0 {
		runes := []rune(t.searchQuery)
		t.searchQuery = string(runes[:len(runes)-1])
	}
	if t.searchQuery == "" {
		t.searchMatches = nil
		t.searchMatchIDs = nil
		t.searchMatchIndex = 0
		return
	}
	t.executeSearch()
}

// isAdvancedQuery returns true if the query string contains advanced filter syntax (bd-08h).
func isAdvancedQuery(q string) bool {
	return strings.Contains(q, ":") || strings.HasPrefix(q, "!")
}

// executeSearch walks ALL nodes (including collapsed ones) and builds the match list.
// Auto-expands ancestors of the first match and navigates to it.
// If the query contains advanced filter syntax (field:value, !negation), uses
// structured predicate matching instead of plain text (bd-08h).
func (t *TreeModel) executeSearch() {
	t.searchMatches = nil
	t.searchMatchIDs = make(map[string]bool)
	t.searchMatchIndex = 0

	if t.searchQuery == "" {
		return
	}

	// Check if this is an advanced filter query (bd-08h)
	var preds []FilterPredicate
	useAdvanced := isAdvancedQuery(t.searchQuery)
	if useAdvanced {
		preds = ParseFilterPredicates(t.searchQuery)
	}
	query := strings.ToLower(t.searchQuery)

	// Walk ALL nodes (including collapsed ones)
	var walk func(node *IssueTreeNode)
	walk = func(node *IssueTreeNode) {
		if node == nil || node.Issue == nil {
			return
		}

		var matches bool
		if useAdvanced {
			matches = t.nodeMatchesAdvancedFilter(node, preds)
		} else {
			matches = strings.Contains(strings.ToLower(node.Issue.Title), query) ||
				strings.Contains(strings.ToLower(node.Issue.ID), query)
		}

		if matches {
			t.searchMatches = append(t.searchMatches, node)
			t.searchMatchIDs[node.Issue.ID] = true
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range t.roots {
		walk(root)
	}

	// Auto-expand and navigate to first match
	if len(t.searchMatches) > 0 {
		t.expandPathToNode(t.searchMatches[0])
		t.rebuildFlatList()
		t.SelectByID(t.searchMatches[0].Issue.ID)
		t.ensureCursorVisible()
	}
}

// NextSearchMatch cycles forward through search matches (n key).
func (t *TreeModel) NextSearchMatch() {
	if len(t.searchMatches) == 0 {
		return
	}
	t.searchMatchIndex = (t.searchMatchIndex + 1) % len(t.searchMatches)
	match := t.searchMatches[t.searchMatchIndex]
	t.expandPathToNode(match)
	t.rebuildFlatList()
	t.SelectByID(match.Issue.ID)
	t.ensureCursorVisible()
}

// PrevSearchMatch cycles backward through search matches (N key).
func (t *TreeModel) PrevSearchMatch() {
	if len(t.searchMatches) == 0 {
		return
	}
	t.searchMatchIndex--
	if t.searchMatchIndex < 0 {
		t.searchMatchIndex = len(t.searchMatches) - 1
	}
	match := t.searchMatches[t.searchMatchIndex]
	t.expandPathToNode(match)
	t.rebuildFlatList()
	t.SelectByID(match.Issue.ID)
	t.ensureCursorVisible()
}

// expandPathToNode expands all ancestors so the node becomes visible.
func (t *TreeModel) expandPathToNode(node *IssueTreeNode) {
	ancestor := node.Parent
	for ancestor != nil {
		ancestor.Expanded = true
		ancestor = ancestor.Parent
	}
}

// ── Bookmark methods (bd-k4n) ──

// ToggleBookmark toggles the bookmark state of the currently selected node.
// Bookmarks are persisted to tree-state.json.
func (t *TreeModel) ToggleBookmark() {
	issue := t.SelectedIssue()
	if issue == nil {
		return
	}
	if t.bookmarks == nil {
		t.bookmarks = make(map[string]bool)
	}
	if t.bookmarks[issue.ID] {
		delete(t.bookmarks, issue.ID)
	} else {
		t.bookmarks[issue.ID] = true
	}
	t.saveState()
}

// CycleBookmark jumps the cursor to the next bookmarked node in flatList order.
// Wraps around to the first bookmark when reaching the end.
func (t *TreeModel) CycleBookmark() {
	if len(t.bookmarks) == 0 || len(t.flatList) == 0 {
		return
	}

	// Find bookmarked nodes in flatList order, starting after current cursor
	startIdx := t.cursor + 1
	n := len(t.flatList)

	for i := 0; i < n; i++ {
		idx := (startIdx + i) % n
		node := t.flatList[idx]
		if node != nil && node.Issue != nil && t.bookmarks[node.Issue.ID] {
			t.cursor = idx
			t.ensureCursorVisible()
			return
		}
	}
}

// TreeBookmarkedIDs returns the list of bookmarked issue IDs in a stable order.
func (t *TreeModel) TreeBookmarkedIDs() []string {
	if len(t.bookmarks) == 0 {
		return nil
	}
	// Return IDs in flatList order for stability
	var ids []string
	for _, node := range t.flatList {
		if node != nil && node.Issue != nil && t.bookmarks[node.Issue.ID] {
			ids = append(ids, node.Issue.ID)
		}
	}
	// Also include bookmarks for nodes not currently visible (collapsed parents)
	seen := make(map[string]bool)
	for _, id := range ids {
		seen[id] = true
	}
	for id := range t.bookmarks {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// IsBookmarked returns true if the given issue ID is bookmarked.
func (t *TreeModel) IsBookmarked(id string) bool {
	return t.bookmarks[id]
}

// ── Follow mode methods (bd-c0c) ──

// ToggleFollowMode toggles follow mode on/off.
// When follow mode is active, external changes auto-reveal the changed node.
func (t *TreeModel) ToggleFollowMode() {
	t.followMode = !t.followMode
}

// GetFollowMode returns whether follow mode is active.
func (t *TreeModel) GetFollowMode() bool {
	return t.followMode
}

// DetectAndFollowChanges compares the current issues against the last known set.
// If follow mode is active and changes are detected, expands and scrolls to the
// first changed/new node. Returns the ID of the followed node, or "" if none.
func (t *TreeModel) DetectAndFollowChanges(currentIssueIDs []string) string {
	if !t.followMode || len(t.lastIssueIDs) == 0 {
		t.lastIssueIDs = currentIssueIDs
		return ""
	}

	// Build set of old IDs for diff
	oldSet := make(map[string]bool, len(t.lastIssueIDs))
	for _, id := range t.lastIssueIDs {
		oldSet[id] = true
	}

	// Find new IDs
	var changedID string
	for _, id := range currentIssueIDs {
		if !oldSet[id] {
			changedID = id
			break
		}
	}

	t.lastIssueIDs = currentIssueIDs

	if changedID == "" {
		return ""
	}

	// Auto-expand and scroll to the changed node
	if node, ok := t.issueMap[changedID]; ok {
		t.expandPathToNode(node)
		t.rebuildFlatList()
		t.SelectByID(changedID)
		t.ensureCursorVisible()
		return changedID
	}

	return ""
}

// renderSearchBar renders the search input bar shown at the bottom of the tree view.
func (t *TreeModel) renderSearchBar() string {
	r := t.theme.Renderer
	searchStyle := r.NewStyle().
		Foreground(t.theme.Primary).
		Bold(true)

	matchInfo := ""
	if len(t.searchMatches) > 0 {
		matchInfo = fmt.Sprintf(" [%d/%d]", t.searchMatchIndex+1, len(t.searchMatches))
	} else if t.searchQuery != "" {
		matchInfo = " [no matches]"
	}

	return searchStyle.Render(fmt.Sprintf("/%s%s", t.searchQuery, matchInfo))
}

// ── Sticky scroll methods (bd-2z9) ──

// StickyScrollLines returns header lines to pin at the top of the viewport when
// the parent (or grandparent) of the first visible node has scrolled off-screen.
// This provides VS Code-style "sticky scroll" context so the user knows where
// they are in the hierarchy. Returns at most 2 lines. Returns nil in flat mode
// or when no ancestors are off-screen.
func (t *TreeModel) StickyScrollLines() []string {
	if t.flatMode || len(t.flatList) == 0 {
		return nil
	}

	start, _ := t.visibleRange()

	// Get the first visible node
	if start >= len(t.flatList) {
		return nil
	}
	firstVisible := t.flatList[start]
	if firstVisible == nil || firstVisible.Issue == nil {
		return nil
	}

	// Collect ancestors that are off-screen (not in the visible range)
	var offScreenAncestors []*IssueTreeNode
	ancestor := firstVisible.Parent
	for ancestor != nil {
		// Check if ancestor is visible in the current viewport
		isVisible := false
		_, end := t.visibleRange()
		for i := start; i < end; i++ {
			if i < len(t.flatList) && t.flatList[i] == ancestor {
				isVisible = true
				break
			}
		}
		if !isVisible {
			offScreenAncestors = append([]*IssueTreeNode{ancestor}, offScreenAncestors...)
		}
		ancestor = ancestor.Parent
	}

	if len(offScreenAncestors) == 0 {
		return nil
	}

	// Limit to at most 2 lines
	if len(offScreenAncestors) > 2 {
		// Show the two closest ancestors (parent and grandparent of first visible)
		offScreenAncestors = offScreenAncestors[len(offScreenAncestors)-2:]
	}

	var lines []string
	for _, node := range offScreenAncestors {
		if node.Issue == nil {
			continue
		}
		line := t.renderStickyLine(node)
		lines = append(lines, line)
	}

	return lines
}

// renderStickyLine renders a single sticky scroll header line in muted style.
func (t *TreeModel) renderStickyLine(node *IssueTreeNode) string {
	if node == nil || node.Issue == nil {
		return ""
	}
	issue := node.Issue
	r := t.theme.Renderer
	mutedStyle := r.NewStyle().Foreground(t.theme.Muted).Faint(true)

	indicator := t.getExpandIndicator(node)
	icon, _ := t.theme.GetTypeIcon(string(issue.IssueType))

	line := fmt.Sprintf("%s %s %s  %s", indicator, icon, issue.ID, issue.Title)
	return mutedStyle.Render(line)
}

// ── Breadcrumb methods (bd-zq0) ──

// Breadcrumb returns the breadcrumb trail from root to the currently selected node.
// Format: "Epic: Title > Feature: Title > Task: Title"
// Returns empty string if no node is selected or the node is a root.
func (t *TreeModel) Breadcrumb() string {
	node := t.SelectedNode()
	if node == nil || node.Issue == nil {
		return ""
	}

	// Collect path from root to current node
	var path []*IssueTreeNode
	current := node
	for current != nil {
		path = append([]*IssueTreeNode{current}, path...)
		current = current.Parent
	}

	if len(path) <= 1 {
		return "" // Root node or single node - no breadcrumb needed
	}

	var parts []string
	for _, n := range path {
		if n.Issue == nil {
			continue
		}
		typeName := string(n.Issue.IssueType)
		if typeName == "" {
			typeName = "Issue"
		}
		// Capitalize type name
		if len(typeName) > 0 {
			typeName = strings.ToUpper(typeName[:1]) + typeName[1:]
		}
		parts = append(parts, fmt.Sprintf("%s: %s", typeName, n.Issue.Title))
	}

	result := strings.Join(parts, " > ")

	// Truncate if too long (use width if available, default 80)
	maxLen := t.width
	if maxLen <= 0 {
		maxLen = 80
	}
	if len([]rune(result)) > maxLen {
		runes := []rune(result)
		result = "..." + string(runes[len(runes)-(maxLen-3):])
	}

	return result
}

// ── Advanced filter support (bd-08h) ──

// FilterPredicate represents a single parsed filter condition.
type FilterPredicate struct {
	Field   string // "" for plain text, "status", "priority", "type" for field filters
	Value   string // The value to match
	Negated bool   // True if prefixed with !
}

// ParseFilterPredicates parses a filter string into structured predicates.
// Supports: "field:value", "!field:value", "!value", "plain text".
// Field-specific tokens are split by space; remaining non-field text is
// combined as a single plain-text fuzzy predicate.
func ParseFilterPredicates(s string) []FilterPredicate {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	tokens := strings.Fields(s)
	var preds []FilterPredicate
	var plainParts []string

	for _, tok := range tokens {
		negated := false
		t := tok

		if strings.HasPrefix(t, "!") {
			negated = true
			t = t[1:]
		}

		if idx := strings.Index(t, ":"); idx > 0 {
			// field:value pattern
			field := strings.ToLower(t[:idx])
			value := strings.ToLower(t[idx+1:])
			preds = append(preds, FilterPredicate{
				Field:   field,
				Value:   value,
				Negated: negated,
			})
		} else if negated {
			// !value (negated text match on status shorthand)
			preds = append(preds, FilterPredicate{
				Field:   "status",
				Value:   strings.ToLower(t),
				Negated: true,
			})
		} else {
			// Plain text token - collect for fuzzy
			plainParts = append(plainParts, tok)
		}
	}

	if len(plainParts) > 0 {
		preds = append(preds, FilterPredicate{
			Value: strings.ToLower(strings.Join(plainParts, " ")),
		})
	}

	return preds
}

// ApplyAdvancedFilter parses the filter string and applies structured predicates (bd-08h).
// Supports: "status:open", "priority:1", "type:epic", "!status:closed", plain text fuzzy.
// Empty string clears the filter.
func (t *TreeModel) ApplyAdvancedFilter(filter string) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		t.currentFilter = "all"
		t.filterMatches = nil
		t.contextAncestors = nil
		t.advancedPredicates = nil
		t.rebuildFlatList()
		return
	}

	preds := ParseFilterPredicates(filter)
	if len(preds) == 0 {
		t.ApplyFilter("all")
		return
	}

	t.currentFilter = "advanced"
	t.advancedPredicates = preds
	t.filterMatches = make(map[string]bool)
	t.contextAncestors = make(map[string]bool)

	for id, node := range t.issueMap {
		if t.nodeMatchesAdvancedFilter(node, preds) {
			t.filterMatches[id] = true
			// Mark all ancestors as context
			ancestor := node.Parent
			for ancestor != nil {
				if ancestor.Issue != nil {
					t.contextAncestors[ancestor.Issue.ID] = true
				}
				ancestor = ancestor.Parent
			}
		}
	}

	t.rebuildFlatList()
}

// nodeMatchesAdvancedFilter checks if a node matches all given predicates (AND logic).
func (t *TreeModel) nodeMatchesAdvancedFilter(node *IssueTreeNode, preds []FilterPredicate) bool {
	if node == nil || node.Issue == nil {
		return false
	}
	issue := node.Issue

	for _, pred := range preds {
		match := false

		switch pred.Field {
		case "status":
			match = strings.ToLower(string(issue.Status)) == pred.Value
		case "priority":
			match = fmt.Sprintf("%d", issue.Priority) == pred.Value
		case "type":
			match = strings.ToLower(string(issue.IssueType)) == pred.Value
		case "": // plain text fuzzy search on title and ID
			match = strings.Contains(strings.ToLower(issue.Title), pred.Value) ||
				strings.Contains(strings.ToLower(issue.ID), pred.Value)
		default:
			// Unknown field - skip (treat as non-match)
			match = false
		}

		if pred.Negated {
			match = !match
		}

		if !match {
			return false // AND logic: all predicates must match
		}
	}
	return true
}

// ── Mark/multi-select methods (bd-cz0) ──

// ToggleMark marks or unmarks the currently selected node.
func (t *TreeModel) ToggleMark() {
	node := t.SelectedNode()
	if node == nil || node.Issue == nil {
		return
	}
	if t.markedIDs == nil {
		t.markedIDs = make(map[string]bool)
	}
	id := node.Issue.ID
	if t.markedIDs[id] {
		delete(t.markedIDs, id)
	} else {
		t.markedIDs[id] = true
	}
}

// UnmarkAll clears all marks.
func (t *TreeModel) UnmarkAll() {
	t.markedIDs = nil
}

// IsMarked returns true if the given issue ID is marked.
func (t *TreeModel) IsMarked(id string) bool {
	return t.markedIDs != nil && t.markedIDs[id]
}

// TreeMarkedIDs returns a sorted slice of marked issue IDs.
func (t *TreeModel) TreeMarkedIDs() []string {
	if len(t.markedIDs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(t.markedIDs))
	for id := range t.markedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ── XRay drill-down methods (bd-0rc) ──

// ToggleXRay enters or exits XRay mode. When entering, it narrows the view to
// the selected node's subtree. When exiting, it restores the full tree.
func (t *TreeModel) ToggleXRay() {
	if t.xrayRoot != nil {
		// Exit XRay mode
		t.xrayRoot = nil
		t.rebuildFlatList()
		t.ensureCursorVisible()
		return
	}

	node := t.SelectedNode()
	if node == nil || len(node.Children) == 0 {
		return // Only enter XRay on parent nodes
	}

	t.xrayRoot = node
	t.rebuildFlatList()
	t.cursor = 0
	t.viewportOffset = 0
}

// ExitXRay exits XRay mode if active.
func (t *TreeModel) ExitXRay() {
	if t.xrayRoot != nil {
		t.xrayRoot = nil
		t.rebuildFlatList()
		t.ensureCursorVisible()
	}
}

// IsXRayMode returns true if XRay (subtree drill-down) mode is active.
func (t *TreeModel) IsXRayMode() bool {
	return t.xrayRoot != nil
}

// XRayTitle returns the title of the XRay root node, or empty string.
func (t *TreeModel) XRayTitle() string {
	if t.xrayRoot != nil && t.xrayRoot.Issue != nil {
		return t.xrayRoot.Issue.Title
	}
	return ""
}

