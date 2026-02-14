> **PREREQUISITE**: Read and follow `~/.cursor/AGENTS.md` first. It contains baseline instructions (beads workflow, commit strategy, progress reporting) that apply to ALL projects. The instructions below are project-specific and supplement those global rules.

# AGENTS.md — beadwork

## RULE 0 - THE FUNDAMENTAL OVERRIDE PEROGATIVE

If I tell you to do something, even if it goes against what follows below, YOU MUST LISTEN TO ME. I AM IN CHARGE, NOT YOU.

---

## RULE 1 – ABSOLUTE (DO NOT EVER VIOLATE THIS)

You may NOT delete any file or directory unless I explicitly give the exact command **in this session**.

- This includes files you just created (tests, tmp files, scripts, etc.).
- You do not get to decide that something is "safe" to remove.
- If you think something should be removed, stop and ask. You must receive clear written approval **before** any deletion command is even proposed.

Treat "never delete files without permission" as a hard invariant.

---

### IRREVERSIBLE GIT & FILESYSTEM ACTIONS

Absolutely forbidden unless I give the **exact command and explicit approval** in the same message:

- `git reset --hard`
- `git clean -fd`
- `rm -rf`
- Any command that can delete or overwrite code/data

Rules:

1. If you are not 100% sure what a command will delete, do not propose or run it. Ask first.
2. Prefer safe tools: `git status`, `git diff`, `git stash`, copying to backups, etc.
3. After approval, restate the command verbatim, list what it will affect, and wait for confirmation.
4. When a destructive command is run, record in your response:
   - The exact user text authorizing it
   - The command run
   - When you ran it

If that audit trail is missing, then you must act as if the operation never happened.

---

## Go Toolchain

- Use **Go 1.22+** (check `go.mod` for exact version).
- Build: `go build ./...`
- Test: `go test ./...` (add `-v` for verbose, `-race` for race detection)
- Vet: `go vet ./...` (run before commits)
- Format: `gofmt -w .` or `goimports -w .`

### Key Commands

```bash
go build ./...                    # Build all packages
go test ./...                     # Run all tests
go test ./... -race               # Run with race detector
go test ./pkg/analysis/... -v     # Verbose tests for specific package
go vet ./...                      # Static analysis
gofmt -w .                        # Format all Go files
```

### Module Management

- Lockfile: `go.sum` (auto-managed by `go mod`)
- Dependencies: `go mod tidy` to clean up unused deps
- Never manually edit `go.sum`

---

### Code Editing Discipline

- Do **not** run scripts that bulk-modify code (codemods, invented one-off scripts, giant `sed`/regex refactors).
- Large mechanical changes: break into smaller, explicit edits and review diffs.
- Subtle/complex changes: edit by hand, file-by-file, with careful reasoning.

---

### Backwards Compatibility & File Sprawl

We optimize for a clean architecture now, not backwards compatibility.

- No "compat shims" or "v2" file clones.
- When changing behavior, migrate callers and remove old code **inside the same file**.
- New files are only for genuinely new domains that don't fit existing modules.
- The bar for adding files is very high.

---

### Logging & Console Output

- Use structured logging patterns; avoid raw `fmt.Println` for production logs.
- TUI output goes through lipgloss styling; don't mix raw prints with styled output.
- Robot mode (`--robot-*`) outputs JSON to stdout; human mode uses styled TUI.
- Errors should be wrapped with `fmt.Errorf("context: %w", err)` for traceability.

---

### Third-Party Libraries

When unsure of an API, look up current docs rather than guessing. Key dependencies:

- **bubbletea**: TUI framework (Elm architecture)
- **lipgloss**: Terminal styling
- **bubbles**: Reusable TUI components
- **cobra**: CLI framework
- **viper**: Configuration management

---

## MCP Agent Mail — Multi-Agent Coordination (Optional)

> **Note:** This section is optional. If you're operating as a single agent or using alternative coordination methods, skip to "Issue Tracking with br" below.

Agent Mail is available as an MCP server for agent-to-agent coordination. If it's not available and you need multi-agent coordination, flag to the user—they may need to start it with `am` alias or manually.

**Troubleshooting:** If Agent Mail fails with "Too many open files" (common on macOS), restart with higher limit: `ulimit -n 4096; python -m mcp_agent_mail.cli serve-http`

What Agent Mail gives:

- Identities, inbox/outbox, searchable threads.
- Advisory file reservations (leases) to avoid agents clobbering each other.
- Persistent artifacts in git (human-auditable).

Core patterns:

1. **Same repo**
   - Register identity: `ensure_project` then `register_agent` with the repo's absolute path as `project_key`.
   - Reserve files before editing: `file_reservation_paths(project_key, agent_name, ["pkg/**"], ttl_seconds=3600, exclusive=true)`.
   - Communicate: `send_message(..., thread_id="bv-123")`.
   - Fast reads: `resource://inbox/{Agent}?project=<abs-path>&limit=20`.

2. **Multiple repos**
   - Same `project_key` for all; use specific reservations (`pkg/**`, `cmd/**`).
   - Or different projects linked via `macro_contact_handshake`.

Macros vs granular:

- Prefer macros when speed matters: `macro_start_session`, `macro_prepare_thread`, `macro_file_reservation_cycle`.
- Use granular tools when you need explicit behavior.

Common pitfalls:

- "from_agent not registered" → call `register_agent` with correct `project_key`.
- `FILE_RESERVATION_CONFLICT` → adjust patterns, wait for expiry, or use non-exclusive reservation.

---

## Issue Tracking with br (beads_rust)

All issue tracking goes through **br** (beads_rust). No other TODO systems.

Key invariants:

- `.beads/` is authoritative state and **must always be committed** with code changes.
- Do not edit `.beads/*.jsonl` directly; only via `br`.
- **br is non-invasive**: it NEVER executes git commands. You must manually commit `.beads/` changes.

### Basics

Check ready work:

```bash
br ready --json
```

Create issues:

```bash
br create "Issue title" -t bug|feature|task -p 0-4 --json
br create "Issue title" -p 1 --deps discovered-from:bv-123 --json
```

Update:

```bash
br update bv-42 --status in_progress --json
br update bv-42 --priority 1 --json
```

Complete:

```bash
br close bv-42 --reason "Completed" --json
```

Types:

- `bug`, `feature`, `task`, `epic`, `chore`

Priorities:

- `0` critical (security, data loss, broken builds)
- `1` high
- `2` medium (default)
- `3` low
- `4` backlog

Agent workflow:

1. `br ready` to find unblocked work.
2. Claim: `br update <id> --status in_progress`.
3. Implement + test.
4. If you discover new work, create a new bead with `discovered-from:<parent-id>`.
5. Close when done.
6. Sync and commit:
   ```bash
   br sync --flush-only    # Export to JSONL (no git ops)
   git add .beads/         # Stage beads changes
   git commit -m "..."     # Commit with code changes
   ```

Never:

- Use markdown TODO lists.
- Use other trackers.
- Duplicate tracking.

---

## Using bv as an AI Sidecar

bv is a graph-aware triage engine for Beads projects (.beads/beads.jsonl). Instead of parsing JSONL or hallucinating graph traversal, use robot flags for deterministic, dependency-aware outputs with precomputed metrics (PageRank, betweenness, critical path, cycles, HITS, eigenvector, k-core).

**Scope boundary:** bv handles *what to work on* (triage, priority, planning). For multi-agent coordination (messaging, work claiming, file reservations), see the optional MCP Agent Mail section above.

**⚠️ CRITICAL: Use ONLY `--robot-*` flags. Bare `bv` launches an interactive TUI that blocks your session.**

### The Workflow: Start With Triage

**`bv --robot-triage` is your single entry point.** It returns everything you need in one call:
- `quick_ref`: at-a-glance counts + top 3 picks
- `recommendations`: ranked actionable items with scores, reasons, unblock info
- `quick_wins`: low-effort high-impact items
- `blockers_to_clear`: items that unblock the most downstream work
- `project_health`: status/type/priority distributions, graph metrics
- `commands`: copy-paste shell commands for next steps

```bash
bv --robot-triage        # THE MEGA-COMMAND: start here
bv --robot-next          # Minimal: just the single top pick + claim command
```

### Other Commands

**Planning:**
| Command | Returns |
|---------|---------|
| `--robot-plan` | Parallel execution tracks with `unblocks` lists |
| `--robot-priority` | Priority misalignment detection with confidence |

**Graph Analysis:**
| Command | Returns |
|---------|---------|
| `--robot-insights` | Full metrics: PageRank, betweenness, HITS, eigenvector, critical path, cycles, k-core, articulation points, slack |
| `--robot-label-health` | Per-label health: `health_level` (healthy\|warning\|critical), `velocity_score`, `staleness`, `blocked_count` |
| `--robot-label-flow` | Cross-label dependency: `flow_matrix`, `dependencies`, `bottleneck_labels` |
| `--robot-label-attention [--attention-limit=N]` | Attention-ranked labels by: (pagerank × staleness × block_impact) / velocity |

**History & Change Tracking:**
| Command | Returns |
|---------|---------|
| `--robot-history` | Bead-to-commit correlations: `stats`, `histories` (per-bead events/commits/milestones), `commit_index` |
| `--robot-diff --diff-since <ref>` | Changes since ref: new/closed/modified issues, cycles introduced/resolved |

**Other Commands:**
| Command | Returns |
|---------|---------|
| `--robot-burndown <sprint>` | Sprint burndown, scope changes, at-risk items |
| `--robot-forecast <id\|all>` | ETA predictions with dependency-aware scheduling |
| `--robot-alerts` | Stale issues, blocking cascades, priority mismatches |
| `--robot-suggest` | Hygiene: duplicates, missing deps, label suggestions, cycle breaks |
| `--robot-graph [--graph-format=json\|dot\|mermaid]` | Dependency graph export |
| `--export-graph <file.html>` | Self-contained interactive HTML visualization |

### Scoping & Filtering

```bash
bv --robot-plan --label backend              # Scope to label's subgraph
bv --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work (no blockers)
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank scores
bv --robot-triage --robot-triage-by-track    # Group by parallel work streams
bv --robot-triage --robot-triage-by-label    # Group by domain
```

### Understanding Robot Output

**All robot JSON includes:**
- `data_hash` — Fingerprint of source beads.jsonl (verify consistency across calls)
- `status` — Per-metric state: `computed|approx|timeout|skipped` + elapsed ms
- `as_of` / `as_of_commit` — Present when using `--as-of`; contains ref and resolved SHA

**Two-phase analysis:**
- **Phase 1 (instant):** degree, topo sort, density — always available immediately
- **Phase 2 (async, 500ms timeout):** PageRank, betweenness, HITS, eigenvector, cycles — check `status` flags

**For large graphs (>500 nodes):** Some metrics may be approximated or skipped. Always check `status`.

### jq Quick Reference

```bash
bv --robot-triage | jq '.quick_ref'                        # At-a-glance summary
bv --robot-triage | jq '.recommendations[0]'               # Top recommendation
bv --robot-plan | jq '.plan.summary.highest_impact'        # Best unblock target
bv --robot-insights | jq '.status'                         # Check metric readiness
bv --robot-insights | jq '.Cycles'                         # Circular deps (must fix!)
bv --robot-label-health | jq '.results.labels[] | select(.health_level == "critical")'
```

**Performance:** Phase 1 instant, Phase 2 async (500ms timeout). Prefer `--robot-plan` over `--robot-insights` when speed matters. Results cached by data hash. Use `bv --profile-startup` for diagnostics.

Use bv instead of parsing beads.jsonl—it computes PageRank, critical paths, cycles, and parallel tracks deterministically.

---

### Hybrid Semantic Search (CLI)

`bv --search` supports hybrid ranking (text + graph metrics).

```bash
# Default (text-only)
bv --search "login oauth"

# Hybrid mode with preset
bv --search "login oauth" --search-mode hybrid --search-preset impact-first

# Hybrid with custom weights
bv --search "login oauth" --search-mode hybrid \
  --search-weights '{"text":0.4,"pagerank":0.2,"status":0.15,"impact":0.1,"priority":0.1,"recency":0.05}'

# Robot JSON output (adds mode/preset/weights + component_scores for hybrid)
bv --search "login oauth" --search-mode hybrid --robot-search
```

Env defaults:
- `BW_SEARCH_MODE` (text|hybrid)
- `BW_SEARCH_PRESET` (default|bug-hunting|sprint-planning|impact-first|text-only)
- `BW_SEARCH_WEIGHTS` (JSON string, overrides preset)

---

### Static Site Export for Stakeholder Reporting

Generate a static dashboard for non-technical stakeholders:

```bash
# Interactive wizard (recommended)
bv --pages

# Or export locally
bv --export-pages ./dashboard --pages-title "Sprint 42 Status"
```

The output is a self-contained HTML/JS bundle that:
- Shows triage recommendations (from --robot-triage)
- Visualizes dependencies
- Supports full-text search (FTS5)
- Works offline after initial load
- Requires no installation to view

**Deployment options:**
- `bv --pages` → Interactive wizard for GitHub Pages deployment
- `bv --export-pages ./dir` → Local export for custom hosting
- `bv --preview-pages ./dir` → Preview bundle locally

**For CI/CD integration:**
```bash
bv --export-pages ./bv-pages --pages-title "Nightly Build"
# Then deploy ./bv-pages to your hosting of choice
```

---

## Testing Guidelines

### Never Open Browsers

**Tests must NEVER automatically open a browser.** All browser-opening functions check `BW_NO_BROWSER` and `BW_TEST_MODE` environment variables. These are set globally via `TestMain` in:
- `tests/e2e/common_test.go`
- `pkg/export/main_test.go`
- `pkg/ui/main_test.go`

When adding new browser-opening code, always check these env vars first:
```go
if os.Getenv("BW_NO_BROWSER") != "" || os.Getenv("BW_TEST_MODE") != "" {
    return nil
}
```

### Test Commands

```bash
go test ./...                           # All tests
go test ./pkg/analysis/... -v           # Verbose for specific package
go test ./tests/e2e/... -v              # E2E tests
go test ./... -race                     # With race detector
go test ./... -cover                    # With coverage
go test -run TestSpecificName ./pkg/... # Run specific test
```

### Test-Driven Development (TDD) — Mandatory for Bug Fixes & Features

**All bug fixes and new features MUST follow test-driven development.** No exceptions.

#### The Cycle: RED → GREEN → REFACTOR

1. **RED — Write a failing test first**
   - Write the smallest test that demonstrates the desired behavior or reproduces the bug.
   - Run it. It MUST fail. If it passes, you're testing existing behavior — fix the test.
   - Verify the failure is for the expected reason (missing feature, not a typo).

2. **GREEN — Write minimal code to pass**
   - Write the simplest implementation that makes the test pass.
   - Don't add features, refactor, or "improve" beyond what the test requires.
   - Run all tests — the new test passes AND no regressions.

3. **REFACTOR — Clean up (optional)**
   - Remove duplication, improve names, extract helpers.
   - Keep all tests green throughout.

#### Rules

- **No production code without a failing test first.** If you wrote code before the test, delete it and start over.
- **One behavior per test.** If the test name contains "and", split it.
- **Verify both RED and GREEN.** You must run the test suite at each phase — never skip.
- **Bug fixes require a regression test.** The test must fail before the fix and pass after.

#### Example: Bug Fix Workflow

```bash
# 1. RED: Write failing test
go test ./pkg/ui/ -run "TestTreeViewPageIndicator" -v
# FAIL: position indicator not found

# 2. GREEN: Implement minimal fix
# (edit tree.go to add page indicator)
go test ./pkg/ui/ -run "TestTreeViewPageIndicator" -v
# PASS

# 3. Verify no regressions
go test ./pkg/ui/ -timeout 120s
# ok
```

#### When NOT Using TDD (requires explicit permission)

- Throwaway prototypes
- Generated code
- Configuration-only changes

### Test Patterns

- Use table-driven tests for multiple cases
- Use `t.TempDir()` for temporary files
- Use `t.Helper()` in test helpers
- Check `testing.Short()` for long-running tests

---

## Go Best Practices

Follow all practices in `GOLANG_BEST_PRACTICES.md`. Key points:

### Error Handling

```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("loading config: %w", err)
}

// Check errors immediately after the call
result, err := doSomething()
if err != nil {
    return err
}
```

### Division Safety

```go
// Always guard against division by zero
if len(items) > 0 {
    avg := total / float64(len(items))
}
```

### Nil Checks

```go
// Check for nil before dereferencing
if dep != nil && dep.Type.IsBlocking() {
    // safe to use dep
}
```

### Concurrency

```go
// Use sync.RWMutex for shared state
mu.RLock()
value := sharedMap[key]
mu.RUnlock()

// Capture channels before unlock to avoid races
mu.RLock()
ch := someChannel
mu.RUnlock()
for item := range ch {
    // process
}
```

---

## ast-grep vs ripgrep

**Use `ast-grep` when structure matters.** It parses code and matches AST nodes, so results ignore comments/strings, understand syntax, and can safely rewrite code.

- Refactors/codemods: rename APIs, change patterns
- Policy checks: enforce patterns across a repo

**Use `ripgrep` when text is enough.** Fastest way to grep literals/regex.

- Recon: find strings, TODOs, config values
- Pre-filter: narrow candidates before precise pass

**Go-specific examples:**

```bash
# Find all error returns without wrapping
ast-grep run -l Go -p 'return err'

# Find all fmt.Println (should use structured logging)
ast-grep run -l Go -p 'fmt.Println($$$)'

# Quick grep for a function name
rg -n 'func.*LoadConfig' -t go

# Combine: find files then match precisely
rg -l -t go 'sync.Mutex' | xargs ast-grep run -l Go -p 'mu.Lock()'
```

---

## Morph Warp Grep — AI-Powered Code Search

**Use `mcp__morph-mcp__warp_grep` for exploratory "how does X work?" questions.** An AI search agent automatically expands your query into multiple search patterns, greps the codebase, reads relevant files, and returns precise line ranges.

**Use `ripgrep` for targeted searches.** When you know exactly what you're looking for.

| Scenario | Tool |
|----------|------|
| "How is graph analysis implemented?" | `warp_grep` |
| "Where is PageRank computed?" | `warp_grep` |
| "Find all uses of `NewAnalyzer`" | `ripgrep` |
| "Rename function across codebase" | `ast-grep` |

**warp_grep usage:**
```
mcp__morph-mcp__warp_grep(
  repoPath: "/path/to/beadwork",
  query: "How does the correlation package detect orphan commits?"
)
```

**Anti-patterns:**
- ❌ Using `warp_grep` to find a known function name → use `ripgrep`
- ❌ Using `ripgrep` to understand architecture → use `warp_grep`

---

## UBS Quick Reference

UBS = "Ultimate Bug Scanner" — static analysis for catching bugs early.

**Golden Rule:** `ubs <changed-files>` before every commit. Exit 0 = safe. Exit >0 = fix & re-run.

```bash
ubs file.go file2.go                    # Specific files (< 1s)
ubs $(git diff --name-only --cached)    # Staged files
ubs --only=go pkg/                      # Go files only
ubs .                                   # Whole project
```

**Output Format:**
```
⚠️  Category (N errors)
    file.go:42:5 – Issue description
    💡 Suggested fix
Exit code: 1
```

**Fix Workflow:**
1. Read finding → understand the issue
2. Navigate `file:line:col` → view context
3. Verify real issue (not false positive)
4. Fix root cause
5. Re-run `ubs <file>` → exit 0
6. Commit

**Bug Severity (Go-specific):**
- **Critical**: nil dereference, division by zero, race conditions, resource leaks
- **Important**: error handling, type assertions without check
- **Contextual**: TODO/FIXME, unused variables

---

## cass — Cross-Agent Session Search

`cass` indexes prior agent conversations (Claude Code, Codex, Cursor, Gemini, ChatGPT, Aider, etc.) into a unified, searchable index so you can reuse solved problems.

**NEVER run bare `cass`** — it launches an interactive TUI. Always use `--robot` or `--json`.

### Quick Start

```bash
# Check if index is healthy (exit 0=ok, 1=run index first)
cass health

# Search across all agent histories
cass search "authentication error" --robot --limit 5

# View a specific result (from search output)
cass view /path/to/session.jsonl -n 42 --json

# Expand context around a line
cass expand /path/to/session.jsonl -n 42 -C 3 --json

# Learn the full API
cass capabilities --json      # Feature discovery
cass robot-docs guide         # LLM-optimized docs
```

### Key Flags

| Flag | Purpose |
|------|---------|
| `--robot` / `--json` | Machine-readable JSON output (required!) |
| `--fields minimal` | Reduce payload: `source_path`, `line_number`, `agent` only |
| `--limit N` | Cap result count |
| `--agent NAME` | Filter to specific agent (claude, codex, cursor, etc.) |
| `--days N` | Limit to recent N days |

**stdout = data only, stderr = diagnostics. Exit 0 = success.**

### Robot Mode Etiquette

- Prefer `cass --robot-help` and `cass robot-docs <topic>` for machine-first docs
- The CLI is forgiving: globals placed before/after subcommand are auto-normalized
- If parsing fails, follow the actionable errors with examples
- Use `--color=never` in non-TTY automation for ANSI-free output

### Pre-Flight Health Check

```bash
cass health --json
```

Returns in <50ms:
- **Exit 0:** Healthy—proceed with queries
- **Exit 1:** Unhealthy—run `cass index --full` first

### Exit Codes

| Code | Meaning | Retryable |
|------|---------|-----------|
| 0 | Success | N/A |
| 1 | Health check failed | Yes—run `cass index --full` |
| 2 | Usage/parsing error | No—fix syntax |
| 3 | Index/DB missing | Yes—run `cass index --full` |

Treat cass as a way to avoid re-solving problems other agents already handled.

---

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   br sync --flush-only    # Export beads to JSONL (no git ops)
   git add .beads/         # Stage beads changes
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Verify** - All changes committed AND pushed
6. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

---

Note for Codex/GPT-5.2:

You constantly bother me and stop working with concerned questions that look similar to this:

```
Unexpected changes (need guidance)

- Working tree still shows edits I did not make in Cargo.toml, Cargo.lock, src/cli/commands/upgrade.rs, src/storage/sqlite.rs, tests/conformance.rs, tests/storage_deps.rs. Please advise whether to keep/commit/revert these before any further work. I did not touch them.

Next steps (pick one)

1. Decide how to handle the unrelated modified files above so we can resume cleanly.
2. Triage beads_rust-orko (clippy/cargo warnings) and beads_rust-ydqr (rustfmt failures).
3. If you want a full suite run later, fix conformance/clippy blockers and re-run cargo test --all.
```

NEVER EVER DO THAT AGAIN. The answer is literally ALWAYS the same: those are changes created by the potentially dozen of other agents working on the project at the same time. This is not only a common occurence, it happens multiple times PER MINUTE. The way to deal with it is simple: you NEVER, under ANY CIRCUMSTANCE, stash, revert, overwrite, or otherwise disturb in ANY way the work of other agents. Just treat those changes identically to changes that you yourself made. Just fool yourself into think YOU made the changes and simply don't recall it for some reason.

---

## Note on Built-in TODO Functionality

Also, if I ask you to explicitly use your built-in TODO functionality, don't complain about this and say you need to use beads. You can use built-in TODOs if I tell you specifically to do so. Always comply with such orders.
