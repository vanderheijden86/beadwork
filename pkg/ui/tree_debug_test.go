package ui_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

func TestDebugTreeOrder(t *testing.T) {
	now := time.Now()
	issues := []model.Issue{
		{ID: "epic-1", Title: "Epic One", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeEpic, CreatedAt: now},
		{
			ID: "task-1", Title: "Task One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(time.Second),
			Dependencies: []*model.Dependency{{IssueID: "task-1", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
		{
			ID: "task-2", Title: "Task Two", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(2 * time.Second),
			Dependencies: []*model.Dependency{{IssueID: "task-2", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
		{
			ID: "task-3", Title: "Task Three", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask,
			CreatedAt: now.Add(3 * time.Second),
			Dependencies: []*model.Dependency{{IssueID: "task-3", DependsOnID: "epic-1", Type: model.DepParentChild}},
		},
		{ID: "standalone-1", Title: "Standalone One", Status: model.StatusOpen, Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(4 * time.Second)},
		{ID: "standalone-2", Title: "Standalone Two", Status: model.StatusOpen, Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(5 * time.Second)},
	}

	m := ui.NewModel(issues, "")
	// Enter tree view
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
	m = newM.(ui.Model)

	t.Logf("FocusState: %s", m.FocusState())
	t.Logf("TreeNodeCount: %d", m.TreeNodeCount())
	t.Logf("Initial TreeSelectedID: %s", m.TreeSelectedID())

	// Check CWD and tree-state
	cwd, _ := os.Getwd()
	t.Logf("CWD: %s", cwd)
	stateData, stateErr := os.ReadFile(".beads/tree-state.json")
	if stateErr == nil {
		t.Logf("tree-state.json EXISTS: %s", string(stateData))
	} else {
		t.Logf("tree-state.json NOT FOUND: %v", stateErr)
	}

	// Check if issues have dependencies
	for _, iss := range issues {
		t.Logf("Issue %s has %d deps", iss.ID, len(iss.Dependencies))
		for _, dep := range iss.Dependencies {
			t.Logf("  dep: %s -> %s (type %v)", dep.IssueID, dep.DependsOnID, dep.Type)
		}
	}

	// Walk through all nodes
	for i := 0; i < m.TreeNodeCount(); i++ {
		t.Logf("Node %d: %s", i, m.TreeSelectedID())
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = newM.(ui.Model)
	}
	t.Logf("After all j: %s", m.TreeSelectedID())
	fmt.Println("done")
}
