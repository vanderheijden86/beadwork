# Beadwork (b9s)

![Go Version](https://img.shields.io/github/go-mod/go-version/vanderheijden86/beadwork?style=for-the-badge&color=6272a4)
![License](https://img.shields.io/badge/License-MIT-50fa7b?style=for-the-badge)

> A fast, focused TUI viewer and editor for [Beads](https://github.com/steveyegge/beads) issue tracking projects.

## What is this?

Beadwork is a terminal-based interface for browsing, editing, and managing issues stored in `.beads/issues.jsonl`. It renders your issue data as an interactive TUI with list, tree, and kanban board views, a detail panel with Markdown rendering, and inline editing.

Originally forked from [b9s](https://github.com/Dicklesworthstone/beads_viewer), beadwork has been **stripped to its core**: the TUI viewer. Added features are a full-fledged treeview and editing capabilities. The upstream project's graph analysis engine, robot protocol, export wizards, semantic search, drift detection, recipe system, and other advanced features have been removed to keep the tool small, fast, and focused on the primary use case: reading and updating issues from the terminal.

### Why strip it down?

The upstream b9s is an impressive piece of software with a graph analysis engine (PageRank, betweenness, HITS, critical path), AI agent protocols, static site export, time-travel diffs, sprint analytics, and more. That breadth is its strength, but it also means ~90k lines of Go source code, ~108k lines of tests, heavy dependencies like `gonum`, and complexity that isn't needed if all you want is a terminal viewer/editor.

Beadwork takes the opposite approach: **do fewer things well**. By stripping the codebase down to ~27k lines of source and ~26k lines of tests, and removing heavy vendor dependencies like `gonum`, beadwork starts faster, compiles faster, and is easier to understand, maintain, and contribute to.

**What was removed:**

- Graph analysis engine (PageRank, betweenness, HITS, critical path, eigenvector, density, cycles)
- Robot mode / AI agent JSON protocol (`--robot-*` flags)
- Static site export (`--export-*`, `--pages`)
- Semantic/hybrid search
- Recipe system (YAML filter presets)
- Time-travel / git history comparison
- Sprint analytics and burndown charts
- Label health scoring and cross-label flow analysis
- Drift detection and baseline comparison
- Correlation engine
- Instance locking
- CASS sessions
- Agent prompt integration
- Velocity comparison and flow matrix views
- `gonum`, `x/image`, `x/sync`, SVG rendering, and other heavy dependencies

**What remains:**

- List view with fuzzy search, sorting, and filtering
- Tree view with parent/child hierarchy and split-pane detail
- Kanban board with status/priority/type swimlanes
- Detail panel with full Markdown rendering
- Inline editing (title, status, priority, assignee, labels, description, notes)
- Live reload on file changes (filesystem watcher + background worker)
- Self-updating (`--update`, `--check-update`, `--rollback`)
- Repository prefix filtering (`--repo`)
- Large dataset handling (tiered loading for 1k-20k+ issues)

### Relationship to the original

Full credit goes to [@Dicklesworthstone](https://github.com/Dicklesworthstone) for the original architecture and implementation of b9s. The Bubbletea model structure, the background worker pattern, the file watcher integration, and the foundational UI components are all his work. Beadwork simply removes the features we don't use and makes different UX choices where our workflows diverge.

Per the upstream project's [contribution guidelines](https://github.com/Dicklesworthstone/beads_viewer/blob/main/CONTRIBUTING.md), b9s does not accept external pull requests. Beadwork exists as a separate fork for users who want a leaner tool and the ability to contribute.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install vanderheijden86/tap/b9s
```

### From source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/vanderheijden86/beadwork.git
cd beadwork
make install
```

This installs the `b9s` binary to your `$GOPATH/bin`. Make sure that directory is on your `PATH`.

For best display, use a terminal with a [Nerd Font](https://www.nerdfonts.com/).

## Quick Start

Navigate to any project initialized with `bd init` and run:

```bash
b9s
```

Press `?` for keyboard shortcuts or `` ` `` (backtick) for the interactive tutorial.

## Keyboard Quick Reference

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j` / `k` | Next / Previous | `q` / `Esc` | Quit / Back |
| `g` / `G` | Top / Bottom | `Tab` | Switch focus |
| `/` | Fuzzy search | `s` | Cycle sort mode |
| `o` / `c` / `r` / `a` | Filter: Open / Closed / Ready / All | `l` | Label picker |

| Key | View |p
|-----|------|
| `b` | Kanban board |
| `E` | Tree view |
| `e` | Edit issue |

## Acknowledgments

- **Steve Yegge** for the vision behind [Beads](https://github.com/steveyegge/beads), a refreshingly simple approach to issue tracking that respects developers' workflows.
- **[@Dicklesworthstone](https://github.com/Dicklesworthstone)** for the original [b9s](https://github.com/Dicklesworthstone/beads_viewer), whose architecture and implementation form the foundation of this project.
- The **[Charm](https://charm.sh)** team for [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), [Huh](https://github.com/charmbracelet/huh), and [Glamour](https://github.com/charmbracelet/glamour), the terminal UI libraries that make building beautiful CLI tools a joy.

## License

MIT License. See [LICENSE](LICENSE) for details.
