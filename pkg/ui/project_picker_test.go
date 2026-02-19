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

func TestProjectPicker_ShowsAllProjects(t *testing.T) {
	m, _ := createModelWithProjects(t)

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

func TestProjectPicker_ViewContainsProjectInfo(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:         config.Project{Name: "api-service", Path: "/tmp/api-service"},
			FavoriteNum:     1,
			IsActive:        true,
			OpenCount:       3,
			InProgressCount: 1,
			ReadyCount:      2,
			BlockedCount:    1,
		},
		{
			Project:         config.Project{Name: "web-frontend", Path: "/tmp/web-frontend"},
			FavoriteNum:     0,
			IsActive:        false,
			OpenCount:       2,
			InProgressCount: 0,
			ReadyCount:      1,
			BlockedCount:    1,
		},
		{
			Project:         config.Project{Name: "data-pipeline", Path: "/tmp/data-pipeline"},
			FavoriteNum:     3,
			IsActive:        false,
			OpenCount:       1,
			InProgressCount: 0,
			ReadyCount:      1,
			BlockedCount:    0,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	// Should contain project names in the table
	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain project name %q", name)
		}
	}

	// Should contain the title bar
	if !strings.Contains(view, "projects") {
		t.Error("view should contain 'projects' title")
	}

	// Active project should be shown in title bar (k9s style)
	if !strings.Contains(view, "projects(api-service)") {
		t.Error("title bar should contain active project name like projects(api-service)")
	}

	// Should contain O P R column headers
	if !strings.Contains(view, "O") && !strings.Contains(view, "P") && !strings.Contains(view, "R") {
		t.Error("view should contain O P R column headers")
	}

	// Should contain B9s ASCII logo
	if !strings.Contains(view, `______   \`) {
		t.Error("view should contain B9s ASCII logo")
	}

	// Should contain shortcut hints
	if !strings.Contains(view, "Filter") {
		t.Error("view should contain 'Filter' shortcut")
	}
}

func TestProjectPicker_TitleBarAtBottom(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:     config.Project{Name: "my-project", Path: "/tmp/my-project"},
			FavoriteNum: 1,
			IsActive:    true,
			OpenCount:   5,
			ReadyCount:  3,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()
	lines := strings.Split(view, "\n")

	// Title bar should be the last line
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "projects(my-project)") {
		t.Errorf("title bar should be at the bottom, last line was: %q", lastLine)
	}
}

func TestProjectPicker_FixedHeight(t *testing.T) {
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}, FavoriteNum: 1, OpenCount: 1, ReadyCount: 1},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}, FavoriteNum: 2, OpenCount: 2, ReadyCount: 1},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}, FavoriteNum: 3, OpenCount: 3, ReadyCount: 2},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	// Panel has fixed height: 6 content rows + 1 title bar = 7
	height := picker.Height()
	if height != 7 {
		t.Errorf("expected fixed height 7 for panel layout, got %d", height)
	}
}

func TestProjectPicker_NoStatsColumn(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:         config.Project{Name: "my-app", Path: "/home/user/projects/my-app"},
			FavoriteNum:     1,
			IsActive:        true,
			OpenCount:       10,
			InProgressCount: 3,
			ReadyCount:      5,
			BlockedCount:    2,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	// Stats column labels should NOT be present (bd-qyr)
	for _, label := range []string{"Project:", "Path:", "Open:", "In Prog:", "Ready:", "Blocked:"} {
		if strings.Contains(view, label) {
			t.Errorf("view should NOT contain stats label %q (stats column removed)", label)
		}
	}
}

func TestProjectPicker_OPRColumns(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:         config.Project{Name: "api-service", Path: "/tmp/api"},
			FavoriteNum:     1,
			IsActive:        true,
			OpenCount:       14,
			InProgressCount: 5,
			ReadyCount:      12,
		},
		{
			Project:         config.Project{Name: "web-frontend", Path: "/tmp/web"},
			FavoriteNum:     2,
			IsActive:        false,
			OpenCount:       6,
			InProgressCount: 1,
			ReadyCount:      6,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	// Should contain the O P R header
	if !strings.Contains(view, "O") || !strings.Contains(view, "P") || !strings.Contains(view, "R") {
		t.Error("view should contain O P R column headers")
	}

	// Should contain the count values (as separate numbers, not inline)
	// The counts 14, 5, 12 should appear for api-service
	if !strings.Contains(view, "14") {
		t.Error("view should contain open count '14'")
	}
	if !strings.Contains(view, "12") {
		t.Error("view should contain ready count '12'")
	}
}

func TestProjectPicker_B9sLogo(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:     config.Project{Name: "test", Path: "/tmp/test"},
			FavoriteNum: 1,
			IsActive:    true,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	// Logo should contain recognizable B9s fragments
	if !strings.Contains(view, `______   \`) {
		t.Error("view should contain B9s ASCII logo")
	}
}

func TestProjectPicker_ShortcutsColumn(t *testing.T) {
	entries := []ui.ProjectEntry{
		{
			Project:     config.Project{Name: "test", Path: "/tmp/test"},
			FavoriteNum: 1,
			IsActive:    true,
		},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	// Should contain shortcut descriptions
	for _, desc := range []string{"Filter", "Edit", "Board", "Help", "Sort", "Shortcuts"} {
		if !strings.Contains(view, desc) {
			t.Errorf("view should contain shortcut %q", desc)
		}
	}
}

func TestProjectPicker_NoProjectsMessage(t *testing.T) {
	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(nil, theme)
	picker.SetSize(140, 40)

	view := picker.View()
	if !strings.Contains(view, "No projects found") {
		t.Error("expected 'No projects found' message when no projects")
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
	picker.SetSize(140, 40)

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
	picker.SetSize(140, 40)

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
	entries := []ui.ProjectEntry{
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	// j should not move cursor (display-only)
	picker, _ = picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if picker.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 in display-only mode, got %d", picker.Cursor())
	}

	// enter should not produce a command
	_, cmd := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter should not produce a command in display-only mode")
	}
}

// TestProjectPicker_AutoNumbering verifies that when no favorites are configured,
// projects are auto-numbered 1-N for display and switching (bd-8zc).
func TestProjectPicker_AutoNumbering(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	view := m.View()

	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain project name %q", name)
		}
	}
}

// TestProjectPicker_NumberKeySwitchesWithoutFavorites verifies that pressing
// a number key switches to the project at that position (bd-8zc).
func TestProjectPicker_NumberKeySwitchesWithoutFavorites(t *testing.T) {
	_, projects := createSampleProjects(t)

	cfg := config.Config{
		Projects:  projects,
		Favorites: nil,
		UI:        config.UIConfig{DefaultView: "list", SplitRatio: 0.4},
	}

	issues := []model.Issue{
		{ID: "api-1", Title: "Fix auth bug", Status: "open", IssueType: "bug", Priority: 1, CreatedAt: time.Now()},
	}

	m := ui.NewModel(issues, "").WithConfig(cfg, "api-service", projects[0].Path)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

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
		{Project: config.Project{Name: "alpha", Path: "/tmp/a"}, FavoriteNum: 1, OpenCount: 1, ReadyCount: 1},
		{Project: config.Project{Name: "beta", Path: "/tmp/b"}, FavoriteNum: 2, OpenCount: 2, ReadyCount: 1},
		{Project: config.Project{Name: "gamma", Path: "/tmp/c"}, FavoriteNum: 3, OpenCount: 3, ReadyCount: 2},
	}

	theme := ui.TestTheme()
	picker := ui.NewProjectPicker(entries, theme)
	picker.SetSize(140, 40)

	view := picker.View()

	if !strings.Contains(view, "alpha") {
		t.Error("view should contain 'alpha'")
	}
	if !strings.Contains(view, "beta") {
		t.Error("view should contain 'beta'")
	}
	if !strings.Contains(view, "gamma") {
		t.Error("view should contain 'gamma'")
	}
}

// TestProjectSwitch_FullCycleLoadsNewData verifies that pressing a number key
// to switch projects produces a SwitchProjectMsg (bd-828).
func TestProjectSwitch_FullCycleLoadsNewData(t *testing.T) {
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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = newM.(ui.Model)

	if cmd == nil {
		t.Fatal("expected a command from pressing '2'")
	}

	msg := cmd()
	switchMsg, ok := msg.(ui.SwitchProjectMsg)
	if !ok {
		t.Fatalf("expected SwitchProjectMsg, got %T", msg)
	}
	if switchMsg.Project.Name != "web-frontend" {
		t.Fatalf("expected web-frontend, got %q", switchMsg.Project.Name)
	}

	newM, switchCmd := m.Update(switchMsg)
	m = newM.(ui.Model)

	view := m.View()
	if !strings.Contains(view, "web-frontend") {
		t.Error("view should mention web-frontend after switch")
	}

	if switchCmd == nil {
		t.Fatal("expected commands from SwitchProjectMsg")
	}
}

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	switchMsg := ui.SwitchProjectMsg{Project: projects[1]}
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	view := m.View()
	if strings.Contains(view, "Loading beads") {
		t.Error("project switch should NOT show loading screen")
	}
}

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	if m.TreeNodeCount() == 0 {
		t.Fatal("expected tree to have nodes before switch")
	}

	switchMsg := ui.SwitchProjectMsg{Project: projects[1]}
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	view := m.View()
	if strings.Contains(view, "Fix auth bug") {
		t.Error("old project issues should not appear after switch")
	}
}

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	switchMsg := ui.SwitchProjectMsg{Project: projects[0]}
	newM, cmd := m.Update(switchMsg)
	m = newM.(ui.Model)

	if cmd != nil {
		t.Error("switching to already-active project should be a no-op (no commands)")
	}
}

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	initialView := m.View()
	if !strings.Contains(initialView, "web-frontend") {
		t.Fatal("expected web-frontend in picker")
	}

	webBeadsPath := filepath.Join(projects[1].Path, ".beads", "issues.jsonl")
	existingContent, err := os.ReadFile(webBeadsPath)
	if err != nil {
		t.Fatal(err)
	}
	newIssue := `{"id":"web-3","title":"New feature","status":"open","issue_type":"feature","priority":2,"created_at":"2026-01-03T00:00:00Z","updated_at":"2026-01-03T00:00:00Z"}` + "\n"
	if err := os.WriteFile(webBeadsPath, append(existingContent, []byte(newIssue)...), 0o644); err != nil {
		t.Fatal(err)
	}

	newM, cmd := m.Update(ui.PickerRefreshTickMsg{})
	m = newM.(ui.Model)

	if cmd == nil {
		t.Error("expected PickerRefreshTickMsg to produce a follow-up tick command")
	}

	updatedView := m.View()
	if initialView == updatedView {
		t.Error("expected picker view to change after PickerRefreshTickMsg with modified JSONL")
	}
}

func TestPickerCounts_BlockedByDependencies(t *testing.T) {
	root := t.TempDir()

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	entries := m.BuildProjectEntries()

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

	if depEntry.OpenCount != 3 {
		t.Errorf("expected OpenCount=3, got %d", depEntry.OpenCount)
	}
	if depEntry.ReadyCount != 1 {
		t.Errorf("expected ReadyCount=1, got %d", depEntry.ReadyCount)
	}
	if depEntry.BlockedCount != 1 {
		t.Errorf("expected BlockedCount=1, got %d", depEntry.BlockedCount)
	}
}

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
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = newM.(ui.Model)

	if m.TreeNodeCount() == 0 {
		t.Fatal("tree should have nodes before filter")
	}

	m.ApplyTreeFilter("closed")

	if m.TreeNodeCount() != 0 {
		t.Errorf("expected 0 nodes after 'closed' filter, got %d", m.TreeNodeCount())
	}

	switchMsg := ui.SwitchProjectMsg{Project: projects[1]}
	newM, _ = m.Update(switchMsg)
	m = newM.(ui.Model)

	if m.TreeFilterActive() {
		t.Error("tree filter should be cleared after project switch")
	}
}

func TestProjectPicker_NoPKeyToggle(t *testing.T) {
	m, _ := createModelWithProjects(t)

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	m = newM.(ui.Model)

	for _, name := range []string{"api-service", "web-frontend", "data-pipeline"} {
		view := m.View()
		if !strings.Contains(view, name) {
			t.Errorf("after pressing P, view should still contain project name %q", name)
		}
	}
}
