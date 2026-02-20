// Package config handles loading and saving b9sconfiguration.
//
// Configuration follows the XDG Base Directory specification:
//   - Config:  ~/.config/bw/config.yaml
//   - Data:    ~/.local/share/bw/ (themes, plugins)
//   - State:   ~/.local/state/bw/ (recent projects, view state cache)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Project represents a registered project in the config.
type Project struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// UIConfig holds UI preference settings.
type UIConfig struct {
	DefaultView string  `yaml:"default_view,omitempty"` // list, tree, board, split
	SplitRatio  float64 `yaml:"split_ratio,omitempty"`  // Default split pane ratio (0.2-0.8)
	Headless    bool    `yaml:"headless,omitempty"`      // Compact header mode
}

// DiscoveryConfig controls auto-discovery of projects.
type DiscoveryConfig struct {
	ScanPaths []string `yaml:"scan_paths,omitempty"` // Directories to scan for .beads/
	MaxDepth  int      `yaml:"max_depth,omitempty"`  // How deep to scan (default 3)
}

// ExperimentalConfig holds experimental feature flags.
type ExperimentalConfig struct {
	BackgroundMode *bool `yaml:"background_mode,omitempty"`
}

// Config is the top-level configuration for b9s.
type Config struct {
	Projects     []Project          `yaml:"projects,omitempty"`
	Favorites    map[int]string     `yaml:"favorites,omitempty"` // Number key (1-9) -> project name
	UI           UIConfig           `yaml:"ui,omitempty"`
	Discovery    DiscoveryConfig    `yaml:"discovery,omitempty"`
	Experimental ExperimentalConfig `yaml:"experimental,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Favorites: make(map[int]string),
		UI: UIConfig{
			DefaultView: "list",
			SplitRatio:  0.4,
		},
		Discovery: DiscoveryConfig{
			MaxDepth: 3,
		},
	}
}

// ConfigDir returns the XDG config directory for b9s.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "b9s")
}

// DataDir returns the XDG data directory for b9s.
func DataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "b9s")
}

// StateDir returns the XDG state directory for b9s.
func StateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "b9s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "b9s")
}

// ConfigPath returns the full path to config.yaml.
func ConfigPath() string {
	dir := ConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

// Load reads the config file from the XDG config directory.
// Returns DefaultConfig if the file doesn't exist.
func Load() (Config, error) {
	path := ConfigPath()
	if path == "" {
		return DefaultConfig(), nil
	}
	return LoadFrom(path)
}

// LoadFrom reads config from a specific path.
// Returns DefaultConfig if the file doesn't exist.
func LoadFrom(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	// Ensure favorites map is initialized
	if cfg.Favorites == nil {
		cfg.Favorites = make(map[int]string)
	}

	// Expand ~ in project paths
	for i := range cfg.Projects {
		cfg.Projects[i].Path = expandHome(cfg.Projects[i].Path)
	}
	for i := range cfg.Discovery.ScanPaths {
		cfg.Discovery.ScanPaths[i] = expandHome(cfg.Discovery.ScanPaths[i])
	}

	return cfg, nil
}

// Save writes the config to the XDG config directory.
func Save(cfg Config) error {
	path := ConfigPath()
	if path == "" {
		return fmt.Errorf("cannot determine config directory")
	}
	return SaveTo(cfg, path)
}

// SaveTo writes the config to a specific path.
func SaveTo(cfg Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// FindProject returns the project with the given name, or nil.
func (c Config) FindProject(name string) *Project {
	for i := range c.Projects {
		if strings.EqualFold(c.Projects[i].Name, name) {
			return &c.Projects[i]
		}
	}
	return nil
}

// FavoriteProject returns the project assigned to number key n (1-9), or nil.
func (c Config) FavoriteProject(n int) *Project {
	name, ok := c.Favorites[n]
	if !ok {
		return nil
	}
	return c.FindProject(name)
}

// SetFavorite assigns a project name to a number key (1-9).
func (c *Config) SetFavorite(n int, projectName string) {
	if c.Favorites == nil {
		c.Favorites = make(map[int]string)
	}
	if projectName == "" {
		delete(c.Favorites, n)
	} else {
		c.Favorites[n] = projectName
	}
}

// ProjectFavoriteNumber returns the favorite number (1-9) for a project name, or 0 if not favorited.
func (c Config) ProjectFavoriteNumber(name string) int {
	for n, pname := range c.Favorites {
		if strings.EqualFold(pname, name) {
			return n
		}
	}
	return 0
}

// ResolvedPath returns the project path with ~ expanded.
func (p Project) ResolvedPath() string {
	return expandHome(p.Path)
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
