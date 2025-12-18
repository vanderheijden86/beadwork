### Using bv as an AI sidecar

bv is a graph-aware triage engine for Beads projects (.beads/beads.jsonl). Instead of parsing JSONL or hallucinating graph traversal, use robot flags for deterministic, dependency-aware outputs with precomputed metrics (PageRank, betweenness, critical path, cycles, HITS, eigenvector, k-core).

**Scope boundary:** bv handles *what to work on* (triage, priority, planning). For agent-to-agent coordination (messaging, work claiming, file reservations), use [MCP Agent Mail](https://github.com/Dicklesworthstone/mcp_agent_mail).

**⚠️ CRITICAL: Use ONLY `--robot-*` flags. Bare `bv` launches an interactive TUI that blocks your session.**

**Troubleshooting Agent Mail:** If Agent Mail fails with "Too many open files" (common on macOS), restart with higher limit: `ulimit -n 4096; python -m mcp_agent_mail.cli serve-http`

#### The Workflow: Start With Triage

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

#### Other Commands

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

#### Scoping & Filtering

```bash
bv --robot-plan --label backend              # Scope to label's subgraph
bv --robot-insights --as-of HEAD~30          # Historical point-in-time
bv --recipe actionable --robot-plan          # Pre-filter: ready to work (no blockers)
bv --recipe high-impact --robot-triage       # Pre-filter: top PageRank scores
bv --robot-triage --robot-triage-by-track    # Group by parallel work streams
bv --robot-triage --robot-triage-by-label    # Group by domain
```

#### Understanding Robot Output

**All robot JSON includes:**
- `data_hash` — Fingerprint of source beads.jsonl (verify consistency across calls)
- `status` — Per-metric state: `computed|approx|timeout|skipped` + elapsed ms
- `as_of` / `as_of_commit` — Present when using `--as-of`; contains ref and resolved SHA

**Two-phase analysis:**
- **Phase 1 (instant):** degree, topo sort, density — always available immediately
- **Phase 2 (async, 500ms timeout):** PageRank, betweenness, HITS, eigenvector, cycles — check `status` flags

**For large graphs (>500 nodes):** Some metrics may be approximated or skipped. Always check `status`.

#### jq Quick Reference

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

### ast-grep vs ripgrep (quick guidance)

**Use `ast-grep` when structure matters.** It parses code and matches AST nodes, so results ignore comments/strings, understand syntax, and can **safely rewrite** code.

* Refactors/codemods: rename APIs, change import forms, rewrite call sites or variable kinds.
* Policy checks: enforce patterns across a repo (`scan` with rules + `test`).
* Editor/automation: LSP mode; `--json` output for tooling.

**Use `ripgrep` when text is enough.** It’s the fastest way to grep literals/regex across files.

* Recon: find strings, TODOs, log lines, config values, or non-code assets.
* Pre-filter: narrow candidate files before a precise pass.

**Rule of thumb**

* Need correctness over speed, or you’ll **apply changes** → start with `ast-grep`.
* Need raw speed or you’re just **hunting text** → start with `rg`.
* Often combine: `rg` to shortlist files, then `ast-grep` to match/modify with precision.

**Snippets**

Find structured code (ignores comments/strings):

```bash
ast-grep run -l TypeScript -p 'import $X from "$P"'
```

Codemod (only real `var` declarations become `let`):

```bash
ast-grep run -l JavaScript -p 'var $A = $B' -r 'let $A = $B' -U
```

Quick textual hunt:

```bash
rg -n 'console\.log\(' -t js
```

Combine speed + precision:

```bash
rg -l -t ts 'useQuery\(' | xargs ast-grep run -l TypeScript -p 'useQuery($A)' -r 'useSuspenseQuery($A)' -U
```

**Mental model**

* Unit of match: `ast-grep` = node; `rg` = line.
* False positives: `ast-grep` low; `rg` depends on your regex.
* Rewrites: `ast-grep` first-class; `rg` requires ad-hoc sed/awk and risks collateral edits.

---

## UBS Quick Reference for AI Agents

UBS stands for "Ultimate Bug Scanner": **The AI Coding Agent's Secret Weapon: Flagging Likely Bugs for Fixing Early On**

**Install:**

```bash
curl -sSL https://raw.githubusercontent.com/Dicklesworthstone/ultimate_bug_scanner/main/install.sh | bash
```

**Golden Rule:** `ubs <changed-files>` before every commit. Exit 0 = safe. Exit >0 = fix & re-run.

**Commands:**

```bash
ubs file.ts file2.ts                    # Specific files (< 1s) — USE THIS
ubs $(git diff --name-only --cached)    # Staged files — before commit
ubs --only=js,ts src/                   # Language filter (3-5x faster)
ubs --ci --fail-on-warning .            # CI mode — before PR
ubs --help                              # Full command reference
ubs sessions --entries 1                # Tail the latest install session log
ubs .                                   # Whole project (ignores things like .next, node_modules automatically)
```

**Output Format:**

```text
⚠️  Category (N errors)
    file.ts:42:5 – Issue description
    💡 Suggested fix
Exit code: 1
```

Parse: `file:line:col` → location | 💡 → how to fix | Exit 0/1 → pass/fail

**Fix Workflow:**

1. Read finding → category + fix suggestion.
2. Navigate `file:line:col` → view context.
3. Verify real issue (not false positive).
4. Fix root cause (not symptom).
5. Re-run `ubs <file>` → exit 0.
6. Commit.

**Speed Critical:** Scope to changed files. `ubs src/file.ts` (< 1s) vs `ubs .` (30s). Never full scan for small edits.

**Bug Severity:**

* **Critical** (always fix): null/undefined safety, injection vulnerabilities, race conditions, resource leaks.
* **Important** (production): type narrowing, error handling, performance landmines.
* **Contextual** (judgment): TODO/FIXME, excessive console logs.

**Anti-Patterns:**

* ❌ Ignore findings → ✅ Investigate each.
* ❌ Full scan per edit → ✅ Scope to changed files.
* ❌ Fix symptom only → ✅ Fix root cause.

---

### Testing: Never Open Browsers

**Tests must NEVER automatically open a browser.** All browser-opening functions check `BV_NO_BROWSER` and `BV_TEST_MODE` environment variables. These are set globally via `TestMain` in:
- `tests/e2e/common_test.go`
- `pkg/export/main_test.go`
- `pkg/ui/main_test.go`

When adding new browser-opening code, always check these env vars first:
```go
if os.Getenv("BV_NO_BROWSER") != "" || os.Getenv("BV_TEST_MODE") != "" {
    return nil
}
```

---

You should try to follow all best practices laid out in the file GOLANG_BEST_PRACTICES.md


---


### Morph Warp Grep — AI-powered code search

**Use `mcp__morph-mcp__warp_grep` for exploratory "how does X work?" questions.** An AI search agent automatically expands your query into multiple search patterns, greps the codebase, reads relevant files, and returns precise line ranges with full context—all in one call.

**Use `ripgrep` (via Grep tool) for targeted searches.** When you know exactly what you're looking for—a specific function name, error message, or config key—ripgrep is faster and more direct.

**Use `ast-grep` for structural code patterns.** When you need to match/rewrite AST nodes while ignoring comments/strings, or enforce codebase-wide rules.

**When to use what**

| Scenario | Tool | Why |
|----------|------|-----|
| "How is authentication implemented?" | `warp_grep` | Exploratory; don't know where to start |
| "Where is the L3 Guardian appeals system?" | `warp_grep` | Need to understand architecture, find multiple related files |
| "Find all uses of `useQuery(`" | `ripgrep` | Targeted literal search |
| "Find files with `console.log`" | `ripgrep` | Simple pattern, known target |
| "Rename `getUserById` → `fetchUser`" | `ast-grep` | Structural refactor, avoid comments/strings |
| "Replace all `var` with `let`" | `ast-grep` | Codemod across codebase |

**warp_grep strengths**

* **Reduces context pollution**: Returns only relevant line ranges, not entire files.
* **Intelligent expansion**: Turns "appeals system" into searches for `appeal`, `Appeals`, `guardian`, `L3`, etc.
* **One-shot answers**: Finds the 3-5 most relevant files with precise locations vs. manual grep→read cycles.
* **Natural language**: Works well with "how", "where", "what" questions.

**warp_grep usage**

```
mcp__morph-mcp__warp_grep(
  repoPath: "/data/projects/communitai",
  query: "How is the L3 Guardian appeals system implemented?"
)
```

Returns structured results with file paths, line ranges, and extracted code snippets.

**Rule of thumb**

* **Don't know where to look** → `warp_grep` (let AI find it)
* **Know the pattern** → `ripgrep` (fastest)
* **Need AST precision** → `ast-grep` (safest for rewrites)

**Anti-patterns**

* ❌ Using `warp_grep` to find a specific function name you already know → use `ripgrep`
* ❌ Using `ripgrep` to understand "how does X work" → wastes time with manual file reads
* ❌ Using `ripgrep` for codemods → misses comments/strings, risks collateral edits

### Morph Warp Grep vs Standard Grep

Warp Grep = AI agent that greps, reads, follows connections, returns synthesized context with line numbers.
Standard Grep = Fast regex match, you interpret results.

Decision: Can you write the grep pattern?
- Yes → Grep
- No, you have a question → mcp__morph-mcp__warp_grep

#### Warp Grep Queries (natural language, unknown location)
"How does the moderation appeals flow work?"
"Where are websocket connections managed?"
"What happens when a user submits a post?"
"Where is rate limiting implemented?"
"How does the auth session get validated on API routes?"
"What services touch the moderationDecisions table?"

#### Standard Grep Queries (known pattern, specific target)
pattern="fileAppeal"                          # known function name
pattern="class.*Service"                      # structural pattern
pattern="TODO|FIXME|HACK"                     # markers
pattern="processenv" path="apps/web"      # specific string
pattern="import.*from [']@/lib/db"          # import tracing

#### What Warp Grep Does Internally
One query → 15-30 operations: greps multiple patterns → reads relevant sections → follows imports/references → returns focused line ranges (e.g., l3-guardian.ts:269-440) not whole files.

#### Anti-patterns
| Don't Use Warp Grep For | Why | Use Instead |
|------------------------|-----|-------------|
| "Find function handleSubmit" | Known name | Grep pattern="handleSubmit" |
| "Read the auth config" | Known file | Read file_path="lib/auth/..." |
| "Check if X exists" | Boolean answer | Grep + check results |
| Quick lookups mid-task | 5-10s latency | Grep is 100ms |

#### When Warp Grep Wins
- Tracing data flow across files (API → service → schema → types)
- Understanding unfamiliar subsystems before modifying
- Answering "how" questions that span 3+ files
- Finding all touching points for a cross-cutting concern
