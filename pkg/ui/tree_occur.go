// tree_occur.go - Occur mode implementation for filtering tree view (bd-sjs.2)
package ui

import (
	"regexp"
)

// EnterOccurMode activates occur mode, filtering to matching issues (bd-sjs.2).
func (t *TreeModel) EnterOccurMode(pattern string) {
	if pattern == "" {
		return
	}
	t.occurMode = true
	t.occurPattern = pattern
	t.rebuildFlatList()
	t.ensureCursorVisible()
}

// ExitOccurMode returns to normal tree view (bd-sjs.2).
func (t *TreeModel) ExitOccurMode() {
	t.occurMode = false
	t.occurPattern = ""
	t.rebuildFlatList()
	t.ensureCursorVisible()
}

// IsOccurMode returns true if occur mode is active (bd-sjs.2).
func (t *TreeModel) IsOccurMode() bool {
	return t.occurMode
}

// OccurPattern returns the current occur pattern (bd-sjs.2).
func (t *TreeModel) OccurPattern() string {
	return t.occurPattern
}

// rebuildOccurFlatList filters flat list to only show matching issues (bd-sjs.2).
func (t *TreeModel) rebuildOccurFlatList() {
	re, err := regexp.Compile("(?i)" + t.occurPattern)
	if err != nil {
		return
	}
	var filtered []*IssueTreeNode
	for _, node := range t.flatList {
		if node.Issue == nil {
			continue
		}
		if re.MatchString(node.Issue.ID) || re.MatchString(node.Issue.Title) ||
			re.MatchString(string(node.Issue.Status)) {
			filtered = append(filtered, node)
		}
	}
	t.flatList = filtered
	if t.cursor >= len(t.flatList) {
		t.cursor = len(t.flatList) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}
