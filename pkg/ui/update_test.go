package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/charmbracelet/bubbles/list"
)

type badItem struct{}

func (badItem) Title() string       { return "bad" }
func (badItem) Description() string { return "bad" }
func (badItem) FilterValue() string { return "bad" }

func TestCopyIssueToClipboardInvalidItem(t *testing.T) {
	m := NewModel(nil, "")
	m.list.SetItems([]list.Item{badItem{}})
	m.list.Select(0)
	m.copyIssueToClipboard()
	if !m.statusIsError || m.statusMsg == "" {
		t.Fatalf("expected error copying invalid item, got %q", m.statusMsg)
	}
}

func TestUpdateFileChangedReloadsSelection(t *testing.T) {
	data := `{"id":"ONE","title":"One","status":"open"}`
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(data), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}
	m := NewModel(nil, beads)
	m.list.SetItems([]list.Item{IssueItem{Issue: model.Issue{ID: "ONE", Title: "One", Status: model.StatusOpen}}})
	m.list.Select(0)

	updated, cmd := m.Update(FileChangedMsg{})
	_ = cmd
	m2 := updated.(Model)
	if m2.statusIsError {
		t.Fatalf("expected successful reload, got error %q", m2.statusMsg)
	}
}

func TestFileChangedMsg_RebuildsTreeWhenFocused(t *testing.T) {
	// Start with one issue on disk and in the model.
	initialIssues := []model.Issue{
		{ID: "parent", Title: "Parent", Status: model.StatusOpen, IssueType: model.TypeTask, Priority: 1},
	}
	data := `{"id":"parent","title":"Parent","status":"open","issue_type":"task","priority":1}` + "\n"
	tmp := t.TempDir()
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(data), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(initialIssues, beads)
	m.width, m.height = 120, 40
	m.tree.SetBeadsDir(beadsDir)

	// Model starts in tree view by default.
	if m.focused != focusTree {
		t.Fatalf("expected focusTree on launch, got %v", m.focused)
	}

	// Initial tree should have 1 node.
	initialCount := m.tree.NodeCount()
	if initialCount != 1 {
		t.Fatalf("expected 1 tree node initially, got %d", initialCount)
	}

	// Write updated data with a second issue to disk.
	data2 := data + `{"id":"child","title":"Child","status":"open","issue_type":"task","priority":2,"dependencies":[{"issue_id":"child","depends_on_id":"parent","type":"parent-child"}]}` + "\n"
	if err := os.WriteFile(beads, []byte(data2), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	// Process FileChangedMsg (sync path, no background worker).
	updated, _ := m.Update(FileChangedMsg{})
	m2 := updated.(Model)

	if m2.statusIsError {
		t.Fatalf("expected successful reload, got error %q", m2.statusMsg)
	}

	// Tree should now have 2 nodes reflecting the updated file.
	if got := m2.tree.NodeCount(); got != 2 {
		t.Fatalf("expected tree rebuilt with 2 nodes after FileChangedMsg, got %d (tree not auto-updated)", got)
	}
}

func TestNewModel_SetsTreeBeadsDirFromBeadsPath(t *testing.T) {
	tmp := t.TempDir()
	beads := filepath.Join(tmp, "beads.jsonl")
	if err := os.WriteFile(beads, []byte(`{"id":"ONE","title":"One","status":"open"}`+"\n"), 0644); err != nil {
		t.Fatalf("write beads: %v", err)
	}

	m := NewModel(nil, beads)
	if m.watcher != nil {
		m.watcher.Stop()
	}

	if got, want := m.tree.beadsDir, filepath.Dir(beads); got != want {
		t.Fatalf("expected tree beadsDir %q, got %q", want, got)
	}
}
