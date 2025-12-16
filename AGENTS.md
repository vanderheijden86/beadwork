### Using bv as an AI sidecar

  bv is a fast terminal UI for Beads projects (.beads/beads.jsonl). It renders lists/details and precomputes dependency metrics (PageRank, critical path, cycles, etc.) so you instantly see blockers and execution order. For agents, it's a graph sidecar: instead of parsing JSONL or risking hallucinated traversal, call the robot flags to get deterministic, dependency-aware outputs.

  **Scope boundary:** bv helps agents understand *what to work on next* (triage, priority, plan). It does NOT handle agent-to-agent communication, registration, or coordination. That functionality is provided by the complementary project [MCP Agent Mail](https://github.com/Dicklesworthstone/mcp_agent_mail), which handles:
  - Agent registration and identity management
  - Message passing between agents
  - Work claiming/handoff coordination
  - File reservation and conflict detection

  If you need multi-agent coordination features, use MCP Agent Mail alongside bv. Keep bv focused on issue triage and work prioritization.

  - bv --robot-help — shows all AI-facing commands.
  - **bv --robot-triage** — THE MEGA-COMMAND. Single entry point for AI agents. Returns unified JSON with:
    - `quick_ref`: at-a-glance summary (open/actionable/blocked counts, top 3 picks)
    - `recommendations`: ranked actionable items with scores, reasons, and unblock info
    - `quick_wins`: low-complexity, high-impact items
    - `blockers_to_clear`: items that unblock the most downstream work
    - `project_health`: counts by status/type/priority, graph metrics
    - `commands`: copy-paste commands for common next steps
  - **bv --robot-next** — Minimal triage. Returns only the single top recommendation with claim/show commands. Use when you just need "what should I work on next?"
  - bv --robot-insights — JSON graph metrics (PageRank, betweenness, HITS, critical path, cycles) with top-N summaries for quick triage.
  - bv --robot-plan — JSON execution plan: parallel tracks, items per track, and unblocks lists showing what each item frees up.
  - bv --robot-priority — JSON priority recommendations with reasoning and confidence.
  - bv --robot-recipes — list recipes (default, actionable, blocked, etc.); apply via bv --recipe <name> to pre-filter/sort before other flags.
  - bv --robot-diff --diff-since <commit|date> — JSON diff of issue changes, new/closed items, and cycles introduced/resolved.
  - **bv --robot-history** — JSON bead-to-commit correlations. Tracks which code changes relate to which beads via git history analysis. Key sections:
    - `stats`: Summary (total beads, beads with commits, avg cycle time)
    - `histories`: Per-bead data (events, commits, milestones, cycle_time)
    - `commit_index`: Reverse lookup from commit SHA to bead IDs
    - Flags: `--bead-history <id>` (filter to single bead), `--history-since <ref>` (limit to recent), `--history-limit <n>` (max commits)

  **Recommended workflow for agents:**
  1. Start with `bv --robot-next` for a quick "what's next?" answer
  2. Use `bv --robot-triage` for comprehensive context when planning
  3. Use `bv --robot-plan` for parallel work partitioning
  4. Use `bv --robot-insights` when you need deep graph analysis

  Use these commands instead of hand-rolling graph logic; bv already computes the hard parts so agents can act safely and quickly.

### bv Performance Considerations for AI Agents

  bv uses a two-phase startup for responsive performance:
  - **Phase 1 (instant):** Degree, topo sort, basic stats - available immediately
  - **Phase 2 (async):** PageRank, betweenness, HITS, cycles - computed in background

  **For large graphs (>500 nodes):**
  - Some expensive metrics (betweenness) may be skipped automatically
  - Cycle detection limited to prevent exponential blowup
  - Use `--robot-insights` and check for skipped metrics

  **Timeout handling:**
  - All expensive algorithms have 500ms timeouts
  - Robot output includes timeout flags when metrics are incomplete
  - Design agents to handle partial data gracefully

  **Diagnostic commands:**
  - `bv --profile-startup` — detailed timing breakdown
  - `bv --profile-startup --profile-json` — machine-readable profile

  **Best practices:**
  - Use `--robot-plan` for immediate actionable items (fast, Phase 1 only)
  - Use `--robot-insights` when you need full graph metrics (waits for Phase 2)
  - Avoid `--force-full-analysis` unless absolutely needed (can be slow)

  See [docs/performance.md](docs/performance.md) for detailed tuning guide.

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