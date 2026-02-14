# Beadwork (bw) — a community fork of Beads Viewer

![Go Version](https://img.shields.io/github/go-mod/go-version/vanderheijden86/beadwork?style=for-the-badge&color=6272a4)
![License](https://img.shields.io/badge/License-MIT-50fa7b?style=for-the-badge)

> **A community-maintained fork of [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), the terminal interface for the [Beads](https://github.com/steveyegge/beads) issue tracker.**

## Why this fork?

The original [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) by [@Dicklesworthstone](https://github.com/Dicklesworthstone) is an excellent piece of software. The architecture, the graph analysis engine, the robot protocol for AI agents: it's one of the best TUIs out there. Full credit goes to him for the design and implementation.

Per the project's [contribution guidelines](https://github.com/Dicklesworthstone/beads_viewer/blob/main/CONTRIBUTING.md), beads_viewer does not accept external pull requests. Feature requests and bug reports are welcome and the maintainer does incorporate community feedback, but the codebase is maintained solely by its author. That's a perfectly valid approach that keeps the project focused and avoids the overhead of reviewing external code.

**Beadwork** exists because I'm a long-time TUI user (K9s, Emacs) with strong opinions about navigation, keybindings, and information density. Rather than filing feature requests for changes that reflect personal workflow preferences, it made more sense to maintain a fork where I can build exactly the UX I want, and share it with anyone who feels the same way.

No hard feelings toward the original project. This is simply a differentiated offering: same powerful foundation, different UX choices. The goal is to expand the overall user base of the [Beads](https://github.com/steveyegge/beads) ecosystem by catering to different preferences.

### What's different from the original?

- **Open to contributions**: PRs are welcome here
- **Renamed binary**: `bw` instead of `bv`, so both can coexist on the same machine
- **Tree view with split pane**: hierarchical issue view with a detail panel alongside
- **Sort improvements**: persistent sort order, sort indicator in the header
- **Opinionated UX defaults**: navigation and layout choices shaped by Emacs/K9s habits

### What's the same?

All the core functionality from beads_viewer is preserved: list view, kanban board, graph view, insights dashboard, robot mode for AI agents, and the full graph analysis engine (PageRank, betweenness, HITS, critical path, and more). The excellent architecture and design are entirely [@Dicklesworthstone](https://github.com/Dicklesworthstone)'s work.

<div align="center" style="margin: 1.2em 0;">
  <table>
    <tr>
      <td align="center" style="padding: 8px;">
        <img src="screenshots/screenshot_01__main_screen.webp" alt="Main split view" width="420" />
        <div><sub>Main split view: fast list + rich details</sub></div>
      </td>
      <td align="center" style="padding: 8px;">
        <img src="screenshots/screenshot_03__kanban_view.webp" alt="Kanban board" width="420" />
        <div><sub>Kanban board (`b`) for flow at a glance</sub></div>
      </td>
    </tr>
    <tr>
      <td align="center" style="padding: 8px;">
        <img src="screenshots/screenshot_02__insights_view.webp" alt="Insights view" width="420" />
        <div><sub>Insights panel: PageRank, critical path, cycles</sub></div>
      </td>
      <td align="center" style="padding: 8px;">
        <img src="screenshots/screenshot_04__graph_view.webp" alt="Graph view" width="420" />
        <div><sub>Graph view (`g`): navigate the dependency DAG</sub></div>
      </td>
    </tr>
  </table>
</div>

## Installation

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/vanderheijden86/beadwork.git
cd beadwork
make install
```

This installs the `bw` binary to your `$GOPATH/bin`. Make sure that directory is on your `PATH`.

For best display, use a terminal with a [Nerd Font](https://www.nerdfonts.com/).

---

## Feature Highlights

- **Instant browsing**: zero-latency navigation with Vim keys (`j`/`k`), split-view dashboard, Markdown rendering, live reload
- **Graph intelligence**: 9 graph-theoretic metrics (PageRank, betweenness, HITS, critical path, eigenvector, degree, density, cycles, topo sort) surface hidden project dynamics
- **Multiple views**: list, tree, kanban board, insights dashboard, graph visualizer, history, plan, flow matrix, attention, and label analytics
- **AI-ready robot mode**: structured JSON output for AI coding agents with pre-computed graph analysis (`--robot-triage`, `--robot-plan`, `--robot-insights`)
- **Time-travel**: compare project state across any two git revisions, detect regressions, track progress
- **Recipe system**: YAML-based view configurations for saved, shareable filter presets
- **Static site export**: self-contained HTML visualization with force-directed graphs
- **Sprint & label analytics**: burndown charts, label health scoring, cross-label flow analysis

---

## Documentation

| Document | Description |
|----------|-------------|
| **[User Guide](docs/USER_GUIDE.md)** | Complete guide to all views, features, keyboard shortcuts, configuration, and troubleshooting |
| **[Architecture & Technical Design](docs/ARCHITECTURE.md)** | System architecture, algorithm deep-dives, performance specs, and design philosophy |
| **[Robot Mode: AI Agent Protocol](docs/ROBOT_MODE.md)** | AI agent integration, CLI reference, JSON schemas, and the AGENTS.md blurb |

---

## Quick Start

### Interactive TUI

Navigate to any project initialized with `bd init` and run:

```bash
bw
```

Press `?` for keyboard shortcuts or `` ` `` (backtick) for the interactive tutorial.

### AI Agent Mode

```bash
bw --robot-triage          # Full triage: ranked recommendations + project health
bw --robot-next            # Minimal: just the top pick + claim command
bw --robot-triage --format toon   # Token-optimized output for LLMs
bw --robot-help            # Full robot help
```

See the [Robot Mode guide](docs/ROBOT_MODE.md) for the complete protocol.

---

## Keyboard Quick Reference

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j` / `k` | Next / Previous | `q` / `Esc` | Quit / Back |
| `g` / `G` | Top / Bottom | `Tab` | Switch focus |
| `/` | Fuzzy search | `s` | Cycle sort mode |
| `o` / `c` / `r` / `a` | Filter: Open / Closed / Ready / All | `l` | Label picker |

| Key | View |
|-----|------|
| `b` | Kanban board |
| `i` | Insights dashboard |
| `g` | Graph visualizer |
| `E` | Tree view |
| `h` | History view |
| `a` | Actionable plan |
| `f` | Flow matrix |
| `t` / `T` | Time-travel / Quick time-travel (HEAD~5) |

Full keyboard reference in the [User Guide](docs/USER_GUIDE.md#keyboard-control-map).

---

## Acknowledgments & Credits

`bv` stands on the shoulders of giants. We're deeply grateful to the maintainers and contributors of these exceptional open source projects:

### Foundation

| Project | Author | Description |
|---------|--------|-------------|
| [**Beads**](https://github.com/steveyegge/beads) | Steve Yegge | The elegant git-native issue tracking system that `bv` was built to complement |

### Go Libraries (TUI & CLI)

| Library | Author | What We Use It For |
|---------|--------|-------------------|
| [**Bubble Tea**](https://github.com/charmbracelet/bubbletea) | [Charm](https://charm.sh) | The Elm-inspired TUI framework powering all interactive views |
| [**Lip Gloss**](https://github.com/charmbracelet/lipgloss) | [Charm](https://charm.sh) | Beautiful terminal styling—colors, borders, layouts |
| [**Bubbles**](https://github.com/charmbracelet/bubbles) | [Charm](https://charm.sh) | Ready-made components: lists, text inputs, spinners, viewports |
| [**Huh**](https://github.com/charmbracelet/huh) | [Charm](https://charm.sh) | Interactive forms and prompts for the deployment wizard |
| [**Glamour**](https://github.com/charmbracelet/glamour) | [Charm](https://charm.sh) | Markdown rendering with syntax highlighting in terminal |
| [**modernc.org/sqlite**](https://modernc.org/sqlite) | modernc.org | Pure-Go SQLite with FTS5 full-text search for static site export |
| [**Gonum**](https://github.com/gonum/gonum) | Gonum Authors | Graph algorithms: PageRank, betweenness centrality, SCC |
| [**fsnotify**](https://github.com/fsnotify/fsnotify) | fsnotify | File system watching for live reload |
| [**clipboard**](https://github.com/atotto/clipboard) | atotto | Cross-platform clipboard for copy-to-clipboard features |

### JavaScript Libraries (Static Viewer)

| Library | Author | What We Use It For |
|---------|--------|-------------------|
| [**force-graph**](https://github.com/vasturiano/force-graph) | [Vasco Asturiano](https://github.com/vasturiano) | Beautiful interactive force-directed graph visualization |
| [**D3.js**](https://d3js.org/) | Mike Bostock / Observable | Data visualization foundation and graph physics |
| [**Alpine.js**](https://alpinejs.dev/) | Caleb Porzio | Lightweight reactive UI framework |
| [**sql.js**](https://github.com/sql-js/sql.js) | sql.js contributors | SQLite compiled to WebAssembly for client-side queries |
| [**Chart.js**](https://www.chartjs.org/) | Chart.js contributors | Interactive charts: burndown, priority distribution, heatmaps |
| [**Mermaid**](https://mermaid.js.org/) | Knut Sveidqvist | Dependency graph diagrams in Markdown |
| [**DOMPurify**](https://github.com/cure53/DOMPurify) | cure53 | XSS-safe HTML sanitization |
| [**Marked**](https://marked.js.org/) | marked contributors | Fast Markdown parsing |
| [**Tailwind CSS**](https://tailwindcss.com/) | Tailwind Labs | Utility-first CSS framework |

### Special Thanks

- The entire **[Charm](https://charm.sh)** team for creating the most delightful terminal UI ecosystem in existence. Their libraries make building beautiful CLI tools a joy.
- **[Vasco Asturiano](https://github.com/vasturiano)** for the incredible `force-graph` library and the broader ecosystem of visualization tools.
- **Steve Yegge** for the vision behind Beads—a refreshingly simple approach to issue tracking that respects developers' workflows.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
