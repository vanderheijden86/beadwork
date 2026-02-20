package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.UI.DefaultView != "list" {
		t.Errorf("expected default view 'list', got %q", cfg.UI.DefaultView)
	}
	if cfg.UI.SplitRatio != 0.4 {
		t.Errorf("expected split ratio 0.4, got %f", cfg.UI.SplitRatio)
	}
	if cfg.Discovery.MaxDepth != 3 {
		t.Errorf("expected max depth 3, got %d", cfg.Discovery.MaxDepth)
	}
	if cfg.Favorites == nil {
		t.Error("expected favorites map to be initialized")
	}
}

func TestLoadFrom_NonExistent(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.UI.DefaultView != "list" {
		t.Errorf("expected default config, got view %q", cfg.UI.DefaultView)
	}
}

func TestLoadFrom_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
projects:
  - name: myproject
    path: ~/work/myproject
  - name: other
    path: /absolute/path

favorites:
  1: myproject
  2: other

ui:
  default_view: tree
  split_ratio: 0.5

discovery:
  scan_paths:
    - ~/work
  max_depth: 2
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "myproject" {
		t.Errorf("expected project name 'myproject', got %q", cfg.Projects[0].Name)
	}
	// Path should have ~ expanded
	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, "work/myproject")
	if cfg.Projects[0].Path != expectedPath {
		t.Errorf("expected expanded path %q, got %q", expectedPath, cfg.Projects[0].Path)
	}
	if cfg.Projects[1].Path != "/absolute/path" {
		t.Errorf("expected absolute path preserved, got %q", cfg.Projects[1].Path)
	}

	if cfg.Favorites[1] != "myproject" {
		t.Errorf("expected favorite 1 = 'myproject', got %q", cfg.Favorites[1])
	}
	if cfg.Favorites[2] != "other" {
		t.Errorf("expected favorite 2 = 'other', got %q", cfg.Favorites[2])
	}

	if cfg.UI.DefaultView != "tree" {
		t.Errorf("expected default_view 'tree', got %q", cfg.UI.DefaultView)
	}
	if cfg.UI.SplitRatio != 0.5 {
		t.Errorf("expected split_ratio 0.5, got %f", cfg.UI.SplitRatio)
	}
	if cfg.Discovery.MaxDepth != 2 {
		t.Errorf("expected max_depth 2, got %d", cfg.Discovery.MaxDepth)
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Config{
		Projects: []Project{
			{Name: "proj1", Path: "/path/to/proj1"},
			{Name: "proj2", Path: "/path/to/proj2"},
		},
		Favorites: map[int]string{
			1: "proj1",
			3: "proj2",
		},
		UI: UIConfig{
			DefaultView: "board",
			SplitRatio:  0.6,
		},
	}

	if err := SaveTo(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if len(loaded.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(loaded.Projects))
	}
	if loaded.Projects[0].Name != "proj1" {
		t.Errorf("expected 'proj1', got %q", loaded.Projects[0].Name)
	}
	if loaded.Favorites[1] != "proj1" {
		t.Errorf("expected favorite 1 = 'proj1', got %q", loaded.Favorites[1])
	}
	if loaded.Favorites[3] != "proj2" {
		t.Errorf("expected favorite 3 = 'proj2', got %q", loaded.Favorites[3])
	}
	if loaded.UI.DefaultView != "board" {
		t.Errorf("expected 'board', got %q", loaded.UI.DefaultView)
	}
}

func TestFindProject(t *testing.T) {
	cfg := Config{
		Projects: []Project{
			{Name: "alpha", Path: "/a"},
			{Name: "Beta", Path: "/b"},
		},
	}

	p := cfg.FindProject("alpha")
	if p == nil || p.Name != "alpha" {
		t.Error("expected to find 'alpha'")
	}

	// Case-insensitive
	p = cfg.FindProject("BETA")
	if p == nil || p.Name != "Beta" {
		t.Error("expected to find 'Beta' case-insensitively")
	}

	p = cfg.FindProject("nonexistent")
	if p != nil {
		t.Error("expected nil for nonexistent project")
	}
}

func TestFavoriteProject(t *testing.T) {
	cfg := Config{
		Projects: []Project{
			{Name: "proj1", Path: "/p1"},
		},
		Favorites: map[int]string{
			1: "proj1",
		},
	}

	p := cfg.FavoriteProject(1)
	if p == nil || p.Name != "proj1" {
		t.Error("expected favorite 1 to return proj1")
	}

	p = cfg.FavoriteProject(5)
	if p != nil {
		t.Error("expected nil for unset favorite")
	}
}

func TestSetFavorite(t *testing.T) {
	cfg := Config{Favorites: make(map[int]string)}

	cfg.SetFavorite(1, "myproj")
	if cfg.Favorites[1] != "myproj" {
		t.Error("expected favorite 1 set to 'myproj'")
	}

	// Clear favorite
	cfg.SetFavorite(1, "")
	if _, ok := cfg.Favorites[1]; ok {
		t.Error("expected favorite 1 to be cleared")
	}
}

func TestProjectFavoriteNumber(t *testing.T) {
	cfg := Config{
		Favorites: map[int]string{
			2: "myproj",
			5: "other",
		},
	}

	if n := cfg.ProjectFavoriteNumber("myproj"); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if n := cfg.ProjectFavoriteNumber("other"); n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
	if n := cfg.ProjectFavoriteNumber("unknown"); n != 0 {
		t.Errorf("expected 0 for unknown, got %d", n)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/", filepath.Join(home, "")},
		{"/absolute", "/absolute"},
		{"relative", "relative"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.expected {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestConfigDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got := ConfigDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestDataDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	got := DataDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStateDir_XDGOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	got := StateDir()
	expected := filepath.Join(dir, "b9s")
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestLoadFrom_EmptyFavorites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
projects:
  - name: solo
    path: /solo
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Favorites == nil {
		t.Error("expected favorites map to be initialized even when empty in config")
	}
}

func TestExperimentalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	boolTrue := true
	content := `
experimental:
  background_mode: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Experimental.BackgroundMode == nil {
		t.Fatal("expected background_mode to be set")
	}
	if *cfg.Experimental.BackgroundMode != boolTrue {
		t.Error("expected background_mode to be true")
	}
}
