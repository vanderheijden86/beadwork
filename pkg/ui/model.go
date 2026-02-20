package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/updater"
	"github.com/vanderheijden86/beadwork/pkg/watcher"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View width thresholds for adaptive layout
const (
	SplitViewThreshold     = 100
	WideViewThreshold      = 140
	UltraWideViewThreshold = 180
	MinDetailPaneWidth     = 40 // Auto-hide detail panel below this width (bd-dy7)
)

// focus represents which UI element has keyboard focus
type focus int

const (
	focusList focus = iota
	focusDetail
	focusBoard
	focusTree
	focusRepoPicker
	focusHelp
	focusQuitConfirm
	focusLabelPicker
	focusTutorial
	focusUpdateModal
	focusStatusPicker
	focusEditModal
)

// SortMode represents the current list sorting mode (bv-3ita)
type SortMode int

const (
	SortDefault     SortMode = iota // Created desc (newest first) (bd-ctu)
	SortCreatedAsc                  // By creation date, oldest first
	SortCreatedDesc                 // By creation date, newest first
	SortPriority                    // By priority only (ascending)
	SortUpdated                     // By last update, newest first
	numSortModes                    // Keep this last - used for cycling
)

// String returns a human-readable label for the sort mode
func (s SortMode) String() string {
	switch s {
	case SortCreatedAsc:
		return "Created ↑"
	case SortCreatedDesc:
		return "Created ↓"
	case SortPriority:
		return "Priority"
	case SortUpdated:
		return "Updated"
	default:
		return "Default"
	}
}

// SortField represents which field to sort by (bd-x3l).
// This replaces the per-mode enum with a field+direction approach.
type SortField int

const (
	SortFieldPriority  SortField = iota // Priority (P0, P1, ...)
	SortFieldCreated                    // Creation date
	SortFieldUpdated                    // Last update date
	SortFieldTitle                      // Issue title (alphabetical)
	SortFieldStatus                     // Issue status
	SortFieldType                       // Issue type (epic, feature, task, ...)
	SortFieldDepsCount                  // Number of dependencies
	SortFieldPageRank                   // PageRank score
	NumSortFields                       // Sentinel: total number of sort fields
)

// String returns a human-readable label for the sort field.
func (f SortField) String() string {
	switch f {
	case SortFieldPriority:
		return "Priority"
	case SortFieldCreated:
		return "Created"
	case SortFieldUpdated:
		return "Updated"
	case SortFieldTitle:
		return "Title"
	case SortFieldStatus:
		return "Status"
	case SortFieldType:
		return "Type"
	case SortFieldDepsCount:
		return "Deps"
	case SortFieldPageRank:
		return "PageRank"
	default:
		return "Unknown"
	}
}

// DefaultDirection returns the natural default sort direction for this field.
func (f SortField) DefaultDirection() SortDirection {
	switch f {
	case SortFieldPriority:
		return SortAscending // P0 first
	case SortFieldTitle:
		return SortAscending // A-Z
	case SortFieldStatus:
		return SortAscending // open before closed
	case SortFieldType:
		return SortAscending // epic before chore
	default:
		return SortDescending // newest/highest first for dates, counts, scores
	}
}

// SortDirection represents ascending or descending sort order (bd-x3l).
type SortDirection int

const (
	SortAscending  SortDirection = iota // ▲ ascending
	SortDescending                      // ▼ descending
)

// String returns a human-readable label for the sort direction.
func (d SortDirection) String() string {
	if d == SortAscending {
		return "Ascending"
	}
	return "Descending"
}

// Indicator returns the arrow indicator for the sort direction.
func (d SortDirection) Indicator() string {
	if d == SortAscending {
		return "▲"
	}
	return "▼"
}

// Toggle returns the opposite direction.
func (d SortDirection) Toggle() SortDirection {
	if d == SortAscending {
		return SortDescending
	}
	return SortAscending
}

// UpdateMsg is sent when a new version is available
type UpdateMsg struct {
	TagName string
	URL     string
}

// FileChangedMsg is sent when the beads file changes on disk
type FileChangedMsg struct{}

// semanticDebounceTickMsg is sent after debounce delay to trigger semantic computation
type semanticDebounceTickMsg struct{}

// workerPollTickMsg drives a small background-mode status refresh (spinner + freshness) (bv-9nfy).
type workerPollTickMsg struct{}

// PickerRefreshTickMsg triggers a periodic refresh of project picker counts (bd-8yc).
type PickerRefreshTickMsg struct{}

var workerSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	freshnessErrorRetries = 3
)

func freshnessWarnThreshold() time.Duration {
	return envDurationSeconds("B9S_FRESHNESS_WARN_S", 30*time.Second)
}

func freshnessStaleThreshold() time.Duration {
	return envDurationSeconds("B9S_FRESHNESS_STALE_S", 2*time.Minute)
}

func workerPollTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return workerPollTickMsg{}
	})
}

// pickerRefreshTickCmd returns a command that fires a PickerRefreshTickMsg after 30s (bd-8yc).
func pickerRefreshTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return PickerRefreshTickMsg{}
	})
}

// ReadyTimeoutMsg is sent after a short delay to ensure the UI becomes ready
// even if the terminal doesn't send WindowSizeMsg promptly (bv-7wl7)
type ReadyTimeoutMsg struct{}

// ReadyTimeoutCmd returns a command that sends ReadyTimeoutMsg after 100ms.
// This ensures the TUI doesn't hang on "Initializing..." if the terminal
// is slow to report its size (common in tmux, SSH, some terminal emulators).
func ReadyTimeoutCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return ReadyTimeoutMsg{}
	})
}

// WatchFileCmd returns a command that waits for file changes and sends FileChangedMsg
func WatchFileCmd(w *watcher.Watcher) tea.Cmd {
	return func() tea.Msg {
		<-w.Changed()
		return FileChangedMsg{}
	}
}

// StartBackgroundWorkerCmd starts the background worker and triggers an initial refresh.
func StartBackgroundWorkerCmd(w *BackgroundWorker) tea.Cmd {
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		if err := w.Start(); err != nil {
			return SnapshotErrorMsg{Err: fmt.Errorf("starting background worker: %w", err), Recoverable: false}
		}
		w.TriggerRefresh()
		return nil
	}
}

// WaitForBackgroundWorkerMsgCmd waits for the next BackgroundWorker message.
func WaitForBackgroundWorkerMsgCmd(w *BackgroundWorker) tea.Cmd {
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		select {
		case msg := <-w.Messages():
			return msg
		case <-w.Done():
			return nil
		}
	}
}

// CheckUpdateCmd returns a command that checks for updates
func CheckUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		tag, url, err := updater.CheckForUpdates()
		if err == nil && tag != "" {
			return UpdateMsg{TagName: tag, URL: url}
		}
		return nil
	}
}

// Model is the main Bubble Tea model for b9s
type Model struct {
	// Data
	issues       []model.Issue
	pooledIssues []*model.Issue // Issue pool refs for sync reloads (return to pool on replace)
	issueMap     map[string]*model.Issue
	beadsPath    string           // Path to beads.jsonl for reloading
	watcher      *watcher.Watcher // File watcher for live reload

	// Background Worker (Phase 2 architecture - bv-m7v8)
	// snapshot is the current immutable data snapshot from BackgroundWorker.
	// Access is safe without locks because Bubble Tea ensures Update() and View()
	// don't run concurrently. When nil, the UI uses legacy m.issues/m.issueMap fields.
	snapshot *DataSnapshot
	// snapshotInitPending is true until we receive the first BackgroundWorker snapshot
	// (or an error), allowing a polished cold-start loading screen (bv-tspo).
	snapshotInitPending bool
	// backgroundWorker manages async data loading (nil if background mode disabled)
	backgroundWorker *BackgroundWorker
	workerSpinnerIdx int // Spinner frame for background worker activity (bv-9nfy)
	lastForceRefresh time.Time

	// UI Components
	list             list.Model
	viewport         viewport.Model
	renderer         *MarkdownRenderer
	board BoardModel
	tree  TreeModel // Hierarchical tree view (bv-gllx)
	theme            Theme

	// Update State
	updateAvailable bool
	updateTag       string
	updateURL       string

	// Focus and View State
	focused              focus
	focusBeforeHelp      focus   // Stores focus before opening help overlay
	treeViewActive       bool    // True when tree view is the active left pane (bd-xfd)
	treeDetailHidden     bool    // True when detail panel is hidden in tree view (bd-80u)
	detailHiddenByNarrow bool    // True when detail was auto-hidden due to narrow window (bd-6eg)
	isSplitView          bool
	splitPaneRatio       float64 // Ratio of list pane width (0.2-0.8), default 0.4
	isBoardView          bool
	showDetails          bool
	showHelp             bool
	helpScroll           int // Scroll offset for help overlay
	showQuitConfirm      bool
	ready                bool
	width                int
	height               int
	pickerVisible bool // bd-2me: Shift+P toggles picker panel

	// Filter and sort state
	currentFilter string
	sortMode      SortMode // bv-3ita: current sort mode

	// Stats (cached)
	countOpen    int
	countReady   int
	countBlocked int
	countClosed  int

	// Label picker (bv-126)
	showLabelPicker bool
	labelPicker     LabelPickerModel

	// Status picker for quick status changes (bd-a83)
	showStatusPicker bool
	statusPicker     StatusPickerModel

	// Repo picker (workspace mode)
	showRepoPicker bool
	repoPicker     RepoPickerModel

	// Time-travel mode
	timeTravelMode   bool
	timeTravelSince  string
	newIssueIDs      map[string]bool // Issues in diff.NewIssues
	closedIssueIDs   map[string]bool // Issues in diff.ClosedIssues
	modifiedIssueIDs map[string]bool // Issues in diff.ModifiedIssues

	// Time-travel input prompt
	timeTravelInput      textinput.Model
	showTimeTravelPrompt bool

	// Status message (for temporary feedback)
	statusMsg     string
	statusIsError bool

	// Workspace mode state
	workspaceMode    bool            // True when viewing multiple repos
	availableRepos   []string        // List of repo prefixes available
	activeRepos      map[string]bool // Which repos are currently shown (nil = all)
	workspaceSummary string          // Summary text for footer (e.g., "3 repos")

	// Sprint view (bv-161)
	sprints []model.Sprint

	// Tutorial integration (bv-8y31)
	showTutorial  bool
	tutorialModel TutorialModel

	// Self-update modal (bv-182)
	showUpdateModal bool
	updateModal     UpdateModal

	// Issue writer for in-app editing (bd-a83)
	issueWriter *IssueWriter

	// Edit modal for full issue editing (bd-a83)
	showEditModal bool
	editModal     EditModal

	// Project switching (bd-q5z, bd-ey3)
	activeProjectName string            // Name of the currently loaded project
	activeProjectPath string            // Path to the project directory
	activeProjectFavN int               // Favorite number (1-9, or 0)
	appConfig         config.Config     // Loaded app configuration
	allProjects       []config.Project  // All known projects
	projectPicker     ProjectPickerModel
}

// labelCount is a simple label->count pair for display
type labelCount struct {
	Label string
	Count int
}

// bodyHeight returns the available height for the main content area,
// accounting for the picker header and footer (bd-ey3, bd-ylz, bd-2me).
func (m Model) bodyHeight() int {
	headerH := 1 // default: single-line global header when no projects
	if len(m.allProjects) > 0 && m.pickerVisible {
		headerH = m.projectPicker.Height()
	}
	h := m.height - headerH - 1 // -1 for footer
	if h < 3 {
		h = 3
	}
	return h
}

// currentViewName returns a human-readable name for the current view mode.
func (m Model) currentViewName() string {
	if m.isBoardView {
		return "board"
	}
	if m.treeViewActive || m.focused == focusTree {
		return "tree"
	}
	if m.isSplitView {
		return "split"
	}
	if m.showDetails {
		return "detail"
	}
	return "list"
}

// renderGlobalHeader renders the single-line global header bar.
// Format:  bw | projectname (1)      list view | ○12 ◉5 ◈3 ●2
func (m Model) renderGlobalHeader() string {
	// Left side: app name + project
	appName := lipgloss.NewStyle().Bold(true).Foreground(ColorText).Render("b9s")
	sep := lipgloss.NewStyle().Foreground(ColorMuted).Render(" | ")

	projectLabel := m.activeProjectName
	if projectLabel == "" {
		projectLabel = "untitled"
	}
	if m.activeProjectFavN > 0 {
		projectLabel = fmt.Sprintf("%s (%d)", projectLabel, m.activeProjectFavN)
	}
	projectSection := lipgloss.NewStyle().Foreground(ColorSubtext).Render(projectLabel)

	leftParts := appName + sep + projectSection

	// Right side: view name + stats
	viewLabel := lipgloss.NewStyle().Foreground(ColorSubtext).Render(m.currentViewName() + " view")

	openStyle := lipgloss.NewStyle().Foreground(ColorStatusOpen)
	readyStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	blockedStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	closedStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	statsContent := fmt.Sprintf("%s%d %s%d %s%d %s%d",
		openStyle.Render("○"), m.countOpen,
		readyStyle.Render("◉"), m.countReady,
		blockedStyle.Render("◈"), m.countBlocked,
		closedStyle.Render("●"), m.countClosed)

	rightParts := viewLabel + sep + statsContent

	// Calculate filler between left and right
	leftWidth := lipgloss.Width(leftParts)
	rightWidth := lipgloss.Width(rightParts)
	fillerWidth := m.width - leftWidth - rightWidth
	if fillerWidth < 1 {
		fillerWidth = 1
	}
	filler := lipgloss.NewStyle().Width(fillerWidth).Render("")

	headerBg := lipgloss.NewStyle().
		Width(m.width).
		Background(ColorBgHighlight)

	return headerBg.Render(leftParts + filler + rightParts)
}

// buildProjectEntries constructs the project picker display data from config.
func (m Model) buildProjectEntries() []ProjectEntry {
	entries := make([]ProjectEntry, 0, len(m.allProjects))
	for _, p := range m.allProjects {
		entry := ProjectEntry{
			Project:     p,
			FavoriteNum: m.appConfig.ProjectFavoriteNumber(p.Name),
			IsActive:    p.Name == m.activeProjectName,
		}
		// Load issue counts if this is the active project
		if entry.IsActive {
			entry.ReadyCount = m.countReady
			entry.BlockedCount = m.countBlocked
			// Count open vs in_progress separately (bd-o23v)
			for _, iss := range m.issues {
				if isClosedLikeStatus(iss.Status) {
					continue
				}
				switch {
				case iss.Status == "in_progress":
					entry.InProgressCount++
				case iss.Status == model.StatusBlocked:
					// already counted via m.countBlocked
				default:
					entry.OpenCount++
				}
			}
		} else {
			// Try to get counts from the project's beads file.
			// Use silent warning handler to avoid corrupting TUI with stderr output (bd-lll).
			// Count logic mirrors snapshot.go's counting (bd-qjc).
			beadsPath := filepath.Join(p.ResolvedPath(), ".beads", "issues.jsonl")
			silentOpts := loader.ParseOptions{WarningHandler: func(string) {}}
			if issues, err := loader.LoadIssuesFromFileWithOptions(beadsPath, silentOpts); err == nil {
				// Build issue map for dependency resolution
				issMap := make(map[string]model.Issue, len(issues))
				for _, iss := range issues {
					issMap[iss.ID] = iss
				}
				for _, iss := range issues {
					if isClosedLikeStatus(iss.Status) {
						continue
					}
					// Count open vs in_progress separately (bd-o23v)
					switch {
					case iss.Status == "in_progress":
						entry.InProgressCount++
					case iss.Status == model.StatusBlocked:
						entry.BlockedCount++
						continue // blocked issues can't be ready
					default:
						entry.OpenCount++
					}
					// Check if blocked by open dependencies
					isBlocked := false
					for _, dep := range iss.Dependencies {
						if dep == nil || !dep.Type.IsBlocking() {
							continue
						}
						if blocker, exists := issMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
							isBlocked = true
							break
						}
					}
					if !isBlocked {
						entry.ReadyCount++
					}
				}
			}
		}
		entries = append(entries, entry)
	}

	// Auto-number projects 1-9 when no favorites are configured (bd-8zc)
	hasFavorites := false
	for _, e := range entries {
		if e.FavoriteNum > 0 {
			hasFavorites = true
			break
		}
	}
	if !hasFavorites {
		for i := range entries {
			if i >= 9 {
				break
			}
			entries[i].FavoriteNum = i + 1
		}
	}

	return entries
}

// filterIssuesByLabel returns issues that contain the given label (case-sensitive match)
func (m Model) filterIssuesByLabel(label string) []model.Issue {
	var out []model.Issue
	for _, iss := range m.issues {
		for _, l := range iss.Labels {
			if l == label {
				out = append(out, iss)
				break
			}
		}
	}
	return out
}

// WorkspaceInfo contains workspace loading metadata for TUI display
type WorkspaceInfo struct {
	Enabled      bool
	RepoCount    int
	FailedCount  int
	TotalIssues  int
	RepoPrefixes []string
}

func (m *Model) updateListDelegate() {
	m.list.SetDelegate(IssueDelegate{
		Theme:         m.theme,
		WorkspaceMode: m.workspaceMode,
	})
}

// NewModel creates a new Model from the given issues
// beadsPath is the path to the beads.jsonl file for live reload support
func NewModel(issues []model.Issue, beadsPath string) Model {
	// Default Sort: creation date descending (newest first) (bd-ctu)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.After(issues[j].CreatedAt) // Newest first
	})

	// Build lookup map
	issueMap := make(map[string]*model.Issue, len(issues))

	// Build list items
	items := make([]list.Item, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]

		items[i] = IssueItem{
			Issue:      issues[i],
			RepoPrefix: ExtractRepoPrefix(issues[i].ID),
		}
	}

	// Compute stats
	cOpen, cReady, cBlocked, cClosed := 0, 0, 0, 0
	for i := range issues {
		issue := &issues[i]
		if isClosedLikeStatus(issue.Status) {
			cClosed++
			continue
		}

		cOpen++
		if issue.Status == model.StatusBlocked {
			cBlocked++
			continue
		}

		// Check if blocked by open dependencies
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			cReady++
		}
	}

	// Theme
	theme := DefaultTheme(lipgloss.NewRenderer(os.Stdout))

	// Default dimensions for immediate ready state (updated when WindowSizeMsg arrives)
	// This eliminates the "Initializing..." phase entirely, fixing slow startup issues
	// in tmux, SSH, and slow terminal emulators where the terminal may delay sending size.
	const defaultWidth = 120
	const defaultHeight = 40

	// List setup - initialize with default dimensions so UI is immediately usable
	delegate := IssueDelegate{Theme: theme, WorkspaceMode: false}
	l := list.New(items, delegate, defaultWidth, defaultHeight-3)
	l.Title = ""
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	// Clear all default styles that might add extra lines
	l.Styles.Title = lipgloss.NewStyle()
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(theme.Primary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(theme.Primary)
	l.Styles.StatusBar = lipgloss.NewStyle()
	l.Styles.StatusEmpty = lipgloss.NewStyle()
	l.Styles.StatusBarActiveFilter = lipgloss.NewStyle()
	l.Styles.StatusBarFilterCount = lipgloss.NewStyle()
	l.Styles.NoItems = lipgloss.NewStyle()
	l.Styles.PaginationStyle = lipgloss.NewStyle()
	l.Styles.HelpStyle = lipgloss.NewStyle()

	// Theme-aware markdown renderer
	renderer := NewMarkdownRendererWithTheme(80, theme)

	// Initialize viewport with default dimensions
	vp := viewport.New(defaultWidth, defaultHeight-2)

	// Initialize sub-components
	board := NewBoardModel(issues, theme)

	// Initialize label picker (bv-126)
	labelSet := make(map[string]int)
	for _, iss := range issues {
		for _, lbl := range iss.Labels {
			labelSet[lbl]++
		}
	}
	labels := make([]string, 0, len(labelSet))
	for lbl := range labelSet {
		labels = append(labels, lbl)
	}
	sort.Strings(labels)
	labelPicker := NewLabelPickerModel(labels, labelSet, theme)

	// Initialize file watcher for live reload
	var fileWatcher *watcher.Watcher
	var watcherErr error
	var backgroundWorker *BackgroundWorker
	var backgroundModeErr error
	backgroundModeRequested := false
	if v := strings.TrimSpace(os.Getenv("B9S_BACKGROUND_MODE")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			backgroundModeRequested = true
		case "0", "false", "no", "off":
			backgroundModeRequested = false
		}
	}

	if beadsPath != "" && backgroundModeRequested {
		bw, err := NewBackgroundWorker(WorkerConfig{
			BeadsPath:     beadsPath,
			DebounceDelay: 200 * time.Millisecond,
		})
		if err != nil {
			backgroundModeErr = err
		} else {
			backgroundWorker = bw
		}
	}

	if beadsPath != "" && backgroundWorker == nil {
		w, err := watcher.NewWatcher(beadsPath,
			watcher.WithDebounceDuration(200*time.Millisecond),
		)
		if err != nil {
			watcherErr = err
		} else if err := w.Start(); err != nil {
			watcherErr = err
		} else {
			fileWatcher = w
		}
	}

	// Build initial status message if watcher failed
	var initialStatus string
	var initialStatusErr bool
	if backgroundWorker != nil {
		initialStatus = "Background mode enabled"
		initialStatusErr = false
	} else if backgroundModeRequested && backgroundModeErr != nil {
		initialStatus = fmt.Sprintf("Background mode unavailable: %v (using sync reload)", backgroundModeErr)
		initialStatusErr = true
	} else if watcherErr != nil {
		initialStatus = fmt.Sprintf("Live reload unavailable: %v", watcherErr)
		initialStatusErr = true
	}

	// Tree view state should persist alongside the beads directory (e.g. BEADS_DIR overrides).
	treeModel := NewTreeModel(theme)
	if beadsPath != "" {
		treeModel.SetBeadsDir(filepath.Dir(beadsPath))
	}

	// Build tree and set size so tree view is ready on launch (bd-dxc)
	treeModel.Build(issues)
	treeModel.SetSize(defaultWidth, defaultHeight-2)

	return Model{
		issues:              issues,
		issueMap:            issueMap,
		beadsPath:           beadsPath,
		watcher:             fileWatcher,
		snapshotInitPending: backgroundWorker != nil,
		backgroundWorker:    backgroundWorker,
		list:                l,
		viewport:            vp,
		renderer:            renderer,
		board: board,
		tree:  treeModel,
		theme:               theme,
		currentFilter:       "all",
		focused:             focusTree, // Tree view is the default on launch (bd-dxc)
		treeViewActive:      true,      // Tree view is the default on launch (bd-dxc)
		treeDetailHidden:    true,      // Start tree-only; user presses 'd' to show detail (bd-x96a)
		splitPaneRatio:      0.4,       // Default: list pane gets 40% of width
		// Initialize as ready with default dimensions to eliminate "Initializing..." phase
		ready:         true,
		width:         defaultWidth,
		height:        defaultHeight,
		countOpen:     cOpen,
		countReady:    cReady,
		countBlocked:  cBlocked,
		countClosed:   cClosed,
		labelPicker:   labelPicker,
		statusMsg:     initialStatus,
		statusIsError: initialStatusErr,
		pickerVisible: true, // bd-2me: visible by default, Shift+P toggles
		// Tutorial integration (bv-8y31)
		tutorialModel: NewTutorialModel(theme),
		// Issue writer for in-app editing (bd-a83)
		issueWriter: NewIssueWriter(),
	}
}

// WithConfig sets the application config and project info on the model.
// Call this after NewModel to enable project switching and favorites.
func (m Model) WithConfig(cfg config.Config, projectName, projectPath string) Model {
	m.appConfig = cfg
	m.activeProjectName = projectName
	m.activeProjectPath = projectPath
	m.activeProjectFavN = cfg.ProjectFavoriteNumber(projectName)
	projects, errs := config.DiscoverProjectsWithErrors(cfg)
	m.allProjects = projects
	for _, e := range errs {
		debug.Log("project discovery: skipping %s", e)
	}
	entries := m.buildProjectEntries()
	m.projectPicker = NewProjectPicker(entries, m.theme)
	return m
}

func (m Model) Init() tea.Cmd {
	// Note: ReadyTimeoutCmd is no longer needed since the model is now
	// initialized as ready with default dimensions in NewModel().
	// This eliminates the "Initializing..." phase entirely.
	cmds := []tea.Cmd{
		CheckUpdateCmd(),
	}
	if m.backgroundWorker != nil {
		cmds = append(cmds, StartBackgroundWorkerCmd(m.backgroundWorker))
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		cmds = append(cmds, workerPollTickCmd())
	} else if m.watcher != nil {
		cmds = append(cmds, WatchFileCmd(m.watcher))
	}
	// Start periodic picker refresh for non-active project counts (bd-8yc)
	if len(m.allProjects) > 1 {
		cmds = append(cmds, pickerRefreshTickCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	var listKeyConsumed bool // set by handleListKeys when key was handled (bd-kob)

	if m.backgroundWorker != nil {
		switch msg.(type) {
		case tea.KeyMsg:
			m.backgroundWorker.recordActivity()
		}
	}

	// Handle edit modal before type switch: huh.Form needs to receive ALL
	// message types (not just tea.KeyMsg) for internal navigation (nextFieldMsg,
	// updateFieldMsg, etc.) to work correctly.
	if m.showEditModal {
		m.editModal, cmd = m.editModal.Update(msg)
		cmds = append(cmds, cmd)
		if m.editModal.IsCancelRequested() {
			m.showEditModal = false
			return m, tea.Batch(cmds...)
		}
		if m.editModal.IsSaveRequested() {
			m.showEditModal = false
			if m.editModal.isCreateMode {
				args := m.editModal.BuildCreateArgs()
				if len(args) > 0 {
					cmds = append(cmds, m.issueWriter.CreateIssue(args))
				}
			} else {
				args := m.editModal.BuildUpdateArgs()
				if len(args) > 0 {
					cmds = append(cmds, m.issueWriter.UpdateIssue(m.editModal.issueID, args))
				}
			}
			return m, tea.Batch(cmds...)
		}
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case UpdateMsg:
		m.updateAvailable = true
		m.updateTag = msg.TagName
		m.updateURL = msg.URL

	case UpdateCompleteMsg:
		// Forward to the update modal
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)
		}

	case UpdateProgressMsg:
		// Forward to the update modal
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)
		}

	case BdResultMsg:
		// Handle bd CLI operation results (bd-a83)
		if msg.Success {
			m.statusMsg = fmt.Sprintf("Updated %s", msg.IssueID)
			if msg.Operation == BdOpCreate {
				m.statusMsg = fmt.Sprintf("Created %s", msg.IssueID)
			} else if msg.Operation == BdOpClose {
				m.statusMsg = fmt.Sprintf("Closed %s", msg.IssueID)
			}
			m.statusIsError = false
			// Trigger reload to pick up changes
			cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
		} else {
			errMsg := "bd command failed"
			if msg.Error != nil {
				errMsg = msg.Error.Error()
			}
			m.statusMsg = errMsg
			m.statusIsError = true
		}

	case ReadyTimeoutMsg:
		// bv-7wl7: Legacy fallback handler (no longer used).
		// The model is now initialized as ready with default dimensions in NewModel(),
		// so this handler should never execute. Kept for backwards compatibility.
		if !m.ready {
			m.width = 120
			m.height = 40
			m.ready = true
			m.list.SetSize(m.width, m.height-3)
			m.viewport = viewport.New(m.width, m.bodyHeight())
		}

	case PickerRefreshTickMsg:
		// Periodic refresh of project picker counts (bd-8yc)
		if len(m.allProjects) > 0 {
			entries := m.buildProjectEntries()
			m.projectPicker = NewProjectPicker(entries, m.theme)
			m.projectPicker.SetSize(m.width, m.height)
		}
		return m, pickerRefreshTickCmd()

	case workerPollTickMsg:
		if m.backgroundWorker != nil {
			state := m.backgroundWorker.State()
			if state == WorkerProcessing {
				m.workerSpinnerIdx = (m.workerSpinnerIdx + 1) % len(workerSpinnerFrames)
			} else {
				m.workerSpinnerIdx = 0
			}
			if state != WorkerStopped {
				cmds = append(cmds, workerPollTickCmd())
			}
		}

	case SnapshotReadyMsg:
		// Background worker has a new snapshot ready (bv-m7v8)
		// This is the atomic pointer swap - O(1), sub-microsecond
		if msg.Snapshot == nil {
			if m.backgroundWorker != nil {
				return m, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker)
			}
			return m, nil
		}

		firstSnapshot := m.snapshotInitPending && m.snapshot == nil
		m.snapshotInitPending = false

		// Store selected issue ID to restore position after swap
		var selectedID string
		if sel := m.list.SelectedItem(); sel != nil {
			if item, ok := sel.(IssueItem); ok {
				selectedID = item.Issue.ID
			}
		}

		// Preserve board selection by issue ID (bv-6n4c).
		var boardSelectedID string
		if m.focused == focusBoard {
			if sel := m.board.SelectedIssue(); sel != nil {
				boardSelectedID = sel.ID
			}
		}

		oldSnapshot := m.snapshot

		// Swap snapshot pointer
		m.snapshot = msg.Snapshot
		if m.backgroundWorker != nil {
			latencyStart := msg.FileChangeAt
			if latencyStart.IsZero() {
				latencyStart = msg.SentAt
			}
			if !latencyStart.IsZero() {
				m.backgroundWorker.recordUIUpdateLatency(time.Since(latencyStart))
			}
		}
		if oldSnapshot != nil && len(oldSnapshot.pooledIssues) > 0 {
			go loader.ReturnIssuePtrsToPool(oldSnapshot.pooledIssues)
		}

		// Update legacy fields for backwards compatibility during migration
		// Eventually these will be removed when all code reads from snapshot
		m.issues = msg.Snapshot.Issues
		m.issueMap = msg.Snapshot.IssueMap
		m.countOpen = msg.Snapshot.CountOpen
		m.countReady = msg.Snapshot.CountReady
		m.countBlocked = msg.Snapshot.CountBlocked
		m.countClosed = msg.Snapshot.CountClosed
		if len(m.pooledIssues) > 0 {
			go loader.ReturnIssuePtrsToPool(m.pooledIssues)
			m.pooledIssues = nil
		}

		// Update list/board views while preserving the current filter state.
		var filteredItems []list.Item
		var filteredIssues []model.Issue

		filteredItems = make([]list.Item, 0, len(msg.Snapshot.ListItems))
		filteredIssues = make([]model.Issue, 0, len(msg.Snapshot.ListItems))

		for _, item := range msg.Snapshot.ListItems {
			issue := item.Issue

			// Workspace repo filter (nil = all repos)
			if m.workspaceMode && m.activeRepos != nil {
				repoKey := strings.ToLower(item.RepoPrefix)
				if repoKey != "" && !m.activeRepos[repoKey] {
					continue
				}
			}

			include := false
			switch m.currentFilter {
			case "all":
				include = true
			case "open":
				include = !isClosedLikeStatus(issue.Status)
			case "closed":
				include = isClosedLikeStatus(issue.Status)
			case "ready":
				if !isClosedLikeStatus(issue.Status) && issue.Status != model.StatusBlocked {
					isBlocked := false
					for _, dep := range issue.Dependencies {
						if dep == nil || !dep.Type.IsBlocking() {
							continue
						}
						if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
							isBlocked = true
							break
						}
					}
					include = !isBlocked
				}
			default:
				if strings.HasPrefix(m.currentFilter, "label:") {
					label := strings.TrimPrefix(m.currentFilter, "label:")
					for _, l := range issue.Labels {
						if l == label {
							include = true
							break
						}
					}
				}
			}

			if include {
				filteredItems = append(filteredItems, item)
				filteredIssues = append(filteredIssues, issue)
			}
		}

		m.sortFilteredItems(filteredItems, filteredIssues)
		m.list.SetItems(filteredItems)
		if m.snapshot != nil && m.snapshot.BoardState != nil && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues) {
			m.board.SetSnapshot(m.snapshot)
		} else {
			m.board.SetIssues(filteredIssues)
		}

		// Restore selection if possible
		if selectedID != "" {
			for i, it := range filteredItems {
				if item, ok := it.(IssueItem); ok && item.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		// Keep selection in bounds
		if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
			m.list.Select(0)
		}

		// Restore board selection after SetIssues rebuilds columns (bv-6n4c).
		if boardSelectedID != "" {
			_ = m.board.SelectIssueByID(boardSelectedID)
		}

		// Always rebuild tree from the new snapshot (bd-qjc).
		// The tree is visible in all non-overlay states (it's the default view),
		// so it must stay in sync even when picker or detail has focus.
		m.tree.BuildFromSnapshot(m.snapshot)
		m.tree.SetSize(m.width, m.bodyHeight())
		m.tree.SetGlobalIssueMap(m.issueMap)

		// Refresh detail pane if visible
		if m.isSplitView || m.showDetails {
			m.updateViewportContent()
		}

		// Rebuild picker entries with updated counts (bd-ey3)
		if len(m.allProjects) > 0 {
			pickerEntries := m.buildProjectEntries()
			m.projectPicker = NewProjectPicker(pickerEntries, m.theme)
			m.projectPicker.SetSize(m.width, m.height)
		}

		if firstSnapshot {
			if msg.Snapshot.LoadWarningCount > 0 {
				m.statusMsg = fmt.Sprintf("Loaded %d issues (%d warnings)", len(m.issues), msg.Snapshot.LoadWarningCount)
			} else {
				m.statusMsg = ""
			}
		} else if msg.Snapshot.LoadWarningCount > 0 {
			m.statusMsg = fmt.Sprintf("Reloaded %d issues (%d warnings)", len(m.issues), msg.Snapshot.LoadWarningCount)
		} else {
			m.statusMsg = fmt.Sprintf("Reloaded %d issues", len(m.issues))
		}
		m.statusIsError = false

		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}

		return m, tea.Batch(cmds...)

	case SnapshotErrorMsg:
		// Background worker encountered an error loading/processing data
		// If recoverable, we'll try again on next file change.
		if m.snapshotInitPending && m.snapshot == nil {
			m.snapshotInitPending = false
		}
		if msg.Err != nil {
			if msg.Recoverable {
				m.statusMsg = fmt.Sprintf("Background reload error (will retry): %v", msg.Err)
			} else {
				m.statusMsg = fmt.Sprintf("Background reload error: %v", msg.Err)
			}
			m.statusIsError = true
		}
		if m.backgroundWorker != nil {
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
		}
		return m, tea.Batch(cmds...)

	case SwitchProjectMsg:
		// Skip if already on this project (bd-3eh)
		if msg.Project.Name == m.activeProjectName {
			return m, nil
		}
		// Switch to a different project (bd-q5z, bd-ey3, bd-87w)
		m.activeProjectName = msg.Project.Name
		m.activeProjectPath = msg.Project.ResolvedPath()
		m.activeProjectFavN = m.appConfig.ProjectFavoriteNumber(msg.Project.Name)
		// Determine new beads path
		beadsDir := filepath.Join(msg.Project.ResolvedPath(), ".beads")
		newPath, err := loader.FindJSONLPath(beadsDir)
		if err != nil {
			m.statusMsg = fmt.Sprintf("No beads found in %s", msg.Project.Name)
			m.statusIsError = true
			return m, nil
		}
		// Stop background worker and old watcher (bd-87w)
		if m.backgroundWorker != nil {
			m.backgroundWorker.Stop()
			m.backgroundWorker = nil
		}
		if m.watcher != nil {
			m.watcher.Stop()
			m.watcher = nil
		}
		m.beadsPath = newPath
		// Clear old project data to prevent stale rendering (bd-lll)
		m.issues = nil
		m.issueMap = nil
		m.snapshot = nil
		m.countOpen, m.countReady, m.countBlocked, m.countClosed = 0, 0, 0, 0
		// Clear tree filter/search state so new project data isn't hidden (bd-qjc)
		m.tree.ApplyFilter("all")
		m.tree.ClearSearch()
		m.tree.Build(nil)
		// Start new background worker for the new path (bd-87w, bd-828)
		// BackgroundWorker creates its own internal file watcher.
		bw, bwErr := NewBackgroundWorker(WorkerConfig{BeadsPath: newPath})
		if bwErr == nil {
			m.backgroundWorker = bw
			// Use StartBackgroundWorkerCmd which calls Start() + TriggerRefresh()
			cmds = append(cmds, StartBackgroundWorkerCmd(bw))
			cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(bw))
		} else {
			// Fallback: no background worker, use watcher + FileChangedMsg
			w, watchErr := watcher.NewWatcher(newPath)
			if watchErr == nil {
				m.watcher = w
				cmds = append(cmds, WatchFileCmd(w))
			}
			cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
		}
		m.statusMsg = fmt.Sprintf("Switched to %s", msg.Project.Name)
		m.statusIsError = false
		// Rebuild picker entries to reflect new active project (bd-ey3)
		entries := m.buildProjectEntries()
		m.projectPicker = NewProjectPicker(entries, m.theme)
		m.projectPicker.SetSize(m.width, m.height)
		return m, tea.Batch(cmds...)

	case ToggleFavoriteMsg:
		// Toggle favorite slot for a project (bd-q5z)
		m.appConfig.SetFavorite(msg.SlotNumber, msg.ProjectName)
		// Update current project's favorite number if it changed
		if msg.ProjectName == m.activeProjectName {
			m.activeProjectFavN = m.appConfig.ProjectFavoriteNumber(m.activeProjectName)
		}
		// Save config
		_ = config.Save(m.appConfig)
		// Refresh picker entries (always visible now, bd-ey3)
		entries := m.buildProjectEntries()
		m.projectPicker = NewProjectPicker(entries, m.theme)
		m.projectPicker.SetSize(m.width, m.height)
		return m, nil

	case FileChangedMsg:
		// File changed on disk - reload issues
		// In background mode the BackgroundWorker owns file watching and snapshot building.
		if m.backgroundWorker != nil {
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		if m.beadsPath == "" {
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		reloadStart := time.Now()
		profileRefresh := debug.Enabled()
		var refreshTimings map[string]time.Duration
		recordTiming := func(name string, d time.Duration) {
			if !profileRefresh {
				return
			}
			if refreshTimings == nil {
				refreshTimings = make(map[string]time.Duration, 12)
			}
			refreshTimings[name] = d
			debug.LogTiming("refresh."+name, d)
		}
		if profileRefresh {
			debug.Log("refresh: file change detected path=%s", m.beadsPath)
		}

		// Reload issues from disk
		var reloadWarnings []string
		var loadStart time.Time
		if profileRefresh {
			loadStart = time.Now()
		}
		loadedIssues, err := loader.LoadIssuesFromFileWithOptionsPooled(m.beadsPath, loader.ParseOptions{
			WarningHandler: func(msg string) {
				reloadWarnings = append(reloadWarnings, msg)
			},
			BufferSize: envMaxLineSizeBytes(),
		})
		if profileRefresh {
			recordTiming("load_issues", time.Since(loadStart))
		}
		if err != nil {
			m.statusMsg = fmt.Sprintf("Reload error: %v", err)
			m.statusIsError = true
			if m.watcher != nil {
				cmds = append(cmds, WatchFileCmd(m.watcher))
			}
			return m, tea.Batch(cmds...)
		}
		if len(m.pooledIssues) > 0 {
			loader.ReturnIssuePtrsToPool(m.pooledIssues)
		}
		m.pooledIssues = loadedIssues.PoolRefs
		newIssues := loadedIssues.Issues

		// Store selected issue ID to restore position after reload
		var selectedID string
		if sel := m.list.SelectedItem(); sel != nil {
			if item, ok := sel.(IssueItem); ok {
				selectedID = item.Issue.ID
			}
		}

		// Apply default sorting: creation date descending (newest first) (bd-ctu)
		var sortStart time.Time
		if profileRefresh {
			sortStart = time.Now()
		}
		sort.Slice(newIssues, func(i, j int) bool {
			return newIssues[i].CreatedAt.After(newIssues[j].CreatedAt)
		})
		if profileRefresh {
			recordTiming("sort_issues", time.Since(sortStart))
		}

		m.issues = newIssues

		// Rebuild lookup map
		var mapStart time.Time
		if profileRefresh {
			mapStart = time.Now()
		}
		m.issueMap = make(map[string]*model.Issue, len(newIssues))
		for i := range m.issues {
			m.issueMap[m.issues[i].ID] = &m.issues[i]
		}
		if profileRefresh {
			recordTiming("issue_map", time.Since(mapStart))
		}

		// Recompute stats
		var statsStart time.Time
		if profileRefresh {
			statsStart = time.Now()
		}
		m.countOpen, m.countReady, m.countBlocked, m.countClosed = 0, 0, 0, 0
		for i := range m.issues {
			issue := &m.issues[i]
			if isClosedLikeStatus(issue.Status) {
				m.countClosed++
				continue
			}
			m.countOpen++
			if issue.Status == model.StatusBlocked {
				m.countBlocked++
				continue
			}
			isBlocked := false
			for _, dep := range issue.Dependencies {
				if dep == nil || !dep.Type.IsBlocking() {
					continue
				}
				if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
					isBlocked = true
					break
				}
			}
			if !isBlocked {
				m.countReady++
			}
		}
		if profileRefresh {
			recordTiming("counts", time.Since(statsStart))
		}

		// Rebuild list items
		var listStart time.Time
		if profileRefresh {
			listStart = time.Now()
		}
		items := make([]list.Item, len(m.issues))
		for i := range m.issues {
			items[i] = IssueItem{
				Issue:      m.issues[i],
				RepoPrefix: ExtractRepoPrefix(m.issues[i].ID),
			}
		}
		if profileRefresh {
			recordTiming("list_items", time.Since(listStart))
		}
		m.list.SetItems(items)

		// Restore selection position
		if selectedID != "" {
			for i, item := range m.list.Items() {
				if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		if m.isBoardView {
			var graphStart time.Time
			if profileRefresh {
				graphStart = time.Now()
			}
			m.refreshBoardAndGraphForCurrentFilter()
			if profileRefresh {
				recordTiming("board_graph", time.Since(graphStart))
			}
		}

		// Rebuild tree view if focused (bd-byp).
		if m.focused == focusTree {
			var treeStart time.Time
			if profileRefresh {
				treeStart = time.Now()
			}
			m.tree.Build(m.issues)
			m.tree.SetSize(m.width, m.bodyHeight())
			m.tree.SetGlobalIssueMap(m.issueMap)
			if profileRefresh {
				recordTiming("tree_rebuild", time.Since(treeStart))
			}
		}

		m.statusMsg = fmt.Sprintf("Reloaded %d issues", len(newIssues))
		if len(reloadWarnings) > 0 {
			m.statusMsg += fmt.Sprintf(" (%d warnings)", len(reloadWarnings))
		}
		reloadDuration := time.Since(reloadStart)
		if profileRefresh {
			recordTiming("total", reloadDuration)
		}
		if reloadDuration >= 500*time.Millisecond {
			m.statusMsg += fmt.Sprintf(" in %s", formatReloadDuration(reloadDuration))
		}
		if profileRefresh && len(refreshTimings) > 0 {
			addTiming := func(label, key string) {
				if d, ok := refreshTimings[key]; ok && d > 0 {
					m.statusMsg += fmt.Sprintf(" %s=%s", label, formatReloadDuration(d))
				}
			}
			m.statusMsg += " [debug"
			addTiming("load", "load_issues")
			addTiming("sort", "sort_issues")
			addTiming("list", "list_items")
			addTiming("graph", "board_graph")
			addTiming("tree", "tree_rebuild")
			addTiming("total", "total")
			m.statusMsg += "]"
		}
		// Auto-enable background mode after slow sync reloads (opt-out via B9S_BACKGROUND_MODE=0).
		autoEnabled := false
		slowReload := reloadDuration >= time.Second
		if slowReload && m.backgroundWorker == nil && m.beadsPath != "" {
			autoAllowed := true
			if v := strings.TrimSpace(os.Getenv("B9S_BACKGROUND_MODE")); v != "" {
				switch strings.ToLower(v) {
				case "0", "false", "no", "off":
					autoAllowed = false
				}
			}
			if autoAllowed {
				bw, err := NewBackgroundWorker(WorkerConfig{
					BeadsPath:     m.beadsPath,
					DebounceDelay: 200 * time.Millisecond,
				})
				if err == nil {
					if m.watcher != nil {
						m.watcher.Stop()
					}
					m.watcher = nil
					m.backgroundWorker = bw
					m.snapshotInitPending = true
					autoEnabled = true
					cmds = append(cmds, StartBackgroundWorkerCmd(m.backgroundWorker))
					cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
				} else {
					m.statusMsg += fmt.Sprintf("; background mode unavailable: %v", err)
				}
			}
		}
		if slowReload {
			if autoEnabled {
				m.statusMsg += "; background mode auto-enabled"
			} else {
				m.statusMsg += "; consider B9S_BACKGROUND_MODE=1"
			}
		}
		m.statusIsError = false
		m.updateViewportContent()

		// Re-start watching for next change
		if m.watcher != nil && !autoEnabled {
			cmds = append(cmds, WatchFileCmd(m.watcher))
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		// Clear status message on any keypress
		m.statusMsg = ""
		m.statusIsError = false

		// Handle status picker modal (bd-a83)
		if m.showStatusPicker {
			switch msg.String() {
			case "j", "down":
				m.statusPicker.MoveDown()
			case "k", "up":
				m.statusPicker.MoveUp()
			case "enter":
				// Apply status change
				selected := m.statusPicker.SelectedStatus()
				if issue := m.getSelectedIssue(); issue != nil && selected != "" {
					cmds = append(cmds, m.issueWriter.SetStatus(issue.ID, selected))
				}
				m.showStatusPicker = false
			case "esc", "q":
				m.showStatusPicker = false
			}
			return m, tea.Batch(cmds...)
		}

		// Edit modal is handled before the type switch (needs all msg types for huh.Form)

		// Handle self-update modal (bv-182)
		if m.showUpdateModal {
			m.updateModal, cmd = m.updateModal.Update(msg)
			cmds = append(cmds, cmd)

			// Handle modal state changes
			switch msg.String() {
			case "esc", "q":
				// Always allow escape to close
				if !m.updateModal.IsInProgress() {
					m.showUpdateModal = false
					m.focused = focusTree
					return m, tea.Batch(cmds...)
				}
			case "enter":
				// Close on enter if complete or if cancelled
				if m.updateModal.IsComplete() {
					m.showUpdateModal = false
					m.focused = focusTree
					return m, tea.Batch(cmds...)
				}
				// If confirming and cancelled, close
				if m.updateModal.IsConfirming() && m.updateModal.IsCancelled() {
					m.showUpdateModal = false
					m.focused = focusTree
					return m, tea.Batch(cmds...)
				}
			case "n", "N":
				// Quick cancel
				if m.updateModal.IsConfirming() {
					m.showUpdateModal = false
					m.focused = focusTree
					return m, tea.Batch(cmds...)
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Handle repo picker overlay (workspace mode) before global keys (esc/q/etc.)
		if m.showRepoPicker {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			m = m.handleRepoPickerKeys(msg)
			return m, nil
		}

		// Handle quit confirmation first
		if m.showQuitConfirm {
			switch msg.String() {
			case "esc", "y", "Y":
				return m, tea.Quit
			default:
				m.showQuitConfirm = false
				m.focused = focusTree
				return m, nil
			}
		}

		// Handle help overlay toggle (? or F1)
		if (msg.String() == "?" || msg.String() == "f1") && m.list.FilterState() != list.Filtering {
			m.showHelp = !m.showHelp
			if m.showHelp {
				m.focusBeforeHelp = m.focused // Store current focus before switching to help
				m.focused = focusHelp
				m.helpScroll = 0 // Reset scroll position when opening help
			} else {
				m.focused = m.restoreFocusFromHelp()
			}
			return m, nil
		}

		// Handle tutorial toggle (backtick `) - bv-8y31
		if msg.String() == "`" && m.list.FilterState() != list.Filtering {
			m.showTutorial = !m.showTutorial
			if m.showTutorial {
				m.showHelp = false // Close help if open
				m.tutorialModel.SetSize(m.width, m.height)
				m.focused = focusTutorial
			} else {
				m.focused = focusTree
			}
			return m, nil
		}

		// Force refresh (bv-4auz): Ctrl+R / F5 triggers an immediate reload.
		if (msg.String() == "ctrl+r" || msg.String() == "f5") && m.list.FilterState() != list.Filtering {
			now := time.Now()
			if !m.lastForceRefresh.IsZero() && now.Sub(m.lastForceRefresh) < time.Second {
				return m, nil
			}
			m.lastForceRefresh = now

			m.statusMsg = "Refreshing…"
			m.statusIsError = false

			if m.backgroundWorker != nil {
				m.backgroundWorker.ForceRefresh()
				cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(m.backgroundWorker))
				return m, tea.Batch(cmds...)
			}

			if m.beadsPath == "" && m.watcher == nil {
				m.statusMsg = "Refresh unavailable"
				m.statusIsError = true
				return m, nil
			}

			cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
			return m, tea.Batch(cmds...)
		}

		// Handle Shift+P to toggle project picker panel (bd-2me)
		if msg.String() == "P" && m.list.FilterState() != list.Filtering {
			m.pickerVisible = !m.pickerVisible
			// Resize tree/board after toggling to reclaim/yield space
			m.tree.SetSize(m.width, m.bodyHeight())
			return m, nil
		}

		// If help is showing, handle navigation keys for scrolling
		if m.focused == focusHelp {
			m = m.handleHelpKeys(msg)
			return m, nil
		}

		// If tutorial is showing, route input to tutorial model (bv-8y31)
		if m.focused == focusTutorial && m.showTutorial {
			var tutorialCmd tea.Cmd
			m.tutorialModel, tutorialCmd = m.tutorialModel.Update(msg)
			// Check if tutorial wants to close
			if m.tutorialModel.ShouldClose() {
				m.showTutorial = false
				m.focused = focusTree
				m.tutorialModel = NewTutorialModel(m.theme) // Reset for next time
			}
			return m, tutorialCmd
		}

		// Project switching keys (bd-8hw.3, bd-8zc) - number keys 1-9 ALWAYS switch regardless of focus
		// Handled at top priority so they work from any view/state.
		// First checks config favorites, then falls back to positional numbering.
		if key := msg.String(); len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			n := int(key[0] - '0')
			// Check config favorites first
			if proj := m.appConfig.FavoriteProject(n); proj != nil {
				return m, func() tea.Msg { return SwitchProjectMsg{Project: *proj} }
			}
			// Auto-number fallback: 1-N maps to project list order (bd-8zc)
			idx := n - 1
			if idx < len(m.allProjects) {
				proj := m.allProjects[idx]
				return m, func() tea.Msg { return SwitchProjectMsg{Project: proj} }
			}
		}

		// Handle keys when not filtering
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit

			case "q":
				// q closes current view or quits if at top level
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					m.focused = focusTree
					return m, nil
				}
				if m.isBoardView {
					m.isBoardView = false
					m.focused = focusTree
					return m, nil
				}
				return m, tea.Quit

			case "esc":
				// Escape closes modals and goes back
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					m.focused = focusTree
					return m, nil
				}
				if m.isBoardView {
					m.isBoardView = false
					m.focused = focusTree
					return m, nil
				}
				// Close label picker if open (bv-126 fix)
				if m.showLabelPicker {
					m.showLabelPicker = false
					m.focused = focusTree
					return m, nil
				}
				// Detail mode: ESC returns to previous view (bd-80u, bd-yo4)
				if m.focused == focusDetail {
					if m.isBoardView {
						m.focused = focusBoard
					} else {
						m.focused = focusTree
					}
					return m, nil
				}
				// Tree view: sort popup escape (bd-u81) takes precedence
				if (m.treeViewActive || m.focused == focusTree) && m.tree.IsSortPopupOpen() {
					m.tree.CloseSortPopup()
					return m, nil
				}
				// Tree view: search mode escape (bd-wf8) takes precedence
				if (m.treeViewActive || m.focused == focusTree) && m.tree.IsSearchMode() {
					m.tree.ClearSearch()
					return m, nil
				}
				// Tree view: first ESC clears tree filter, second exits tree view (bd-kob)
				if m.treeViewActive || m.focused == focusTree {
					if m.tree.GetFilter() != "" && m.tree.GetFilter() != "all" {
						m.tree.ApplyFilter("all")
						m.syncTreeToDetail()
						return m, nil
					}
					// Tree view is permanent (bd-8hw.4), don't exit
				}
				// At main list - first ESC clears filters, second shows quit confirm
				if m.hasActiveFilters() {
					m.clearAllFilters()
					return m, nil
				}
				// No filters active - show quit confirmation
				m.showQuitConfirm = true
				m.focused = focusQuitConfirm
				return m, nil

			case "tab":
				// Tab is always fold (CycleNodeVisibility) — handled by view-specific keys (bd-lt1l)
				// Falls through to handleTreeKeys / handleBoardKeys / handleListKeys

			case "<":
				// Shrink list pane (move divider left)
				if m.isSplitView {
					m.splitPaneRatio -= 0.05
					if m.splitPaneRatio < 0.2 {
						m.splitPaneRatio = 0.2
					}
					m.recalculateSplitPaneSizes()
				}

			case ">":
				// Expand list pane (move divider right)
				if m.isSplitView {
					m.splitPaneRatio += 0.05
					if m.splitPaneRatio > 0.8 {
						m.splitPaneRatio = 0.8
					}
					m.recalculateSplitPaneSizes()
				}

			case "b":
				// Toggle board view from any context (bd-8hw.4: tree is permanent)
				m.isBoardView = !m.isBoardView
				if m.isBoardView {
					m.focused = focusBoard
					m.refreshBoardAndGraphForCurrentFilter()
				} else {
					m.focused = focusTree
				}
				return m, nil

			case "g":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 'g' for jump-to-top (bd-mwi)
				}

			case "a":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 'a' for all-filter (bd-mwi)
				}

			case "t", "E":
				// Tree view is always active (bd-8hw.4), t/E is a no-op from list focus
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 't' for in-tree use
				}
				// If somehow not in tree, switch to it
				m.isBoardView = false
				m.focused = focusTree
				return m, nil

			case "p":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 'p' for jump-to-parent (bd-ryu)
				}

			case "h":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 'h' for collapse/parent (bd-mwi)
				}

			case "e":
				if m.focused == focusTree {
					// Edit in tree view
					if issue := m.getSelectedIssue(); issue != nil {
						m.editModal = NewEditModal(issue, m.theme)
						m.editModal.SetSize(m.width, m.height)
						m.showEditModal = true
						return m, m.editModal.Init()
					}
					return m, nil
				}
				// For other views, pass through to their handlers

			case "ctrl+n":
				// Create new issue (bd-a83)
				m.editModal = NewCreateModal(m.theme)
				m.editModal.SetSize(m.width, m.height)
				m.showEditModal = true
				return m, m.editModal.Init()

			case "[":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle '[' for prev-sibling (bd-ryu)
				}

			case "]":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle ']' for next-sibling (bd-ryu)
				}

			case "w":
				// Toggle repo picker overlay (workspace mode)
				if !m.workspaceMode || len(m.availableRepos) == 0 {
					m.statusMsg = "Repo filter available only in workspace mode"
					m.statusIsError = false
					return m, nil
				}
				m.showRepoPicker = !m.showRepoPicker
				if m.showRepoPicker {
					m.repoPicker = NewRepoPickerModel(m.availableRepos, m.theme)
					m.repoPicker.SetActiveRepos(m.activeRepos)
					m.repoPicker.SetSize(m.width, m.height-1)
					m.focused = focusRepoPicker
				} else {
					m.focused = focusTree
				}
				return m, nil

			case "l":
				if m.focused == focusTree {
					break // Let handleTreeKeys handle 'l' for expand/child (bd-mwi)
				}
				// Open label picker for quick filter (bv-126)
				if len(m.issues) == 0 {
					return m, nil
				}
				// Update labels in case they changed
				labels := make([]string, 0)
				labelCounts := make(map[string]int)
				seen := make(map[string]bool)
				for _, issue := range m.issues {
					for _, lbl := range issue.Labels {
						labelCounts[lbl]++
						if !seen[lbl] {
							seen[lbl] = true
							labels = append(labels, lbl)
						}
					}
				}
				sort.Strings(labels)
				m.labelPicker.SetLabels(labels, labelCounts)
				m.labelPicker.Reset()
				m.labelPicker.SetSize(m.width, m.height-1)
				m.showLabelPicker = true
				m.focused = focusLabelPicker
				return m, nil

			}

			// Focus-specific key handling
			switch m.focused {
			case focusRepoPicker:
				m = m.handleRepoPickerKeys(msg)

			case focusLabelPicker:
				m = m.handleLabelPickerKeys(msg)

			case focusBoard:
				m = m.handleBoardKeys(msg)

			case focusTree:
				m = m.handleTreeKeys(msg)

			case focusList:
				// Handle priority quick-keys (bd-a83) before other list keys
				// These need access to cmds, so can't be in handleListKeys
				if !m.list.SettingFilter() && m.list.FilterState() != list.Filtering {
					switch msg.String() {
					case "1", "2", "3", "4":
						if issue := m.getSelectedIssue(); issue != nil {
							priority := int(msg.String()[0] - '0')
							cmds = append(cmds, m.issueWriter.SetPriority(issue.ID, priority))
							listKeyConsumed = true
						}
					}
				}
				if !listKeyConsumed {
					m, listKeyConsumed = m.handleListKeys(msg)
				}

			case focusDetail:
				// Enter returns to previous view from detail (bd-y0m, bd-yo4)
				if msg.String() == "enter" {
					if m.isBoardView {
						m.focused = focusBoard
					} else {
						m.focused = focusTree
					}
				} else if m.treeViewActive && msg.String() == "d" {
					// Toggle detail panel from detail focus in tree view (bd-80u)
					m.treeDetailHidden = !m.treeDetailHidden
					if m.treeDetailHidden {
						m.focused = focusTree
					}
					if !m.treeDetailHidden {
						m.syncTreeToDetail()
					}
				} else {
					m.viewport, cmd = m.viewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.isSplitView = msg.Width > SplitViewThreshold
		m.ready = true
		bodyHeight := m.height - 1 // keep 1 row for footer
		if bodyHeight < 5 {
			bodyHeight = 5
		}

		if m.isSplitView {
			// Calculate dimensions accounting for 2 panels with borders(2)+padding(2) = 4 overhead each
			// Total overhead = 8
			availWidth := msg.Width - 8
			if availWidth < 10 {
				availWidth = 10
			}

			// Use configurable split ratio (default 0.4, adjustable via [ and ])
			listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
			detailInnerWidth := availWidth - listInnerWidth

			// listHeight fits header (1) + page line (1) inside a panel with Border (2)
			listHeight := bodyHeight - 4
			if listHeight < 3 {
				listHeight = 3
			}

			m.list.SetSize(listInnerWidth, listHeight)
			m.viewport = viewport.New(detailInnerWidth, bodyHeight-2) // Account for border

			m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
		} else {
			listHeight := bodyHeight - 2
			if listHeight < 3 {
				listHeight = 3
			}
			m.list.SetSize(msg.Width, listHeight)
			m.viewport = viewport.New(msg.Width, bodyHeight-1)

			// Update renderer for full width
			m.renderer.SetWidthWithTheme(msg.Width, m.theme)
		}

		// Auto-hide detail panel when window is too narrow for split view (bd-6eg, bd-dy7).
		// In narrow windows, Enter opens full-screen detail instead.
		// Resizing from narrow to wide does NOT auto-show; user presses 'd'.
		// But resizing within split view (pane too narrow -> fits) does auto-show.
		if !m.isSplitView {
			m.treeDetailHidden = true
			m.detailHiddenByNarrow = true
			if m.treeViewActive && m.focused == focusDetail {
				m.focused = focusTree
			}
		} else if m.detailPaneWidth() < MinDetailPaneWidth {
			m.treeDetailHidden = true
			if m.treeViewActive && m.focused == focusDetail {
				m.focused = focusTree
			}
		}
		// Detail is never auto-shown; user presses 'd' to toggle (bd-x96a)

		m.updateListDelegate()

		m.updateViewportContent()
	}

	// Update list for navigation, but NOT for WindowSizeMsg
	// (we handle sizing ourselves to account for header/footer)
	// Only forward keyboard messages to list when list has focus (bv-hmkz fix)
	// This prevents j/k keys in detail view from changing list selection
	// Skip forwarding when handleListKeys already consumed the key (bd-kob fix)
	// to prevent filter shortcut keys (o/c/r/a etc.) from starting the
	// built-in fuzzy search, which captures arrow keys and escape.
	if m.focused == focusList {
		if _, isWindowSize := msg.(tea.WindowSizeMsg); !isWindowSize && !listKeyConsumed {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		}
		m.updateListDelegate()
	}

	// Update viewport if list selection changed in split view
	if m.isSplitView && m.focused == focusList {
		m.updateViewportContent()
	}

	return m, tea.Batch(cmds...)
}

// handleBoardKeys handles keyboard input when the board is focused (bv-yg39)
func (m Model) handleBoardKeys(msg tea.KeyMsg) Model {
	key := msg.String()

	// ═══════════════════════════════════════════════════════════════════════════
	// Search mode input handling (bv-yg39)
	// ═══════════════════════════════════════════════════════════════════════════
	if m.board.IsSearchMode() {
		switch key {
		case "esc":
			m.board.CancelSearch()
		case "enter":
			// Keep search results but exit search mode
			m.board.FinishSearch()
		case "backspace":
			m.board.BackspaceSearch()
		case "n":
			m.board.NextMatch()
		case "N":
			m.board.PrevMatch()
		default:
			// Append printable characters to search query
			if len(key) == 1 {
				m.board.AppendSearchChar(rune(key[0]))
			}
		}
		return m
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// Vim 'gg' combo handling (bv-yg39)
	// ═══════════════════════════════════════════════════════════════════════════
	if m.board.IsWaitingForG() {
		m.board.ClearWaitingForG()
		if key == "g" {
			m.board.MoveToTop()
			return m
		}
		// Not a second 'g', fall through to normal handling
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// Normal key handling (bv-yg39 enhanced)
	// ═══════════════════════════════════════════════════════════════════════════
	switch key {
	// Basic navigation (existing)
	case "h", "left":
		m.board.MoveLeft()
	case "l", "right":
		m.board.MoveRight()
	case "j", "down":
		m.board.MoveDown()
	case "k", "up":
		m.board.MoveUp()
	case "home":
		m.board.MoveToTop()
	case "G", "end":
		m.board.MoveToBottom()
	case "ctrl+d":
		m.board.PageDown(m.height / 3)
	case "ctrl+u":
		m.board.PageUp(m.height / 3)

	// Column jumping (bv-yg39)
	case "1":
		m.board.JumpToColumn(ColOpen)
	case "2":
		m.board.JumpToColumn(ColInProgress)
	case "3":
		m.board.JumpToColumn(ColBlocked)
	case "4":
		m.board.JumpToColumn(ColClosed)
	case "H":
		m.board.JumpToFirstColumn()
	case "L":
		m.board.JumpToLastColumn()

	// Vim-style navigation (bv-yg39)
	case "g":
		m.board.SetWaitingForG() // Wait for second 'g'
	case "0":
		m.board.MoveToTop() // First item in column
	case "$":
		m.board.MoveToBottom() // Last item in column

	// Search (bv-yg39)
	case "/":
		m.board.StartSearch()

	// Search navigation when not in search mode (bv-yg39)
	case "n":
		if m.board.SearchMatchCount() > 0 {
			m.board.NextMatch()
		}
	case "N":
		if m.board.SearchMatchCount() > 0 {
			m.board.PrevMatch()
		}

	// Copy ID to clipboard (bv-yg39)
	case "y":
		if selected := m.board.SelectedIssue(); selected != nil {
			if err := clipboard.WriteAll(selected.ID); err != nil {
				m.statusMsg = fmt.Sprintf("❌ Clipboard error: %v", err)
				m.statusIsError = true
			} else {
				m.statusMsg = fmt.Sprintf("📋 Copied %s to clipboard", selected.ID)
				m.statusIsError = false
			}
		}

	// Global filter keys (bv-naov) - consistent with list view
	case "o":
		m.currentFilter = "open"
		m.applyFilter()
		m.statusMsg = "Filter: Open issues"
		m.statusIsError = false
	case "c":
		m.currentFilter = "closed"
		m.applyFilter()
		m.statusMsg = "Filter: Closed issues"
		m.statusIsError = false
	case "r":
		m.currentFilter = "ready"
		m.applyFilter()
		m.statusMsg = "Filter: Ready (no blockers)"
		m.statusIsError = false

	// Swimlane mode cycling (bv-wjs0)
	case "s":
		m.board.CycleSwimLaneMode()
		modeName := m.board.GetSwimLaneModeName()
		m.statusMsg = fmt.Sprintf("🔀 Swimlane: %s", modeName)
		m.statusIsError = false

	// Empty column visibility toggle (bv-tf6j)
	case "e":
		m.board.ToggleEmptyColumns()
		visMode := m.board.GetEmptyColumnVisibilityMode()
		hidden := m.board.HiddenColumnCount()
		if hidden > 0 {
			m.statusMsg = fmt.Sprintf("👁 Empty columns: %s (%d hidden)", visMode, hidden)
		} else {
			m.statusMsg = fmt.Sprintf("👁 Empty columns: %s", visMode)
		}
		m.statusIsError = false

	// Inline card expansion (bd-1of: Tab replaces d for card cycling)
	case "tab":
		m.board.ToggleExpand()
		if m.board.HasExpandedCard() {
			m.statusMsg = "📋 Card expanded (tab=collapse, j/k=auto-collapse)"
		} else {
			m.statusMsg = "📋 Card collapsed"
		}
		m.statusIsError = false

	case "ctrl+j":
		if m.board.IsDetailShown() {
			m.board.DetailScrollDown(3)
		}
	case "ctrl+k":
		if m.board.IsDetailShown() {
			m.board.DetailScrollUp(3)
		}

	// Enter toggles detail view (bd-yo4: full detail like tree view)
	case "enter":
		if m.focused == focusDetail {
			// Already in detail — handled by focusDetail case in Update
		} else {
			m.syncBoardToDetail()
			m.focused = focusDetail
		}
	}
	return m
}

// syncTreeToDetail synchronizes the detail panel with the currently selected tree node.
// It finds the matching issue in the list and updates the viewport content.
func (m *Model) syncTreeToDetail() {
	selected := m.tree.SelectedIssue()
	if selected == nil {
		return
	}
	for i, item := range m.list.Items() {
		if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
			m.list.Select(i)
			break
		}
	}
	m.updateViewportContent()
}

// syncBoardToDetail synchronizes the detail panel with the currently selected board card (bd-yo4).
func (m *Model) syncBoardToDetail() {
	selected := m.board.SelectedIssue()
	if selected == nil {
		return
	}
	for i, item := range m.list.Items() {
		if issueItem, ok := item.(IssueItem); ok && issueItem.Issue.ID == selected.ID {
			m.list.Select(i)
			break
		}
	}
	m.updateViewportContent()
}

// handleTreeKeys handles keyboard input when tree view is focused (bv-gllx)
func (m Model) handleTreeKeys(msg tea.KeyMsg) Model {
	// Sort popup mode: consume j/k/enter/esc/s only (bd-u81)
	if m.tree.IsSortPopupOpen() {
		switch msg.String() {
		case "j", "down":
			m.tree.SortPopupDown()
		case "k", "up":
			m.tree.SortPopupUp()
		case "enter":
			m.tree.SortPopupSelect()
			m.syncTreeToDetail()
		case "esc", "s":
			m.tree.CloseSortPopup()
		}
		return m
	}

	// Search mode: forward input to tree search (bd-wf8)
	if m.tree.IsSearchMode() {
		switch msg.String() {
		case "esc":
			m.tree.ClearSearch()
			return m
		case "enter":
			m.tree.ExitSearchMode()
			m.syncTreeToDetail()
			return m
		case "backspace":
			m.tree.SearchBackspace()
			return m
		default:
			if len(msg.String()) == 1 {
				m.tree.SearchAddChar(rune(msg.String()[0]))
			}
			return m
		}
	}

	switch msg.String() {
	case "j", "down":
		m.tree.MoveDown()
		m.syncTreeToDetail()
	case "k", "up":
		m.tree.MoveUp()
		m.syncTreeToDetail()
	case "tab":
		// Tab always cycles node visibility (bd-lt1l)
		m.tree.CycleNodeVisibility()
		m.syncTreeToDetail()
	case "shift+tab":
		// Shift+Tab cycles global visibility (bd-1of)
		m.tree.CycleGlobalVisibility()
		m.syncTreeToDetail()
	case "enter":
		// Enter always opens detail view (bd-1of)
		m.syncTreeToDetail()
		m.focused = focusDetail
	case "ctrl+a":
		m.tree.ToggleExpandCollapseAll()
		m.syncTreeToDetail()
	case "h":
		m.tree.CollapseOrJumpToParent()
		m.syncTreeToDetail()
	case "l":
		m.tree.ExpandOrMoveToChild()
		m.syncTreeToDetail()
	case "left":
		m.tree.PageBackwardFull()
		m.syncTreeToDetail()
	case "right":
		m.tree.PageForwardFull()
		m.syncTreeToDetail()
	case "g":
		// Jump to top (vim-style)
		m.tree.JumpToTop()
		m.syncTreeToDetail()
	case "G":
		m.tree.JumpToBottom()
		m.syncTreeToDetail()
	case "X":
		m.tree.ExpandAll()
	case "Z":
		m.tree.CollapseAll()
	case "ctrl+d", "pgdown":
		m.tree.PageDown()
		m.syncTreeToDetail()
	case "ctrl+u", "pgup":
		m.tree.PageUp()
		m.syncTreeToDetail()
	case "s":
		// Open sort popup menu (bd-u81)
		m.tree.OpenSortPopup()
	case "/":
		// Enter search mode (bd-wf8)
		m.tree.EnterSearchMode()
	case "n":
		// Next search match (bd-wf8)
		m.tree.NextSearchMatch()
		m.syncTreeToDetail()
	case "N":
		// Previous search match (bd-wf8)
		m.tree.PrevSearchMatch()
		m.syncTreeToDetail()
	case "`":
		// Toggle flat/tree mode (bd-39v)
		m.tree.ToggleFlatMode()
		m.syncTreeToDetail()
	case "o":
		// Filter: open issues (bd-5nw)
		m.tree.ApplyFilter("open")
		m.syncTreeToDetail()
	case "c":
		// Filter: closed issues (bd-5nw)
		m.tree.ApplyFilter("closed")
		m.syncTreeToDetail()
	case "r":
		// Filter: ready issues (bd-5nw)
		m.tree.ApplyFilter("ready")
		m.syncTreeToDetail()
	case "a":
		// Filter: all issues (bd-5nw)
		m.tree.ApplyFilter("all")
		m.syncTreeToDetail()
	case "p":
		// Jump to parent node (bd-ryu) — 'P' is now picker toggle (bd-ey3)
		m.tree.JumpToParent()
		m.syncTreeToDetail()
	case "]":
		// Next sibling (bd-ryu)
		m.tree.NextSibling()
		m.syncTreeToDetail()
	case "[":
		// Previous sibling (bd-ryu)
		m.tree.PrevSibling()
		m.syncTreeToDetail()
	case "{":
		// First sibling (bd-ryu)
		m.tree.FirstSibling()
		m.syncTreeToDetail()
	case "}":
		// Last sibling (bd-ryu)
		m.tree.LastSibling()
		m.syncTreeToDetail()
	// NOTE: TAB, shift+tab, and 1-9 removed from tree keys (bd-8zc)
	// TAB is handled by Model.Update for tree↔detail focus switching
	// 1-9 are handled by Model.Update for project switching
	case "m":
		// Toggle mark on current node (bd-cz0)
		m.tree.ToggleMark()
	case "M":
		// Unmark all (bd-cz0)
		m.tree.UnmarkAll()
	case "x":
		// Toggle XRay drill-down mode (bd-0rc)
		m.tree.ToggleXRay()
		m.syncTreeToDetail()
	case "b":
		// Toggle bookmark on current node (bd-k4n)
		m.tree.ToggleBookmark()
	case "B":
		// Cycle through bookmarked nodes (bd-k4n)
		m.tree.CycleBookmark()
		m.syncTreeToDetail()
	case "F":
		// Toggle follow mode (bd-c0c)
		m.tree.ToggleFollowMode()
	case "O":
		// Toggle occur mode (bd-sjs.2) - uses current search pattern
		if m.tree.IsOccurMode() {
			m.tree.ExitOccurMode()
			m.syncTreeToDetail()
		} else if m.tree.SearchQuery() != "" {
			m.tree.EnterOccurMode(m.tree.SearchQuery())
			m.syncTreeToDetail()
		} else {
			m.statusMsg = "Search with / first, then O for occur mode"
			m.statusIsError = false
		}
	case "d":
		// Toggle detail panel visibility (bd-80u)
		m.treeDetailHidden = !m.treeDetailHidden
		m.detailHiddenByNarrow = false // User took manual control (bd-6eg)
		if m.treeDetailHidden && m.focused == focusDetail {
			m.focused = focusTree
		}
		if !m.treeDetailHidden {
			m.syncTreeToDetail()
		}
	case "esc":
		// Escape hierarchy: occur, XRay, tree filter (bd-sjs.2, bd-kob, bd-0rc)
		// Tree view is always active (bd-8hw.4), esc doesn't exit tree
		if m.tree.IsOccurMode() {
			m.tree.ExitOccurMode()
			m.syncTreeToDetail()
		} else if m.tree.IsXRayMode() {
			m.tree.ExitXRay()
			m.syncTreeToDetail()
		} else if m.tree.GetFilter() != "" && m.tree.GetFilter() != "all" {
			m.tree.ApplyFilter("all")
			m.syncTreeToDetail()
		}
		// No else: tree view is permanent
	}
	return m
}


// handleRepoPickerKeys handles keyboard input when repo picker is focused (workspace mode).
func (m Model) handleRepoPickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.repoPicker.MoveDown()
	case "k", "up":
		m.repoPicker.MoveUp()
	case " ", "space":
		m.repoPicker.ToggleSelected()
	case "a":
		m.repoPicker.SelectAll()
	case "esc", "q":
		m.showRepoPicker = false
		m.focused = focusTree
	case "enter":
		selected := m.repoPicker.SelectedRepos()

		// Normalize: nil means "all repos" (no filter). Also treat empty as "all" to avoid hiding everything.
		if len(selected) == 0 || len(selected) == len(m.availableRepos) {
			m.activeRepos = nil
			m.statusMsg = "Repo filter: all repos"
		} else {
			m.activeRepos = selected
			m.statusMsg = fmt.Sprintf("Repo filter: %s", formatRepoList(sortedRepoKeys(selected), 3))
		}
		m.statusIsError = false

		// Apply filter to views
		m.applyFilter()

		m.showRepoPicker = false
		m.focused = focusTree
	}
	return m
}

// handleLabelPickerKeys handles keyboard input when label picker is focused (bv-126)
func (m Model) handleLabelPickerKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "esc":
		m.showLabelPicker = false
		m.focused = focusTree
	case "j", "down", "ctrl+n":
		m.labelPicker.MoveDown()
	case "k", "up", "ctrl+p":
		m.labelPicker.MoveUp()
	case "enter":
		if selected := m.labelPicker.SelectedLabel(); selected != "" {
			m.currentFilter = "label:" + selected
			m.applyFilter()
			m.statusMsg = fmt.Sprintf("Filtered by label: %s", selected)
			m.statusIsError = false
		}
		m.showLabelPicker = false
		m.focused = focusTree
	default:
		// Pass other keys to text input for fuzzy search
		m.labelPicker.UpdateInput(msg)
	}
	return m
}

// handleListKeys handles keyboard input when the list is focused.
// Returns (model, consumed) where consumed=true means the key was handled
// and should NOT be forwarded to the bubbles/list component (bd-kob).
func (m Model) handleListKeys(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "enter":
		if !m.isSplitView {
			m.showDetails = true
			m.focused = focusDetail
			m.viewport.GotoTop() // Reset scroll position for new issue
			m.updateViewportContent()
		}
		return m, true
	case "home":
		m.list.Select(0)
		return m, true
	case "G", "end":
		if len(m.list.Items()) > 0 {
			m.list.Select(len(m.list.Items()) - 1)
		}
		return m, true
	case "ctrl+d":
		// Page down
		itemCount := len(m.list.Items())
		if itemCount > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx + m.height/3
			if newIdx >= itemCount {
				newIdx = itemCount - 1
			}
			m.list.Select(newIdx)
		}
		return m, true
	case "ctrl+u":
		// Page up
		if len(m.list.Items()) > 0 {
			currentIdx := m.list.Index()
			newIdx := currentIdx - m.height/3
			if newIdx < 0 {
				newIdx = 0
			}
			m.list.Select(newIdx)
		}
		return m, true
	case "o":
		m.currentFilter = "open"
		m.applyFilter()
		return m, true
	case "c":
		m.currentFilter = "closed"
		m.applyFilter()
		return m, true
	case "r":
		m.currentFilter = "ready"
		m.applyFilter()
		return m, true
	case "a":
		m.currentFilter = "all"
		m.applyFilter()
		return m, true
	case "C":
		// Copy selected issue to clipboard
		m.copyIssueToClipboard()
		return m, true
	case "O":
		// Open beads.jsonl in editor
		m.openInEditor()
		return m, true
	case "s":
		// Cycle sort mode (bv-3ita)
		m.cycleSortMode()
		return m, true
	case "U":
		// Show self-update modal (bv-182)
		m.showSelfUpdateModal()
		return m, true
	case "y":
		// Copy ID to clipboard (consistent with board view - bv-yg39)
		selectedItem := m.list.SelectedItem()
		if selectedItem == nil {
			m.statusMsg = "❌ No issue selected"
			m.statusIsError = true
		} else if issueItem, ok := selectedItem.(IssueItem); ok {
			if err := clipboard.WriteAll(issueItem.Issue.ID); err != nil {
				m.statusMsg = fmt.Sprintf("❌ Clipboard error: %v", err)
				m.statusIsError = true
			} else {
				m.statusMsg = fmt.Sprintf("📋 Copied %s to clipboard", issueItem.Issue.ID)
				m.statusIsError = false
			}
		}
		return m, true
	// Space removed from list view (bd-1of)
	case "e":
		// Open edit modal (bd-a83)
		// Skip if filtering is active
		if m.list.SettingFilter() || m.list.FilterState() == list.Filtering {
			return m, false
		}
		if issue := m.getSelectedIssue(); issue != nil {
			m.editModal = NewEditModal(issue, m.theme)
			m.editModal.SetSize(m.width, m.height)
			m.showEditModal = true
			m.focused = focusEditModal
		}
		return m, true
	}
	return m, false
}

// restoreFocusFromHelp returns the appropriate focus based on current view state.
// This fixes the bug where dismissing help would always return to focusList,
// even when the user was in a specialized view (board, tree, etc.).
func (m Model) restoreFocusFromHelp() focus {
	// Full-screen detail view (not split mode)
	if m.showDetails && !m.isSplitView {
		return focusDetail
	}
	// Specialized views take precedence
	if m.isBoardView {
		return focusBoard
	}
	if m.treeViewActive {
		return focusTree
	}
	if m.focusBeforeHelp == focusLabelPicker {
		return focusLabelPicker
	}
	// Default: return to list
	return focusList
}

// handleHelpKeys handles keyboard input when the help overlay is focused
func (m Model) handleHelpKeys(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "j", "down":
		m.helpScroll++
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
	case "ctrl+d":
		m.helpScroll += 10
	case "ctrl+u":
		m.helpScroll -= 10
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	case "home", "g":
		m.helpScroll = 0
	case "G", "end":
		// Will be clamped in render
		m.helpScroll = 999
	case "q", "esc", "?", "f1":
		// Close help overlay and restore previous focus
		m.showHelp = false
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	// Space removed from help overlay (bd-1of)
	default:
		// Any other key dismisses help and restores previous focus
		m.showHelp = false
		m.helpScroll = 0
		m.focused = m.restoreFocusFromHelp()
	}
	return m
}

func (m Model) renderLoadingScreen() string {
	frame := workerSpinnerFrames[0]
	if m.backgroundWorker != nil && m.backgroundWorker.State() == WorkerProcessing {
		frame = workerSpinnerFrames[m.workerSpinnerIdx%len(workerSpinnerFrames)]
	}

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	lines := []string{
		spinnerStyle.Render(frame),
		"",
		titleStyle.Render("Loading beads..."),
	}
	if m.beadsPath != "" {
		lines = append(lines, "", subStyle.Render(m.beadsPath))
	}

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var body string
	isOverlay := false // Track whether an overlay is active (no global header)

	// Quit confirmation overlay takes highest priority
	if m.showQuitConfirm {
		body = m.renderQuitConfirm()
		isOverlay = true
	} else if m.showEditModal {
		// Edit modal (bd-a83)
		body = m.editModal.View()
		isOverlay = true
	} else if m.showUpdateModal {
		// Self-update modal (bv-182)
		body = m.updateModal.CenterModal(m.width, m.height-1)
		isOverlay = true
	} else if m.showStatusPicker {
		// Status picker modal (bd-a83)
		body = m.statusPicker.View()
		isOverlay = true
	} else if m.showRepoPicker {
		body = m.repoPicker.View()
		isOverlay = true
	} else if m.showLabelPicker {
		body = m.labelPicker.View()
		isOverlay = true
	} else if m.showHelp {
		body = m.renderHelpOverlay()
		isOverlay = true
	} else if m.showTutorial {
		// Interactive tutorial (bv-8y31) - full screen overlay
		body = m.tutorialModel.View()
		isOverlay = true
	} else if m.snapshotInitPending && m.snapshot == nil {
		body = m.renderLoadingScreen()
		isOverlay = true
	} else if m.focused == focusDetail && m.isBoardView {
		// Board detail-only mode: full-screen viewport (bd-yo4)
		body = m.viewport.View()
	} else if m.focused == focusTree || (m.focused == focusDetail && m.treeViewActive) {
		// Hierarchical tree view (bv-gllx) with split view support (bd-xfd)
		if m.focused == focusDetail && m.treeDetailHidden {
			// Detail-only mode: full-screen viewport (bd-80u)
			body = m.viewport.View()
		} else if m.isSplitView && !m.treeDetailHidden {
			body = m.renderTreeSplitView()
		} else {
			m.tree.SetSize(m.width, m.bodyHeight())
			body = m.tree.View()
		}
	} else if m.isBoardView {
		body = m.board.View(m.width, m.bodyHeight())
	} else if m.isSplitView {
		body = m.renderSplitView()
	} else {
		// Tree view is always the default (bd-8hw.4)
		m.tree.SetSize(m.width, m.bodyHeight())
		body = m.tree.View()
	}

	footer := m.renderFooter()

	// Ensure the final output fits exactly in the terminal height
	// This prevents the header from being pushed off the top
	finalStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		MaxHeight(m.height)

	if isOverlay {
		return finalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))
	}

	// Compact project picker header, toggleable via Shift+P (bd-ey3, bd-ylz, bd-2me)
	var pickerHeader string
	if len(m.allProjects) > 0 && m.pickerVisible {
		m.projectPicker.SetSize(m.width, m.height)
		pickerHeader = m.projectPicker.View()
	} else if len(m.allProjects) > 0 && !m.pickerVisible {
		// Minimized one-line bar: show project numbers + names, highlight active (bd-4s6)
		m.projectPicker.SetSize(m.width, m.height)
		pickerHeader = m.projectPicker.ViewMinimized()
	} else if len(m.allProjects) == 0 {
		// No projects configured: fall back to the original global header
		pickerHeader = m.renderGlobalHeader()
	}

	if pickerHeader != "" {
		return finalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, pickerHeader, body, footer))
	}
	return finalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, body, footer))
}

func (m Model) renderQuitConfirm() string {
	t := m.theme

	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Blocked).
		Padding(1, 3).
		Align(lipgloss.Center)

	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Blocked).
		Bold(true)

	textStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	keyStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	content := titleStyle.Render("Quit bv?") + "\n\n" +
		textStyle.Render("Press ") + keyStyle.Render("Esc") + textStyle.Render(" or ") + keyStyle.Render("Y") + textStyle.Render(" to quit\n") +
		textStyle.Render("Press any other key to cancel")

	box := boxStyle.Render(content)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

func (m Model) renderListWithHeader() string {
	t := m.theme

	// Calculate dimensions based on actual list height set in sizing
	availableHeight := m.list.Height()
	if availableHeight == 0 {
		availableHeight = m.height - 3 // fallback
	}

	// Render column header
	headerStyle := t.Renderer.NewStyle().
		Background(t.Primary).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Bold(true).
		Width(m.width - 2)

	headerText := "  TYPE PRI STATUS      ID                                   TITLE"
	if m.workspaceMode {
		// Account for repo badges like [API] shown in workspace mode.
		headerText = "  REPO TYPE PRI STATUS      ID                               TITLE"
	}
	header := headerStyle.Render(headerText)

	// Page info
	totalItems := len(m.list.Items())
	currentIdx := m.list.Index()
	itemsPerPage := availableHeight
	if itemsPerPage < 1 {
		itemsPerPage = 1
	}
	currentPage := (currentIdx / itemsPerPage) + 1
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	startItem := 0
	endItem := 0
	if totalItems > 0 {
		startItem = (currentPage-1)*itemsPerPage + 1
		endItem = startItem + itemsPerPage - 1
		if endItem > totalItems {
			endItem = totalItems
		}
	}

	pageInfo := fmt.Sprintf(" Page %d of %d (items %d-%d of %d) ", currentPage, totalPages, startItem, endItem, totalItems)
	pageStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Align(lipgloss.Right).
		Width(m.width - 2)

	// Combine header with page info on the right
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top,
		header,
	)

	// List view - just render it normally since bubbles handles scrolling
	listView := m.list.View()

	// Page indicator line
	pageLine := pageStyle.Render(pageInfo)

	// Combine all elements and force exact height
	// bodyHeight = m.height - 2 (1 for global header, 1 for footer)
	bodyHeight := m.height - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Build content with explicit height constraint
	// Header (1) + List + PageLine (1) must fit in bodyHeight
	content := lipgloss.JoinVertical(lipgloss.Left, headerLine, listView, pageLine)

	// Force exact height to prevent overflow
	return lipgloss.NewStyle().
		Width(m.width).
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(content)
}

func (m Model) renderSplitView() string {
	t := m.theme

	var listStyle, detailStyle lipgloss.Style

	if m.focused == focusList {
		listStyle = FocusedPanelStyle
		detailStyle = PanelStyle
	} else {
		listStyle = PanelStyle
		detailStyle = FocusedPanelStyle
	}

	// m.list.Width() is the inner width (set in Update)
	listInnerWidth := m.list.Width()
	panelHeight := m.height - 2 // 1 for global header, 1 for footer

	// Create header row for list
	headerStyle := t.Renderer.NewStyle().
		Background(t.Primary).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Bold(true).
		Width(listInnerWidth)

	header := headerStyle.Render("  TYPE PRI STATUS      ID                     TITLE")

	// Page info for list
	totalItems := len(m.list.Items())
	currentIdx := m.list.Index()
	listHeight := m.list.Height()
	if listHeight == 0 {
		listHeight = panelHeight - 3 // fallback
	}
	if listHeight < 1 {
		listHeight = 1
	}
	currentPage := (currentIdx / listHeight) + 1
	totalPages := (totalItems + listHeight - 1) / listHeight
	if totalPages < 1 {
		totalPages = 1
	}
	startItem := 0
	endItem := 0
	if totalItems > 0 {
		startItem = (currentPage-1)*listHeight + 1
		endItem = startItem + listHeight - 1
		if endItem > totalItems {
			endItem = totalItems
		}
	}

	pageInfo := fmt.Sprintf("Page %d/%d (%d-%d of %d) ", currentPage, totalPages, startItem, endItem, totalItems)
	pageStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Width(listInnerWidth).
		Align(lipgloss.Center)

	pageLine := pageStyle.Render(pageInfo)

	// Combine header + list + page indicator
	listContent := lipgloss.JoinVertical(lipgloss.Left, header, m.list.View(), pageLine)

	// List Panel Width: Inner + 2 (Padding). Border adds another 2.
	// Use MaxHeight to ensure content doesn't overflow
	listView := listStyle.
		Width(listInnerWidth + 2).
		Height(panelHeight).
		MaxHeight(panelHeight).
		Render(listContent)

	// Detail Panel Width: Inner + 2 (Padding). Border adds another 2.
	detailView := detailStyle.
		Width(m.viewport.Width + 2).
		Height(panelHeight).
		MaxHeight(panelHeight).
		Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

// renderTreeSplitView renders the tree view in a split layout with a detail panel on the right,
// mirroring renderSplitView but using the tree for the left pane.
func (m Model) renderTreeSplitView() string {
	var treeStyle, detailStyle lipgloss.Style

	if m.focused == focusTree {
		treeStyle = FocusedPanelStyle
		detailStyle = PanelStyle
	} else {
		treeStyle = PanelStyle
		detailStyle = FocusedPanelStyle
	}

	// Use the same inner width as the list panel for consistent sizing
	treeInnerWidth := m.list.Width()
	panelHeight := m.height - 2 // 1 for global header, 1 for footer

	// Set tree size to fit inside the panel (border takes 2 lines)
	// The header row is now rendered inside tree.View() via RenderHeader() (bd-s2k)
	treeHeight := panelHeight - 2
	if treeHeight < 1 {
		treeHeight = 1
	}
	m.tree.SetSize(treeInnerWidth, treeHeight)

	// tree.View() includes the header row (bd-s2k)
	treeContent := m.tree.View()

	// Tree Panel Width: Inner + 2 (Padding). Border adds another 2.
	treeView := treeStyle.
		Width(treeInnerWidth + 2).
		Height(panelHeight).
		MaxHeight(panelHeight).
		Render(treeContent)

	// Detail Panel Width: Inner + 2 (Padding). Border adds another 2.
	detailView := detailStyle.
		Width(m.viewport.Width + 2).
		Height(panelHeight).
		MaxHeight(panelHeight).
		Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, treeView, detailView)
}

func (m *Model) renderHelpOverlay() string {
	t := m.theme

	// Determine layout based on terminal width
	// 3 columns for wide (≥120), 2 columns for medium (≥80), 1 column for narrow
	numCols := 3
	if m.width < 120 {
		numCols = 2
	}
	if m.width < 80 {
		numCols = 1
	}

	// Calculate column width (accounting for gaps and outer padding)
	totalPadding := 8 // outer padding
	gapWidth := 2     // gap between columns
	availableWidth := m.width - totalPadding - (gapWidth * (numCols - 1))
	colWidth := availableWidth / numCols
	if colWidth < 28 {
		colWidth = 28
	}

	// Define color palette (Dracula-inspired gradient)
	colors := []lipgloss.AdaptiveColor{
		{Light: "#7D56F4", Dark: "#BD93F9"}, // Purple
		{Light: "#FF79C6", Dark: "#FF79C6"}, // Pink
		{Light: "#8BE9FD", Dark: "#8BE9FD"}, // Cyan
		{Light: "#50FA7B", Dark: "#50FA7B"}, // Green
		{Light: "#FFB86C", Dark: "#FFB86C"}, // Orange
		{Light: "#F1FA8C", Dark: "#F1FA8C"}, // Yellow
	}

	// Helper to render a section panel
	renderPanel := func(title string, icon string, colorIdx int, shortcuts []struct{ key, desc string }) string {
		color := colors[colorIdx%len(colors)]

		headerStyle := t.Renderer.NewStyle().
			Foreground(color).
			Bold(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderBottom(true).
			BorderForeground(color).
			Width(colWidth-4).
			Padding(0, 1)

		keyStyle := t.Renderer.NewStyle().
			Foreground(color).
			Bold(true).
			Width(10)

		descStyle := t.Renderer.NewStyle().
			Foreground(t.Base.GetForeground()).
			Width(colWidth - 16)

		var content strings.Builder
		content.WriteString(headerStyle.Render(icon + " " + title))
		content.WriteString("\n")

		for _, s := range shortcuts {
			content.WriteString(keyStyle.Render(s.key))
			content.WriteString(descStyle.Render(s.desc))
			content.WriteString("\n")
		}

		panelStyle := t.Renderer.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(color).
			Padding(0, 1).
			Width(colWidth)

		return panelStyle.Render(content.String())
	}

	// Define all sections
	navSection := []struct{ key, desc string }{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"G/end", "Go to last"},
		{"Ctrl+d", "Page down"},
		{"Ctrl+u", "Page up"},
		{"Tab", "Switch focus"},
		{"Enter", "View details"},
		{"Esc", "Back / close"},
	}

	viewsSection := []struct{ key, desc string }{
		{"E", "Tree view"},
		{"b", "Kanban board"},
		{"g", "Graph view"},
		{"i", "Insights"},
		{"h", "History view"},
		{"a", "Actionable"},
		{"f", "Flow matrix"},
		{"[", "Label dashboard"},
		{"]", "Attention view"},
	}

	globalSection := []struct{ key, desc string }{
		{"?", "This help"},
		{";", "Shortcuts bar"},
		{"!", "Alerts panel"},
		{"'", "Recipes"},
		{"w", "Repo picker"},
		{"q", "Back / Quit"},
		{"Ctrl+c", "Force quit"},
	}

	filterSection := []struct{ key, desc string }{
		{"/", "Fuzzy search"},
		{"Ctrl+S", "Semantic search"},
		{"H", "Hybrid ranking"},
		{"Alt+H", "Hybrid preset"},
		{"o", "Open issues"},
		{"c", "Closed issues"},
		{"r", "Ready (unblocked)"},
		{"l", "Filter by label"},
		{"s", "Cycle sort"},
		{"S", "Triage sort"},
	}

	graphSection := []struct{ key, desc string }{
		{"hjkl", "Navigate nodes"},
		{"H/L", "Scroll left/right"},
		{"PgUp/Dn", "Scroll up/down"},
		{"Enter", "Jump to issue"},
	}

	insightsSection := []struct{ key, desc string }{
		{"h/l/Tab", "Switch panels"},
		{"j/k", "Navigate items"},
		{"e", "Explanations"},
		{"x", "Calc details"},
		{"m", "Toggle heatmap"},
		{"Enter", "Jump to issue"},
	}

	historySection := []struct{ key, desc string }{
		{"j/k", "Navigate beads"},
		{"J/K", "Navigate commits"},
		{"Tab", "Toggle focus"},
		{"y", "Copy SHA"},
		{"c", "Confidence filter"},
	}

	actionsSection := []struct{ key, desc string }{
		{"p", "Priority hints"},
		{"Ctrl+R", "Force refresh"},
		{"F5", "Force refresh"},
		{"t", "Time-travel"},
		{"T", "Quick time-travel"},
		{"x", "Export markdown"},
		{"C", "Copy to clipboard"},
		{"O", "Open in editor"},
	}

	treeSection := []struct{ key, desc string }{
		{"j/k/↕", "Move up/down"},
		{"h", "Collapse / parent"},
		{"l", "Expand / child"},
		{"←/→", "Page back/forward"},
		{"Enter/Spc", "Toggle expand"},
		{"g/G", "Top / bottom"},
		{"p", "Jump to parent"},
		{"X/Z", "Expand / collapse all"},
		{"Tab", "Cycle node visibility"},
		{"1-9", "Expand to level N"},
		{"d", "Toggle detail panel"},
		{"o/c/r/a", "Filter: open/closed/ready/all"},
		{"s", "Sort popup"},
		{"/", "Search tree"},
		{"n/N", "Next/prev match"},
		{"O", "Occur (search filter)"},
		{"x", "XRay drill-down"},
		{"b/B", "Bookmark / cycle"},
		{"m/M", "Mark / unmark all"},
	}

	editingSection := []struct{ key, desc string }{
		{"e", "Edit issue"},
		{"Space", "Status picker (list)"},
		{"1-4", "Set priority (list)"},
		{"Ctrl+n", "Create new issue"},
		{"Ctrl+s", "Save (in editor)"},
		{"Esc", "Cancel (in editor)"},
	}

	statusSection := []struct{ key, desc string }{
		{"◌ metrics", "Phase 2 metrics computing"},
		{"⚠ age", "Snapshot getting stale"},
		{"⚠ STALE", "Snapshot is stale"},
		{"✗ bg", "Background worker errors"},
		{"↻ recov", "Worker self-healed"},
		{"⚠ dead", "Worker unresponsive"},
		{"polling", "Live reload uses polling"},
	}

	// Build panels - ordered for balanced 3-column layout (4-4-2 split)
	// Col 1: Nav(8)+Views(9)+Global(7)+History(5) = 29
	// Col 2: Tree(9)+Graph(4)+Insights(6)+Status(7) = 26
	// Col 3: Filters(10)+Actions(8) = 18
	panels := []string{
		renderPanel("Navigation", "🧭", 0, navSection),
		renderPanel("Views", "👁", 1, viewsSection),
		renderPanel("Global", "🌐", 2, globalSection),
		renderPanel("History", "📜", 3, historySection),
		renderPanel("Tree View", "🌳", 4, treeSection),
		renderPanel("Graph View", "📊", 5, graphSection),
		renderPanel("Insights", "💡", 0, insightsSection),
		renderPanel("Status", "🩺", 2, statusSection),
		renderPanel("Filters & Sort", "🔍", 3, filterSection),
		renderPanel("Actions", "⚡", 1, actionsSection),
		renderPanel("Editing", "✏️", 4, editingSection),
	}

	// Arrange panels into columns
	var columns []string
	panelsPerCol := (len(panels) + numCols - 1) / numCols

	for col := 0; col < numCols; col++ {
		start := col * panelsPerCol
		end := start + panelsPerCol
		if end > len(panels) {
			end = len(panels)
		}
		if start >= len(panels) {
			break
		}

		colPanels := panels[start:end]
		columns = append(columns, lipgloss.JoinVertical(lipgloss.Left, colPanels...))
	}

	// Join columns horizontally
	body := lipgloss.JoinHorizontal(lipgloss.Top, columns...)

	// Title bar
	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(0, 2)

	subtitleStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	title := titleStyle.Render("⌨️  Keyboard Shortcuts")
	subtitle := subtitleStyle.Render("Space: Tutorial │ ? or Esc to close")
	titleBar := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", subtitle)

	// Combine title and body
	content := lipgloss.JoinVertical(lipgloss.Center, titleBar, "", body)

	// Apply scroll offset: split into lines, window visible portion
	contentLines := strings.Split(content, "\n")
	totalLines := len(contentLines)
	availableHeight := m.height - 6 // border (2) + padding (2) + footer hint (1) + margin (1)
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Clamp scroll offset
	maxScroll := totalLines - availableHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}

	// Build scroll hint
	scrollHint := ""
	if maxScroll > 0 {
		scrollHint = subtitleStyle.Render(fmt.Sprintf("  j/k scroll (%d/%d)", m.helpScroll+1, maxScroll+1))
	}

	// Window the content
	startLine := m.helpScroll
	endLine := startLine + availableHeight
	if endLine > totalLines {
		endLine = totalLines
	}
	visibleContent := strings.Join(contentLines[startLine:endLine], "\n")
	if scrollHint != "" {
		visibleContent += "\n" + scrollHint
	}

	// Outer container
	containerStyle := t.Renderer.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2)

	helpBox := containerStyle.Render(visibleContent)

	// Center in viewport
	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		helpBox,
	)
}

func (m *Model) renderFooter() string {
	// ══════════════════════════════════════════════════════════════════════════
	// POLISHED FOOTER - Stripe-level status bar with visual hierarchy
	// ══════════════════════════════════════════════════════════════════════════

	// If there's a status message, show it prominently with polished styling
	if m.statusMsg != "" {
		var msgStyle lipgloss.Style
		if m.statusIsError {
			msgStyle = lipgloss.NewStyle().
				Background(ColorPrioCriticalBg).
				Foreground(ColorPrioCritical).
				Bold(true).
				Padding(0, 2)
		} else {
			msgStyle = lipgloss.NewStyle().
				Background(ColorStatusOpenBg).
				Foreground(ColorSuccess).
				Bold(true).
				Padding(0, 2)
		}
		prefix := "✓ "
		if m.statusIsError {
			prefix = "✗ "
		}
		msgSection := msgStyle.Render(prefix + m.statusMsg)
		remaining := m.width - lipgloss.Width(msgSection)
		if remaining < 0 {
			remaining = 0
		}
		filler := lipgloss.NewStyle().Width(remaining).Render("")
		return lipgloss.JoinHorizontal(lipgloss.Bottom, msgSection, filler)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// CONTEXT-SENSITIVE SHORTCUT BAR
	// ─────────────────────────────────────────────────────────────────────────
	keyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	labelStyle := lipgloss.NewStyle().Foreground(ColorText)

	// Build shortcut hints based on current view
	type hint struct {
		key   string
		label string
	}
	var hints []hint

	viewName := m.currentViewName()
	switch viewName {
	case "tree":
		hints = []hint{
			{"1-9", "project"},
			{"tab", "fold"},
			{"enter", "detail"},
			{"j/k", "nav"},
			{"s", "sort"},
			{"/", "search"},
			{"e", "edit"},
			{"q", "back"},
		}
	case "board":
		hints = []hint{
			{"1-9", "project"},
			{"tab", "fold"},
			{"enter", "detail"},
			{"j/k", "card"},
			{"m", "move"},
			{"s", "swim"},
			{"/", "search"},
			{"q", "back"},
		}
	case "split":
		hints = []hint{
			{"1-9", "project"},
			{"tab", "fold"},
			{"</>", "resize"},
			{"t", "tree"},
			{"b", "board"},
			{"e", "edit"},
			{"?", "help"},
			{"q", "quit"},
		}
	case "detail":
		hints = []hint{
			{"1-9", "project"},
			{"esc", "back"},
			{"e", "edit"},
			{"C", "copy"},
			{"O", "open"},
			{"?", "help"},
			{"q", "quit"},
		}
	default: // list view
		hints = []hint{
			{"1-9", "project"},
			{"t", "tree"},
			{"b", "board"},
			{"s", "split"},
			{"/", "filter"},
			{"e", "edit"},
			{"n", "new"},
			{"?", "help"},
			{"q", "quit"},
		}
	}

	// Add picker toggle hint if multiple projects exist (bd-e4un)
	if len(m.allProjects) > 1 {
		pickerLabel := "hide projects"
		if !m.pickerVisible {
			pickerLabel = "show projects"
		}
		hints = append(hints, hint{"P", pickerLabel})
	}

	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, keyStyle.Render(h.key)+":"+labelStyle.Render(h.label))
	}
	shortcutBar := " " + strings.Join(hintParts, "  ")

	// Render the full-width footer line
	barWidth := lipgloss.Width(shortcutBar)
	remaining := m.width - barWidth
	if remaining < 0 {
		remaining = 0
	}
	filler := lipgloss.NewStyle().Width(remaining).Render("")

	return lipgloss.JoinHorizontal(lipgloss.Bottom, shortcutBar, filler)
}

// getDiffStatus returns the diff status for an issue if time-travel mode is active
func (m Model) getDiffStatus(id string) DiffStatus {
	if !m.timeTravelMode {
		return DiffStatusNone
	}
	if m.newIssueIDs[id] {
		return DiffStatusNew
	}
	if m.closedIssueIDs[id] {
		return DiffStatusClosed
	}
	if m.modifiedIssueIDs[id] {
		return DiffStatusModified
	}
	return DiffStatusNone
}

// hasActiveFilters returns true if any filter is currently applied
// (status filter, label filter, recipe filter, or fuzzy search)
func (m *Model) hasActiveFilters() bool {
	// Check status/label/recipe filter
	if m.currentFilter != "all" {
		return true
	}
	// Check if fuzzy search filter is active
	if m.list.FilterState() == list.Filtering || m.list.FilterState() == list.FilterApplied {
		return true
	}
	return false
}

// clearAllFilters resets all filters to their default state
func (m *Model) clearAllFilters() {
	m.currentFilter = "all"
	// Reset the fuzzy search filter by resetting the filter state
	m.list.ResetFilter()
	m.applyFilter()
}

func (m *Model) matchesCurrentFilter(issue model.Issue) bool {
	// Workspace repo filter (nil = all repos)
	if m.workspaceMode && m.activeRepos != nil {
		repoKey := strings.ToLower(ExtractRepoPrefix(issue.ID))
		if repoKey != "" && !m.activeRepos[repoKey] {
			return false
		}
	}

	switch m.currentFilter {
	case "all":
		return true
	case "open":
		return !isClosedLikeStatus(issue.Status)
	case "closed":
		return isClosedLikeStatus(issue.Status)
	case "ready":
		// Ready = Open/InProgress AND NO Open Blockers
		if isClosedLikeStatus(issue.Status) || issue.Status == model.StatusBlocked {
			return false
		}
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := m.issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				return false
			}
		}
		return true
	default:
		if strings.HasPrefix(m.currentFilter, "label:") {
			label := strings.TrimPrefix(m.currentFilter, "label:")
			for _, l := range issue.Labels {
				if l == label {
					return true
				}
			}
		}
		return false
	}
}

func (m *Model) filteredIssuesForActiveView() []model.Issue {
	filtered := make([]model.Issue, 0, len(m.issues))
	for _, issue := range m.issues {
		if m.matchesCurrentFilter(issue) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func (m *Model) refreshBoardAndGraphForCurrentFilter() {
	if !m.isBoardView {
		return
	}

	filteredIssues := m.filteredIssuesForActiveView()
	useSnapshot := m.snapshot != nil && m.snapshot.BoardState != nil && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues)
	if useSnapshot {
		useSnapshot = m.currentFilter == "all"
	}
	if useSnapshot {
		m.board.SetSnapshot(m.snapshot)
	} else {
		m.board.SetIssues(filteredIssues)
	}
}

func (m *Model) applyFilter() {
	var filteredItems []list.Item
	var filteredIssues []model.Issue

	for _, issue := range m.issues {
		if m.matchesCurrentFilter(issue) {
			item := IssueItem{
				Issue:      issue,
				DiffStatus: m.getDiffStatus(issue.ID),
				RepoPrefix: ExtractRepoPrefix(issue.ID),
			}
			filteredItems = append(filteredItems, item)
			filteredIssues = append(filteredIssues, issue)
		}
	}

	// Apply sort mode (bv-3ita)
	m.sortFilteredItems(filteredItems, filteredIssues)

	m.list.SetItems(filteredItems)
	if m.snapshot != nil && m.snapshot.BoardState != nil && m.currentFilter == "all" && (!m.workspaceMode || m.activeRepos == nil) && len(filteredIssues) == len(m.snapshot.Issues) {
		m.board.SetSnapshot(m.snapshot)
	} else {
		m.board.SetIssues(filteredIssues)
	}

	// Keep selection in bounds
	if len(filteredItems) > 0 && m.list.Index() >= len(filteredItems) {
		m.list.Select(0)
	}
	m.updateViewportContent()
}

// cycleSortMode cycles through available sort modes (bv-3ita)
func (m *Model) cycleSortMode() {
	m.sortMode = (m.sortMode + 1) % numSortModes
	m.applyFilter() // Re-apply filter with new sort
}

// sortFilteredItems sorts the filtered items based on current sortMode (bv-3ita)
func (m *Model) sortFilteredItems(items []list.Item, issues []model.Issue) {
	if len(items) == 0 {
		return
	}

	// Sort indices to keep items and issues in sync
	indices := make([]int, len(items))
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		iItem := items[indices[i]].(IssueItem)
		jItem := items[indices[j]].(IssueItem)

		switch m.sortMode {
		case SortCreatedAsc:
			// Oldest first
			return iItem.Issue.CreatedAt.Before(jItem.Issue.CreatedAt)
		case SortCreatedDesc:
			// Newest first
			return iItem.Issue.CreatedAt.After(jItem.Issue.CreatedAt)
		case SortPriority:
			// Priority ascending (P0 first)
			return iItem.Issue.Priority < jItem.Issue.Priority
		case SortUpdated:
			// Most recently updated first
			return iItem.Issue.UpdatedAt.After(jItem.Issue.UpdatedAt)
		default:
			// Default: creation date descending (newest first) (bd-ctu)
			return iItem.Issue.CreatedAt.After(jItem.Issue.CreatedAt)
		}
	})

	// Reorder items and issues based on sorted indices
	sortedItems := make([]list.Item, len(items))
	sortedIssues := make([]model.Issue, len(issues))
	for newIdx, oldIdx := range indices {
		sortedItems[newIdx] = items[oldIdx]
		sortedIssues[newIdx] = issues[oldIdx]
	}
	copy(items, sortedItems)
	copy(issues, sortedIssues)
}

func matchesRecipeStatus(status model.Status, filter string) bool {
	normalized := strings.ToLower(strings.TrimSpace(filter))
	statusKey := strings.ToLower(string(status))
	switch normalized {
	case string(model.StatusClosed):
		return isClosedLikeStatus(status)
	case string(model.StatusTombstone):
		return status == model.StatusTombstone
	case string(model.StatusOpen):
		return status == model.StatusOpen
	case string(model.StatusInProgress):
		return status == model.StatusInProgress
	case string(model.StatusBlocked):
		return status == model.StatusBlocked
	default:
		return statusKey == normalized
	}
}

// recalculateSplitPaneSizes updates list and viewport dimensions after pane ratio changes
func (m *Model) recalculateSplitPaneSizes() {
	if !m.isSplitView {
		return
	}

	bodyHeight := m.height - 1
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	// Calculate dimensions accounting for 2 panels with borders(2)+padding(2) = 4 overhead each
	availWidth := m.width - 8
	if availWidth < 10 {
		availWidth = 10
	}

	listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
	detailInnerWidth := availWidth - listInnerWidth

	listHeight := bodyHeight - 4
	if listHeight < 3 {
		listHeight = 3
	}

	m.list.SetSize(listInnerWidth, listHeight)
	m.viewport = viewport.New(detailInnerWidth, bodyHeight-2)
	m.renderer.SetWidthWithTheme(detailInnerWidth, m.theme)
	m.updateViewportContent()
}

// detailPaneWidth returns the inner width the detail pane would have at the
// current terminal width and split ratio. Used to decide whether the pane is
// too narrow to be readable (bd-dy7).
func (m *Model) detailPaneWidth() int {
	availWidth := m.width - 8
	if availWidth < 10 {
		availWidth = 10
	}
	listInnerWidth := int(float64(availWidth) * m.splitPaneRatio)
	return availWidth - listInnerWidth
}

func (m *Model) updateViewportContent() {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.viewport.SetContent("No issues selected")
		return
	}

	// Safe type assertion
	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		m.viewport.SetContent("Error: invalid item type")
		return
	}
	item := issueItem.Issue

	var sb strings.Builder

	if m.updateAvailable {
		sb.WriteString(fmt.Sprintf("⭐ **Update Available:** [%s](%s)\n\n", m.updateTag, m.updateURL))
	}

	// Title Block
	sb.WriteString(fmt.Sprintf("# %s %s\n", GetTypeIconMD(string(item.IssueType)), item.Title))

	// Meta Table
	sb.WriteString("| ID | Status | Priority | Assignee | Created |\n|---|---|---|---|---|\n")
	sb.WriteString(fmt.Sprintf("| **%s** | **%s** | %s | @%s | %s |\n\n",
		item.ID,
		strings.ToUpper(string(item.Status)),
		GetPriorityIcon(item.Priority),
		item.Assignee,
		item.CreatedAt.Format("2006-01-02"),
	))

	// Labels (bv-f103 fix: display labels in detail view)
	if len(item.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("**Labels:** %s\n\n", strings.Join(item.Labels, ", ")))
	}

	// Description
	if item.Description != "" {
		sb.WriteString("### Description\n")
		sb.WriteString(item.Description + "\n\n")
	}

	// Design Notes
	if item.Design != "" {
		sb.WriteString("### Design Notes\n")
		sb.WriteString(item.Design + "\n\n")
	}

	// Acceptance Criteria
	if item.AcceptanceCriteria != "" {
		sb.WriteString("### Acceptance Criteria\n")
		sb.WriteString(item.AcceptanceCriteria + "\n\n")
	}

	// Notes
	if item.Notes != "" {
		sb.WriteString("### Notes\n")
		sb.WriteString(item.Notes + "\n\n")
	}

	// Dependency Graph (Tree)
	if len(item.Dependencies) > 0 {
		rootNode := BuildDependencyTree(item.ID, m.issueMap, 3) // Max depth 3
		treeStr := RenderDependencyTree(rootNode)
		sb.WriteString("```\n" + treeStr + "```\n\n")
	}

	// Comments
	if len(item.Comments) > 0 {
		sb.WriteString(fmt.Sprintf("### Comments (%d)\n", len(item.Comments)))
		for _, comment := range item.Comments {
			sb.WriteString(fmt.Sprintf("> **%s** (%s)\n> \n> %s\n\n",
				comment.Author,
				FormatTimeRel(comment.CreatedAt),
				strings.ReplaceAll(comment.Text, "\n", "\n> ")))
		}
	}

	rendered, err := m.renderer.Render(sb.String())
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering markdown: %v", err))
	} else {
		m.viewport.SetContent(rendered)
	}
}

// truncateString truncates a string to maxLen runes with ellipsis.
// Uses rune-based counting to safely handle UTF-8 multi-byte characters.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// GetTypeIconMD returns the emoji icon for an issue type (for markdown)
func GetTypeIconMD(t string) string {
	switch t {
	case "bug":
		return "●"
	case "feature":
		return "▲"
	case "task":
		return "✔"
	case "epic":
		return "⚡"
	case "chore":
		return "○"
	default:
		return "·"
	}
}

// SetFilter sets the current filter and applies it (exposed for testing)
func (m *Model) SetFilter(f string) {
	m.currentFilter = f
	m.applyFilter()
}

// FilteredIssues returns the currently visible issues (exposed for testing)
func (m Model) FilteredIssues() []model.Issue {
	items := m.list.Items()
	issues := make([]model.Issue, 0, len(items))
	for _, item := range items {
		if issueItem, ok := item.(IssueItem); ok {
			issues = append(issues, issueItem.Issue)
		}
	}
	return issues
}

// EnableWorkspaceMode configures the model for workspace (multi-repo) view
func (m *Model) EnableWorkspaceMode(info WorkspaceInfo) {
	m.workspaceMode = info.Enabled
	m.availableRepos = normalizeRepoPrefixes(info.RepoPrefixes)
	m.activeRepos = nil // nil means all repos are active

	if info.RepoCount > 0 {
		if info.FailedCount > 0 {
			m.workspaceSummary = fmt.Sprintf("%d/%d repos", info.RepoCount-info.FailedCount, info.RepoCount)
		} else {
			m.workspaceSummary = fmt.Sprintf("%d repos", info.RepoCount)
		}
	}

	// Update delegate to show repo badges
	m.updateListDelegate()
}

// IsWorkspaceMode returns whether workspace mode is active
func (m Model) IsWorkspaceMode() bool {
	return m.workspaceMode
}

// rebuildListWithDiffInfo recreates list items with current diff state
func (m *Model) rebuildListWithDiffInfo() {
	m.applyFilter()
}

// IsTimeTravelMode returns whether time-travel mode is active
func (m Model) IsTimeTravelMode() bool {
	return m.timeTravelMode
}

// FocusState returns the current focus state as a string for testing (bv-5e5q).
// This enables testing focus transitions without exposing the internal focus type.
func (m Model) FocusState() string {
	switch m.focused {
	case focusList:
		return "list"
	case focusDetail:
		return "detail"
	case focusBoard:
		return "board"
	case focusTree:
		return "tree"
	case focusRepoPicker:
		return "repo_picker"
	case focusHelp:
		return "help"
	case focusQuitConfirm:
		return "quit_confirm"
	case focusLabelPicker:
		return "label_picker"
	case focusTutorial:
		return "tutorial"
	case focusUpdateModal:
		return "update_modal"
	case focusStatusPicker:
		return "status_picker"
	case focusEditModal:
		return "edit_modal"
	default:
		return "unknown"
	}
}

// getSelectedIssue returns the currently selected issue from the active view (list or tree).
// Returns nil if no issue is selected.
func (m *Model) getSelectedIssue() *model.Issue {
	if m.focused == focusTree || m.treeViewActive {
		id := m.tree.GetSelectedID()
		if id != "" {
			if issue, ok := m.issueMap[id]; ok {
				return issue
			}
		}
		return nil
	}
	// List view
	sel := m.list.SelectedItem()
	if sel == nil {
		return nil
	}
	item, ok := sel.(IssueItem)
	if !ok {
		return nil
	}
	if issue, ok := m.issueMap[item.Issue.ID]; ok {
		return issue
	}
	return nil
}

// IsBoardView returns true if the board view is active (bv-5e5q).
func (m Model) IsBoardView() bool {
	return m.isBoardView
}


// TreeSelectedID returns the ID of the currently selected tree node, or "".
func (m Model) TreeSelectedID() string {
	return m.tree.GetSelectedID()
}

// TreeNodeCount returns the number of visible nodes in the tree.
func (m Model) TreeNodeCount() int {
	return m.tree.NodeCount()
}

// BuildProjectEntries exposes buildProjectEntries for testing (bd-qjc).
func (m Model) BuildProjectEntries() []ProjectEntry {
	return m.buildProjectEntries()
}

// ApplyTreeFilter sets a filter on the tree view for testing (bd-qjc).
func (m *Model) ApplyTreeFilter(filter string) {
	m.tree.ApplyFilter(filter)
}

// TreeFilterActive returns true if the tree has an active filter (bd-qjc).
func (m Model) TreeFilterActive() bool {
	f := m.tree.GetFilter()
	return f != "" && f != "all"
}

// TreeSortField returns the current sort field of the tree view (bd-x3l).
func (m Model) TreeSortField() SortField {
	return m.tree.GetSortField()
}

// TreeSortDirection returns the current sort direction of the tree view (bd-x3l).
func (m Model) TreeSortDirection() SortDirection {
	return m.tree.GetSortDirection()
}

// TreeSortPopupOpen returns whether the sort popup overlay is visible (bd-u81).
func (m Model) TreeSortPopupOpen() bool {
	return m.tree.IsSortPopupOpen()
}

// TreeBookmarkedIDs returns the IDs of bookmarked tree nodes (bd-k4n).
func (m Model) TreeBookmarkedIDs() []string {
	return m.tree.TreeBookmarkedIDs()
}

// TreeFollowMode returns whether follow mode is active (bd-c0c).
func (m Model) TreeFollowMode() bool {
	return m.tree.GetFollowMode()
}

// TreeDetailHidden returns whether the detail panel is hidden in tree view (bd-80u).
func (m Model) TreeDetailHidden() bool {
	return m.treeDetailHidden
}


// ActiveProjectName returns the name of the currently loaded project (bd-q5z.8).
func (m Model) ActiveProjectName() string {
	return m.activeProjectName
}

// ProjectPickerCursor returns the cursor position in the project picker (bd-q5z.8).
func (m Model) ProjectPickerCursor() int {
	return m.projectPicker.Cursor()
}

// ProjectPickerFilteredCount returns how many projects match the current filter (bd-q5z.8).
func (m Model) ProjectPickerFilteredCount() int {
	return m.projectPicker.FilteredCount()
}

// PickerVisible returns whether the project picker panel is visible (bd-2me).
func (m Model) PickerVisible() bool {
	return m.pickerVisible
}

// exportToMarkdown exports all issues to a Markdown file with auto-generated filename
// renderTimeTravelPrompt renders the time-travel revision input overlay
func (m Model) renderTimeTravelPrompt() string {
	t := m.theme

	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 3).
		Align(lipgloss.Center)

	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	subtitleStyle := t.Renderer.NewStyle().
		Foreground(t.Subtext).
		Italic(true)

	exampleStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary)

	keyStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	textStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	// Build content
	content := titleStyle.Render("⏱️  Time-Travel Mode") + "\n\n" +
		subtitleStyle.Render("Compare current state with a historical revision") + "\n\n" +
		m.timeTravelInput.View() + "\n\n" +
		exampleStyle.Render("Examples: HEAD~5, main, v1.0.0, 2024-01-01, abc123") + "\n\n" +
		textStyle.Render("Press ") + keyStyle.Render("Enter") + textStyle.Render(" to compare, ") +
		keyStyle.Render("Esc") + textStyle.Render(" to cancel")

	box := boxStyle.Render(content)

	return lipgloss.Place(
		m.width,
		m.height-1,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// copyIssueToClipboard copies the selected issue to clipboard as Markdown
func (m *Model) copyIssueToClipboard() {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.statusMsg = "❌ No issue selected"
		m.statusIsError = true
		return
	}

	issueItem, ok := selectedItem.(IssueItem)
	if !ok {
		m.statusMsg = "❌ Invalid item type"
		m.statusIsError = true
		return
	}
	issue := issueItem.Issue

	// Format issue as Markdown
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s %s\n\n", GetTypeIconMD(string(issue.IssueType)), issue.Title))
	sb.WriteString(fmt.Sprintf("**ID:** %s  \n", issue.ID))
	sb.WriteString(fmt.Sprintf("**Status:** %s  \n", strings.ToUpper(string(issue.Status))))
	sb.WriteString(fmt.Sprintf("**Priority:** P%d  \n", issue.Priority))
	if issue.Assignee != "" {
		sb.WriteString(fmt.Sprintf("**Assignee:** @%s  \n", issue.Assignee))
	}
	sb.WriteString(fmt.Sprintf("**Created:** %s  \n", issue.CreatedAt.Format("2006-01-02")))

	if len(issue.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("**Labels:** %s  \n", strings.Join(issue.Labels, ", ")))
	}

	if issue.Description != "" {
		sb.WriteString(fmt.Sprintf("\n## Description\n\n%s\n", issue.Description))
	}

	if issue.AcceptanceCriteria != "" {
		sb.WriteString(fmt.Sprintf("\n## Acceptance Criteria\n\n%s\n", issue.AcceptanceCriteria))
	}

	// Dependencies
	if len(issue.Dependencies) > 0 {
		sb.WriteString("\n## Dependencies\n\n")
		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", dep.DependsOnID, dep.Type))
		}
	}

	// Copy to clipboard
	err := clipboard.WriteAll(sb.String())
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Clipboard error: %v", err)
		m.statusIsError = true
		return
	}

	m.statusMsg = fmt.Sprintf("📋 Copied %s to clipboard", issue.ID)
	m.statusIsError = false
}

// showSelfUpdateModal shows the self-update modal (bv-182)
func (m *Model) showSelfUpdateModal() {
	// Check if an update is available
	if !m.updateAvailable || m.updateTag == "" {
		m.statusMsg = "No update available - you're running the latest version"
		m.statusIsError = false
		return
	}

	// Create and show the modal
	m.updateModal = NewUpdateModal(m.updateTag, m.updateURL, m.theme)
	m.updateModal.SetSize(m.width, m.height)
	m.showUpdateModal = true
	m.focused = focusUpdateModal
}

func parseCommandLine(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for i := 0; i < len(input); {
		ch := input[i]
		if inSingle {
			if ch == '\'' {
				inSingle = false
				i++
				continue
			}
			current.WriteByte(ch)
			i++
			continue
		}
		if inDouble {
			switch ch {
			case '"':
				inDouble = false
				i++
				continue
			case '\\':
				if i+1 >= len(input) {
					return nil, fmt.Errorf("unterminated escape")
				}
				next := input[i+1]
				// In double quotes, only treat \" and \\ as escapes; otherwise preserve backslash.
				if next == '"' || next == '\\' {
					current.WriteByte(next)
					i += 2
					continue
				}
				current.WriteByte('\\')
				i++
				continue
			default:
				current.WriteByte(ch)
				i++
				continue
			}
		}

		switch ch {
		case ' ', '\t', '\n', '\r':
			flush()
			i++
		case '\'':
			inSingle = true
			i++
		case '"':
			inDouble = true
			i++
		case '\\':
			if i+1 >= len(input) {
				return nil, fmt.Errorf("unterminated escape")
			}
			next := input[i+1]
			if next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '\\' || next == '"' || next == '\'' {
				current.WriteByte(next)
				i += 2
				continue
			}
			current.WriteByte('\\')
			i++
		default:
			current.WriteByte(ch)
			i++
		}
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}
	flush()
	return args, nil
}

type editorCommandKind int

const (
	editorCommandOK editorCommandKind = iota
	editorCommandEmpty
	editorCommandTerminal
	editorCommandForbidden
)

type allowlistedGUIEditorKind int

const (
	allowlistedGUIEditorUnknown allowlistedGUIEditorKind = iota
	allowlistedGUIEditorOpenText
	allowlistedGUIEditorXdgOpen
	allowlistedGUIEditorCode
	allowlistedGUIEditorCodeInsiders
	allowlistedGUIEditorCursor
	allowlistedGUIEditorGedit
	allowlistedGUIEditorKate
	allowlistedGUIEditorXed
	allowlistedGUIEditorNotepad
)

var terminalEditorExecutables = map[string]bool{
	"vim":   true,
	"vi":    true,
	"nvim":  true,
	"nano":  true,
	"emacs": true,
	"pico":  true,
	"joe":   true,
	"ne":    true,
}

var forbiddenEditorExecutables = map[string]bool{
	// Shells and command interpreters.
	"sh":         true,
	"bash":       true,
	"zsh":        true,
	"fish":       true,
	"cmd":        true,
	"powershell": true,
	"pwsh":       true,
}

func normalizeExecutableBase(executable string) string {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return ""
	}
	base := executable
	if idx := strings.LastIndexAny(base, `/\`); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.ToLower(base)
	return strings.TrimSuffix(base, ".exe")
}

func classifyEditorCommand(editorArgs []string) (string, editorCommandKind) {
	if len(editorArgs) == 0 {
		return "", editorCommandEmpty
	}
	base := normalizeExecutableBase(editorArgs[0])
	if base == "" {
		return "", editorCommandEmpty
	}
	if terminalEditorExecutables[base] {
		return base, editorCommandTerminal
	}
	if forbiddenEditorExecutables[base] {
		return base, editorCommandForbidden
	}
	return base, editorCommandOK
}

func allowlistedGUIEditorKindForBase(base string) allowlistedGUIEditorKind {
	switch base {
	case "open":
		return allowlistedGUIEditorOpenText
	case "xdg-open":
		return allowlistedGUIEditorXdgOpen
	case "code":
		return allowlistedGUIEditorCode
	case "code-insiders":
		return allowlistedGUIEditorCodeInsiders
	case "cursor":
		return allowlistedGUIEditorCursor
	case "gedit":
		return allowlistedGUIEditorGedit
	case "kate":
		return allowlistedGUIEditorKate
	case "xed":
		return allowlistedGUIEditorXed
	case "notepad":
		return allowlistedGUIEditorNotepad
	default:
		return allowlistedGUIEditorUnknown
	}
}

func allowlistedGUIEditorDisplayName(kind allowlistedGUIEditorKind) string {
	switch kind {
	case allowlistedGUIEditorOpenText:
		return "default text editor"
	case allowlistedGUIEditorXdgOpen:
		return "default app"
	case allowlistedGUIEditorCode:
		return "code"
	case allowlistedGUIEditorCodeInsiders:
		return "code-insiders"
	case allowlistedGUIEditorCursor:
		return "cursor"
	case allowlistedGUIEditorGedit:
		return "gedit"
	case allowlistedGUIEditorKate:
		return "kate"
	case allowlistedGUIEditorXed:
		return "xed"
	case allowlistedGUIEditorNotepad:
		return "notepad"
	default:
		return "editor"
	}
}

func startAllowlistedGUIEditor(kind allowlistedGUIEditorKind, targetFile string) (allowlistedGUIEditorKind, error) {
	switch kind {
	case allowlistedGUIEditorOpenText:
		return kind, exec.Command("open", "-t", targetFile).Start()
	case allowlistedGUIEditorXdgOpen:
		return kind, exec.Command("xdg-open", targetFile).Start()
	case allowlistedGUIEditorCode:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `code` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Visual Studio Code", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("code"); err == nil {
			return kind, exec.Command("code", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("code not found in PATH")
	case allowlistedGUIEditorCodeInsiders:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `code-insiders` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Visual Studio Code - Insiders", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("code-insiders"); err == nil {
			return kind, exec.Command("code-insiders", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("code-insiders not found in PATH")
	case allowlistedGUIEditorCursor:
		if runtime.GOOS == "darwin" {
			// Prefer launching the app directly so we don't depend on the `cursor` CLI being installed in PATH.
			if err := exec.Command("open", "-a", "Cursor", targetFile).Start(); err == nil {
				return kind, nil
			}
		}
		if _, err := exec.LookPath("cursor"); err == nil {
			return kind, exec.Command("cursor", targetFile).Start()
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("xdg-open"); err == nil {
				return allowlistedGUIEditorXdgOpen, exec.Command("xdg-open", targetFile).Start()
			}
		}
		return kind, fmt.Errorf("cursor not found in PATH")
	case allowlistedGUIEditorGedit:
		return kind, exec.Command("gedit", targetFile).Start()
	case allowlistedGUIEditorKate:
		return kind, exec.Command("kate", targetFile).Start()
	case allowlistedGUIEditorXed:
		return kind, exec.Command("xed", targetFile).Start()
	case allowlistedGUIEditorNotepad:
		return kind, exec.Command("notepad", targetFile).Start()
	default:
		return kind, fmt.Errorf("unsupported editor")
	}
}

// openInEditor opens the beads file in the user's preferred editor
// Uses m.beadsPath which respects issues.jsonl (canonical per beads upstream)
func (m *Model) openInEditor() {
	// Use the configured beadsPath instead of hardcoded path
	beadsFile := m.beadsPath
	if beadsFile == "" {
		cwd, _ := os.Getwd()
		if found, err := loader.FindJSONLPath(filepath.Join(cwd, ".beads")); err == nil {
			beadsFile = found
		}
	}
	if beadsFile == "" {
		m.statusMsg = "❌ No .beads directory or beads.jsonl found"
		m.statusIsError = true
		return
	}
	if _, err := os.Stat(beadsFile); os.IsNotExist(err) {
		m.statusMsg = fmt.Sprintf("❌ Beads file not found: %s", beadsFile)
		m.statusIsError = true
		return
	}

	// Determine editor - prefer GUI editors that work in background
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	ignoredEditorBase := ""
	var requestedEditorKind allowlistedGUIEditorKind
	if editor != "" {
		editorArgs, err := parseCommandLine(editor)
		if err != nil {
			m.statusMsg = fmt.Sprintf("❌ Invalid $EDITOR/$VISUAL: %v", err)
			m.statusIsError = true
			return
		}

		editorBase, kind := classifyEditorCommand(editorArgs)
		switch kind {
		case editorCommandTerminal:
			m.statusMsg = fmt.Sprintf("⚠️ %s is a terminal editor - set $EDITOR to a GUI editor or quit first", editorBase)
			m.statusIsError = true
			return
		case editorCommandForbidden:
			m.statusMsg = fmt.Sprintf("❌ Refusing to run %s as editor (shell/interpreter). Set $EDITOR to a GUI editor", editorBase)
			m.statusIsError = true
			return
		case editorCommandEmpty:
			m.statusMsg = "❌ Invalid $EDITOR/$VISUAL: empty command"
			m.statusIsError = true
			return
		default:
			requestedEditorKind = allowlistedGUIEditorKindForBase(editorBase)
			if requestedEditorKind == allowlistedGUIEditorUnknown {
				ignoredEditorBase = editorBase
				editor = ""
			}
		}
	}

	// If no editor set, try platform-specific GUI options
	if editor == "" && requestedEditorKind == allowlistedGUIEditorUnknown {
		switch runtime.GOOS {
		case "darwin":
			requestedEditorKind = allowlistedGUIEditorOpenText
		case "windows":
			requestedEditorKind = allowlistedGUIEditorNotepad
		case "linux":
			// Try xdg-open first, then common GUI editors
			for _, tryEditor := range []string{"xdg-open", "code", "code-insiders", "cursor", "gedit", "kate", "xed"} {
				if _, err := exec.LookPath(tryEditor); err == nil {
					requestedEditorKind = allowlistedGUIEditorKindForBase(tryEditor)
					break
				}
			}
		}
	}

	if requestedEditorKind == allowlistedGUIEditorUnknown {
		m.statusMsg = "❌ No GUI editor found. Set $EDITOR to a GUI editor"
		m.statusIsError = true
		return
	}

	actualKind, err := startAllowlistedGUIEditor(requestedEditorKind, beadsFile)
	if err != nil {
		m.statusMsg = fmt.Sprintf("❌ Failed to open editor: %v", err)
		m.statusIsError = true
		return
	}
	requestedEditorKind = actualKind

	if ignoredEditorBase != "" {
		m.statusMsg = fmt.Sprintf("📝 Opened in %s (ignored $EDITOR=%s)", allowlistedGUIEditorDisplayName(requestedEditorKind), ignoredEditorBase)
	} else {
		m.statusMsg = fmt.Sprintf("📝 Opened in %s", allowlistedGUIEditorDisplayName(requestedEditorKind))
	}
	m.statusIsError = false
}

// Stop cleans up resources (file watcher, instance lock, background worker, etc.)
// Should be called when the program exits
func (m *Model) Stop() {
	if m.backgroundWorker != nil {
		m.backgroundWorker.Stop()
	}
	if m.watcher != nil {
		m.watcher.Stop()
	}
	if len(m.pooledIssues) > 0 {
		loader.ReturnIssuePtrsToPool(m.pooledIssues)
		m.pooledIssues = nil
	}
}

// RenderDebugView renders a specific view for debugging purposes.
// This is used by --debug-render to capture TUI output without running interactively.
func (m *Model) RenderDebugView(viewName string, width, height int) string {
	m.width = width
	m.height = height
	m.ready = true

	switch viewName {
	case "board":
		return m.board.View(width, height-1)
	default:
		return "Unknown view: " + viewName
	}
}

func formatReloadDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
