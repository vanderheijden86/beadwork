# Beads Viewer (bv) vs Beads (bd): Separation of Concerns

This document maps out what the original Beads Viewer (`steveyegge/beads_viewer`) handles versus what Beads (`steveyegge/beads`) handles, and where they overlap. Beadwork (`bw`) is a stripped-down fork of Beads Viewer that keeps only the TUI viewer/editor and removes the analysis, export, and agent protocol layers.

## Summary

**Beads (bd)** is the issue tracker: it owns the data. Create, update, close, sync, dependencies, labels, comments, and the JSONL/SQLite storage layer.

**Beads Viewer (bv)** started as a TUI viewer for that data, but grew into a full analysis and intelligence platform. It reads the same `.beads/issues.jsonl` file and adds graph analysis, AI agent protocols, deployment, search, and project health monitoring on top.

**Beadwork (bw)** is a fork of Beads Viewer that strips it back to a pure TUI viewer/editor. It keeps the interactive views (list, tree, board) and the edit modal, and removes the ~42k lines of analysis, export, correlation, and robot protocol code.

---

## Feature Matrix

### 1. Core Issue CRUD

| Capability | bd | bv (Beads Viewer) |
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

**Verdict**: bd owns CRUD. bv delegates mutations back to `bd` CLI. No overlap in write path.

---

### 2. Views and Visualization (bv only)

These are purely bv concerns with no equivalent in bd:

| Feature | Package | Description |
|---|---|---|
| List view | `pkg/ui/` | Paginated issue list with sort, filter |
| Tree view | `pkg/ui/` | Hierarchical epic/child view with split pane |
| Board view | `pkg/ui/` | Kanban-style columns by status |
| Graph view | `pkg/ui/` | Interactive dependency DAG navigation |
| Insights dashboard | `pkg/ui/` | PageRank, bottlenecks, critical path display |

---

### 3. Graph Analysis Engine (bv only)

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

**bd has none of this.** bd stores dependencies but does not analyze them. The graph intelligence is entirely bv's domain.

---

### 4. AI Agent Protocol (bv only)

The `--robot-*` flags expose bv's analysis as structured JSON for AI agents. This is a major feature area (~30 flags):

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

### 5. Git-to-Issue Correlation (bv only)

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

### 6. Drift Detection and Baselines (bv only)

| Feature | Package | Description |
|---|---|---|
| Save baseline | `pkg/baseline/` | Snapshot current metrics as reference point |
| Drift detection | `pkg/drift/` | Compare current state to baseline, flag regressions |
| CI integration | `--check-drift` | Exit codes for CI pipelines (0=OK, 1=critical, 2=warning) |

**bd has none of this.**

---

### 7. Export, Deployment, and Static Sites (bv only)

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

### 8. Semantic Search (bv only)

| Feature | Package | Description |
|---|---|---|
| Vector embeddings | `pkg/search/embedder.go` | Hash-based or external embeddings |
| Hybrid scoring | `pkg/search/hybrid_scorer.go` | Combined lexical + semantic ranking |
| Search presets | `pkg/search/presets.go` | Pre-configured search profiles |
| Index sync | `pkg/search/index_sync.go` | Keep search index in sync with issues |
| Cass integration | `pkg/cass/` | Optional external semantic code search |

**bd** has `bd search` (text substring matching) but no vector/semantic search.

---

### 9. Project Health and Monitoring (bv only)

| Feature | Package | Description |
|---|---|---|
| Performance metrics | `pkg/metrics/` | Internal timing and cache stats |
| Recipes | `pkg/recipe/` | Saved view/filter configurations |
| File watching | `pkg/watcher/` | Live-reload on `.beads/` changes |
| Multi-repo workspaces | `pkg/workspace/` | Aggregate issues across repositories |
| Self-update | `pkg/updater/` | Check/download new versions from GitHub |

---

### 10. Shared Concerns (overlap)

| Concern | bd | bv |
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
         Beads (bd)            Beads Viewer (bv)
              |                         |
     Issue tracker CLI       Analysis + TUI platform
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

## What Beadwork Keeps vs Strips

Beadwork (`bw`) is a fork of Beads Viewer that strips the intelligence/export layers and keeps the TUI core. The reasoning: the analysis engine, robot protocol, export pipeline, and correlation engine serve a "project intelligence platform" vision that is not needed for a focused TUI viewer/editor.

### Kept in Beadwork

| Feature | Package | Lines | Why |
|---|---|---|---|
| List view, tree view, board view | `pkg/ui/` | ~10,000 | Core TUI interaction |
| Edit modal (huh forms) | `pkg/ui/` | ~500 | Issue editing from inside the TUI |
| JSONL loader | `pkg/loader/` | ~800 | Required for data loading |
| Issue model | `pkg/model/` | ~800 | Required for type definitions |
| File watcher | `pkg/watcher/` | ~730 | Live-reload on changes |
| Self-updater | `pkg/updater/` | ~700 | Version management |
| Debug utilities | `pkg/debug/` | ~200 | Development support |
| Version info | `pkg/version/` | ~100 | Build metadata |
| CLI entry point | `cmd/bw/` | Stripped | Robot flags and export commands removed |
| E2E tests | `tests/e2e/` | Kept | TUI tests only |
| **Total** | | **~13,500** | |

### Stripped from Beadwork

| Feature | Package | Lines | Why removed |
|---|---|---|---|
| Graph analysis engine | `pkg/analysis/` | ~15,000 | `bd` handles dependency ordering; agents use `bd ready` |
| Robot protocol (30+ flags) | `cmd/bw/` | ~3,000 | Agents use `bd` CLI directly, not `bv --robot-*` |
| Git correlation engine | `pkg/correlation/` | ~8,000 | `bd` tracks issue lifecycle natively |
| Export/deployment | `pkg/export/` | ~9,000 | Static sites, Pages, Cloudflare never used |
| Drift detection + baselines | `pkg/baseline/`, `pkg/drift/` | ~1,200 | No CI integration configured |
| Semantic search | `pkg/search/` | ~1,600 | `bd search` handles text search |
| Cass integration | `pkg/cass/` | ~1,400 | External tool not installed |
| Recipes | `pkg/recipe/` | ~350 | Never configured |
| Hooks (export) | `pkg/hooks/` | ~500 | Never configured |
| Multi-repo workspaces | `pkg/workspace/` | ~650 | Single-repo usage |
| Agent file management | `pkg/agents/` | ~800 | `bd onboard`/`bd setup` handle this |
| Instance locking | `pkg/instance/` | ~300 | `bd` handles its own locking |
| Performance metrics | `pkg/metrics/` | ~420 | Instrumentation for the analysis engine |
| Insights dashboard (TUI) | `pkg/ui/insights.go` | Part of ui | No data without analysis engine |
| Graph view (TUI) | `pkg/ui/graph.go` | Part of ui | Depends on analysis engine |
| **Total** | | **~42,200** | |

**~76% of Beads Viewer's codebase is removed in Beadwork.** The kept portion (~13,500 lines) is the TUI, loader, watcher, and updater.

---

## Conclusion

Beads Viewer evolved from a "viewer" into a project intelligence platform. The name "beads_viewer" is a historical artifact. In practice, bv provides:

1. **Visualization** (TUI with 5 views)
2. **Analysis** (graph algorithms, priority scoring, risk, ETA)
3. **AI agent interface** (robot protocol with 30+ commands)
4. **Deployment** (static sites to GitHub/Cloudflare Pages)
5. **Correlation** (git history to issue lifecycle mapping)
6. **Monitoring** (drift detection, baselines, CI integration)
7. **Search** (hybrid lexical + semantic vector search)

Only #1 is "viewing." The rest is independent intelligence that could exist as separate tools.

Beadwork keeps #1 (plus editing via huh forms) and removes the other six capability areas, focusing on being a clean, fast TUI for interacting with beads issue data.
