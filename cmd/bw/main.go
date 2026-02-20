package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
	"github.com/vanderheijden86/beadwork/pkg/updater"
	"github.com/vanderheijden86/beadwork/pkg/version"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cpuProfile := flag.String("cpu-profile", "", "Write CPU profile to file")
	help := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	updateFlag := flag.Bool("update", false, "Update bw to the latest version")
	checkUpdateFlag := flag.Bool("check-update", false, "Check if a new version is available")
	rollbackFlag := flag.Bool("rollback", false, "Rollback to the previous version (from backup)")
	yesFlag := flag.Bool("yes", false, "Skip confirmation prompts (use with --update)")
	repoFilter := flag.String("repo", "", "Filter issues by repository prefix (e.g., 'api-' or 'api')")
	backgroundMode := flag.Bool("background-mode", false, "Enable experimental background snapshot loading (TUI only)")
	noBackgroundMode := flag.Bool("no-background-mode", false, "Disable experimental background snapshot loading (TUI only)")
	flag.Parse()

	// CPU profiling support
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "Could not start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	if *help {
		fmt.Println("Usage: bw [options]")
		fmt.Println("\nA TUI viewer for beads issue tracker.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("bw %s\n", version.Version)
		os.Exit(0)
	}

	// Handle --check-update
	if *checkUpdateFlag {
		available, newVersion, releaseURL, err := updater.CheckUpdateAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
			os.Exit(1)
		}
		if available {
			fmt.Printf("New version available: %s (current: %s)\n", newVersion, version.Version)
			fmt.Printf("Download: %s\n", releaseURL)
			fmt.Println("\nRun 'bw --update' to update automatically")
		} else {
			fmt.Printf("bw is up to date (version %s)\n", version.Version)
		}
		os.Exit(0)
	}

	// Handle --update
	if *updateFlag {
		release, err := updater.GetLatestRelease()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching release info: %v\n", err)
			os.Exit(1)
		}

		available, newVersion, _, _ := updater.CheckUpdateAvailable()
		if !available {
			fmt.Printf("bw is already up to date (version %s)\n", version.Version)
			os.Exit(0)
		}

		if !*yesFlag {
			fmt.Printf("Update bw from %s to %s? [Y/n]: ", version.Version, newVersion)
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "" && response != "y" && response != "yes" {
				fmt.Println("Update cancelled")
				os.Exit(0)
			}
		}

		result, err := updater.PerformUpdate(release, *yesFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			if result != nil && result.BackupPath != "" {
				fmt.Fprintf(os.Stderr, "Backup preserved at: %s\n", result.BackupPath)
			}
			os.Exit(1)
		}

		fmt.Println(result.Message)
		if result.BackupPath != "" {
			fmt.Printf("Backup saved to: %s\n", result.BackupPath)
			fmt.Println("Run 'bw --rollback' to restore if needed")
		}
		os.Exit(0)
	}

	// Handle --rollback
	if *rollbackFlag {
		if err := updater.Rollback(); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Load issues from current directory
	issues, err := datasource.LoadIssues("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading beads: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you are in a project initialized with 'bd init'.")
		os.Exit(1)
	}

	// Get beads file path for live reload (respects BEADS_DIR env var)
	beadsDir, _ := loader.GetBeadsDir("")
	beadsPath, _ := loader.FindJSONLPath(beadsDir)

	// Automatically ensure .bv/ is in .gitignore
	projectDir := filepath.Dir(beadsDir)
	_ = loader.EnsureBVInGitignore(projectDir)

	// Apply --repo filter if specified
	if *repoFilter != "" {
		issues = filterByRepo(issues, *repoFilter)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found. Create some with 'bd create'!")
		os.Exit(0)
	}

	// Load bw config for project switching, favorites, and experimental flags
	appCfg, cfgErr := config.Load()
	if cfgErr != nil {
		// Non-fatal: continue without config
		appCfg = config.DefaultConfig()
	}

	// Background mode rollout:
	// CLI flags override env var, env var overrides config file
	if *backgroundMode && *noBackgroundMode {
		fmt.Fprintln(os.Stderr, "Error: --background-mode and --no-background-mode are mutually exclusive")
		os.Exit(2)
	}
	if *backgroundMode {
		_ = os.Setenv("BW_BACKGROUND_MODE", "1")
	} else if *noBackgroundMode {
		_ = os.Setenv("BW_BACKGROUND_MODE", "0")
	} else if v, ok := os.LookupEnv("BW_BACKGROUND_MODE"); ok && strings.TrimSpace(v) != "" {
		// Respect explicit user env var.
		_ = v
	} else if appCfg.Experimental.BackgroundMode != nil {
		if *appCfg.Experimental.BackgroundMode {
			_ = os.Setenv("BW_BACKGROUND_MODE", "1")
		} else {
			_ = os.Setenv("BW_BACKGROUND_MODE", "0")
		}
	} else if enabled, ok := loadBackgroundModeFromUserConfig(); ok {
		// Legacy fallback: check ~/.config/bv/config.yaml
		if enabled {
			_ = os.Setenv("BW_BACKGROUND_MODE", "1")
		} else {
			_ = os.Setenv("BW_BACKGROUND_MODE", "0")
		}
	}

	// Detect current project name from cwd
	projectName := filepath.Base(projectDir)
	projectPath := projectDir

	// Launch TUI
	m := ui.NewModel(issues, beadsPath).WithConfig(appCfg, projectName, projectPath)
	defer m.Stop()

	if err := runTUIProgram(m); err != nil {
		fmt.Printf("Error running beads viewer: %v\n", err)
		os.Exit(1)
	}
}

func runTUIProgram(m ui.Model) error {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithoutSignalHandler(),
	)

	runDone := make(chan struct{})
	defer close(runDone)

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-runDone:
			return
		case <-sigCh:
		}

		p.Quit()

		select {
		case <-runDone:
			return
		case <-sigCh:
		case <-time.After(5 * time.Second):
		}

		p.Kill()
	}()

	// Optional auto-quit for automated tests: set BW_TUI_AUTOCLOSE_MS.
	if v := os.Getenv("BW_TUI_AUTOCLOSE_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			go func() {
				timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
				defer timer.Stop()

				select {
				case <-runDone:
					return
				case <-timer.C:
				}

				p.Quit()

				select {
				case <-runDone:
					return
				case <-time.After(2 * time.Second):
				}

				p.Kill()
			}()
		}
	}

	_, err := p.Run()
	if err != nil && errors.Is(err, tea.ErrProgramKilled) {
		if err == tea.ErrProgramKilled || errors.Is(err, tea.ErrInterrupted) {
			return nil
		}
	}
	return err
}

func filterByRepo(issues []model.Issue, repoFilter string) []model.Issue {
	if repoFilter == "" {
		return issues
	}

	filter := repoFilter
	filterLower := strings.ToLower(filter)
	needsFlexibleMatch := !strings.HasSuffix(filter, "-") &&
		!strings.HasSuffix(filter, ":") &&
		!strings.HasSuffix(filter, "_")

	var result []model.Issue
	for _, issue := range issues {
		idLower := strings.ToLower(issue.ID)

		if strings.HasPrefix(idLower, filterLower) {
			result = append(result, issue)
			continue
		}

		if needsFlexibleMatch {
			if strings.HasPrefix(idLower, filterLower+"-") ||
				strings.HasPrefix(idLower, filterLower+":") ||
				strings.HasPrefix(idLower, filterLower+"_") {
				result = append(result, issue)
				continue
			}
		}

		if issue.SourceRepo != "" && issue.SourceRepo != "." {
			sourceRepoLower := strings.ToLower(issue.SourceRepo)
			if strings.HasPrefix(sourceRepoLower, filterLower) {
				result = append(result, issue)
			}
		}
	}

	return result
}

func loadBackgroundModeFromUserConfig() (bool, bool) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return false, false
	}
	configPath := filepath.Join(homeDir, ".config", "bv", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, false
	}

	var cfg struct {
		Experimental struct {
			BackgroundMode *bool `yaml:"background_mode"`
		} `yaml:"experimental"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return false, false
	}
	if cfg.Experimental.BackgroundMode == nil {
		return false, false
	}
	return *cfg.Experimental.BackgroundMode, true
}
