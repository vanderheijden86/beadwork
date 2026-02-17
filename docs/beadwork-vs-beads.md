# Beadwork (bw) vs Beads (bd): Separation of Concerns

This document maps out what beadwork handles, what beads handles, and where they overlap. The goal is to clarify the boundaries between the two tools.

## Summary

**Beads (bd)** is the issue tracker: it owns the data. Create, update, close, sync, dependencies, labels, comments, and the JSONL/SQLite storage layer.

**Beadwork (bw)** started as a TUI viewer for that data, but has grown into a full analysis and intelligence platform. It reads the same `.beads/issues.jsonl` file and adds graph analysis, AI agent protocols, deployment, search, and project health monitoring on top.

The result: bw is no longer "just a viewer." It is a project intelligence layer that happens to include a TUI.

---

## Feature Matrix

### 1. Core Issue CRUD

| Capability | bd | bw |
|---|---|---|
| Create issues | Yes | Yes (edit modal via `bd` CLI) |
| Update issues | Yes | Yes (edit modal via `bd` CLI) |
| Close issues | Yes | No |
| Delete issues | Yes | No |
| List/filter issues | Yes | Yes (TUI + robot flags) |
| Comments | Yes | Display only |
| Labels | Yes (manage) | Yes (display + health analysis) |
| Dependencies | Yes (manage) | Yes (display + graph analysis) |
| Epics | Yes (manage) | Display only |
| Search (text) | Yes | Yes (plus semantic/vector search) |

**Verdict**: bd owns CRUD. bw delegates mutations back to `bd` CLI. No overlap in write path.

---

### 2. Views and Visualization (bw only)

These are purely bw concerns with no equivalent in bd:

| Feature | Package | Description |
|---|---|---|
| List view | `pkg/ui/` | Paginated issue list with sort, filter |
| Tree view | `pkg/ui/` | Hierarchical epic/child view with split pane |
| Board view | `pkg/ui/` | Kanban-style columns by status |
| Graph view | `pkg/ui/` | Interactive dependency DAG navigation |
| Insights dashboard | `pkg/ui/` | PageRank, bottlenecks, critical path display |

---

### 3. Graph Analysis Engine (bw only)

This is the biggest area that goes well beyond "viewing." The analysis engine (~15k lines) computes graph-theoretic metrics over the dependency DAG:

| Feature | Files | What it does |
|---|---|---|
| PageRank | `pkg/analysis/graph.go` | Recursive dependency importance scoring |
| Betweenness centrality | `pkg/analysis/betweenness_approx.go` | Identifies bottleneck issues |
| HITS (hubs/authorities) | `pkg/analysis/graph.go` | Distinguishes epics from leaf tasks |
| Critical path | `pkg/analysis/graph.go` | Longest dependency chain |
| Cycle detection | `pkg/analysis/graph_cycles.go` | Finds circular dependencies |
| Articulation points | `pkg/analysis/graph.go` | Single-point-of-failure issues |
| Triage/priority scoring | `pkg/analysis/triage.go`, `priority.go` | AI-ready ranked work recommendations |
| What-if analysis | `pkg/analysis/whatif.go` | "What happens if I close X?" |
| Dependency suggestions | `pkg/analysis/dependency_suggest.go` | Suggests missing edges |
| Duplicate detection | `pkg/analysis/duplicates.go` | Finds similar issues |
| Label health | `pkg/analysis/label_health.go` | Label coverage, staleness |
| Label suggestions | `pkg/analysis/label_suggest.go` | Auto-suggest labels for issues |
| Risk scoring | `pkg/analysis/risk.go` | Risk assessment per issue |
| ETA estimation | `pkg/analysis/eta.go` | Completion time forecasting |
| Execution planning | `pkg/analysis/plan.go` | Dependency-respecting work plans |
| Suggestion engine | `pkg/analysis/suggest_all.go` | Combined suggestions (deps, labels, dupes, cycles) |

**bd has none of this.** bd stores dependencies but does not analyze them. The graph intelligence is entirely bw's domain.

---

### 4. AI Agent Protocol (bw only)

The `--robot-*` flags expose bw's analysis as structured JSON for AI agents. This is a major feature area (~30 flags):

- `--robot-triage`: Unified mega-command with ranked recommendations
- `--robot-next`: Single top pick for "what should I work on?"
- `--robot-insights`: Full graph analysis output
- `--robot-plan`: Dependency-respecting execution plan
- `--robot-priority`: Priority-scored recommendations
- `--robot-impact`: Impact analysis for a specific issue
- `--robot-suggest`: Smart suggestions (duplicates, deps, labels)
- `--robot-alerts`: Drift + proactive alerts
- `--robot-graph`: Dependency graph as JSON/DOT/Mermaid
- `--robot-search`: Semantic search results
- `--robot-docs`: Self-documenting JSON for agents
- Plus ~20 more specialized robot commands

**bd has no robot protocol.** It has `--json` output for individual commands, but no analysis-driven agent interface.

---

### 5. Git-to-Issue Correlation (bw only)

| Feature | Package | Description |
|---|---|---|
| Bead history | `pkg/correlation/` | Links git commits to issue lifecycle events |
| Co-commit analysis | `pkg/correlation/cocommit.go` | Files that change together |
| Temporal analysis | `pkg/correlation/temporal.go` | Time-based commit patterns |
| Causality detection | `pkg/correlation/causality.go` | Causal relationships between changes |
| File hotspots | `pkg/correlation/` | Most-changed files per issue |
| Orphan detection | `pkg/correlation/orphan.go` | Issues referenced in commits but still open |
| Feedback loop | `pkg/correlation/feedback.go` | Accept/reject correlations to tune accuracy |

**bd** has `bd orphans` (finds orphaned issues) but no deep correlation analysis.

---

### 6. Drift Detection and Baselines (bw only)

| Feature | Package | Description |
|---|---|---|
| Save baseline | `pkg/baseline/` | Snapshot current metrics as reference point |
| Drift detection | `pkg/drift/` | Compare current state to baseline, flag regressions |
| CI integration | `--check-drift` | Exit codes for CI pipelines (0=OK, 1=critical, 2=warning) |

**bd has none of this.**

---

### 7. Export, Deployment, and Static Sites (bw only)

| Feature | Package | Description |
|---|---|---|
| Markdown export | `pkg/export/markdown.go` | Generate report as `.md` |
| Static site export | `pkg/export/` | Full HTML site with interactive graphs |
| GitHub Pages deploy | `pkg/export/github.go` | Automated deployment wizard |
| Cloudflare Pages deploy | `pkg/export/cloudflare.go` | Automated deployment wizard |
| Preview server | `pkg/export/preview.go` | Local HTTP preview with live-reload |
| SQLite export | `pkg/export/sqlite_export.go` | Export to queryable SQLite DB |
| Graph export | `pkg/export/graph_export.go` | SVG/PNG/HTML graph rendering |
| Hooks | `pkg/hooks/` | Pre/post-export automation hooks |
| Mermaid diagrams | `pkg/export/mermaid_generator.go` | Generate Mermaid graph syntax |

**bd** has `bd export` (JSONL/Obsidian) and `bd sync` (JSONL flush), but no static sites, no deployment wizards, no graph rendering.

---

### 8. Semantic Search (bw only)

| Feature | Package | Description |
|---|---|---|
| Vector embeddings | `pkg/search/embedder.go` | Hash-based or external embeddings |
| Hybrid scoring | `pkg/search/hybrid_scorer.go` | Combined lexical + semantic ranking |
| Search presets | `pkg/search/presets.go` | Pre-configured search profiles |
| Index sync | `pkg/search/index_sync.go` | Keep search index in sync with issues |
| Cass integration | `pkg/cass/` | Optional external semantic code search |

**bd** has `bd search` (text substring matching) but no vector/semantic search.

---

### 9. Project Health and Monitoring (bw only)

| Feature | Package | Description |
|---|---|---|
| Performance metrics | `pkg/metrics/` | Internal timing and cache stats |
| Recipes | `pkg/recipe/` | Saved view/filter configurations |
| File watching | `pkg/watcher/` | Live-reload on `.beads/` changes |
| Multi-repo workspaces | `pkg/workspace/` | Aggregate issues across repositories |
| Self-update | `pkg/updater/` | Check/download new bw versions from GitHub |

---

### 10. Shared Concerns (overlap)

| Concern | bd | bw |
|---|---|---|
| JSONL loading | SQLite + JSONL import | Direct JSONL parse (`pkg/loader/`) |
| Issue model | Own types | Own types (`pkg/model/`) |
| Agent file management | `bd onboard`, `bd setup` | `pkg/agents/` (detect/inject AGENTS.md blurb) |
| Git worktree support | `bd worktree` | Loader resolves main repo root |
| Instance locking | `bd` uses SQLite locks | `pkg/instance/` (PID-based lock file) |

The data model is independently defined in both projects. They agree on the JSONL schema but have separate Go type definitions.

---

## Architectural Relationship

```
                    .beads/issues.jsonl
                           |
              +------------+------------+
              |                         |
         Beads (bd)               Beadwork (bw)
              |                         |
     Issue tracker CLI          Analysis + TUI platform
              |                         |
  - CRUD operations            - Graph engine (15k lines)
  - Dependencies               - 9 graph metrics
  - Labels, comments           - AI robot protocol (30+ flags)
  - Sync/daemon                - Git correlation engine
  - SQLite backend             - Drift detection
  - Merge driver               - Export/deploy (Pages, CF)
  - Jira/Linear/GitLab         - Semantic search
  - Gates, slots               - Static site generator
  - Formulas, molecules        - Multi-repo workspaces
                               - Recipes, hooks
                               - Self-updater
```

---

## What Do We Actually Use? (Usage Audit)

To decide what to keep, we audited the actual workflow: the global CLAUDE.md, CLAUDE-REFERENCE.md, beads plugin (skills, task agent, workflow commands), and daily session patterns.

### Actively Used

| Feature | Evidence | Verdict |
|---|---|---|
| **TUI viewer** (list, tree, board views) | Launched interactively as `bw` to browse issues | **KEEP** |
| **Edit modal** (huh-based forms) | Recently built; edits issues via `bd` CLI from inside the TUI | **KEEP** |
| **JSONL loader** (`pkg/loader/`) | Core data pipeline, required for anything to work | **KEEP** |
| **Issue model** (`pkg/model/`) | Core types, required for anything to work | **KEEP** |
| **File watcher** (`pkg/watcher/`) | Live-reload when `.beads/` changes on disk | **KEEP** (supports the TUI) |
| **Self-updater** (`pkg/updater/`) | `--check-update` for new versions | **KEEP** (small, useful) |

### Referenced in CLAUDE.md but Not Actually Invoked

| Feature | Evidence | Verdict |
|---|---|---|
| `bv --robot-next` | Listed as "OR" alternative to `bd ready` in CLAUDE.md step 1; never invoked in any observed session | **NOT USED** |
| `bv --robot-triage` | Listed as "OR" alternative to `bd ready` in CLAUDE.md step 1; never invoked in any observed session | **NOT USED** |
| Robot commands (30+ flags) | Documented extensively in CLAUDE-REFERENCE.md; the beads plugin and task agent use `bd` commands exclusively, not `bw --robot-*` | **NOT USED** |

The CLAUDE.md and CLAUDE-REFERENCE.md reference robot commands because the upstream beads_viewer documentation promotes them. But the actual daily workflow uses `bd ready`, `bd list`, `bd show` for finding work. The robot protocol is entirely bypassed.

### Never Used

| Feature | Lines of Code | Reason it is unused |
|---|---|---|
| **Graph analysis engine** (PageRank, betweenness, HITS, critical path, triage, risk, ETA, what-if, suggestions) | ~15,000 | `bd` already handles dependency ordering; agents use `bd ready` not `bw --robot-triage` |
| **Robot protocol** (30+ `--robot-*` flags) | ~3,000 (in `cmd/`) | The beads plugin has zero references to bw/bv; agents use `bd` CLI directly |
| **Git correlation engine** (commit-to-issue linking, co-commit analysis, temporal patterns, causality) | ~8,000 | No workflow references anywhere; `bd` tracks issue lifecycle natively |
| **Export/deployment** (static sites, GitHub Pages, Cloudflare Pages, preview server, SQLite export) | ~9,000 | Never referenced in any workflow config or session |
| **Drift detection + baselines** | ~1,200 | No CI integration configured; never invoked |
| **Semantic search** (vector embeddings, hybrid scoring) | ~1,600 | `bd search` handles text search; no vector search usage |
| **Cass integration** | ~1,400 | External tool not installed; feature is a dead path |
| **Recipes** | ~350 | Never configured or invoked |
| **Hooks** (pre/post-export) | ~500 | Never configured |
| **Multi-repo workspaces** | ~650 | Single-repo usage only |
| **Agents file management** (`pkg/agents/`) | ~800 | `bd onboard` and `bd setup` handle this |
| **Instance locking** (`pkg/instance/`) | ~300 | `bd` handles its own locking |
| **Performance metrics** (`pkg/metrics/`) | ~420 | Internal instrumentation for the analysis engine (which is itself unused) |
| **Insights dashboard** (TUI view) | Part of `pkg/ui/` | Displays analysis engine output; without the engine, it has no data |
| **Graph view** (TUI view) | Part of `pkg/ui/` | Interactive DAG navigation; depends on the analysis engine |

### Usage Summary

```
Feature Area                    Lines    Used?
----------------------------------------------
TUI core (list, tree, board)   ~10,000   YES
Edit modal (huh forms)           ~500    YES
Loader + model                 ~1,600    YES
Watcher                          ~730    YES
Updater                          ~700    YES
                               ------
Subtotal (used)               ~13,500

Analysis engine                ~15,000   NO
Correlation engine              ~8,000   NO
Export/deploy                   ~9,000   NO
Robot protocol (cmd layer)      ~3,000   NO
Search                          ~1,600   NO
Cass                            ~1,400   NO
Baseline + drift                ~1,200   NO
Agents, instance, metrics       ~1,500   NO
Recipes, hooks, workspace       ~1,500   NO
                               ------
Subtotal (unused)             ~42,200
```

**~76% of the codebase is unused.** The used portion (~13,500 lines) is the TUI, loader, watcher, and updater. Everything else is inherited from the upstream beads_viewer project and serves use cases (AI agent protocol, project intelligence, deployment) that this fork does not exercise.

---

## The Case for Stripping

### Why strip?

1. **Maintenance burden**: 42k lines of code that must compile, pass tests, and stay compatible with upstream changes, but provides zero value to this fork's workflow.
2. **Test suite noise**: Export tests alone take ~20s and test deployment wizards for GitHub/Cloudflare Pages that will never be used. The analysis engine has extensive benchmarks for graph algorithms that are irrelevant.
3. **Dependency bloat**: The analysis engine pulls in `gonum.org/v1/gonum` (large numerical computing library). Semantic search adds embedding infrastructure. Correlation adds git log parsing. Removing these shrinks the binary and dependency tree.
4. **Cognitive load**: Contributors (human or AI) must understand which of the 22 packages matter and which are dead weight. A leaner codebase is faster to navigate and reason about.
5. **Fork identity**: This fork already differentiates on UX (tree view, split pane, huh forms, navigation). Stripping the intelligence layer makes the positioning clearer: "beadwork is an opinionated TUI for beads, not a competing analysis platform."

### What to keep

The core TUI viewer and editor:
- `pkg/ui/` (list view, tree view, board view, edit modal)
- `pkg/loader/` (JSONL loading, worktree resolution)
- `pkg/model/` (issue types)
- `pkg/watcher/` (live-reload)
- `pkg/updater/` (self-update)
- `pkg/debug/` (debug utilities)
- `pkg/version/` (version info)
- `cmd/bw/` (CLI entry point, stripped of robot flags and export commands)
- `tests/e2e/` (TUI end-to-end tests)

### What to remove

Everything else:
- `pkg/analysis/` (graph engine, triage, suggestions, insights)
- `pkg/correlation/` (git-to-issue linking)
- `pkg/export/` (static sites, deployment, markdown, SQLite, graphs, hooks)
- `pkg/hooks/` (export automation)
- `pkg/search/` (semantic/vector search)
- `pkg/cass/` (external search tool integration)
- `pkg/baseline/` (metrics snapshots)
- `pkg/drift/` (drift detection)
- `pkg/metrics/` (performance instrumentation)
- `pkg/recipe/` (saved views)
- `pkg/workspace/` (multi-repo)
- `pkg/agents/` (AGENTS.md management)
- `pkg/instance/` (PID locking)
- All `--robot-*` flags from `cmd/bw/main.go`
- All `--export-*`, `--pages`, `--check-drift`, `--search`, `--recipe`, `--workspace` flags

### What needs careful consideration

- **Graph view** (`pkg/ui/graph.go`): Renders the dependency DAG in the TUI. Currently depends on the analysis engine for graph construction. Could be kept if refactored to build a simple adjacency list from `model.Issue` dependencies directly, without PageRank/betweenness/HITS. The question: is the graph view useful enough to justify keeping? If so, it needs a lightweight graph builder that replaces the dependency on `pkg/analysis/`.
- **Insights dashboard** (`pkg/ui/insights.go`): Displays analysis engine metrics. Without the engine, this view has no data. Remove unless the graph view is kept and a minimal insights display is desired.
- **Cycle detection**: Even without the full analysis engine, detecting circular dependencies is valuable for the TUI (warning users). This is a small, self-contained algorithm that could live in `pkg/model/` or `pkg/ui/` directly.

---

## Conclusion

Beadwork has evolved from a "viewer" into a project intelligence platform. The name "beads_viewer" (the upstream project) is a historical artifact. In practice, bw provides:

1. **Visualization** (TUI with 5 views)
2. **Analysis** (graph algorithms, priority scoring, risk, ETA)
3. **AI agent interface** (robot protocol with 30+ commands)
4. **Deployment** (static sites to GitHub/Cloudflare Pages)
5. **Correlation** (git history to issue lifecycle mapping)
6. **Monitoring** (drift detection, baselines, CI integration)
7. **Search** (hybrid lexical + semantic vector search)

Only #1 is "viewing." The rest is independent intelligence that could, in theory, exist as separate tools.

For this fork's purposes, only #1 (plus editing, via huh forms) is actively used. The other 6 capability areas (~42k lines, ~76% of the codebase) are inherited upstream features that serve a different vision for the tool.
