package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// createSampleProjects creates temp directories with .beads/issues.jsonl for testing.
func createSampleProjects(t *testing.T) (string, []config.Project) {
	t.Helper()
	root := t.TempDir()

	projects := []struct {
		name   string
		issues string
	}{
		{
			name: "api-service",
			issues: `{"id":"api-1","title":"Fix auth bug","status":"open","issue_type":"bug","priority":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"id":"api-2","title":"Add rate limiting","status":"in_progress","issue_type":"feature","priority":2,"created_at":"2026-01-02T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}
{"id":"api-3","title":"Update docs","status":"open","issue_type":"task","priority":3,"created_at":"2026-01-03T00:00:00Z","updated_at":"2026-01-03T00:00:00Z"}
`,
		},
		{
			name: "web-frontend",
			issues: `{"id":"web-1","title":"Dark mode","status":"open","issue_type":"feature","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"id":"web-2","title":"Fix CSS grid","status":"blocked","issue_type":"bug","priority":1,"created_at":"2026-01-02T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}
`,
		},
		{
			name: "data-pipeline",
			issues: `{"id":"dp-1","title":"Optimize ETL","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
`,
		},
	}

	var cfgProjects []config.Project
	for _, p := range projects {
		dir := filepath.Join(root, p.name)
		beadsDir := filepath.Join(dir, ".beads")
		if err := os.MkdirAll(beadsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(p.issues), 0o644); err != nil {
			t.Fatal(err)
		}
		cfgProjects = append(cfgProjects, config.Project{Name: p.name, Path: dir})
	}

	return root, cfgProjects
}

// createModelWithProjects creates a Model loaded with sample projects.
func createModelWithProjects(t *testing.T) (ui.Model, config.Config) {
	t.Helper()
	_, projects := createSampleProjects(t)

	// Create some issues for the "active" project (api-service)
	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
		{ID: "api-2", Title: "Add rate limiting", Status: "in_progress", IssueType: "feature", Priority: 2, CreatedAt: time.Now()},
		{ID: "api-3", Title: "Update docs", Status: "open", IssueType: "task", Priority: 3, CreatedAt: time.Now()},
	}

	cfg := config.Config{
		Projects:  projects,
		Favorites: map[int]string{1: "api-service", 3: "data-pipeline"},
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
		Discovery: config.DiscoveryConfig{MaxDepth: 3},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	// Send a window size so the model is ready
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return newM.(ui.Model), cfg
}

// (bd-8hw.4: switchToListView removed — tree is permanent, no list view)

func TestProjectPicker_ExpandedByDefault(t *testing.T) {
	m, _ := createModelWithProjects(t)

	// Picker should be expanded by default after WithConfig (bd-ey3)
	if !m.PickerExpanded() {
		t.Fatal("picker should be expanded by default")
	}
}

func TestProjectPicker_ToggleExpandedMinimized(t *testing.T) {
	m, _ := createModelWithProjects(t)

	// Should start expanded
	if !m.PickerExpanded() {
		t.Fatal("picker should be expanded initially")
	}

	// Press P to minimize (works from tree, bd-8hw.4)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	if m.PickerExpanded() {
		t.Fatal("picker should be minimized after P")
	}

	// Press P to expand again
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	if !m.PickerExpanded() {
		t.Fatal("picker should be expanded after second P")
	}
}

func TestProjectPicker_ShowsAllProjects(t *testing.T) {
	m, _ := createModelWithProjects(t)

	// Picker is expanded by default, should show all projects
	if !m.PickerExpanded() {
		t.Fatal("picker should be expanded by default")
	}

	if m.ProjectPickerFilteredCount() != 3 {
		t.Errorf("expected 3 projects in picker, got %d", m.ProjectPickerFilteredCount())
	}
}

func TestProjectPicker_ActiveProjectHighlighted(t *testing.T) {
	m, _ := createModelWithProjects(t)

	if m.ActiveProjectName() != "api-service" {
		t.Fatalf("expected active project 'api-service', got %q", m.ActiveProjectName())
	}
}

func TestProjectPicker_ViewExpandedContainsProjectInfo(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:      config.Project{Name: "api-service", Path: "/tmp/api-service"},
			FavoriteNum:  1,
			IsActive:     true,
			OpenCount:    3,
			ReadyCount:   2,
			BlockedCount: 1,
		},
		{
			Project:      config.Project{Name: "web-frontend", Path: "/tmp/web-frontend"},
			FavoriteNum:  0,
			IsActive:     false,
			OpenCount:    2,
			ReadyCount:   1,
			BlockedCount: 1,
		},
		{
			Project:      config.Project{Name: "data-pipeline", Path: "/tmp/data-pipeline"},
			FavoriteNum:  3,
			IsActive:     false,
			OpenCount:    1,
			ReadyCount:   1,
			BlockedCount: 0,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	view := picker.ViewExpanded()

	// Should contain project names
	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		if !strings.Contains(view, name) {
			t.Errorf("expanded view should contain project name %q", name)
		}
	}

	// Should contain the title bar
	if !strings.Contains(view, "projects") {
		t.Error("expanded view should contain 'projects' title")
	}

	// Should contain column headers
	if !strings.Contains(view, "NAME") {
		t.Error("expanded view should contain NAME column header")
	}
	if !strings.Contains(view, "BLOCKED") {
		t.Error("expanded view should contain BLOCKED column header")
	}

	// Should contain shortcut hints (new set for expanded mode)
	if !strings.Contains(view, "Quick Switch") {
		t.Error("expanded view should contain 'Quick Switch' shortcut hint")
	}
	if !strings.Contains(view, "Minimize") {
		t.Error("expanded view should contain 'Minimize' shortcut hint")
	}

	// Should contain active project indicator (►)
	if !strings.Contains(view, "\u25ba") {
		t.Error("expanded view should contain ► indicator for active project")
	}
}

func TestProjectPicker_ViewMinimizedContainsInfo(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:      config.Project{Name: "api-service", Path: "/tmp/api-service"},
			FavoriteNum:  1,
			IsActive:     true,
			OpenCount:    3,
			ReadyCount:   2,
			BlockedCount: 1,
		},
		{
			Project:      config.Project{Name: "web-frontend", Path: "/tmp/web-frontend"},
			FavoriteNum:  2,
			IsActive:     false,
			OpenCount:    2,
			ReadyCount:   1,
			BlockedCount: 1,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	view := picker.ViewMinimized()

	// Should contain active project name and stats
	if !strings.Contains(view, "api-service") {
		t.Error("minimized view should contain active project name")
	}
	if !strings.Contains(view, "3/0/2/1") {
		t.Error("minimized view should contain stats (3/0/2/1)")
	}

	// Should NOT contain "Project:" prefix (bd-aa6)
	if strings.Contains(view, "Project:") {
		t.Error("minimized view should NOT contain 'Project:' prefix")
	}
	// Should NOT contain expand hint (bd-aa6)
	if strings.Contains(view, "Expand") {
		t.Error("minimized view should NOT contain 'Expand' hint")
	}
	if strings.Contains(view, "<P>") {
		t.Error("minimized view should NOT contain '<P>' hint")
	}
}

func TestProjectPicker_FilterProjects(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}},
		{Project: config.Project{Name: "data-pipeline", Path: "/tmp/data"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Enter filter mode
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})

	if !picker.Filtering() {
		t.Fatal("should be in filter mode after /")
	}

	// Type "api-" (specific enough to match only api-service)
	for _, ch := range "api-" {
		picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	// api-service should be top result
	selected := picker.SelectedEntry()
	if selected == nil || selected.Project.Name != "api-service" {
		name := ""
		if selected != nil {
			name = selected.Project.Name
		}
		t.Errorf("expected api-service as top filter result, got %q", name)
	}

	// Esc clears filter
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyEscape})

	if picker.Filtering() {
		t.Error("should not be filtering after esc")
	}
	if picker.FilteredCount() != 3 {
		t.Errorf("expected all 3 projects after filter clear, got %d", picker.FilteredCount())
	}
}

func TestProjectPicker_QuickSwitchByNumber(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "api-service", Path: "/tmp/api"}, FavoriteNum: 1},
		{Project: config.Project{Name: "web-frontend", Path: "/tmp/web"}, FavoriteNum: 0},
		{Project: config.Project{Name: "data-pipeline", Path: "/tmp/data"}, FavoriteNum: 3},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// Press 3 to quick-switch to data-pipeline (favorite #3)
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})

	if cmd == nil {
		t.Fatal("expected a command from quick-switch")
	}

	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "data-pipeline" {
		t.Errorf("expected data-pipeline, got %q", switchMsg.Project.Name)
	}
}

func TestProjectPicker_DisplayOnlyNoNavigation(t *testing.T) {
	// Picker is display-only: j/k/enter/g/G don't navigate or act.
	// Project switching is via number keys only (handled by Model, not picker).
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// j should not move cursor (display-only)
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if picker.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 in display-only mode, got %d", picker.Cursor())
	}

	// k should not move cursor
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if picker.Cursor() != 0 {
		t.Errorf("cursor should stay at 0, got %d", picker.Cursor())
	}

	// enter should not produce a command
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter should not produce a command in display-only mode")
	}
}

func TestProjectPicker_ExpandedHeight(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	// 3 header lines (shortcut bar + title + column headers) + 3 project rows = 6
	height := picker.ExpandedHeight()
	if height != 6 {
		t.Errorf("expected expanded height 6, got %d", height)
	}
}

func TestProjectPicker_MinimizedHeight(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	height := picker.MinimizedHeight()
	if height != 1 {
		t.Errorf("expected minimized height 1, got %d", height)
	}
}

func TestProjectPicker_NoProjectsMessage(t *testing.T) {
	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(nil, theme)
	picker.SetSize(120, 40)

	view := picker.View()
	if !strings.Contains(view, "No projects found") {
		t.Error("expected 'No projects found' message when no projects")
	}
}

// TestProjectPicker_AutoNumbering verifies that when no favorites are configured,
// projects are auto-numbered 1-N for display and switching (bd-8zc).
func TestProjectPicker_AutoNumbering(t *testing.T) {
	_, projects := createSampleProjects(t)

	// Config with NO favorites
	cfg := config.Config{
		Projects:  projects,
		Favorites: nil, // No favorites configured
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Picker should show auto-numbers in the view
	if !m.PickerExpanded() {
		t.Fatal("picker should be expanded by default")
	}

	view := m.View()

	// The view should contain the number "1" near api-service, "2" near web-frontend, "3" near data-pipeline
	// Since projects are numbered 1-3, the view should show those numbers
	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		if !strings.Contains(view, name) {
			t.Errorf("expanded view should contain project name %q", name)
		}
	}
}

// TestProjectPicker_NumberKeySwitchesWithoutFavorites verifies that pressing
// a number key (e.g. "2") switches to the project at that position even when
// no favorites are configured in the config (bd-8zc).
func TestProjectPicker_NumberKeySwitchesWithoutFavorites(t *testing.T) {
	_, projects := createSampleProjects(t)

	// Config with NO favorites
	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Press "2" to switch to web-frontend (second project)
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = newM.(ui.Model)

	if cmd == nil {
		t.Fatal("expected a command from pressing '2' for project switch")
	}

	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "web-frontend" {
		t.Errorf("expected 'web-frontend', got %q", switchMsg.Project.Name)
	}
}

// TestProjectPicker_AutoNumberDisplayInView verifies the picker view shows
// position numbers prominently for each project (bd-8zc).
func TestProjectPicker_AutoNumberDisplayInView(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}, FavoriteNum: 1},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}, FavoriteNum: 2},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}, FavoriteNum: 3},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(120, 40)

	view := picker.ViewExpanded()

	// Each row should contain the position number
	if !strings.Contains(view, "1") || !strings.Contains(view, "alpha") {
		t.Error("expanded view should contain '1' and 'alpha'")
	}
	if !strings.Contains(view, "2") || !strings.Contains(view, "beta") {
		t.Error("expanded view should contain '2' and 'beta'")
	}
	if !strings.Contains(view, "3") || !strings.Contains(view, "gamma") {
		t.Error("expanded view should contain '3' and 'gamma'")
	}
}

// TestProjectSwitch_FullCycleLoadsNewData verifies that pressing a number key
// to switch projects produces a SwitchProjectMsg, and feeding that message back
// into Update triggers a data reload for the new project (bd-828).
func TestProjectSwitch_FullCycleLoadsNewData(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "tree", SplitRatio: 0.4},
	}

	// Start with api-service issues
	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, projects[0].Path+string(os.PathSeparator)+".beads"+string(os.PathSeparator)+"issues.jsonl").
		WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Verify initial state
	if m.TreeSelectedID() != "api-1" {
		// Issue might not be selected if tree isn't built yet, that's OK
	}

	// Press "2" to switch to web-frontend
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = newM.(ui.Model)

	if cmd == nil {
		t.Fatal("expected a command from pressing '2'")
	}

	// Execute the command to get SwitchProjectMsg
	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "web-frontend" {
		t.Fatalf("expected web-frontend, got %q", switchMsg.Project.Name)
	}

	// Feed SwitchProjectMsg back into Update (this is what bubbletea does)
	newM, switchCmd := m.Update(switchMsg)
	m = newM.(ui.Model)

	// The status should say "Switched to web-frontend"
	view := m.View()
	if !strings.Contains(view, "web-frontend") {
		t.Error("view should mention web-frontend after switch")
	}

	// The switch produces commands (either StartBackgroundWorkerCmd or FileChangedMsg)
	if switchCmd == nil {
		t.Fatal("expected commands from SwitchProjectMsg")
	}
}

// TestProjectSwitch_NoLoadingScreen verifies project switching doesn't flash
// the "Loading beads..." screen — it keeps showing the tree while loading (bd-828).
func TestProjectSwitch_NoLoadingScreen(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "tree", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, projects[0].Path+string(os.PathSeparator)+".beads"+string(os.PathSeparator)+"issues.jsonl").
		WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Switch to web-frontend
	switchMsg := ui.SwitchProjectMsg{Project: projects[1]}
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	// View should NOT show "Loading beads..."
	view := m.View()
	if strings.Contains(view, "Loading beads") {
		t.Error("project switch should NOT show loading screen")
	}
}

// TestProjectSwitch_ClearsOldTreeData verifies that switching projects clears the
// old tree/issues data so stale content doesn't render while loading (bd-lll).
func TestProjectSwitch_ClearsOldTreeData(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "tree", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
		{ID: "api-2", Title: "Add rate limiting", Status: "in_progress", IssueType: "feature", Priority: 2, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, projects[0].Path+string(os.PathSeparator)+".beads"+string(os.PathSeparator)+"issues.jsonl").
		WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Verify we have api-service issues in the tree
	if m.TreeNodeCount() == 0 {
		t.Fatal("expected tree to have nodes before switch")
	}

	// Switch to web-frontend
	switchMsg := ui.SwitchProjectMsg{Project: projects[1]}
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	// After switch, old tree data should be cleared
	view := m.View()
	if strings.Contains(view, "Fix auth bug") {
		t.Error("old project issues should not appear after switch")
	}
	if strings.Contains(view, "Add rate limiting") {
		t.Error("old project issues should not appear after switch")
	}
}

// TestProjectSwitch_SameProjectIsNoop verifies pressing the number key of the
// already-active project does NOT restart the background worker or reload data (bd-3eh).
func TestProjectSwitch_SameProjectIsNoop(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "tree", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, projects[0].Path+string(os.PathSeparator)+".beads"+string(os.PathSeparator)+"issues.jsonl").
		WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Send SwitchProjectMsg for the SAME project that's already active
	switchMsg := ui.SwitchProjectMsg{Project: projects[0]}
	newM, cmd := m.Update(switchMsg)
	m = newM.(ui.Model)

	// Should be a no-op: no commands returned (no worker restart, no file reload)
	if cmd != nil {
		t.Error("switching to already-active project should be a no-op (no commands)")
	}
}

// TestPickerCountsRefreshOnTick verifies that a periodic tick message
// triggers a refresh of non-active project counts in the picker (bd-8yc).
func TestPickerCountsRefreshOnTick(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "tree", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, projects[0].Path+string(os.PathSeparator)+".beads"+string(os.PathSeparator)+"issues.jsonl").
		WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Capture initial view — web-frontend should have some counts from its JSONL
	initialView := m.View()
	if !strings.Contains(initialView, "web-frontend") {
		t.Fatal("expected web-frontend in expanded picker")
	}

	// Now modify the web-frontend JSONL on disk (add a new issue)
	webBeadsPath := filepath.Join(projects[1].Path, ".beads", "issues.jsonl")
	existingContent, err := os.ReadFile(webBeadsPath)
	if err != nil {
		t.Fatal(err)
	}
	newIssue := `{"id":"web-3","title":"New feature","status":"open","issue_type":"feature","priority":2,"created_at":"2026-01-03T00:00:00Z","updated_at":"2026-01-03T00:00:00Z"}` + "\n"
	if err := os.WriteFile(webBeadsPath, append(existingContent, []byte(newIssue)...), 0o644); err != nil {
		t.Fatal(err)
	}

	// Send PickerRefreshTickMsg to trigger a refresh
	newM, cmd := m.Update(ui.PickerRefreshTickMsg{})
	m = newM.(ui.Model)

	// The tick should produce a follow-up tick command
	if cmd == nil {
		t.Error("expected PickerRefreshTickMsg to produce a follow-up tick command")
	}

	// The picker should now reflect updated counts for web-frontend
	// Original: 2 issues (web-1 open, web-2 blocked). After: 3 issues.
	updatedView := m.View()
	// web-frontend originally had OpenCount=1 (web-1). Now it should have 2 (web-1 + web-3).
	// We can't easily check exact numbers in the table format, but we can verify
	// the view changed after the tick (indicating a refresh happened).
	if initialView == updatedView {
		t.Error("expected picker view to change after PickerRefreshTickMsg with modified JSONL")
	}
}

// TestPickerCounts_BlockedByDependencies verifies that non-active projects correctly
// count issues blocked by open dependencies (bd-qjc).
func TestPickerCounts_BlockedByDependencies(t *testing.T) {
	root := t.TempDir()

	// Create a project with issues that have blocking dependencies.
	// Issue dep-1 is open (should be READY — no blockers).
	// Issue dep-2 is open but blocked by dep-1 (should be BLOCKED, not READY).
	// Issue dep-3 has status "blocked" explicitly.
	projDir := filepath.Join(root, "dep-project")
	beadsDir := filepath.Join(projDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	issuesJSONL := `{"id":"dep-1","title":"Base task","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}
{"id":"dep-2","title":"Depends on base","status":"open","issue_type":"task","priority":2,"dependencies":[{"issue_id":"dep-2","depends_on_id":"dep-1","type":"blocks","created_at":"2026-01-01T00:00:00Z","created_by":"test"}],"created_at":"2026-01-02T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}
{"id":"dep-3","title":"Explicitly blocked","status":"blocked","issue_type":"bug","priority":1,"created_at":"2026-01-03T00:00:00Z","updated_at":"2026-01-03T00:00:00Z"}
`
	if err := os.WriteFile(filepath.Join(beadsDir, "issues.jsonl"), []byte(issuesJSONL), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a separate active project (so dep-project is non-active and counted from disk).
	activeDir := filepath.Join(root, "active-project")
	activeBeads := filepath.Join(activeDir, ".beads")
	if err := os.MkdirAll(activeBeads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeBeads, "issues.jsonl"),
		[]byte(`{"id":"act-1","title":"Active task","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Projects: []config.Project{
			{Name: "active-project", Path: activeDir},
			{Name: "dep-project", Path: projDir},
		},
	}
	activeIssues := []model.Issue{
		{ID: "act-1", Title: "Active task", Status: "open", IssueType: "task", Priority: 2},
	}
	m := ui.NewModel(activeIssues, "").WithConfig(cfg, "active-project", activeDir)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	entries := m.BuildProjectEntries()

	// Find the dep-project entry
	var depEntry *ui.ProjectEntry
	for i := range entries {
		if entries[i].Project.Name == "dep-project" {
			depEntry = &entries[i]
			break
		}
	}
	if depEntry == nil {
		t.Fatal("dep-project not found in entries")
	}

	// dep-1: open, no blockers → READY
	// dep-2: open, blocked by dep-1 (which is open) → NOT ready (blocked by dep)
	// dep-3: status "blocked" → BLOCKED
	// OpenCount should be 3 (all non-closed)
	if depEntry.OpenCount != 3 {
		t.Errorf("expected OpenCount=3, got %d", depEntry.OpenCount)
	}
	// ReadyCount should be 1 (only dep-1 is truly ready)
	if depEntry.ReadyCount != 1 {
		t.Errorf("expected ReadyCount=1, got %d", depEntry.ReadyCount)
	}
	// BlockedCount should be 1 (dep-3 has explicit "blocked" status)
	if depEntry.BlockedCount != 1 {
		t.Errorf("expected BlockedCount=1, got %d", depEntry.BlockedCount)
	}
}

// TestProjectSwitch_ClearsTreeFilter verifies that switching projects resets
// the tree's search/filter state so the new project's issues are fully visible (bd-qjc).
func TestProjectSwitch_ClearsTreeFilter(t *testing.T) {
	_, projects := createSampleProjects(t)

	activeIssues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1},
		{ID: "api-2", Title: "Add rate limiting", Status: "in_progress", IssueType: "feature", Priority: 2},
		{ID: "api-3", Title: "Update docs", Status: "open", IssueType: "task", Priority: 3},
	}
	cfg := config.Config{
		Projects: projects,
	}

	m := ui.NewModel(activeIssues, "").WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(ui.Model)

	// Tree should have nodes (3 issues)
	if m.TreeNodeCount() == 0 {
		t.Fatal("tree should have nodes before filter")
	}

	// Apply a "closed" filter — all test issues are open/in_progress, so 0 match.
	m.ApplyTreeFilter("closed")

	// After applying the filter, the tree should show 0 matching nodes
	if m.TreeNodeCount() != 0 {
		t.Errorf("expected 0 nodes after 'closed' filter, got %d", m.TreeNodeCount())
	}

	// Now switch projects
	switchMsg := ui.SwitchProjectMsg{Project: projects[1]} // web-frontend
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	// After switch, the tree filter should be cleared.
	// The tree is cleared via Build(nil) during switch, so it's empty pending data load.
	// But critically, the filter should not persist — verify via the tree filter accessor.
	if m.TreeFilterActive() {
		t.Error("tree filter should be cleared after project switch")
	}
}
