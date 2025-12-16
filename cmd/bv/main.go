package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/baseline"
	"github.com/Dicklesworthstone/beads_viewer/pkg/correlation"
	"github.com/Dicklesworthstone/beads_viewer/pkg/drift"
	"github.com/Dicklesworthstone/beads_viewer/pkg/export"
	"github.com/Dicklesworthstone/beads_viewer/pkg/hooks"
	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/recipe"
	"github.com/Dicklesworthstone/beads_viewer/pkg/search"
	"github.com/Dicklesworthstone/beads_viewer/pkg/ui"
	"github.com/Dicklesworthstone/beads_viewer/pkg/updater"
	"github.com/Dicklesworthstone/beads_viewer/pkg/version"
	"github.com/Dicklesworthstone/beads_viewer/pkg/workspace"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	help := flag.Bool("help", false, "Show help")
	versionFlag := flag.Bool("version", false, "Show version")
	// Update flags (bv-182)
	updateFlag := flag.Bool("update", false, "Update bv to the latest version")
	checkUpdateFlag := flag.Bool("check-update", false, "Check if a new version is available")
	rollbackFlag := flag.Bool("rollback", false, "Rollback to the previous version (from backup)")
	yesFlag := flag.Bool("yes", false, "Skip confirmation prompts (use with --update)")
	exportFile := flag.String("export-md", "", "Export issues to a Markdown file (e.g., report.md)")
	robotHelp := flag.Bool("robot-help", false, "Show AI agent help")
	robotInsights := flag.Bool("robot-insights", false, "Output graph analysis and insights as JSON for AI agents")
	robotPlan := flag.Bool("robot-plan", false, "Output dependency-respecting execution plan as JSON for AI agents")
	robotPriority := flag.Bool("robot-priority", false, "Output priority recommendations as JSON for AI agents")
	robotTriage := flag.Bool("robot-triage", false, "Output unified triage as JSON (the mega-command for AI agents)")
	robotTriageByTrack := flag.Bool("robot-triage-by-track", false, "Group triage recommendations by execution track (bv-87)")
	robotTriageByLabel := flag.Bool("robot-triage-by-label", false, "Group triage recommendations by label (bv-87)")
	robotNext := flag.Bool("robot-next", false, "Output only the top pick recommendation as JSON (minimal triage)")
	robotDiff := flag.Bool("robot-diff", false, "Output diff as JSON (use with --diff-since)")
	robotRecipes := flag.Bool("robot-recipes", false, "Output available recipes as JSON for AI agents")
	robotLabelHealth := flag.Bool("robot-label-health", false, "Output label health metrics as JSON for AI agents")
	robotLabelFlow := flag.Bool("robot-label-flow", false, "Output cross-label dependency flow as JSON for AI agents")
	robotLabelAttention := flag.Bool("robot-label-attention", false, "Output attention-ranked labels as JSON for AI agents")
	attentionLimit := flag.Int("attention-limit", 5, "Limit number of labels in --robot-label-attention output")
	robotAlerts := flag.Bool("robot-alerts", false, "Output alerts (drift + proactive) as JSON for AI agents")
	// Graph export (bv-136)
	robotGraph := flag.Bool("robot-graph", false, "Output dependency graph as JSON/DOT/Mermaid for AI agents")
	graphFormat := flag.String("graph-format", "json", "Graph output format: json, dot, mermaid")
	graphRoot := flag.String("graph-root", "", "Subgraph from specific root issue ID")
	graphDepth := flag.Int("graph-depth", 0, "Max depth for subgraph (0 = unlimited)")
	// Robot output filters (bv-84)
	robotMinConf := flag.Float64("robot-min-confidence", 0.0, "Filter robot outputs by minimum confidence (0.0-1.0)")
	robotMaxResults := flag.Int("robot-max-results", 0, "Limit robot output count (0 = use defaults)")
	robotByLabel := flag.String("robot-by-label", "", "Filter robot outputs by label (exact match)")
	robotByAssignee := flag.String("robot-by-assignee", "", "Filter robot outputs by assignee (exact match)")
	// Label subgraph scoping (bv-122)
	labelScope := flag.String("label", "", "Scope analysis to label's subgraph (affects --robot-insights, --robot-plan, --robot-priority)")
	alertSeverity := flag.String("severity", "", "Filter robot alerts by severity (info|warning|critical)")
	alertType := flag.String("alert-type", "", "Filter robot alerts by alert type (e.g., stale_issue)")
	alertLabel := flag.String("alert-label", "", "Filter robot alerts by label match")
	recipeName := flag.String("recipe", "", "Apply named recipe (e.g., triage, actionable, high-impact)")
	recipeShort := flag.String("r", "", "Shorthand for --recipe")
	semanticQuery := flag.String("search", "", "Semantic search query (vector-based; builds/updates index on first run)")
	robotSearch := flag.Bool("robot-search", false, "Output semantic search results as JSON for AI agents (use with --search)")
	searchLimit := flag.Int("search-limit", 10, "Max results for --search/--robot-search")
	diffSince := flag.String("diff-since", "", "Show changes since historical point (commit SHA, branch, tag, or date)")
	asOf := flag.String("as-of", "", "View state at point in time (commit SHA, branch, tag, or date)")
	forceFullAnalysis := flag.Bool("force-full-analysis", false, "Compute all metrics regardless of graph size (may be slow for large graphs)")
	profileStartup := flag.Bool("profile-startup", false, "Output detailed startup timing profile for diagnostics")
	profileJSON := flag.Bool("profile-json", false, "Output profile in JSON format (use with --profile-startup)")
	noHooks := flag.Bool("no-hooks", false, "Skip running hooks during export")
	workspaceConfig := flag.String("workspace", "", "Load issues from workspace config file (.bv/workspace.yaml)")
	repoFilter := flag.String("repo", "", "Filter issues by repository prefix (e.g., 'api-' or 'api')")
	saveBaseline := flag.String("save-baseline", "", "Save current metrics as baseline with optional description")
	baselineInfo := flag.Bool("baseline-info", false, "Show information about the current baseline")
	checkDrift := flag.Bool("check-drift", false, "Check for drift from baseline (exit codes: 0=OK, 1=critical, 2=warning)")
	robotDriftCheck := flag.Bool("robot-drift", false, "Output drift check as JSON (use with --check-drift)")
	robotHistory := flag.Bool("robot-history", false, "Output bead-to-commit correlations as JSON")
	beadHistory := flag.String("bead-history", "", "Show history for specific bead ID")
	historySince := flag.String("history-since", "", "Limit history to commits after this date/ref (e.g., '30 days ago', '2024-01-01')")
	historyLimit := flag.Int("history-limit", 500, "Max commits to analyze (0 = unlimited)")
	minConfidence := flag.Float64("min-confidence", 0.0, "Filter correlations by minimum confidence (0.0-1.0)")
	// Sprint flags (bv-156)
	robotSprintList := flag.Bool("robot-sprint-list", false, "Output sprints as JSON")
	robotSprintShow := flag.String("robot-sprint-show", "", "Output specific sprint details as JSON")
	// Forecast flags (bv-158)
	robotForecast := flag.String("robot-forecast", "", "Output ETA forecast for bead ID, or 'all' for all open issues")
	forecastLabel := flag.String("forecast-label", "", "Filter forecast by label")
	forecastSprint := flag.String("forecast-sprint", "", "Filter forecast by sprint ID")
	forecastAgents := flag.Int("forecast-agents", 1, "Number of parallel agents for capacity calculation")
	// Capacity simulation flags (bv-160)
	robotCapacity := flag.Bool("robot-capacity", false, "Output capacity simulation and completion projection as JSON")
	capacityAgents := flag.Int("agents", 1, "Number of parallel agents for capacity simulation")
	capacityLabel := flag.String("capacity-label", "", "Filter capacity simulation by label")
	// Burndown flags (bv-159)
	robotBurndown := flag.String("robot-burndown", "", "Output burndown data for sprint ID, or 'current' for active sprint")
	// Action script emission flags (bv-89)
	emitScript := flag.Bool("emit-script", false, "Emit shell script for top-N recommendations (agent workflows)")
	scriptLimit := flag.Int("script-limit", 5, "Limit number of items in emitted script (use with --emit-script)")
	scriptFormat := flag.String("script-format", "bash", "Script format: bash, fish, or zsh (use with --emit-script)")
	// Feedback loop flags (bv-90)
	feedbackAccept := flag.String("feedback-accept", "", "Record accept feedback for issue ID (tunes recommendation weights)")
	feedbackIgnore := flag.String("feedback-ignore", "", "Record ignore feedback for issue ID (tunes recommendation weights)")
	feedbackReset := flag.Bool("feedback-reset", false, "Reset all feedback data to defaults")
	feedbackShow := flag.Bool("feedback-show", false, "Show current feedback status and weight adjustments")
	// Priority brief export (bv-96)
	priorityBrief := flag.String("priority-brief", "", "Export priority brief to Markdown file (e.g., brief.md)")
	// Agent brief bundle (bv-131)
	agentBrief := flag.String("agent-brief", "", "Export agent brief bundle to directory (includes triage.json, insights.json, brief.md, helpers.md)")
	// Static pages export flags (bv-73f)
	exportPages := flag.String("export-pages", "", "Export static site to directory (e.g., ./bv-pages)")
	pagesTitle := flag.String("pages-title", "", "Custom title for static site")
	pagesIncludeClosed := flag.Bool("pages-include-closed", false, "Include closed issues in export")
	previewPages := flag.String("preview-pages", "", "Preview existing static site bundle")
	pagesWizard := flag.Bool("pages", false, "Launch interactive Pages deployment wizard")
	flag.Parse()

	// Ensure static export flags are retained even when build tags strip features in some environments.
	_ = exportPages
	_ = pagesTitle
	_ = pagesIncludeClosed
	_ = previewPages
	_ = pagesWizard
	_ = robotForecast
	_ = forecastLabel
	_ = forecastSprint
	_ = forecastAgents
	_ = robotCapacity
	_ = capacityAgents
	_ = capacityLabel
	_ = labelScope
	_ = agentBrief

	envRobot := os.Getenv("BV_ROBOT") == "1"
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Handle -r shorthand
	if *recipeShort != "" && *recipeName == "" {
		*recipeName = *recipeShort
	}

	if *help {
		fmt.Println("Usage: bv [options]")
		fmt.Println("\nA TUI viewer for beads issue tracker.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *robotHelp {
		fmt.Println("bv (Beads Viewer) AI Agent Interface")
		fmt.Println("====================================")
		fmt.Println("This tool provides structural analysis of the issue tracker graph (DAG).")
		fmt.Println("Use these commands to understand project state without parsing raw JSONL.")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  --robot-plan")
		fmt.Println("      Outputs a dependency-respecting execution plan as JSON.")
		fmt.Println("      Shows what can be worked on now and what it unblocks.")
		fmt.Println("      Key fields:")
		fmt.Println("      - tracks: Independent work streams that can be parallelized")
		fmt.Println("      - items: Actionable issues sorted by priority within each track")
		fmt.Println("      - unblocks: Issues that become actionable when this item is done")
		fmt.Println("      - summary: Highlights highest-impact item to work on first")
		fmt.Println("")
		fmt.Println("  --robot-insights")
		fmt.Println("      Outputs a JSON object containing deep graph analysis.")
		fmt.Println("      Key metrics explained:")
		fmt.Println("      - PageRank: Measures 'blocking power'. High score = Fundamental dependency.")
		fmt.Println("      - Betweenness: Measures 'bottleneck status'. High score = Connects disparate clusters.")
		fmt.Println("      - CriticalPathScore: Heuristic for depth. High score = Blocking a long chain of work.")
		fmt.Println("      - Hubs/Authorities: HITS algorithm scores for dependency relationships.")
		fmt.Println("      - Cycles: Lists of circular dependencies (unhealthy state).")
		fmt.Println("")
		fmt.Println("  --robot-priority")
		fmt.Println("      Outputs priority recommendations as JSON.")
		fmt.Println("      Compares impact scores to current priorities and suggests adjustments.")
		fmt.Println("      Key fields:")
		fmt.Println("      - recommendations: Sorted by confidence, then impact score")
		fmt.Println("      - confidence: 0-1 score indicating strength of recommendation")
		fmt.Println("      - reasoning: Human-readable explanations for the suggestion")
		fmt.Println("      - direction: 'increase' or 'decrease' priority")
		fmt.Println("")
		fmt.Println("  --robot-triage")
		fmt.Println("      THE MEGA-COMMAND: Unified triage output combining all analysis.")
		fmt.Println("      Single entry point for AI agents - one call gets everything needed.")
		fmt.Println("      Key sections:")
		fmt.Println("      - meta: Generation timestamp, data stats")
		fmt.Println("      - quick_ref: At-a-glance summary (open/actionable/blocked counts, top 3 picks)")
		fmt.Println("      - recommendations: Ranked actionable items with scores and reasoning")
		fmt.Println("      - quick_wins: Low-complexity, high-impact items")
		fmt.Println("      - blockers_to_clear: Items that unblock the most downstream work")
		fmt.Println("      - project_health: Counts, graph metrics, overall status")
		fmt.Println("      - commands: Copy-paste commands for common next steps")
		fmt.Println("")
		fmt.Println("  --robot-next")
		fmt.Println("      Minimal triage: returns only the single top recommendation.")
		fmt.Println("      Output includes: id, title, score, reasons, claim_command, show_command")
		fmt.Println("      Use when you just need to know \"what should I work on next?\"")
		fmt.Println("")
		fmt.Println("  --search \"query\" [--robot-search]")
		fmt.Println("      Semantic vector search over issue titles/descriptions.")
		fmt.Println("      Builds/updates a local on-disk vector index on first run.")
		fmt.Println("      Use --robot-search to emit JSON for automation.")
		fmt.Println("")
		fmt.Println("  --emit-script [--script-limit=N]")
		fmt.Println("      Emits a shell script for top-N recommendations (default: 5).")
		fmt.Println("      Includes hash/config header for deterministic ordering.")
		fmt.Println("      Output: bd show commands for each item, commented claim commands")
		fmt.Println("      Options: --script-format=bash|fish|zsh, --script-limit=N")
		fmt.Println("      Example: bv --emit-script > work.sh && bash work.sh")
		fmt.Println("      Example: bv --emit-script --script-limit=3")
		fmt.Println("")
		fmt.Println("  --robot-history")
		fmt.Println("      Outputs bead-to-commit correlations as JSON.")
		fmt.Println("      Tracks which code changes relate to which beads via git history analysis.")
		fmt.Println("      Key sections:")
		fmt.Println("      - stats: Summary (total beads, beads with commits, avg cycle time)")
		fmt.Println("      - histories: Per-bead data (events, commits, milestones, cycle_time)")
		fmt.Println("      - commit_index: Reverse lookup from commit SHA to bead IDs")
		fmt.Println("      Flags:")
		fmt.Println("      - --bead-history <id>: Filter to single bead")
		fmt.Println("      - --history-since <ref>: Limit to recent commits")
		fmt.Println("      - --history-limit <n>: Max commits to analyze (default: 500)")
		fmt.Println("      - --min-confidence <0.0-1.0>: Filter by minimum confidence score")
		fmt.Println("      Example: bv --robot-history --history-since '30 days ago'")
		fmt.Println("      Example: bv --robot-history --min-confidence 0.7")
		fmt.Println("")
		fmt.Println("  --robot-sprint-list")
		fmt.Println("      Outputs all sprints as JSON for planning and forecasting.")
		fmt.Println("      Key fields:")
		fmt.Println("      - generated_at: Timestamp of the output")
		fmt.Println("      - sprint_count: Number of sprints")
		fmt.Println("      - sprints: Array of sprint objects (id, name, start_date, end_date, bead_ids)")
		fmt.Println("      Example: bv --robot-sprint-list")
		fmt.Println("")
		fmt.Println("  --robot-sprint-show <id>")
		fmt.Println("      Outputs details for a specific sprint as JSON.")
		fmt.Println("      Returns the full sprint object with all fields.")
		fmt.Println("      Example: bv --robot-sprint-show sprint-1")
		fmt.Println("")
		fmt.Println("  --robot-burndown <id|current>")
		fmt.Println("      Outputs burndown data for a sprint as JSON.")
		fmt.Println("      Use 'current' to get the active sprint, or specify sprint ID.")
		fmt.Println("      Key fields:")
		fmt.Println("      - total_days, elapsed_days, remaining_days")
		fmt.Println("      - total_issues, completed_issues, remaining_issues")
		fmt.Println("      - ideal_burn_rate, actual_burn_rate")
		fmt.Println("      - projected_complete: Estimated completion date")
		fmt.Println("      - on_track: Whether sprint will complete on time")
		fmt.Println("      - daily_points: Actual burndown data points")
		fmt.Println("      - ideal_line: Expected burndown line")
		fmt.Println("      Example: bv --robot-burndown current")
		fmt.Println("      Example: bv --robot-burndown sprint-1")
		fmt.Println("")
		fmt.Println("  --robot-forecast <id|all>")
		fmt.Println("      Outputs ETA forecast for a specific bead or all open issues.")
		fmt.Println("      Returns estimated completion date, confidence, and factors.")
		fmt.Println("      Options:")
		fmt.Println("        --forecast-label=X    Filter by label")
		fmt.Println("        --forecast-sprint=Y   Filter by sprint")
		fmt.Println("        --forecast-agents=N   Parallel agents (default: 1)")
		fmt.Println("      Example: bv --robot-forecast bv-123")
		fmt.Println("      Example: bv --robot-forecast all --forecast-label=backend")
		fmt.Println("      Example: bv --robot-forecast all --forecast-agents=2")
		fmt.Println("")
		fmt.Println("  --robot-capacity [--agents=N] [--capacity-label=X]")
		fmt.Println("      Outputs capacity simulation and completion projection as JSON.")
		fmt.Println("      Analyzes work remaining, parallelizability, and bottlenecks.")
		fmt.Println("      Key fields:")
		fmt.Println("        - total_minutes: Sum of estimated work across open issues")
		fmt.Println("        - parallelizable_pct: Percentage that can be parallelized")
		fmt.Println("        - serial_minutes: Work that must be done sequentially")
		fmt.Println("        - estimated_days: Projected completion time with N agents")
		fmt.Println("        - bottlenecks: Issues limiting parallelization")
		fmt.Println("      Options:")
		fmt.Println("        --agents=N           Number of parallel agents (default: 1)")
		fmt.Println("        --capacity-label=X   Filter analysis to label's subgraph")
		fmt.Println("      Example: bv --robot-capacity --agents=3")
		fmt.Println("      Example: bv --robot-capacity --capacity-label=backend")
		fmt.Println("")
		fmt.Println("  --emit-script [--script-limit=N] [--script-format=bash|fish|zsh]")
		fmt.Println("      Emits a shell script for top-N priority recommendations.")
		fmt.Println("      Useful for agent workflows and automation.")
		fmt.Println("      Output includes:")
		fmt.Println("        - Header comment with data hash and generation time")
		fmt.Println("        - bd show commands for each recommended item")
		fmt.Println("        - Commented bd update commands to claim items")
		fmt.Println("      Options:")
		fmt.Println("        --script-limit=N      Number of items (default: 5)")
		fmt.Println("        --script-format=X     Script format: bash, fish, zsh")
		fmt.Println("      Example: bv --emit-script")
		fmt.Println("      Example: bv --emit-script --script-limit=3")
		fmt.Println("      Example: bv --emit-script --script-format=fish > work.fish")
		fmt.Println("      Example: bv --emit-script | bash  # Show top 5 items")
		fmt.Println("")
		fmt.Println("  --export-md <file>")
		fmt.Println("      Generates a readable status report with Mermaid.js visualizations.")
		fmt.Println("      Runs pre-export and post-export hooks if configured in .bv/hooks.yaml")
		fmt.Println("")
		fmt.Println("  --no-hooks")
		fmt.Println("      Skip running hooks during export. Useful for CI or quick exports.")
		fmt.Println("")
		fmt.Println("  Hook Configuration (.bv/hooks.yaml)")
		fmt.Println("      Configure hooks to automate export workflows:")
		fmt.Println("      - pre-export: Validation, notifications (failure cancels export)")
		fmt.Println("      - post-export: Notifications, uploads (failure logged only)")
		fmt.Println("      Environment variables: BV_EXPORT_PATH, BV_EXPORT_FORMAT,")
		fmt.Println("        BV_ISSUE_COUNT, BV_TIMESTAMP")
		fmt.Println("")
		fmt.Println("  --diff-since <commit|date>")
		fmt.Println("      Shows changes since a historical point.")
		fmt.Println("      Accepts: SHA, branch name, tag, HEAD~N, or date (YYYY-MM-DD)")
		fmt.Println("      Key output:")
		fmt.Println("      - new_issues: Issues added since then")
		fmt.Println("      - closed_issues: Issues that were closed")
		fmt.Println("      - removed_issues: Issues deleted from tracker")
		fmt.Println("      - modified_issues: Issues with field changes")
		fmt.Println("      - new_cycles: Circular dependencies introduced")
		fmt.Println("      - resolved_cycles: Circular dependencies fixed")
		fmt.Println("      - summary.health_trend: 'improving', 'degrading', or 'stable'")
		fmt.Println("")
		fmt.Println("  --as-of <commit|date>")
		fmt.Println("      View issue state at a point in time.")
		fmt.Println("      Useful for reviewing historical project state.")
		fmt.Println("")
		fmt.Println("  --robot-diff")
		fmt.Println("      Output diff as JSON (use with --diff-since).")
		fmt.Println("      Fields: generated_at, resolved_revision, from_data_hash, to_data_hash, diff{...}")
		fmt.Println("      Diff payload includes metric deltas, cycles introduced/resolved, and modified issues.")
		fmt.Println("")
		fmt.Println("  --robot-recipes")
		fmt.Println("      Lists all available recipes as JSON.")
		fmt.Println("      Output: {recipes: [{name, description, source}]}")
		fmt.Println("      Sources: 'builtin', 'user' (~/.config/bv/recipes.yaml), 'project' (.bv/recipes.yaml)")
		fmt.Println("")
		fmt.Println("  --robot-label-health")
		fmt.Println("      Outputs label health metrics as JSON (velocity, freshness, flow, criticality).")
		fmt.Println("      Includes label summaries, detailed metrics, and cross-label dependencies.")
		fmt.Println("      Key fields: health_level (healthy|warning|critical), velocity_score, flow_score.")
		fmt.Println("")
		fmt.Println("  --robot-label-flow")
		fmt.Println("      Outputs cross-label dependency flow as JSON (label->label edges).")
		fmt.Println("      Key fields: labels[], flow_matrix[from][to], dependencies[{from,to,count,issue_ids}],")
		fmt.Println("                  bottleneck_labels (highest outgoing), total_cross_label_deps.")
		fmt.Println("      Use when you need to see which labels are blocking others at a glance.")
		fmt.Println("")
		fmt.Println("  --robot-label-attention [--attention-limit=N]")
		fmt.Println("      Outputs attention-ranked labels as JSON (default limit: 5).")
		fmt.Println("      Labels ranked by attention score = (pagerank * staleness * block_impact) / velocity.")
		fmt.Println("      Key fields: rank, label, attention_score, normalized_score, reason, blocked_count, stale_count.")
		fmt.Println("      Use to identify which labels need the most focus based on centrality and health factors.")
		fmt.Println("")
		fmt.Println("  --robot-alerts")
		fmt.Println("      Outputs drift + proactive alerts as JSON (staleness, cascades, density, cycles).")
		fmt.Println("      Filters: --severity=<info|warning|critical>, --alert-type=<type>, --alert-label=<label>")
		fmt.Println("      Fields: type, severity, message, issue_id, label, detected_at, details[].")
		fmt.Println("")
		fmt.Println("  --robot-graph [--graph-format=json|dot|mermaid] [--graph-root=ID] [--graph-depth=N]")
		fmt.Println("      Outputs dependency graph in specified format (default: JSON adjacency).")
		fmt.Println("      Formats:")
		fmt.Println("        - json: Adjacency list with nodes[], edges[], metadata")
		fmt.Println("        - dot: Graphviz DOT format (render with: dot -Tpng file.dot -o graph.png)")
		fmt.Println("        - mermaid: Mermaid diagram format (paste into GitHub/markdown)")
		fmt.Println("      Options:")
		fmt.Println("        --label LABEL: Filter to issues with specific label")
		fmt.Println("        --graph-root ID: Extract subgraph starting from root issue")
		fmt.Println("        --graph-depth N: Limit subgraph depth (0 = unlimited)")
		fmt.Println("      Fields: format, graph (string for dot/mermaid), nodes, edges, filters_applied, explanation")
		fmt.Println("      Example: bv --robot-graph --graph-format=dot --label=api > api-deps.dot")
		fmt.Println("")
		fmt.Println("  --robot-insights")
		fmt.Println("      Graph metrics JSON for agents.")
		fmt.Println("      Top lists: Bottlenecks (betweenness), Keystones (critical path), Influencers (eigenvector),")
		fmt.Println("                 Cores (k-core), Articulation points (cut vertices), Slack (parallelism headroom).")
		fmt.Println("      Full maps (capped by BV_INSIGHTS_MAP_LIMIT): pagerank, betweenness, eigenvector, hubs/authorities, core_number, slack.")
		fmt.Println("      status captures per-metric state: computed|approx|timeout|skipped with elapsed_ms and reasons.")
		fmt.Println("      Shared fields: data_hash, analysis_config.")
		fmt.Println("      Quick jq: jq '.full_stats.core_number | to_entries | sort_by(-.value)[:5]'   # top k-core nodes")
		fmt.Println("                 jq '.Articulation'                                                  # structural cut points")
		fmt.Println("                 jq '.Slack[:5]'                                                     # highest slack (parallel-friendly)")
		fmt.Println("      advanced_insights: Canonical structure for advanced graph features:")
		fmt.Println("        - topk_set: Best k issues for maximum downstream unlock (status: pending)")
		fmt.Println("        - coverage_set: Minimal set covering all critical paths (status: pending)")
		fmt.Println("        - k_paths: K-shortest critical paths through the graph (status: pending)")
		fmt.Println("        - parallel_cut: Suggestions for maximizing parallel work (status: pending)")
		fmt.Println("        - parallel_gain: Parallelization metrics for recommendations (status: pending)")
		fmt.Println("        - cycle_break: Suggestions for breaking cycles with minimal impact (status: available)")
		fmt.Println("        Per-feature: status (available|pending|skipped|error), items, usage hints")
		fmt.Println("        Config: caps for deterministic output (topk<=5, paths<=5, path_len<=50, etc.)")
		fmt.Println("        Quick jq: jq '.advanced_insights.cycle_break'   # cycle break suggestions")
		fmt.Println("")
		fmt.Println("  --robot-plan")
		fmt.Println("      Execution tracks grouped for parallel work. Includes data_hash, analysis_config, status.")
		fmt.Println("      plan.tracks[].items[].unblocks shows what completes next; summary.highest_impact surfaces best unblocker.")
		fmt.Println("")
		fmt.Println("  --robot-priority")
		fmt.Println("      Priority recommendations with explanations. Includes data_hash, analysis_config, status.")
		fmt.Println("      recommendation fields: id, current_priority, suggested_priority, impact_score, confidence, reasoning[].")
		fmt.Println("      explanation.what_if: impact of completing (direct_unblocks, transitive_unblocks, estimated_days_saved).")
		fmt.Println("      explanation.top_reasons: top 3 factors (pagerank, betweenness, blockers, staleness, etc.).")
		fmt.Println("")
		fmt.Println("  Robot Output Filters (bv-84):")
		fmt.Println("      --robot-min-confidence 0.6    Filter by minimum confidence (0.0-1.0)")
		fmt.Println("      --robot-max-results 5         Limit to top N results")
		fmt.Println("      --robot-by-label bug          Filter by label (exact match)")
		fmt.Println("      --robot-by-assignee alice     Filter by assignee (exact match)")
		fmt.Println("")
		fmt.Println("  Label Subgraph Scoping (bv-122):")
		fmt.Println("      --label LABEL                 Scope analysis to label's subgraph")
		fmt.Println("      Affects: --robot-insights, --robot-plan, --robot-priority")
		fmt.Println("      Filters issues to those with the label, then runs analysis on subgraph.")
		fmt.Println("      Includes label_scope and label_context in output with health metrics.")
		fmt.Println("      Example: bv --robot-insights --label api")
		fmt.Println("")
		fmt.Println("  --robot-triage / --robot-next")
		fmt.Println("      Unified triage (mega command) or single top pick. QuickRef includes top picks, quick_wins, blockers_to_clear.")
		fmt.Println("")
		fmt.Println("  --recipe NAME, -r NAME")
		fmt.Println("      Apply a named recipe to filter and sort issues.")
		fmt.Println("      Example: bv --recipe actionable")
		fmt.Println("      Built-in recipes: default, actionable, recent, blocked, high-impact, stale")
		fmt.Println("")
		fmt.Println("  --profile-startup")
		fmt.Println("      Outputs detailed startup timing profile for diagnostics.")
		fmt.Println("      Shows Phase 1 (blocking) and Phase 2 (async) breakdown.")
		fmt.Println("      Provides recommendations based on timing analysis.")
		fmt.Println("      Use with --profile-json for machine-readable output.")
		fmt.Println("")
		fmt.Println("  --workspace CONFIG")
		fmt.Println("      Load issues from workspace configuration file.")
		fmt.Println("      Path: typically .bv/workspace.yaml")
		fmt.Println("      Aggregates issues from multiple repositories with namespaced IDs.")
		fmt.Println("      Example: bv --workspace .bv/workspace.yaml")
		fmt.Println("")
		fmt.Println("  --repo PREFIX")
		fmt.Println("      Filter issues by repository prefix.")
		fmt.Println("      Use with --workspace to focus on one repo in a multi-repo view.")
		fmt.Println("      Matches ID prefixes like 'api-', 'web-', or partial 'api'.")
		fmt.Println("      Example: bv --workspace .bv/workspace.yaml --repo api")
		fmt.Println("")
		fmt.Println("  --save-baseline \"description\"")
		fmt.Println("      Save current metrics as a baseline snapshot.")
		fmt.Println("      Stores graph stats, top metrics, and cycle info in .bv/baseline.json.")
		fmt.Println("      Use for drift detection: compare current state to saved baseline.")
		fmt.Println("      Example: bv --save-baseline \"Before major refactor\"")
		fmt.Println("")
		fmt.Println("  --baseline-info")
		fmt.Println("      Show information about the saved baseline.")
		fmt.Println("      Displays: creation date, git commit, graph stats, top metrics.")
		fmt.Println("")
		fmt.Println("  --check-drift")
		fmt.Println("      Check current metrics against saved baseline for drift.")
		fmt.Println("      Exit codes for CI integration:")
		fmt.Println("        0 = No critical or warning alerts (info-only OK)")
		fmt.Println("        1 = Critical alerts (new cycles detected)")
		fmt.Println("        2 = Warning alerts (blocked increase, density growth)")
		fmt.Println("      Human-readable output by default, use --robot-drift for JSON.")
		fmt.Println("")
		fmt.Println("  --robot-drift")
		fmt.Println("      Output drift check as JSON (use with --check-drift).")
		fmt.Println("      Output: {has_drift, exit_code, summary, alerts, baseline}")
		fmt.Println("")
		fmt.Println("  Static Site Export & GitHub Pages (bv-7pu):")
		fmt.Println("      --pages")
		fmt.Println("          Launch interactive Pages deployment wizard.")
		fmt.Println("          Guides you through export -> preview -> deploy to GitHub Pages.")
		fmt.Println("          Handles gh CLI authentication and repository creation.")
		fmt.Println("")
		fmt.Println("      --export-pages <dir>")
		fmt.Println("          Export static HTML site to directory.")
		fmt.Println("          Creates self-contained bundle viewable in any browser.")
		fmt.Println("          Output: index.html, beads.sqlite3, data/*.json, viewer assets")
		fmt.Println("          Example: bv --export-pages ./bv-pages")
		fmt.Println("")
		fmt.Println("      --preview-pages <dir>")
		fmt.Println("          Start local server to preview existing export.")
		fmt.Println("          Opens http://localhost:9000 in your browser.")
		fmt.Println("          Example: bv --preview-pages ./bv-pages")
		fmt.Println("")
		fmt.Println("      --pages-title <title>")
		fmt.Println("          Custom title for the static site (default: 'Project Issues')")
		fmt.Println("")
		fmt.Println("      --pages-include-closed")
		fmt.Println("          Include closed issues in export (default: open only)")
		fmt.Println("")
		fmt.Println("  Drift Detection Configuration (.bv/drift.yaml)")
		fmt.Println("      Customize drift detection thresholds:")
		fmt.Println("      - density_warning_pct: 50    # Warn if density +50%")
		fmt.Println("      - blocked_increase_threshold: 5   # Warn if 5+ more blocked")
		fmt.Println("      Run 'bv --baseline-info' to see current baseline state.")
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("bv %s\n", version.Version)
		os.Exit(0)
	}

	// Handle --check-update (bv-182)
	if *checkUpdateFlag {
		available, newVersion, releaseURL, err := updater.CheckUpdateAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
			os.Exit(1)
		}
		if available {
			fmt.Printf("New version available: %s (current: %s)\n", newVersion, version.Version)
			fmt.Printf("Download: %s\n", releaseURL)
			fmt.Println("\nRun 'bv --update' to update automatically")
		} else {
			fmt.Printf("bv is up to date (version %s)\n", version.Version)
		}
		os.Exit(0)
	}

	// Handle --update (bv-182)
	if *updateFlag {
		release, err := updater.GetLatestRelease()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching release info: %v\n", err)
			os.Exit(1)
		}

		// Check if update is needed
		available, newVersion, _, _ := updater.CheckUpdateAvailable()
		if !available {
			fmt.Printf("bv is already up to date (version %s)\n", version.Version)
			os.Exit(0)
		}

		// Confirm unless --yes is provided
		if !*yesFlag {
			fmt.Printf("Update bv from %s to %s? [Y/n]: ", version.Version, newVersion)
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
			fmt.Println("Run 'bv --rollback' to restore if needed")
		}
		os.Exit(0)
	}

	// Handle --rollback (bv-182)
	if *rollbackFlag {
		if err := updater.Rollback(); err != nil {
			fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle feedback commands (bv-90)
	if *feedbackAccept != "" || *feedbackIgnore != "" || *feedbackReset || *feedbackShow {
		beadsDir, err := loader.GetBeadsDir("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
			os.Exit(1)
		}

		feedback, err := analysis.LoadFeedback(beadsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading feedback: %v\n", err)
			os.Exit(1)
		}

		if *feedbackReset {
			feedback.Reset()
			if err := feedback.Save(beadsDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Feedback data reset to defaults.")
			os.Exit(0)
		}

		if *feedbackShow {
			feedbackJSON := feedback.ToJSON()
			data, _ := json.MarshalIndent(feedbackJSON, "", "  ")
			fmt.Println(string(data))
			os.Exit(0)
		}

		// For accept/ignore, we need to get the issue's score breakdown
		if *feedbackAccept != "" || *feedbackIgnore != "" {
			issueID := *feedbackAccept
			action := "accept"
			if *feedbackIgnore != "" {
				issueID = *feedbackIgnore
				action = "ignore"
			}

			// Load issues to get score breakdown
			issues, err := loader.LoadIssues("")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading issues: %v\n", err)
				os.Exit(1)
			}

			// Find the issue
			var foundIssue *model.Issue
			for i := range issues {
				if issues[i].ID == issueID {
					foundIssue = &issues[i]
					break
				}
			}

			if foundIssue == nil {
				fmt.Fprintf(os.Stderr, "Issue not found: %s\n", issueID)
				os.Exit(1)
			}

			// Compute impact score for the issue to get breakdown
			an := analysis.NewAnalyzer(issues)
			scores := an.ComputeImpactScores()

			var score float64
			var breakdown analysis.ScoreBreakdown
			for _, s := range scores {
				if s.IssueID == issueID {
					score = s.Score
					breakdown = s.Breakdown
					break
				}
			}

			if err := feedback.RecordFeedback(issueID, action, score, breakdown); err != nil {
				fmt.Fprintf(os.Stderr, "Error recording feedback: %v\n", err)
				os.Exit(1)
			}

			if err := feedback.Save(beadsDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving feedback: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Recorded %s feedback for %s (score: %.3f)\n", action, issueID, score)
			fmt.Println(feedback.Summary())
			os.Exit(0)
		}
	}

	// Load recipes (needed for both --robot-recipes and --recipe)
	recipeLoader, err := recipe.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error loading recipes: %v\n", err)
		// Create empty loader to continue
		recipeLoader = recipe.NewLoader()
	}

	// Handle --robot-recipes (before loading issues)
	if *robotRecipes {
		summaries := recipeLoader.ListSummaries()
		// Sort by name for consistent output
		sort.Slice(summaries, func(i, j int) bool {
			return summaries[i].Name < summaries[j].Name
		})

		output := struct {
			Recipes []recipe.RecipeSummary `json:"recipes"`
		}{
			Recipes: summaries,
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding recipes: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Get project directory for baseline operations (moved up to allow info check without loading issues)
	projectDir, _ := os.Getwd()
	baselinePath := baseline.DefaultPath(projectDir)

	// Handle --baseline-info
	if *baselineInfo {
		if !baseline.Exists(baselinePath) {
			fmt.Println("No baseline found.")
			fmt.Println("Create one with: bv --save-baseline \"description\"")
			os.Exit(0)
		}
		bl, err := baseline.Load(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(bl.Summary())
		os.Exit(0)
	}

	// Validate recipe name if provided (before loading issues)
	var activeRecipe *recipe.Recipe
	if *recipeName != "" {
		activeRecipe = recipeLoader.Get(*recipeName)
		if activeRecipe == nil {
			fmt.Fprintf(os.Stderr, "Error: Unknown recipe '%s'\n\n", *recipeName)
			fmt.Fprintln(os.Stderr, "Available recipes:")
			for _, name := range recipeLoader.Names() {
				r := recipeLoader.Get(name)
				fmt.Fprintf(os.Stderr, "  %-15s %s\n", name, r.Description)
			}
			os.Exit(1)
		}
	}

	// Load issues from current directory or workspace (with timing for profile)
	loadStart := time.Now()
	var issues []model.Issue
	var beadsPath string
	var workspaceInfo *workspace.LoadSummary

	if *workspaceConfig != "" {
		// Load from workspace configuration
		loadedIssues, results, err := workspace.LoadAllFromConfig(context.Background(), *workspaceConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading workspace: %v\n", err)
			os.Exit(1)
		}
		issues = loadedIssues
		summary := workspace.Summarize(results)
		workspaceInfo = &summary

		// Print workspace loading summary
		if summary.FailedRepos > 0 {
			fmt.Fprintf(os.Stderr, "Warning: %d repos failed to load\n", summary.FailedRepos)
			for _, name := range summary.FailedRepoNames {
				fmt.Fprintf(os.Stderr, "  - %s\n", name)
			}
		}
		// No live reload for workspace mode (multiple files)
		beadsPath = ""
	} else {
		// Load from single repo (original behavior)
		var err error
		issues, err = loader.LoadIssues("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading beads: %v\n", err)
			fmt.Fprintln(os.Stderr, "Make sure you are in a project initialized with 'bd init'.")
			os.Exit(1)
		}
		// Get beads file path for live reload (respects BEADS_DIR env var)
		beadsDir, _ := loader.GetBeadsDir("")
		beadsPath, _ = loader.FindJSONLPath(beadsDir)
	}
	loadDuration := time.Since(loadStart)

	// Apply --repo filter if specified
	if *repoFilter != "" {
		issues = filterByRepo(issues, *repoFilter)
	}

	issuesForSearch := issues

	// Stable data hash for robot outputs (after repo filter but before recipes/TUI)
	dataHash := analysis.ComputeDataHash(issues)

	// Label subgraph scoping (bv-122)
	// When --label is specified, extract the label's subgraph and use it for all robot analysis.
	// This includes label health context in the output.
	var labelScopeContext *analysis.LabelHealth
	if *labelScope != "" {
		sg := analysis.ComputeLabelSubgraph(issues, *labelScope)
		if sg.IssueCount == 0 {
			fmt.Fprintf(os.Stderr, "Warning: No issues found with label %q\n", *labelScope)
		} else {
			// Replace issues with the subgraph issues
			subgraphIssues := make([]model.Issue, 0, len(sg.AllIssues))
			for _, id := range sg.AllIssues {
				if iss, ok := sg.IssueMap[id]; ok {
					subgraphIssues = append(subgraphIssues, iss)
				}
			}
			issues = subgraphIssues
			// Compute label health for context
			cfg := analysis.DefaultLabelHealthConfig()
			allHealth := analysis.ComputeAllLabelHealth(issues, cfg, time.Now().UTC(), nil)
			for i := range allHealth.Labels {
				if allHealth.Labels[i].Label == *labelScope {
					labelScopeContext = &allHealth.Labels[i]
					break
				}
			}
		}
	}

	// Handle semantic search CLI (bv-9gf.3)
	if *robotSearch && *semanticQuery == "" {
		fmt.Fprintln(os.Stderr, "Error: --robot-search requires --search \"query\"")
		os.Exit(1)
	}
	if *semanticQuery != "" {
		cfg := search.EmbeddingConfigFromEnv()
		embedder, err := search.NewEmbedderFromConfig(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		projectDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		indexPath := search.DefaultIndexPath(projectDir, cfg)
		idx, loaded, err := search.LoadOrNewVectorIndex(indexPath, embedder.Dim())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		docs := search.DocumentsFromIssues(issuesForSearch)
		if !*robotSearch && !loaded {
			fmt.Fprintf(os.Stderr, "Building semantic index (%d issues)...\n", len(docs))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		syncStats, err := search.SyncVectorIndex(ctx, idx, embedder, docs, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building semantic index: %v\n", err)
			os.Exit(1)
		}
		if !loaded || syncStats.Changed() {
			if err := idx.Save(indexPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving semantic index: %v\n", err)
				os.Exit(1)
			}
		}

		qvecs, err := embedder.Embed(ctx, []string{*semanticQuery})
		if err != nil || len(qvecs) != 1 {
			if err == nil {
				err = fmt.Errorf("embedder returned %d vectors for query", len(qvecs))
			}
			fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
			os.Exit(1)
		}

		limit := *searchLimit
		if limit <= 0 {
			limit = 10
		}
		results, err := idx.SearchTopK(qvecs[0], limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error searching index: %v\n", err)
			os.Exit(1)
		}

		titleByID := make(map[string]string, len(issuesForSearch))
		for _, iss := range issuesForSearch {
			titleByID[iss.ID] = iss.Title
		}

		if *robotSearch {
			type resultRow struct {
				IssueID string  `json:"issue_id"`
				Score   float64 `json:"score"`
				Title   string  `json:"title,omitempty"`
			}
			out := struct {
				GeneratedAt string                `json:"generated_at"`
				DataHash    string                `json:"data_hash"`
				Query       string                `json:"query"`
				Provider    search.Provider       `json:"provider"`
				Model       string                `json:"model,omitempty"`
				Dim         int                   `json:"dim"`
				IndexPath   string                `json:"index_path"`
				Index       search.IndexSyncStats `json:"index"`
				Loaded      bool                  `json:"loaded"`
				Limit       int                   `json:"limit"`
				Results     []resultRow           `json:"results"`
				UsageHints  []string              `json:"usage_hints"`
			}{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				DataHash:    dataHash,
				Query:       *semanticQuery,
				Provider:    cfg.Provider,
				Model:       cfg.Model,
				Dim:         embedder.Dim(),
				IndexPath:   indexPath,
				Index:       syncStats,
				Loaded:      loaded,
				Limit:       limit,
				UsageHints: []string{
					"jq '.results[] | {id: .issue_id, score: .score, title: .title}' - Extract results",
					"jq '.index' - Index update stats (added/updated/removed/embedded)",
				},
			}
			out.Results = make([]resultRow, 0, len(results))
			for _, r := range results {
				out.Results = append(out.Results, resultRow{
					IssueID: r.IssueID,
					Score:   r.Score,
					Title:   titleByID[r.IssueID],
				})
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(out); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding robot-search: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Human-readable output
		if !loaded || syncStats.Changed() {
			fmt.Fprintf(os.Stderr, "Index: +%d ~%d -%d (%d total) → %s\n", syncStats.Added, syncStats.Updated, syncStats.Removed, idx.Size(), indexPath)
		}
		for _, r := range results {
			fmt.Printf("%.4f\t%s\t%s\n", r.Score, r.IssueID, titleByID[r.IssueID])
		}
		os.Exit(0)
	}

	// Handle --pages wizard (bv-10g)
	if *pagesWizard {
		if err := runPagesWizard(issues, beadsPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --preview-pages (before export since it doesn't need analysis)
	if *previewPages != "" {
		if err := runPreviewServer(*previewPages); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting preview server: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --export-pages (bv-73f)
	if *exportPages != "" {
		fmt.Println("Exporting static site...")
		fmt.Printf("  → Loading %d issues\n", len(issues))

		// Filter closed issues if not requested
		exportIssues := issues
		if !*pagesIncludeClosed {
			var openIssues []model.Issue
			for _, issue := range issues {
				if issue.Status != model.StatusClosed {
					openIssues = append(openIssues, issue)
				}
			}
			exportIssues = openIssues
			fmt.Printf("  → Filtering to %d open issues\n", len(exportIssues))
		}

		// Load and run pre-export hooks (bv-qjc.3)
		cwd, _ := os.Getwd()
		var pagesExecutor *hooks.Executor
		if !*noHooks {
			hookLoader := hooks.NewLoader(hooks.WithProjectDir(cwd))
			if err := hookLoader.Load(); err != nil {
				fmt.Printf("  → Warning: failed to load hooks: %v\n", err)
			} else if hookLoader.HasHooks() {
				fmt.Println("  → Running pre-export hooks...")
				ctx := hooks.ExportContext{
					ExportPath:   *exportPages,
					ExportFormat: "html",
					IssueCount:   len(exportIssues),
					Timestamp:    time.Now(),
				}
				pagesExecutor = hooks.NewExecutor(hookLoader.Config(), ctx)

				if err := pagesExecutor.RunPreExport(); err != nil {
					fmt.Fprintf(os.Stderr, "Error: pre-export hook failed: %v\n", err)
					os.Exit(1)
				}
			}
		}

		// Build graph and compute stats
		fmt.Println("  → Running graph analysis...")
		analyzer := analysis.NewAnalyzer(exportIssues)
		stats := analyzer.AnalyzeAsync()
		stats.WaitForPhase2()

		// Compute triage
		fmt.Println("  → Generating triage data...")
		triage := analysis.ComputeTriage(exportIssues)

		// Extract dependencies
		var deps []*model.Dependency
		for i := range exportIssues {
			issue := &exportIssues[i]
			for _, dep := range issue.Dependencies {
				if dep == nil || !dep.Type.IsBlocking() {
					continue
				}
				deps = append(deps, &model.Dependency{
					IssueID:     issue.ID,
					DependsOnID: dep.DependsOnID,
					Type:        dep.Type,
				})
			}
		}

		// Create exporter
		issuePointers := make([]*model.Issue, len(exportIssues))
		for i := range exportIssues {
			issuePointers[i] = &exportIssues[i]
		}
		exporter := export.NewSQLiteExporter(issuePointers, deps, stats, &triage)
		if *pagesTitle != "" {
			exporter.Config.Title = *pagesTitle
		}

		// Export SQLite database
		fmt.Println("  → Writing database and JSON files...")
		if err := exporter.Export(*exportPages); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
			os.Exit(1)
		}

		// Copy viewer assets
		fmt.Println("  → Copying viewer assets...")
		if err := copyViewerAssets(*exportPages, *pagesTitle); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying assets: %v\n", err)
			os.Exit(1)
		}

		// Run post-export hooks (bv-qjc.3)
		if pagesExecutor != nil {
			fmt.Println("  → Running post-export hooks...")
			if err := pagesExecutor.RunPostExport(); err != nil {
				fmt.Printf("  → Warning: post-export hook failed: %v\n", err)
				// Don't exit, just warn
			}

			// Print hook summary if any hooks ran
			if len(pagesExecutor.Results()) > 0 {
				fmt.Println("")
				fmt.Println(pagesExecutor.Summary())
			}
		}

		fmt.Println("")
		fmt.Printf("✓ Static site exported to: %s\n", *exportPages)
		fmt.Println("")
		fmt.Println("To preview locally:")
		fmt.Printf("  bv --preview-pages %s\n", *exportPages)
		fmt.Println("")
		fmt.Println("Or open in browser:")
		fmt.Printf("  open %s/index.html\n", *exportPages)
		os.Exit(0)
	}

	// Handle --robot-label-health
	if *robotLabelHealth {
		cfg := analysis.DefaultLabelHealthConfig()
		results := analysis.ComputeAllLabelHealth(issues, cfg, time.Now().UTC(), nil)

		output := struct {
			GeneratedAt    string                       `json:"generated_at"`
			DataHash       string                       `json:"data_hash"`
			AnalysisConfig analysis.LabelHealthConfig   `json:"analysis_config"`
			Results        analysis.LabelAnalysisResult `json:"results"`
			UsageHints     []string                     `json:"usage_hints"`
		}{
			GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
			DataHash:       dataHash,
			AnalysisConfig: cfg,
			Results:        results,
			UsageHints: []string{
				"jq '.results.summaries | sort_by(.health) | .[:3]' - Critical labels",
				"jq '.results.labels[] | select(.health_level == \"critical\")' - Critical details",
				"jq '.results.cross_label_flow.bottleneck_labels' - Bottleneck labels",
				"jq '.results.attention_needed' - Labels needing attention",
			},
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding label health: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-label-flow (can be used stand-alone to avoid full health computation)
	if *robotLabelFlow {
		cfg := analysis.DefaultLabelHealthConfig()
		flow := analysis.ComputeCrossLabelFlow(issues, cfg)
		output := struct {
			GeneratedAt string                     `json:"generated_at"`
			DataHash    string                     `json:"data_hash"`
			Flow        analysis.CrossLabelFlow    `json:"flow"`
			Config      analysis.LabelHealthConfig `json:"analysis_config"`
			UsageHints  []string                   `json:"usage_hints"`
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DataHash:    dataHash,
			Flow:        flow,
			Config:      cfg,
			UsageHints: []string{
				"jq '.flow.bottleneck_labels' - labels blocking the most others",
				"jq '.flow.dependencies[] | select(.issue_count > 0) | {from:.from_label,to:.to_label,count:.issue_count}'",
				"jq '.flow.flow_matrix' - raw matrix (row=from, col=to, align with .flow.labels)",
			},
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding label flow: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-label-attention (bv-121)
	if *robotLabelAttention {
		cfg := analysis.DefaultLabelHealthConfig()
		result := analysis.ComputeLabelAttentionScores(issues, cfg, time.Now().UTC())

		// Apply limit
		limit := *attentionLimit
		if limit <= 0 {
			limit = 5
		}
		if limit > len(result.Labels) {
			limit = len(result.Labels)
		}

		// Build limited output
		type AttentionOutput struct {
			GeneratedAt string `json:"generated_at"`
			DataHash    string `json:"data_hash"`
			Limit       int    `json:"limit"`
			TotalLabels int    `json:"total_labels"`
			Labels      []struct {
				Rank            int     `json:"rank"`
				Label           string  `json:"label"`
				AttentionScore  float64 `json:"attention_score"`
				NormalizedScore float64 `json:"normalized_score"`
				Reason          string  `json:"reason"`
				OpenCount       int     `json:"open_count"`
				BlockedCount    int     `json:"blocked_count"`
				StaleCount      int     `json:"stale_count"`
				PageRankSum     float64 `json:"pagerank_sum"`
				VelocityFactor  float64 `json:"velocity_factor"`
			} `json:"labels"`
			UsageHints []string `json:"usage_hints"`
		}

		output := AttentionOutput{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DataHash:    dataHash,
			Limit:       limit,
			TotalLabels: result.TotalLabels,
			UsageHints: []string{
				"jq '.labels[0]' - top attention label details",
				"jq '.labels[] | select(.blocked_count > 0)' - labels with blocked issues",
				"jq '.labels[] | {label:.label,score:.attention_score,reason:.reason}'",
			},
		}

		for i := 0; i < limit; i++ {
			score := result.Labels[i]
			// Build human-readable reason
			reason := buildAttentionReason(score)
			output.Labels = append(output.Labels, struct {
				Rank            int     `json:"rank"`
				Label           string  `json:"label"`
				AttentionScore  float64 `json:"attention_score"`
				NormalizedScore float64 `json:"normalized_score"`
				Reason          string  `json:"reason"`
				OpenCount       int     `json:"open_count"`
				BlockedCount    int     `json:"blocked_count"`
				StaleCount      int     `json:"stale_count"`
				PageRankSum     float64 `json:"pagerank_sum"`
				VelocityFactor  float64 `json:"velocity_factor"`
			}{
				Rank:            score.Rank,
				Label:           score.Label,
				AttentionScore:  score.AttentionScore,
				NormalizedScore: score.NormalizedScore,
				Reason:          reason,
				OpenCount:       score.OpenCount,
				BlockedCount:    score.BlockedCount,
				StaleCount:      score.StaleCount,
				PageRankSum:     score.PageRankSum,
				VelocityFactor:  score.VelocityFactor,
			})
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding label attention: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-graph (bv-136)
	if *robotGraph {
		analyzer := analysis.NewAnalyzer(issues)
		stats := analyzer.Analyze()

		// Determine format
		var format export.GraphExportFormat
		switch strings.ToLower(*graphFormat) {
		case "dot":
			format = export.GraphFormatDOT
		case "mermaid":
			format = export.GraphFormatMermaid
		default:
			format = export.GraphFormatJSON
		}

		config := export.GraphExportConfig{
			Format:   format,
			Label:    *labelScope,
			Root:     *graphRoot,
			Depth:    *graphDepth,
			DataHash: dataHash,
		}

		result, err := export.ExportGraph(issues, &stats, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting graph: %v\n", err)
			os.Exit(1)
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding graph: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-alerts (drift + proactive)
	if *robotAlerts {
		projectDir, _ := os.Getwd()
		driftConfig, err := drift.LoadConfig(projectDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading drift config: %v\n", err)
			os.Exit(1)
		}

		analyzer := analysis.NewAnalyzer(issues)
		stats := analyzer.Analyze()

		openCount, closedCount, blockedCount := 0, 0, 0
		for _, issue := range issues {
			switch issue.Status {
			case model.StatusClosed:
				closedCount++
			case model.StatusBlocked:
				blockedCount++
			default:
				openCount++
			}
		}
		curStats := baseline.GraphStats{
			NodeCount:       stats.NodeCount,
			EdgeCount:       stats.EdgeCount,
			Density:         stats.Density,
			OpenCount:       openCount,
			ClosedCount:     closedCount,
			BlockedCount:    blockedCount,
			CycleCount:      len(stats.Cycles()),
			ActionableCount: len(analyzer.GetActionableIssues()),
		}
		bl := &baseline.Baseline{Stats: curStats}
		cur := &baseline.Baseline{Stats: curStats, Cycles: stats.Cycles()}

		calc := drift.NewCalculator(bl, cur, driftConfig)
		calc.SetIssues(issues)
		driftResult := calc.Calculate()

		// Apply optional filters
		filtered := driftResult.Alerts[:0]
		for _, a := range driftResult.Alerts {
			if *alertSeverity != "" && string(a.Severity) != *alertSeverity {
				continue
			}
			if *alertType != "" && string(a.Type) != *alertType {
				continue
			}
			if *alertLabel != "" {
				found := false
				for _, d := range a.Details {
					if strings.Contains(strings.ToLower(d), strings.ToLower(*alertLabel)) {
						found = true
						break
					}
				}
				if !found && a.Label != "" && !strings.Contains(strings.ToLower(a.Label), strings.ToLower(*alertLabel)) {
					continue
				}
			}
			filtered = append(filtered, a)
		}
		driftResult.Alerts = filtered

		output := struct {
			GeneratedAt string        `json:"generated_at"`
			DataHash    string        `json:"data_hash"`
			Alerts      []drift.Alert `json:"alerts"`
			Summary     struct {
				Total    int `json:"total"`
				Critical int `json:"critical"`
				Warning  int `json:"warning"`
				Info     int `json:"info"`
			} `json:"summary"`
			UsageHints []string `json:"usage_hints"`
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DataHash:    dataHash,
			Alerts:      driftResult.Alerts,
			UsageHints: []string{
				"--severity=warning --alert-type=stale_issue   # stale warnings only",
				"--alert-type=blocking_cascade                 # high-unblock opportunities",
				"jq '.alerts | map(.issue_id)'                # list impacted issues",
			},
		}
		for _, a := range driftResult.Alerts {
			switch a.Severity {
			case drift.SeverityCritical:
				output.Summary.Critical++
			case drift.SeverityWarning:
				output.Summary.Warning++
			case drift.SeverityInfo:
				output.Summary.Info++
			}
			output.Summary.Total++
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding alerts: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --profile-startup
	if *profileStartup {
		runProfileStartup(issues, loadDuration, *profileJSON, *forceFullAnalysis)
		os.Exit(0)
	}

	// Handle --save-baseline
	if *saveBaseline != "" {
		analyzer := analysis.NewAnalyzer(issues)
		if *forceFullAnalysis {
			cfg := analysis.FullAnalysisConfig()
			analyzer.SetConfig(&cfg)
		}
		stats := analyzer.Analyze()

		// Compute status counts from issues
		openCount, closedCount, blockedCount := 0, 0, 0
		for _, issue := range issues {
			switch issue.Status {
			case model.StatusOpen, model.StatusInProgress:
				openCount++
			case model.StatusClosed:
				closedCount++
			case model.StatusBlocked:
				blockedCount++
			}
		}

		// Get actionable count from analyzer
		actionableCount := len(analyzer.GetActionableIssues())

		// Get cycles (method returns a copy)
		cycles := stats.Cycles()

		// Build GraphStats from analysis
		graphStats := baseline.GraphStats{
			NodeCount:       stats.NodeCount,
			EdgeCount:       stats.EdgeCount,
			Density:         stats.Density,
			OpenCount:       openCount,
			ClosedCount:     closedCount,
			BlockedCount:    blockedCount,
			CycleCount:      len(cycles),
			ActionableCount: actionableCount,
		}

		// Build TopMetrics from analysis (top 10 for each)
		// Methods return copies of the maps
		topMetrics := baseline.TopMetrics{
			PageRank:     buildMetricItems(stats.PageRank(), 10),
			Betweenness:  buildMetricItems(stats.Betweenness(), 10),
			CriticalPath: buildMetricItems(stats.CriticalPathScore(), 10),
			Hubs:         buildMetricItems(stats.Hubs(), 10),
			Authorities:  buildMetricItems(stats.Authorities(), 10),
		}

		bl := baseline.New(graphStats, topMetrics, cycles, *saveBaseline)

		if err := bl.Save(baselinePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving baseline: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Baseline saved to %s\n", baselinePath)
		fmt.Print(bl.Summary())
		os.Exit(0)
	}

	// Handle --check-drift
	if *checkDrift {
		if !baseline.Exists(baselinePath) {
			fmt.Fprintln(os.Stderr, "Error: No baseline found.")
			fmt.Fprintln(os.Stderr, "Create one with: bv --save-baseline \"description\"")
			os.Exit(1)
		}

		bl, err := baseline.Load(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
			os.Exit(1)
		}

		// Run analysis on current issues
		analyzer := analysis.NewAnalyzer(issues)
		if *forceFullAnalysis {
			cfg := analysis.FullAnalysisConfig()
			analyzer.SetConfig(&cfg)
		}
		stats := analyzer.Analyze()

		// Compute status counts from issues
		openCount, closedCount, blockedCount := 0, 0, 0
		for _, issue := range issues {
			switch issue.Status {
			case model.StatusOpen, model.StatusInProgress:
				openCount++
			case model.StatusClosed:
				closedCount++
			case model.StatusBlocked:
				blockedCount++
			}
		}
		actionableCount := len(analyzer.GetActionableIssues())
		cycles := stats.Cycles()

		// Build current snapshot as baseline for comparison
		currentStats := baseline.GraphStats{
			NodeCount:       stats.NodeCount,
			EdgeCount:       stats.EdgeCount,
			Density:         stats.Density,
			OpenCount:       openCount,
			ClosedCount:     closedCount,
			BlockedCount:    blockedCount,
			CycleCount:      len(cycles),
			ActionableCount: actionableCount,
		}
		currentMetrics := baseline.TopMetrics{
			PageRank:     buildMetricItems(stats.PageRank(), 10),
			Betweenness:  buildMetricItems(stats.Betweenness(), 10),
			CriticalPath: buildMetricItems(stats.CriticalPathScore(), 10),
			Hubs:         buildMetricItems(stats.Hubs(), 10),
			Authorities:  buildMetricItems(stats.Authorities(), 10),
		}
		current := baseline.New(currentStats, currentMetrics, cycles, "current")

		// Load drift config and run calculator
		driftConfig, err := drift.LoadConfig(projectDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error loading drift config: %v\n", err)
			driftConfig = drift.DefaultConfig()
		}

		calc := drift.NewCalculator(bl, current, driftConfig)
		result := calc.Calculate()

		if *robotDriftCheck {
			// JSON output
			output := struct {
				GeneratedAt string `json:"generated_at"`
				HasDrift    bool   `json:"has_drift"`
				ExitCode    int    `json:"exit_code"`
				Summary     struct {
					Critical int `json:"critical"`
					Warning  int `json:"warning"`
					Info     int `json:"info"`
				} `json:"summary"`
				Alerts   []drift.Alert `json:"alerts"`
				Baseline struct {
					CreatedAt string `json:"created_at"`
					CommitSHA string `json:"commit_sha,omitempty"`
				} `json:"baseline"`
			}{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				HasDrift:    result.HasDrift,
				ExitCode:    result.ExitCode(),
				Alerts:      result.Alerts,
			}
			output.Summary.Critical = result.CriticalCount
			output.Summary.Warning = result.WarningCount
			output.Summary.Info = result.InfoCount
			output.Baseline.CreatedAt = bl.CreatedAt.Format(time.RFC3339)
			output.Baseline.CommitSHA = bl.CommitSHA

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding drift result: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Human-readable output
			fmt.Print(result.Summary())
		}

		os.Exit(result.ExitCode())
	}

	if *robotInsights {
		analyzer := analysis.NewAnalyzer(issues)
		if *forceFullAnalysis {
			cfg := analysis.FullAnalysisConfig()
			analyzer.SetConfig(&cfg)
		}
		stats := analyzer.Analyze()
		// Generate top 50 lists for summary, but full stats are included in the struct
		insights := stats.GenerateInsights(50)

		// Add project-level velocity snapshot (reuse triage computation for consistency)
		if triage := analysis.ComputeTriage(issues); triage.ProjectHealth.Velocity != nil {
			v := triage.ProjectHealth.Velocity
			snap := &analysis.VelocitySnapshot{
				Closed7:   v.ClosedLast7Days,
				Closed30:  v.ClosedLast30Days,
				AvgDays:   v.AvgDaysToClose,
				Estimated: v.Estimated,
			}
			if len(v.Weekly) > 0 {
				snap.Weekly = make([]int, len(v.Weekly))
				for i := range v.Weekly {
					snap.Weekly[i] = v.Weekly[i].Closed
				}
			}
			insights.Velocity = snap
		}

		// Optional cap for metric maps to avoid overload
		limitMaps := func(m map[string]float64, limit int) map[string]float64 {
			if limit <= 0 || limit >= len(m) {
				return m
			}
			type kv struct {
				k string
				v float64
			}
			var items []kv
			for k, v := range m {
				items = append(items, kv{k, v})
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].v == items[j].v {
					return items[i].k < items[j].k
				}
				return items[i].v > items[j].v
			})
			trim := make(map[string]float64, limit)
			for i := 0; i < limit; i++ {
				trim[items[i].k] = items[i].v
			}
			return trim
		}

		limitMapInt := func(m map[string]int, limit int) map[string]int {
			if limit <= 0 || len(m) <= limit {
				return m
			}
			trim := make(map[string]int, limit)
			count := 0
			for k, v := range m {
				trim[k] = v
				count++
				if count >= limit {
					break
				}
			}
			return trim
		}

		limitSlice := func(s []string, limit int) []string {
			if limit <= 0 || len(s) <= limit {
				return s
			}
			return s[:limit]
		}

		// Default cap to keep payload small; allow override via env
		mapLimit := 200
		if v := os.Getenv("BV_INSIGHTS_MAP_LIMIT"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				mapLimit = n
			}
		}

		fullStats := struct {
			PageRank          map[string]float64 `json:"pagerank"`
			Betweenness       map[string]float64 `json:"betweenness"`
			Eigenvector       map[string]float64 `json:"eigenvector"`
			Hubs              map[string]float64 `json:"hubs"`
			Authorities       map[string]float64 `json:"authorities"`
			CriticalPathScore map[string]float64 `json:"critical_path_score"`
			CoreNumber        map[string]int     `json:"core_number"`
			Slack             map[string]float64 `json:"slack"`
			Articulation      []string           `json:"articulation_points"`
		}{
			PageRank:          limitMaps(stats.PageRank(), mapLimit),
			Betweenness:       limitMaps(stats.Betweenness(), mapLimit),
			Eigenvector:       limitMaps(stats.Eigenvector(), mapLimit),
			Hubs:              limitMaps(stats.Hubs(), mapLimit),
			Authorities:       limitMaps(stats.Authorities(), mapLimit),
			CriticalPathScore: limitMaps(stats.CriticalPathScore(), mapLimit),
			CoreNumber:        limitMapInt(stats.CoreNumber(), mapLimit),
			Slack:             limitMaps(stats.Slack(), mapLimit),
			Articulation:      limitSlice(stats.ArticulationPoints(), mapLimit),
		}

		// Get top what-if deltas for issues with highest downstream impact (bv-83)
		topWhatIfs := analyzer.TopWhatIfDeltas(10)

		// Generate advanced insights with canonical structure (bv-181)
		advancedInsights := analyzer.GenerateAdvancedInsights(analysis.DefaultAdvancedInsightsConfig())

		output := struct {
			GeneratedAt    string                  `json:"generated_at"`
			DataHash       string                  `json:"data_hash"`
			AnalysisConfig analysis.AnalysisConfig `json:"analysis_config"`
			Status         analysis.MetricStatus   `json:"status"`
			LabelScope     string                  `json:"label_scope,omitempty"`   // bv-122: Label filter applied
			LabelContext   *analysis.LabelHealth   `json:"label_context,omitempty"` // bv-122: Health context for scoped label
			analysis.Insights
			FullStats        interface{}                `json:"full_stats"`
			TopWhatIfs       []analysis.WhatIfEntry     `json:"top_what_ifs,omitempty"`      // Issues with highest downstream impact (bv-83)
			AdvancedInsights *analysis.AdvancedInsights `json:"advanced_insights,omitempty"` // bv-181: Canonical advanced features
			UsageHints       []string                   `json:"usage_hints"`                 // bv-84: Agent-friendly hints
		}{
			GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
			DataHash:         dataHash,
			AnalysisConfig:   stats.Config,
			Status:           stats.Status(),
			LabelScope:       *labelScope,
			LabelContext:     labelScopeContext,
			Insights:         insights,
			FullStats:        fullStats,
			TopWhatIfs:       topWhatIfs,
			AdvancedInsights: advancedInsights,
			UsageHints: []string{
				"jq '.Bottlenecks[:5] | map(.ID)' - Top 5 bottleneck IDs",
				"jq '.CriticalPath[:3]' - Top 3 critical path items",
				"jq '.top_what_ifs[] | select(.delta.direct_unblocks > 2)' - High-impact items",
				"jq '.full_stats.pagerank | to_entries | sort_by(-.value)[:5]' - Top PageRank",
				"jq '.full_stats.core_number | to_entries | sort_by(-.value)[:5]' - Strongly embedded nodes (k-core)",
				"jq '.full_stats.articulation_points' - Structural cut points",
				"jq '.Slack[:5]' - Nodes with slack (good parallel work candidates)",
				"jq '.Cycles | length' - Count of detected cycles",
				"jq '.advanced_insights.cycle_break' - Cycle break suggestions (bv-181)",
				"BV_INSIGHTS_MAP_LIMIT=50 bv --robot-insights - Reduce map sizes",
			},
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding insights: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *robotPlan {
		analyzer := analysis.NewAnalyzer(issues)
		// ensure config captured for output
		cfg := analysis.ConfigForSize(len(issues), countEdges(issues))
		if *forceFullAnalysis {
			cfg = analysis.FullAnalysisConfig()
		}
		analyzer.SetConfig(&cfg)
		plan := analyzer.GetExecutionPlan()
		status := analyzer.AnalyzeAsyncWithConfig(cfg).Status() // reuse config for status snapshot

		// Wrap with metadata
		output := struct {
			GeneratedAt    string                  `json:"generated_at"`
			DataHash       string                  `json:"data_hash"`
			AnalysisConfig analysis.AnalysisConfig `json:"analysis_config"`
			Status         analysis.MetricStatus   `json:"status"`
			LabelScope     string                  `json:"label_scope,omitempty"`   // bv-122: Label filter applied
			LabelContext   *analysis.LabelHealth   `json:"label_context,omitempty"` // bv-122: Health context for scoped label
			Plan           analysis.ExecutionPlan  `json:"plan"`
			UsageHints     []string                `json:"usage_hints"` // bv-84: Agent-friendly hints
		}{
			GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
			DataHash:       dataHash,
			AnalysisConfig: cfg,
			Status:         status,
			LabelScope:     *labelScope,
			LabelContext:   labelScopeContext,
			Plan:           plan,
			UsageHints: []string{
				"jq '.plan.tracks | length' - Number of parallel execution tracks",
				"jq '.plan.tracks[0].items | map(.id)' - First track item IDs",
				"jq '.plan.tracks[].items[] | select(.unblocks | length > 0)' - Items that unblock others",
				"jq '.plan.summary' - High-level execution summary",
				"jq '[.plan.tracks[].items[]] | length' - Total items across all tracks",
			},
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding execution plan: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *robotPriority {
		analyzer := analysis.NewAnalyzer(issues)
		cfg := analysis.ConfigForSize(len(issues), countEdges(issues))
		if *forceFullAnalysis {
			cfg = analysis.FullAnalysisConfig()
		}
		analyzer.SetConfig(&cfg)
		status := analyzer.AnalyzeAsyncWithConfig(cfg).Status()

		// Use enhanced recommendations with what-if deltas and top reasons (bv-83)
		recommendations := analyzer.GenerateEnhancedRecommendations()

		// Apply robot filters (bv-84)
		filtered := make([]analysis.EnhancedPriorityRecommendation, 0, len(recommendations))
		issueMap := make(map[string]model.Issue, len(issues))
		for _, iss := range issues {
			issueMap[iss.ID] = iss
		}
		for _, rec := range recommendations {
			// Filter by minimum confidence
			if *robotMinConf > 0 && rec.Confidence < *robotMinConf {
				continue
			}
			// Filter by label
			if *robotByLabel != "" {
				if iss, ok := issueMap[rec.IssueID]; ok {
					hasLabel := false
					for _, lbl := range iss.Labels {
						if lbl == *robotByLabel {
							hasLabel = true
							break
						}
					}
					if !hasLabel {
						continue
					}
				} else {
					continue
				}
			}
			// Filter by assignee
			if *robotByAssignee != "" {
				if iss, ok := issueMap[rec.IssueID]; ok {
					if iss.Assignee != *robotByAssignee {
						continue
					}
				} else {
					continue
				}
			}
			filtered = append(filtered, rec)
		}
		recommendations = filtered

		// Apply max results limit
		maxResults := 10 // Default cap
		if *robotMaxResults > 0 {
			maxResults = *robotMaxResults
		}
		if len(recommendations) > maxResults {
			recommendations = recommendations[:maxResults]
		}

		// Count high confidence recommendations
		highConfidence := 0
		for _, rec := range recommendations {
			if rec.Confidence >= 0.7 {
				highConfidence++
			}
		}

		// Build output with summary
		output := struct {
			GeneratedAt       string                                    `json:"generated_at"`
			DataHash          string                                    `json:"data_hash"`
			AnalysisConfig    analysis.AnalysisConfig                   `json:"analysis_config"`
			Status            analysis.MetricStatus                     `json:"status"`
			LabelScope        string                                    `json:"label_scope,omitempty"`   // bv-122: Label filter applied
			LabelContext      *analysis.LabelHealth                     `json:"label_context,omitempty"` // bv-122: Health context for scoped label
			Recommendations   []analysis.EnhancedPriorityRecommendation `json:"recommendations"`
			FieldDescriptions map[string]string                         `json:"field_descriptions"`
			Filters           struct {
				MinConfidence float64 `json:"min_confidence,omitempty"`
				MaxResults    int     `json:"max_results"`
				ByLabel       string  `json:"by_label,omitempty"`
				ByAssignee    string  `json:"by_assignee,omitempty"`
			} `json:"filters"`
			Summary struct {
				TotalIssues     int `json:"total_issues"`
				Recommendations int `json:"recommendations"`
				HighConfidence  int `json:"high_confidence"`
			} `json:"summary"`
			Usage []string `json:"usage_hints"` // bv-84: Agent-friendly hints
		}{
			GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
			DataHash:          dataHash,
			AnalysisConfig:    cfg,
			Status:            status,
			LabelScope:        *labelScope,
			LabelContext:      labelScopeContext,
			Recommendations:   recommendations,
			FieldDescriptions: analysis.DefaultFieldDescriptions(),
			Usage: []string{
				"jq '.recommendations[] | select(.confidence > 0.7)' - Filter high confidence",
				"jq '.recommendations[0].explanation.what_if' - Get top item's impact",
				"jq '.recommendations | map({id: .issue_id, score: .impact_score})' - Extract IDs and scores",
				"jq '.recommendations[] | select(.explanation.what_if.parallelization_gain > 0)' - Find items that increase parallel work capacity",
				"--robot-min-confidence 0.6 - Pre-filter by confidence",
				"--robot-max-results 5 - Limit to top N results",
				"--robot-by-label bug - Filter by specific label",
			},
		}
		output.Filters.MinConfidence = *robotMinConf
		output.Filters.MaxResults = maxResults
		output.Filters.ByLabel = *robotByLabel
		output.Filters.ByAssignee = *robotByAssignee
		output.Summary.TotalIssues = len(issues)
		output.Summary.Recommendations = len(recommendations)
		output.Summary.HighConfidence = highConfidence

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding priority recommendations: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *robotTriage || *robotNext || *robotTriageByTrack || *robotTriageByLabel {
		// bv-87: Support track/label-aware grouping for multi-agent coordination
		opts := analysis.TriageOptions{
			GroupByTrack: *robotTriageByTrack,
			GroupByLabel: *robotTriageByLabel,
		}
		triage := analysis.ComputeTriageWithOptions(issues, opts)

		// bv-90: Load feedback data for output
		var feedbackInfo *analysis.FeedbackJSON
		if robotTriageBeadsDir, err := loader.GetBeadsDir(""); err == nil {
			if feedbackData, err := analysis.LoadFeedback(robotTriageBeadsDir); err == nil && len(feedbackData.Events) > 0 {
				info := feedbackData.ToJSON()
				feedbackInfo = &info
			}
		}

		if *robotNext {
			// Minimal output: just the top pick
			if len(triage.QuickRef.TopPicks) == 0 {
				output := struct {
					GeneratedAt string `json:"generated_at"`
					Message     string `json:"message"`
				}{
					GeneratedAt: time.Now().UTC().Format(time.RFC3339),
					Message:     "No actionable items available",
				}
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(output); err != nil {
					fmt.Fprintf(os.Stderr, "Error encoding robot-next: %v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}

			top := triage.QuickRef.TopPicks[0]
			output := struct {
				GeneratedAt string   `json:"generated_at"`
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Score       float64  `json:"score"`
				Reasons     []string `json:"reasons"`
				Unblocks    int      `json:"unblocks"`
				ClaimCmd    string   `json:"claim_command"`
				ShowCmd     string   `json:"show_command"`
			}{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				ID:          top.ID,
				Title:       top.Title,
				Score:       top.Score,
				Reasons:     top.Reasons,
				Unblocks:    top.Unblocks,
				ClaimCmd:    fmt.Sprintf("bd update %s --status=in_progress", top.ID),
				ShowCmd:     fmt.Sprintf("bd show %s", top.ID),
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding robot-next: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}

		// Full triage output with usage hints
		output := struct {
			GeneratedAt string                 `json:"generated_at"`
			DataHash    string                 `json:"data_hash"`
			Triage      analysis.TriageResult  `json:"triage"`
			Feedback    *analysis.FeedbackJSON `json:"feedback,omitempty"` // bv-90: Feedback loop state
			UsageHints  []string               `json:"usage_hints"`        // bv-84: Agent-friendly hints
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DataHash:    dataHash,
			Triage:      triage,
			Feedback:    feedbackInfo,
			UsageHints: []string{
				"jq '.triage.quick_ref.top_picks[:3]' - Top 3 picks for immediate work",
				"jq '.triage.quick_ref.next_up' - Secondary candidates after top picks",
				"jq '.triage.blockers | map(.id)' - All blocking issue IDs",
				"jq '.triage.categories.bugs' - Bug-specific triage",
				"jq '.triage.quick_ref.top_picks[] | select(.unblocks > 2)' - High-impact picks",
				"--robot-next - Get only the single top recommendation",
				"--robot-triage-by-track - Group by execution track for multi-agent coordination",
				"--robot-triage-by-label - Group by label for area-focused agents",
				"jq '.triage.recommendations_by_track[].top_pick' - Top pick per track",
				"jq '.triage.recommendations_by_label[].claim_command' - Claim commands per label",
				"jq '.feedback.weight_adjustments' - View feedback-adjusted weights (bv-90)",
			},
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding robot-triage: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --priority-brief flag (bv-96)
	if *priorityBrief != "" {
		fmt.Printf("Generating priority brief to %s...\n", *priorityBrief)
		triage := analysis.ComputeTriage(issues)

		// Marshal triage to JSON for the export function
		triageJSON, err := json.Marshal(triage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling triage data: %v\n", err)
			os.Exit(1)
		}

		// Generate the brief
		config := export.DefaultPriorityBriefConfig()
		config.DataHash = dataHash
		brief, err := export.GeneratePriorityBriefFromTriageJSON(triageJSON, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating priority brief: %v\n", err)
			os.Exit(1)
		}

		// Write to file
		if err := os.WriteFile(*priorityBrief, []byte(brief), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing priority brief: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Done! Priority brief saved to %s\n", *priorityBrief)
		os.Exit(0)
	}

	// Handle --agent-brief flag (bv-131)
	if *agentBrief != "" {
		fmt.Printf("Generating agent brief bundle to %s/...\n", *agentBrief)

		// Create output directory
		if err := os.MkdirAll(*agentBrief, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}

		// Generate triage data
		triage := analysis.ComputeTriage(issues)
		triageJSON, err := json.MarshalIndent(triage, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling triage: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(*agentBrief, "triage.json"), triageJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing triage.json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  → triage.json")

		// Generate insights
		analyzer := analysis.NewAnalyzer(issues)
		stats := analyzer.Analyze()
		insights := stats.GenerateInsights(50)
		insightsJSON, err := json.MarshalIndent(insights, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling insights: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(*agentBrief, "insights.json"), insightsJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing insights.json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  → insights.json")

		// Generate priority brief
		config := export.DefaultPriorityBriefConfig()
		config.DataHash = dataHash
		brief, err := export.GeneratePriorityBriefFromTriageJSON(triageJSON, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating brief: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(*agentBrief, "brief.md"), []byte(brief), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing brief.md: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  → brief.md")

		// Generate jq helpers
		helpers := generateJQHelpers()
		if err := os.WriteFile(filepath.Join(*agentBrief, "helpers.md"), []byte(helpers), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing helpers.md: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  → helpers.md")

		// Generate meta.json with hash and config
		meta := struct {
			GeneratedAt string   `json:"generated_at"`
			DataHash    string   `json:"data_hash"`
			IssueCount  int      `json:"issue_count"`
			Version     string   `json:"version"`
			Files       []string `json:"files"`
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DataHash:    dataHash,
			IssueCount:  len(issues),
			Version:     "1.0.0",
			Files:       []string{"triage.json", "insights.json", "brief.md", "helpers.md", "meta.json"},
		}
		metaJSON, _ := json.MarshalIndent(meta, "", "  ")
		if err := os.WriteFile(filepath.Join(*agentBrief, "meta.json"), metaJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing meta.json: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("  → meta.json")

		fmt.Printf("\nDone! Agent brief bundle saved to %s/\n", *agentBrief)
		os.Exit(0)
	}

	// Handle --emit-script flag (bv-89)
	if *emitScript {
		triage := analysis.ComputeTriage(issues)

		// Determine script limit
		limit := *scriptLimit
		if limit <= 0 {
			limit = 5
		}

		// Collect top recommendations
		recs := triage.Recommendations
		if len(recs) > limit {
			recs = recs[:limit]
		}

		// Build script header with hash/config
		var sb strings.Builder
		switch *scriptFormat {
		case "fish":
			sb.WriteString("#!/usr/bin/env fish\n")
		case "zsh":
			sb.WriteString("#!/usr/bin/env zsh\n")
		default:
			sb.WriteString("#!/usr/bin/env bash\n")
			sb.WriteString("set -euo pipefail\n")
		}

		sb.WriteString(fmt.Sprintf("# Generated by bv --emit-script at %s\n", time.Now().UTC().Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("# Data hash: %s\n", dataHash))
		sb.WriteString(fmt.Sprintf("# Top %d recommendations from %d actionable items\n", len(recs), len(triage.Recommendations)))
		sb.WriteString("#\n")
		sb.WriteString("# Usage: source this script or run it directly\n")
		sb.WriteString("# Each command will claim and show the recommended issue\n")
		sb.WriteString("#\n\n")

		if len(recs) == 0 {
			sb.WriteString("echo 'No actionable recommendations available'\n")
			sb.WriteString("exit 0\n")
		} else {
			// Generate commands for each recommendation
			for i, rec := range recs {
				sb.WriteString(fmt.Sprintf("# %d. %s (score: %.3f)\n", i+1, rec.Title, rec.Score))
				if len(rec.Reasons) > 0 {
					sb.WriteString(fmt.Sprintf("#    Reason: %s\n", rec.Reasons[0]))
				}
				if len(rec.UnblocksIDs) > 0 {
					sb.WriteString(fmt.Sprintf("#    Unblocks: %d downstream items\n", len(rec.UnblocksIDs)))
				}

				// Claim command
				sb.WriteString(fmt.Sprintf("# To claim: bd update %s --status=in_progress\n", rec.ID))
				// Show command
				sb.WriteString(fmt.Sprintf("bd show %s\n", rec.ID))
				sb.WriteString("\n")
			}

			// Add summary section
			sb.WriteString("# === Quick Actions ===\n")
			sb.WriteString("# To claim the top pick:\n")
			if len(recs) > 0 {
				sb.WriteString(fmt.Sprintf("# bd update %s --status=in_progress\n", recs[0].ID))
			}
			sb.WriteString("#\n")
			sb.WriteString("# To claim all listed items (uncomment to enable):\n")
			for _, rec := range recs {
				sb.WriteString(fmt.Sprintf("# bd update %s --status=in_progress\n", rec.ID))
			}
		}

		fmt.Print(sb.String())
		os.Exit(0)
	}

	// Handle --robot-history flag
	if *robotHistory || *beadHistory != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		// Validate repository
		if err := correlation.ValidateRepository(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Resolve beads file path (bv-history fix, respects BEADS_DIR)
		beadsDir, err := loader.GetBeadsDir("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting beads directory: %v\n", err)
			os.Exit(1)
		}
		beadsPath, err := loader.FindJSONLPath(beadsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding beads file: %v\n", err)
			os.Exit(1)
		}

		// Build correlator options
		opts := correlation.CorrelatorOptions{
			BeadID: *beadHistory,
			Limit:  *historyLimit,
		}

		// Parse --history-since if provided
		if *historySince != "" {
			since, err := recipe.ParseRelativeTime(*historySince, time.Now())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing --history-since: %v\n", err)
				os.Exit(1)
			}
			if !since.IsZero() {
				opts.Since = &since
			}
		}

		// Convert issues to BeadInfo for correlator
		beadInfos := make([]correlation.BeadInfo, len(issues))
		for i, issue := range issues {
			beadInfos[i] = correlation.BeadInfo{
				ID:     issue.ID,
				Title:  issue.Title,
				Status: string(issue.Status),
			}
		}

		// Generate report with explicit beads path
		correlator := correlation.NewCorrelator(cwd, beadsPath)
		report, err := correlator.GenerateReport(beadInfos, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating history report: %v\n", err)
			os.Exit(1)
		}

		// Apply confidence filter if specified
		if *minConfidence > 0 {
			scorer := correlation.NewScorer()
			report.Histories = scorer.FilterHistoriesByConfidence(report.Histories, *minConfidence)

			// Rebuild commit index after filtering
			report.CommitIndex = make(correlation.CommitIndex)
			for beadID, history := range report.Histories {
				for _, commit := range history.Commits {
					report.CommitIndex[commit.SHA] = append(report.CommitIndex[commit.SHA], beadID)
				}
			}

			// Update stats
			report.Stats.BeadsWithCommits = 0
			for _, history := range report.Histories {
				if len(history.Commits) > 0 {
					report.Stats.BeadsWithCommits++
				}
			}
		}

		// Output JSON
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding history report: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-sprint-list and --robot-sprint-show flags (bv-156)
	if *robotSprintList || *robotSprintShow != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		sprints, err := loader.LoadSprints(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading sprints: %v\n", err)
			os.Exit(1)
		}

		if *robotSprintShow != "" {
			// Find specific sprint
			var found *model.Sprint
			for i := range sprints {
				if sprints[i].ID == *robotSprintShow {
					found = &sprints[i]
					break
				}
			}
			if found == nil {
				fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", *robotSprintShow)
				os.Exit(1)
			}
			// Output single sprint as JSON
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(found); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding sprint: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Output all sprints as JSON
			output := struct {
				GeneratedAt time.Time      `json:"generated_at"`
				SprintCount int            `json:"sprint_count"`
				Sprints     []model.Sprint `json:"sprints"`
			}{
				GeneratedAt: time.Now().UTC(),
				SprintCount: len(sprints),
				Sprints:     sprints,
			}
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding sprints: %v\n", err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	}

	// Handle --robot-burndown flag (bv-159)
	if *robotBurndown != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		sprints, err := loader.LoadSprints(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading sprints: %v\n", err)
			os.Exit(1)
		}

		// Find the target sprint
		var targetSprint *model.Sprint
		if *robotBurndown == "current" {
			// Find active sprint
			for i := range sprints {
				if sprints[i].IsActive() {
					targetSprint = &sprints[i]
					break
				}
			}
			if targetSprint == nil {
				fmt.Fprintf(os.Stderr, "No active sprint found\n")
				os.Exit(1)
			}
		} else {
			// Find sprint by ID
			for i := range sprints {
				if sprints[i].ID == *robotBurndown {
					targetSprint = &sprints[i]
					break
				}
			}
			if targetSprint == nil {
				fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", *robotBurndown)
				os.Exit(1)
			}
		}

		// Build burndown data
		burndown := calculateBurndown(targetSprint, issues)

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(burndown); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding burndown: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-forecast flag (bv-158)
	if *robotForecast != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		// Build graph stats for depth calculation
		analyzer := analysis.NewAnalyzer(issues)
		graphStats := analyzer.Analyze()

		// Filter issues by label and sprint if specified
		targetIssues := make([]model.Issue, 0, len(issues))
		var sprintBeadIDs map[string]bool
		if *forecastSprint != "" {
			sprints, err := loader.LoadSprints(cwd)
			if err == nil {
				for _, s := range sprints {
					if s.ID == *forecastSprint {
						sprintBeadIDs = make(map[string]bool)
						for _, bid := range s.BeadIDs {
							sprintBeadIDs[bid] = true
						}
						break
					}
				}
			}
			if sprintBeadIDs == nil {
				fmt.Fprintf(os.Stderr, "Sprint not found: %s\n", *forecastSprint)
				os.Exit(1)
			}
		}

		for _, iss := range issues {
			// Filter by label
			if *forecastLabel != "" {
				hasLabel := false
				for _, l := range iss.Labels {
					if l == *forecastLabel {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					continue
				}
			}
			// Filter by sprint
			if sprintBeadIDs != nil && !sprintBeadIDs[iss.ID] {
				continue
			}
			targetIssues = append(targetIssues, iss)
		}

		now := time.Now()
		agents := *forecastAgents
		if agents <= 0 {
			agents = 1
		}

		type ForecastSummary struct {
			TotalMinutes  int       `json:"total_minutes"`
			TotalDays     float64   `json:"total_days"`
			AvgConfidence float64   `json:"avg_confidence"`
			EarliestETA   time.Time `json:"earliest_eta"`
			LatestETA     time.Time `json:"latest_eta"`
		}
		type ForecastOutput struct {
			GeneratedAt   time.Time              `json:"generated_at"`
			Agents        int                    `json:"agents"`
			Filters       map[string]string      `json:"filters,omitempty"`
			ForecastCount int                    `json:"forecast_count"`
			Forecasts     []analysis.ETAEstimate `json:"forecasts"`
			Summary       *ForecastSummary       `json:"summary,omitempty"`
		}

		var forecasts []analysis.ETAEstimate
		var outputErr error

		if *robotForecast == "all" {
			// Forecast all open issues
			for _, iss := range targetIssues {
				if iss.Status == model.StatusClosed {
					continue
				}
				eta, err := analysis.EstimateETAForIssue(issues, &graphStats, iss.ID, agents, now)
				if err != nil {
					continue
				}
				forecasts = append(forecasts, eta)
			}
		} else {
			// Single issue forecast
			eta, err := analysis.EstimateETAForIssue(issues, &graphStats, *robotForecast, agents, now)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			forecasts = append(forecasts, eta)
		}

		// Build summary if multiple forecasts
		var summary *ForecastSummary
		if len(forecasts) > 1 {
			totalMin := 0
			totalConf := 0.0
			earliest := forecasts[0].ETADate
			latest := forecasts[0].ETADate
			for _, f := range forecasts {
				totalMin += f.EstimatedMinutes
				totalConf += f.Confidence
				if f.ETADate.Before(earliest) {
					earliest = f.ETADate
				}
				if f.ETADate.After(latest) {
					latest = f.ETADate
				}
			}
			summary = &ForecastSummary{
				TotalMinutes:  totalMin,
				TotalDays:     float64(totalMin) / (60.0 * 8.0), // 8hr workday
				AvgConfidence: totalConf / float64(len(forecasts)),
				EarliestETA:   earliest,
				LatestETA:     latest,
			}
		}

		// Build output
		filters := make(map[string]string)
		if *forecastLabel != "" {
			filters["label"] = *forecastLabel
		}
		if *forecastSprint != "" {
			filters["sprint"] = *forecastSprint
		}

		output := ForecastOutput{
			GeneratedAt:   now.UTC(),
			Agents:        agents,
			ForecastCount: len(forecasts),
			Forecasts:     forecasts,
			Summary:       summary,
		}
		if len(filters) > 0 {
			output.Filters = filters
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if outputErr = encoder.Encode(output); outputErr != nil {
			fmt.Fprintf(os.Stderr, "Error encoding forecast: %v\n", outputErr)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --robot-capacity flag (bv-160)
	if *robotCapacity {
		// Build graph stats for analysis
		analyzer := analysis.NewAnalyzer(issues)
		graphStats := analyzer.Analyze()

		// Filter issues by label if specified
		targetIssues := issues
		if *capacityLabel != "" {
			filtered := make([]model.Issue, 0)
			for _, iss := range issues {
				for _, l := range iss.Labels {
					if l == *capacityLabel {
						filtered = append(filtered, iss)
						break
					}
				}
			}
			targetIssues = filtered
		}

		// Calculate open issues only
		openIssues := make([]model.Issue, 0)
		issueMap := make(map[string]model.Issue)
		for _, iss := range targetIssues {
			issueMap[iss.ID] = iss
			if iss.Status != model.StatusClosed {
				openIssues = append(openIssues, iss)
			}
		}

		now := time.Now()
		agents := *capacityAgents
		if agents <= 0 {
			agents = 1
		}

		// Calculate total work remaining
		medianMinutes := 60 // default
		totalMinutes := 0
		for _, iss := range openIssues {
			eta, err := analysis.EstimateETAForIssue(targetIssues, &graphStats, iss.ID, 1, now)
			if err == nil {
				totalMinutes += eta.EstimatedMinutes
			}
		}

		// Analyze parallelizability by finding dependency chains
		// Serial work = longest chain (critical path)
		// Parallelizable = work that can run concurrently

		// Build dependency adjacency for open issues
		blockedBy := make(map[string][]string) // issue -> its blockers
		blocks := make(map[string][]string)    // issue -> issues it blocks
		for _, iss := range openIssues {
			for _, dep := range iss.Dependencies {
				if dep == nil {
					continue
				}
				depID := dep.DependsOnID
				if _, exists := issueMap[depID]; exists {
					blockedBy[iss.ID] = append(blockedBy[iss.ID], depID)
					blocks[depID] = append(blocks[depID], iss.ID)
				}
			}
		}

		// Find issues with no blockers (can start immediately)
		actionable := make([]string, 0)
		for _, iss := range openIssues {
			hasOpenBlocker := false
			for _, depID := range blockedBy[iss.ID] {
				if dep, ok := issueMap[depID]; ok && dep.Status != model.StatusClosed {
					hasOpenBlocker = true
					break
				}
			}
			if !hasOpenBlocker {
				actionable = append(actionable, iss.ID)
			}
		}

		// Calculate critical path (longest chain)
		var longestChain []string
		var dfs func(id string, path []string)
		visited := make(map[string]bool)
		dfs = func(id string, path []string) {
			if visited[id] {
				return
			}
			visited[id] = true
			path = append(path, id)
			if len(path) > len(longestChain) {
				longestChain = make([]string, len(path))
				copy(longestChain, path)
			}
			for _, nextID := range blocks[id] {
				if dep, ok := issueMap[nextID]; ok && dep.Status != model.StatusClosed {
					dfs(nextID, path)
				}
			}
			visited[id] = false
		}
		for _, startID := range actionable {
			dfs(startID, nil)
		}

		// Calculate serial minutes (work on critical path)
		serialMinutes := 0
		for _, id := range longestChain {
			eta, err := analysis.EstimateETAForIssue(targetIssues, &graphStats, id, 1, now)
			if err == nil {
				serialMinutes += eta.EstimatedMinutes
			}
		}

		// Parallelizable percentage
		parallelizablePct := 0.0
		if totalMinutes > 0 {
			parallelizablePct = float64(totalMinutes-serialMinutes) / float64(totalMinutes) * 100
		}

		// Calculate estimated completion with N agents
		// Serial work must be done sequentially, parallel work can be divided
		parallelMinutes := totalMinutes - serialMinutes
		effectiveMinutes := serialMinutes + parallelMinutes/agents
		estimatedDays := float64(effectiveMinutes) / (60.0 * 8.0) // 8hr workday

		// Find bottlenecks (issues blocking the most other issues)
		type Bottleneck struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			BlocksCount int      `json:"blocks_count"`
			Blocks      []string `json:"blocks,omitempty"`
		}
		bottlenecks := make([]Bottleneck, 0)
		for _, iss := range openIssues {
			if len(blocks[iss.ID]) > 1 {
				blockedIssues := blocks[iss.ID]
				bottlenecks = append(bottlenecks, Bottleneck{
					ID:          iss.ID,
					Title:       iss.Title,
					BlocksCount: len(blockedIssues),
					Blocks:      blockedIssues,
				})
			}
		}
		// Sort by blocks count descending
		sort.Slice(bottlenecks, func(i, j int) bool {
			return bottlenecks[i].BlocksCount > bottlenecks[j].BlocksCount
		})
		if len(bottlenecks) > 5 {
			bottlenecks = bottlenecks[:5]
		}

		// Build output
		type CapacityOutput struct {
			GeneratedAt       time.Time    `json:"generated_at"`
			Agents            int          `json:"agents"`
			Label             string       `json:"label,omitempty"`
			OpenIssueCount    int          `json:"open_issue_count"`
			TotalMinutes      int          `json:"total_minutes"`
			TotalDays         float64      `json:"total_days"`
			SerialMinutes     int          `json:"serial_minutes"`
			ParallelMinutes   int          `json:"parallel_minutes"`
			ParallelizablePct float64      `json:"parallelizable_pct"`
			EstimatedDays     float64      `json:"estimated_days"`
			CriticalPathLen   int          `json:"critical_path_length"`
			CriticalPath      []string     `json:"critical_path,omitempty"`
			ActionableCount   int          `json:"actionable_count"`
			Actionable        []string     `json:"actionable,omitempty"`
			Bottlenecks       []Bottleneck `json:"bottlenecks,omitempty"`
		}

		output := CapacityOutput{
			GeneratedAt:       now.UTC(),
			Agents:            agents,
			OpenIssueCount:    len(openIssues),
			TotalMinutes:      totalMinutes,
			TotalDays:         float64(totalMinutes) / (60.0 * 8.0),
			SerialMinutes:     serialMinutes,
			ParallelMinutes:   parallelMinutes,
			ParallelizablePct: parallelizablePct,
			EstimatedDays:     estimatedDays,
			CriticalPathLen:   len(longestChain),
			CriticalPath:      longestChain,
			ActionableCount:   len(actionable),
			Actionable:        actionable,
			Bottlenecks:       bottlenecks,
		}
		if *capacityLabel != "" {
			output.Label = *capacityLabel
		}

		// Suppress unused variable warning
		_ = medianMinutes

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding capacity: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle --diff-since flag
	if *diffSince != "" {
		// Auto-enable robot diff for non-interactive/agent contexts
		if !*robotDiff && (envRobot || !stdoutIsTTY) {
			*robotDiff = true
			if stdoutIsTTY {
				fmt.Fprintln(os.Stderr, "Auto-enabled --robot-diff for non-interactive output; pass --robot-diff explicitly to control format.")
			}
		}

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		gitLoader := loader.NewGitLoader(cwd)

		// Load historical issues
		historicalIssues, err := gitLoader.LoadAt(*diffSince)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues at %s: %v\n", *diffSince, err)
			os.Exit(1)
		}

		// Get revision info for timestamp
		revision, err := gitLoader.ResolveRevision(*diffSince)
		if err != nil {
			revision = *diffSince
		}

		// Create snapshots
		fromSnapshot := analysis.NewSnapshotAt(historicalIssues, time.Time{}, revision)
		toSnapshot := analysis.NewSnapshot(issues)

		// Compute diff
		diff := analysis.CompareSnapshots(fromSnapshot, toSnapshot)

		if *robotDiff {
			// JSON output
			output := struct {
				GeneratedAt      string                 `json:"generated_at"`
				ResolvedRevision string                 `json:"resolved_revision"`
				FromDataHash     string                 `json:"from_data_hash"`
				ToDataHash       string                 `json:"to_data_hash"`
				Diff             *analysis.SnapshotDiff `json:"diff"`
			}{
				GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
				ResolvedRevision: revision,
				FromDataHash:     analysis.ComputeDataHash(historicalIssues),
				ToDataHash:       dataHash,
				Diff:             diff,
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding diff: %v\n", err)
				os.Exit(1)
			}
		} else {
			// Human-readable output
			printDiffSummary(diff, *diffSince)
		}
		os.Exit(0)
	}

	// Handle --as-of flag
	if *asOf != "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		gitLoader := loader.NewGitLoader(cwd)

		// Load historical issues
		historicalIssues, err := gitLoader.LoadAt(*asOf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading issues at %s: %v\n", *asOf, err)
			os.Exit(1)
		}

		if len(historicalIssues) == 0 {
			fmt.Printf("No issues found at %s.\n", *asOf)
			os.Exit(0)
		}

		// Launch TUI with historical issues (no live reload for historical view)
		m := ui.NewModel(historicalIssues, activeRecipe, "")
		p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

		// Optional auto-quit for automated tests: set BV_TUI_AUTOCLOSE_MS
		if v := os.Getenv("BV_TUI_AUTOCLOSE_MS"); v != "" {
			if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
				go func() {
					delay := time.Duration(ms) * time.Millisecond
					time.Sleep(delay)
					p.Send(tea.Quit)
					// Failsafe: hard exit soon after to avoid hanging tests
					time.Sleep(2 * time.Second)
					os.Exit(0)
				}()
			}
		}
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running beads viewer: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *exportFile != "" {
		fmt.Printf("Exporting to %s...\n", *exportFile)

		// Load and run pre-export hooks
		cwd, _ := os.Getwd()
		var executor *hooks.Executor
		if !*noHooks {
			hookLoader := hooks.NewLoader(hooks.WithProjectDir(cwd))
			if err := hookLoader.Load(); err != nil {
				fmt.Printf("Warning: failed to load hooks: %v\n", err)
			} else if hookLoader.HasHooks() {
				ctx := hooks.ExportContext{
					ExportPath:   *exportFile,
					ExportFormat: "markdown",
					IssueCount:   len(issues),
					Timestamp:    time.Now(),
				}
				executor = hooks.NewExecutor(hookLoader.Config(), ctx)

				// Run pre-export hooks
				if err := executor.RunPreExport(); err != nil {
					fmt.Printf("Error: pre-export hook failed: %v\n", err)
					os.Exit(1)
				}
			}
		}

		// Perform the export
		if err := export.SaveMarkdownToFile(issues, *exportFile); err != nil {
			fmt.Printf("Error exporting: %v\n", err)
			os.Exit(1)
		}

		// Run post-export hooks
		if executor != nil {
			if err := executor.RunPostExport(); err != nil {
				fmt.Printf("Warning: post-export hook failed: %v\n", err)
				// Don't exit, just warn
			}

			// Print hook summary if any hooks ran
			if len(executor.Results()) > 0 {
				fmt.Println(executor.Summary())
			}
		}

		fmt.Println("Done!")
		os.Exit(0)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found. Create some with 'bd create'!")
		os.Exit(0)
	}

	// Apply recipe filters and sorting if specified
	if activeRecipe != nil {
		issues = applyRecipeFilters(issues, activeRecipe)
		issues = applyRecipeSort(issues, activeRecipe)
	}

	// Initial Model with live reload support
	m := ui.NewModel(issues, activeRecipe, beadsPath)
	defer m.Stop() // Clean up file watcher

	// Enable workspace mode if loading from workspace config
	if workspaceInfo != nil {
		m.EnableWorkspaceMode(ui.WorkspaceInfo{
			Enabled:      true,
			RepoCount:    workspaceInfo.TotalRepos,
			FailedCount:  workspaceInfo.FailedRepos,
			TotalIssues:  workspaceInfo.TotalIssues,
			RepoPrefixes: workspaceInfo.RepoPrefixes,
		})
	}

	// Run Program
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Optional auto-quit for automated tests: set BV_TUI_AUTOCLOSE_MS
	if v := os.Getenv("BV_TUI_AUTOCLOSE_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			go func() {
				delay := time.Duration(ms) * time.Millisecond
				time.Sleep(delay)
				p.Send(tea.Quit)
				// Failsafe: hard exit soon after to avoid hanging tests
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}()
		}
	}
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running beads viewer: %v\n", err)
		os.Exit(1)
	}
}

// countEdges counts blocking dependencies for config sizing
func countEdges(issues []model.Issue) int {
	count := 0
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type == model.DepBlocks {
				count++
			}
		}
	}
	return count
}

// parseTimeRef parses common date/time formats used for history flags.
func parseTimeRef(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		switch layout {
		case time.RFC3339:
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			}
		default:
			if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time reference %q", s)
}

// printDiffSummary prints a human-readable diff summary
func printDiffSummary(diff *analysis.SnapshotDiff, since string) {
	fmt.Printf("Changes since %s\n", since)
	fmt.Println("=" + repeatChar('=', len("Changes since "+since)))
	fmt.Println()

	// Health trend
	trendEmoji := "→"
	switch diff.Summary.HealthTrend {
	case "improving":
		trendEmoji = "↑"
	case "degrading":
		trendEmoji = "↓"
	}
	fmt.Printf("Health Trend: %s %s\n\n", trendEmoji, diff.Summary.HealthTrend)

	// Summary counts
	fmt.Println("Summary:")
	if diff.Summary.IssuesAdded > 0 {
		fmt.Printf("  + %d new issues\n", diff.Summary.IssuesAdded)
	}
	if diff.Summary.IssuesClosed > 0 {
		fmt.Printf("  ✓ %d issues closed\n", diff.Summary.IssuesClosed)
	}
	if diff.Summary.IssuesRemoved > 0 {
		fmt.Printf("  - %d issues removed\n", diff.Summary.IssuesRemoved)
	}
	if diff.Summary.IssuesReopened > 0 {
		fmt.Printf("  ↺ %d issues reopened\n", diff.Summary.IssuesReopened)
	}
	if diff.Summary.IssuesModified > 0 {
		fmt.Printf("  ~ %d issues modified\n", diff.Summary.IssuesModified)
	}
	if diff.Summary.CyclesIntroduced > 0 {
		fmt.Printf("  ⚠ %d new cycles introduced\n", diff.Summary.CyclesIntroduced)
	}
	if diff.Summary.CyclesResolved > 0 {
		fmt.Printf("  ✓ %d cycles resolved\n", diff.Summary.CyclesResolved)
	}
	fmt.Println()

	// New issues
	if len(diff.NewIssues) > 0 {
		fmt.Println("New Issues:")
		for _, issue := range diff.NewIssues {
			fmt.Printf("  + [%s] %s (P%d)\n", issue.ID, issue.Title, issue.Priority)
		}
		fmt.Println()
	}

	// Closed issues
	if len(diff.ClosedIssues) > 0 {
		fmt.Println("Closed Issues:")
		for _, issue := range diff.ClosedIssues {
			fmt.Printf("  ✓ [%s] %s\n", issue.ID, issue.Title)
		}
		fmt.Println()
	}

	// Reopened issues
	if len(diff.ReopenedIssues) > 0 {
		fmt.Println("Reopened Issues:")
		for _, issue := range diff.ReopenedIssues {
			fmt.Printf("  ↺ [%s] %s\n", issue.ID, issue.Title)
		}
		fmt.Println()
	}

	// Modified issues (show first 10)
	if len(diff.ModifiedIssues) > 0 {
		fmt.Println("Modified Issues:")
		shown := 0
		for _, mod := range diff.ModifiedIssues {
			if shown >= 10 {
				fmt.Printf("  ... and %d more\n", len(diff.ModifiedIssues)-10)
				break
			}
			fmt.Printf("  ~ [%s] %s\n", mod.IssueID, mod.Title)
			for _, change := range mod.Changes {
				fmt.Printf("      %s: %s → %s\n", change.Field, change.OldValue, change.NewValue)
			}
			shown++
		}
		fmt.Println()
	}

	// New cycles
	if len(diff.NewCycles) > 0 {
		fmt.Println("⚠ New Circular Dependencies:")
		for _, cycle := range diff.NewCycles {
			fmt.Printf("  %s\n", formatCycle(cycle))
		}
		fmt.Println()
	}

	// Metric deltas
	fmt.Println("Metric Changes:")
	if diff.MetricDeltas.TotalIssues != 0 {
		fmt.Printf("  Total issues: %+d\n", diff.MetricDeltas.TotalIssues)
	}
	if diff.MetricDeltas.OpenIssues != 0 {
		fmt.Printf("  Open issues: %+d\n", diff.MetricDeltas.OpenIssues)
	}
	if diff.MetricDeltas.BlockedIssues != 0 {
		fmt.Printf("  Blocked issues: %+d\n", diff.MetricDeltas.BlockedIssues)
	}
	if diff.MetricDeltas.CycleCount != 0 {
		fmt.Printf("  Cycles: %+d\n", diff.MetricDeltas.CycleCount)
	}
}

// repeatChar creates a string of n repeated characters
func repeatChar(c rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

// formatCycle formats a cycle for display
func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return "(empty)"
	}
	result := cycle[0]
	for i := 1; i < len(cycle); i++ {
		result += " → " + cycle[i]
	}
	result += " → " + cycle[0]
	return result
}

// naturalLess compares two strings using natural sort order (numeric parts sorted numerically)
func naturalLess(s1, s2 string) bool {
	// Simple heuristic: if both strings end with numbers, compare the prefix then the number
	// e.g. "bv-2" vs "bv-10" -> "bv-" == "bv-", 2 < 10

	// Helper to split into prefix and numeric suffix
	split := func(s string) (string, int, bool) {
		lastDigit := -1
		for i := len(s) - 1; i >= 0; i-- {
			if s[i] >= '0' && s[i] <= '9' {
				lastDigit = i
			} else {
				break
			}
		}
		if lastDigit == -1 {
			return s, 0, false
		}
		// If the whole string is number, prefix is empty
		prefix := s[:lastDigit]
		numStr := s[lastDigit:]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return s, 0, false
		}
		return prefix, num, true
	}

	p1, n1, ok1 := split(s1)
	p2, n2, ok2 := split(s2)

	if ok1 && ok2 && p1 == p2 {
		return n1 < n2
	}

	return s1 < s2
}

// applyRecipeFilters filters issues based on recipe configuration
func applyRecipeFilters(issues []model.Issue, r *recipe.Recipe) []model.Issue {
	if r == nil {
		return issues
	}

	f := r.Filters
	now := time.Now()

	// Build a set of open blocker IDs for actionable filtering
	openBlockers := make(map[string]bool)
	for _, issue := range issues {
		if issue.Status != model.StatusClosed {
			openBlockers[issue.ID] = true
		}
	}

	var result []model.Issue
	for _, issue := range issues {
		// Status filter
		if len(f.Status) > 0 {
			match := false
			for _, s := range f.Status {
				if strings.EqualFold(string(issue.Status), s) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Priority filter
		if len(f.Priority) > 0 {
			match := false
			for _, p := range f.Priority {
				if issue.Priority == p {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		// Tags filter (must have all)
		if len(f.Tags) > 0 {
			match := true
			for _, tag := range f.Tags {
				found := false
				for _, label := range issue.Labels {
					if strings.EqualFold(label, tag) {
						found = true
						break
					}
				}
				if !found {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}

		// ExcludeTags filter
		if len(f.ExcludeTags) > 0 {
			excluded := false
			for _, excludeTag := range f.ExcludeTags {
				for _, label := range issue.Labels {
					if strings.EqualFold(label, excludeTag) {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			if excluded {
				continue
			}
		}

		// CreatedAfter filter
		if f.CreatedAfter != "" {
			threshold, err := recipe.ParseRelativeTime(f.CreatedAfter, now)
			if err == nil && !issue.CreatedAt.IsZero() && issue.CreatedAt.Before(threshold) {
				continue
			}
		}

		// CreatedBefore filter
		if f.CreatedBefore != "" {
			threshold, err := recipe.ParseRelativeTime(f.CreatedBefore, now)
			if err == nil && !issue.CreatedAt.IsZero() && issue.CreatedAt.After(threshold) {
				continue
			}
		}

		// UpdatedAfter filter
		if f.UpdatedAfter != "" {
			threshold, err := recipe.ParseRelativeTime(f.UpdatedAfter, now)
			if err == nil && !issue.UpdatedAt.IsZero() && issue.UpdatedAt.Before(threshold) {
				continue
			}
		}

		// UpdatedBefore filter
		if f.UpdatedBefore != "" {
			threshold, err := recipe.ParseRelativeTime(f.UpdatedBefore, now)
			if err == nil && !issue.UpdatedAt.IsZero() && issue.UpdatedAt.After(threshold) {
				continue
			}
		}

		// HasBlockers filter
		if f.HasBlockers != nil {
			hasOpenBlockers := false
			for _, dep := range issue.Dependencies {
				if dep.Type == model.DepBlocks && openBlockers[dep.DependsOnID] {
					hasOpenBlockers = true
					break
				}
			}
			if *f.HasBlockers != hasOpenBlockers {
				continue
			}
		}

		// Actionable filter (no open blockers)
		if f.Actionable != nil && *f.Actionable {
			hasOpenBlockers := false
			for _, dep := range issue.Dependencies {
				if dep.Type == model.DepBlocks && openBlockers[dep.DependsOnID] {
					hasOpenBlockers = true
					break
				}
			}
			if hasOpenBlockers {
				continue
			}
		}

		// TitleContains filter
		if f.TitleContains != "" {
			if !strings.Contains(strings.ToLower(issue.Title), strings.ToLower(f.TitleContains)) {
				continue
			}
		}

		// IDPrefix filter
		if f.IDPrefix != "" {
			if !strings.HasPrefix(issue.ID, f.IDPrefix) {
				continue
			}
		}

		result = append(result, issue)
	}

	return result
}

// applyRecipeSort sorts issues based on recipe configuration
func applyRecipeSort(issues []model.Issue, r *recipe.Recipe) []model.Issue {
	if r == nil || r.Sort.Field == "" {
		return issues
	}

	s := r.Sort
	ascending := s.Direction != "desc"

	// For priority, default to ascending (P0 first)
	if s.Field == "priority" && s.Direction == "" {
		ascending = true
	}
	// For dates, default to descending (newest first)
	if (s.Field == "created" || s.Field == "updated") && s.Direction == "" {
		ascending = false
	}

	sort.SliceStable(issues, func(i, j int) bool {
		var less bool

		switch s.Field {
		case "priority":
			less = issues[i].Priority < issues[j].Priority
		case "created":
			less = issues[i].CreatedAt.Before(issues[j].CreatedAt)
		case "updated":
			less = issues[i].UpdatedAt.Before(issues[j].UpdatedAt)
		case "title":
			less = strings.ToLower(issues[i].Title) < strings.ToLower(issues[j].Title)
		case "id":
			less = naturalLess(issues[i].ID, issues[j].ID)
		case "status":
			less = issues[i].Status < issues[j].Status
		default:
			// Unknown sort field, maintain order
			return false
		}

		if ascending {
			return less
		}
		return !less
	})

	return issues
}

// runProfileStartup runs profiled startup analysis and outputs results
func runProfileStartup(issues []model.Issue, loadDuration time.Duration, jsonOutput bool, forceFullAnalysis bool) {
	// Get actual beads path (respects BEADS_DIR)
	beadsDir, _ := loader.GetBeadsDir("")
	dataPath, _ := loader.FindJSONLPath(beadsDir)
	if dataPath == "" {
		dataPath = beadsDir // fallback
	}

	// Time analyzer construction
	buildStart := time.Now()
	analyzer := analysis.NewAnalyzer(issues)
	buildDuration := time.Since(buildStart)

	// Select config
	var config analysis.AnalysisConfig
	if forceFullAnalysis {
		config = analysis.FullAnalysisConfig()
	} else {
		nodeCount := len(issues)
		// Estimate edge count from issues
		edgeCount := 0
		for _, issue := range issues {
			edgeCount += len(issue.Dependencies)
		}
		config = analysis.ConfigForSize(nodeCount, edgeCount)
	}

	// Run profiled analysis
	_, profile := analyzer.AnalyzeWithProfile(config)

	// Add load and build durations to profile
	profile.BuildGraph = buildDuration

	// Calculate total including load
	totalWithLoad := loadDuration + profile.Total

	if jsonOutput {
		// JSON output
		output := struct {
			GeneratedAt     string                   `json:"generated_at"`
			DataPath        string                   `json:"data_path"`
			LoadJSONL       string                   `json:"load_jsonl"`
			Profile         *analysis.StartupProfile `json:"profile"`
			TotalWithLoad   string                   `json:"total_with_load"`
			Recommendations []string                 `json:"recommendations"`
		}{
			GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
			DataPath:        dataPath,
			LoadJSONL:       loadDuration.String(),
			Profile:         profile,
			TotalWithLoad:   totalWithLoad.String(),
			Recommendations: generateProfileRecommendations(profile, loadDuration, totalWithLoad),
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding profile: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Human-readable output
		printProfileReport(profile, loadDuration, totalWithLoad)
	}
}

// printProfileReport outputs a human-readable startup profile
func printProfileReport(profile *analysis.StartupProfile, loadDuration, totalWithLoad time.Duration) {
	fmt.Println("Startup Profile")
	fmt.Println("===============")
	fmt.Printf("Data: %d issues, %d dependencies, density=%.4f\n\n",
		profile.NodeCount, profile.EdgeCount, profile.Density)

	// Phase 1
	fmt.Println("Phase 1 (blocking):")
	fmt.Printf("  Load JSONL:      %v\n", formatDuration(loadDuration))
	fmt.Printf("  Build graph:     %v\n", formatDuration(profile.BuildGraph))
	fmt.Printf("  Degree:          %v\n", formatDuration(profile.Degree))
	fmt.Printf("  TopoSort:        %v\n", formatDuration(profile.TopoSort))
	fmt.Printf("  Total Phase 1:   %v\n\n", formatDuration(loadDuration+profile.BuildGraph+profile.Phase1))

	// Phase 2
	fmt.Println("Phase 2 (async in normal mode, sync for profiling):")
	printMetricLine("PageRank", profile.PageRank, profile.PageRankTO, profile.Config.ComputePageRank)
	printMetricLine("Betweenness", profile.Betweenness, profile.BetweennessTO, profile.Config.ComputeBetweenness)
	printMetricLine("Eigenvector", profile.Eigenvector, false, profile.Config.ComputeEigenvector)
	printMetricLine("HITS", profile.HITS, profile.HITSTO, profile.Config.ComputeHITS)
	printMetricLine("Critical Path", profile.CriticalPath, false, profile.Config.ComputeCriticalPath)
	printCyclesLine(profile)
	fmt.Printf("  Total Phase 2:   %v\n\n", formatDuration(profile.Phase2))

	// Total
	fmt.Printf("Total startup:     %v\n\n", formatDuration(totalWithLoad))

	// Configuration used
	fmt.Println("Configuration:")
	fmt.Printf("  Size tier: %s\n", getSizeTier(profile.NodeCount))
	skipped := profile.Config.SkippedMetrics()
	if len(skipped) > 0 {
		var names []string
		for _, s := range skipped {
			names = append(names, s.Name)
		}
		fmt.Printf("  Skipped metrics: %s\n", strings.Join(names, ", "))
	} else {
		fmt.Println("  All metrics computed")
	}
	fmt.Println()

	// Recommendations
	recommendations := generateProfileRecommendations(profile, loadDuration, totalWithLoad)
	if len(recommendations) > 0 {
		fmt.Println("Recommendations:")
		for _, rec := range recommendations {
			fmt.Printf("  %s\n", rec)
		}
	}
}

// printMetricLine prints a single metric timing line
func printMetricLine(name string, duration time.Duration, timedOut, computed bool) {
	if !computed {
		fmt.Printf("  %-14s [Skipped]\n", name+":")
		return
	}
	suffix := ""
	if timedOut {
		suffix = " (TIMEOUT)"
	}
	fmt.Printf("  %-14s %v%s\n", name+":", formatDuration(duration), suffix)
}

// printCyclesLine prints the cycles metric line with count
func printCyclesLine(profile *analysis.StartupProfile) {
	if !profile.Config.ComputeCycles {
		fmt.Printf("  %-14s [Skipped]\n", "Cycles:")
		return
	}
	suffix := ""
	if profile.CyclesTO {
		suffix = " (TIMEOUT)"
	} else if profile.CycleCount > 0 {
		suffix = fmt.Sprintf(" (found: %d)", profile.CycleCount)
	} else {
		suffix = " (none)"
	}
	fmt.Printf("  %-14s %v%s\n", "Cycles:", formatDuration(profile.Cycles), suffix)
}

// formatDuration formats a duration for display, right-aligned
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%6.2fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%6dms", d.Milliseconds())
}

// getSizeTier returns the size tier name based on node count
func getSizeTier(nodeCount int) string {
	switch {
	case nodeCount < 100:
		return "Small (<100 issues)"
	case nodeCount < 500:
		return "Medium (100-500 issues)"
	case nodeCount < 2000:
		return "Large (500-2000 issues)"
	default:
		return "XL (>2000 issues)"
	}
}

// generateProfileRecommendations generates actionable recommendations based on profile
func generateProfileRecommendations(profile *analysis.StartupProfile, loadDuration, totalWithLoad time.Duration) []string {
	var recs []string

	// Check overall startup time
	if totalWithLoad < 500*time.Millisecond {
		recs = append(recs, "✓ Startup within acceptable range (<500ms)")
	} else if totalWithLoad < 1*time.Second {
		recs = append(recs, "✓ Startup acceptable (<1s)")
	} else if totalWithLoad < 2*time.Second {
		// Check if full analysis is being used (no skipped metrics on a large graph)
		if len(profile.Config.SkippedMetrics()) == 0 && profile.NodeCount >= 500 {
			recs = append(recs, "⚠ Startup is slow (1-2s) - if using --force-full-analysis, consider removing it")
		} else {
			recs = append(recs, "⚠ Startup is slow (1-2s)")
		}
	} else {
		recs = append(recs, "⚠ Startup is very slow (>2s) - optimization recommended")
	}

	// Check for timeouts
	if profile.PageRankTO {
		recs = append(recs, "⚠ PageRank timed out - graph may be too large or dense")
	}
	if profile.BetweennessTO {
		recs = append(recs, "⚠ Betweenness timed out - this is expected for large graphs (>500 nodes)")
	}
	if profile.HITSTO {
		recs = append(recs, "⚠ HITS timed out - graph may have convergence issues")
	}
	if profile.CyclesTO {
		recs = append(recs, "⚠ Cycle detection timed out - graph may have many overlapping cycles")
	}

	// Check which metric is taking longest
	if profile.Config.ComputeBetweenness && profile.Betweenness > 0 {
		phase2NoZero := profile.Phase2
		if phase2NoZero > 0 {
			betweennessPercent := float64(profile.Betweenness) / float64(phase2NoZero) * 100
			if betweennessPercent > 50 {
				recs = append(recs, fmt.Sprintf("⚠ Betweenness taking %.0f%% of Phase 2 time - consider skipping for large graphs", betweennessPercent))
			}
		}
	}

	// Check for cycles
	if profile.CycleCount > 0 {
		recs = append(recs, fmt.Sprintf("⚠ Found %d circular dependencies - resolve to improve graph health", profile.CycleCount))
	}

	return recs
}

// filterByRepo filters issues to only include those from a specific repository.
// The filter matches issue IDs that start with the given prefix.
// If the prefix doesn't end with a separator character, it normalizes by checking
// common patterns (prefix-, prefix:, etc.).
func filterByRepo(issues []model.Issue, repoFilter string) []model.Issue {
	if repoFilter == "" {
		return issues
	}

	// Normalize the filter - ensure it's a proper prefix
	filter := repoFilter
	filterLower := strings.ToLower(filter)
	// If filter doesn't end with common separators, try matching as-is or with separators
	needsFlexibleMatch := !strings.HasSuffix(filter, "-") &&
		!strings.HasSuffix(filter, ":") &&
		!strings.HasSuffix(filter, "_")

	var result []model.Issue
	for _, issue := range issues {
		idLower := strings.ToLower(issue.ID)

		// Check if issue ID starts with the filter (case-insensitive)
		if strings.HasPrefix(idLower, filterLower) {
			result = append(result, issue)
			continue
		}

		// If flexible matching is needed, try with common separators
		if needsFlexibleMatch {
			if strings.HasPrefix(idLower, filterLower+"-") ||
				strings.HasPrefix(idLower, filterLower+":") ||
				strings.HasPrefix(idLower, filterLower+"_") {
				result = append(result, issue)
				continue
			}
		}

		// Also check SourceRepo field if set (case-insensitive)
		if issue.SourceRepo != "" && issue.SourceRepo != "." {
			sourceRepoLower := strings.ToLower(issue.SourceRepo)
			if strings.HasPrefix(sourceRepoLower, filterLower) {
				result = append(result, issue)
			}
		}
	}

	return result
}

// buildMetricItems converts a metrics map to a sorted slice of MetricItems
func buildMetricItems(metrics map[string]float64, limit int) []baseline.MetricItem {
	if len(metrics) == 0 {
		return nil
	}

	// Convert to slice for sorting
	items := make([]baseline.MetricItem, 0, len(metrics))
	for id, value := range metrics {
		items = append(items, baseline.MetricItem{ID: id, Value: value})
	}

	// Sort by value descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Value > items[j].Value
	})

	// Limit to top N
	if len(items) > limit {
		items = items[:limit]
	}

	return items
}

// buildAttentionReason creates a human-readable reason for attention score
func buildAttentionReason(score analysis.LabelAttentionScore) string {
	var parts []string

	// High PageRank
	if score.PageRankSum > 0.5 {
		parts = append(parts, "High PageRank")
	}

	// Blocked issues
	if score.BlockedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", score.BlockedCount))
	}

	// Stale issues
	if score.StaleCount > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", score.StaleCount))
	}

	// Low velocity (VelocityFactor = ClosedLast30Days + 1, so 1.0 means zero closures)
	if score.VelocityFactor <= 1.0 {
		parts = append(parts, "low velocity")
	}

	// If no specific reasons, note the open count
	if len(parts) == 0 {
		return fmt.Sprintf("%d open issues", score.OpenCount)
	}

	return strings.Join(parts, ", ")
}

// ============================================================================
// Static Pages Export Helpers (bv-73f)
// ============================================================================

// copyViewerAssets copies the viewer HTML/JS/CSS assets to the output directory.
// If title is provided, it replaces the default title in index.html.
func copyViewerAssets(outputDir, title string) error {
	// Get the path to the viewer assets embedded in the binary or relative to the executable
	// For development, we look relative to the project root
	// For production, assets should be embedded using go:embed

	// Find viewer assets directory
	assetsDir := findViewerAssetsDir()
	if assetsDir == "" {
		return fmt.Errorf("viewer assets not found")
	}

	// Files to copy
	files := []string{
		"index.html",
		"viewer.js",
		"styles.css",
		"graph.js",
		"coi-serviceworker.js",
	}

	for _, file := range files {
		src := filepath.Join(assetsDir, file)
		dst := filepath.Join(outputDir, file)

		// Special handling for index.html to replace title
		if file == "index.html" && title != "" {
			if err := copyFileWithTitleReplacement(src, dst, title); err != nil {
				return fmt.Errorf("copy %s: %w", file, err)
			}
			continue
		}

		if err := copyFile(src, dst); err != nil {
			// Skip missing optional files
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("copy %s: %w", file, err)
		}
	}

	// Copy vendor directory
	vendorSrc := filepath.Join(assetsDir, "vendor")
	vendorDst := filepath.Join(outputDir, "vendor")
	if err := copyDir(vendorSrc, vendorDst); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("copy vendor: %w", err)
		}
	}

	return nil
}

// findViewerAssetsDir locates the viewer assets directory.
func findViewerAssetsDir() string {
	// Try relative to current working directory (development)
	candidates := []string{
		"pkg/export/viewer_assets",
		"../pkg/export/viewer_assets",
		"../../pkg/export/viewer_assets",
	}

	// Try relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "pkg/export/viewer_assets"),
			filepath.Join(exeDir, "../pkg/export/viewer_assets"),
			filepath.Join(exeDir, "../../pkg/export/viewer_assets"),
		)
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return ""
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyFileWithTitleReplacement copies a file while replacing the default title.
func copyFileWithTitleReplacement(src, dst, title string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Replace title in <title> tag and in the h1 header
	result := strings.Replace(string(content), "<title>Beads Viewer</title>", "<title>"+title+"</title>", 1)
	result = strings.Replace(result, `<h1 class="text-xl font-semibold">Beads Viewer</h1>`, `<h1 class="text-xl font-semibold">`+title+`</h1>`, 1)

	return os.WriteFile(dst, []byte(result), 0644)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// runPreviewServer starts a local HTTP server to preview the static site.
func runPreviewServer(dir string) error {
	// Check directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory not found: %s", dir)
	}

	// Check for index.html
	indexPath := filepath.Join(dir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return fmt.Errorf("index.html not found in %s (did you run --export-pages first?)", dir)
	}

	port := 9000
	fmt.Printf("Starting preview server at http://localhost:%d\n", port)
	fmt.Printf("Serving files from: %s\n", dir)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("")

	// Try to open browser
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://localhost:%d", port))
	}()

	// Start HTTP server
	http.Handle("/", http.FileServer(http.Dir(dir)))
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

// openBrowser opens the default browser to the given URL.
func openBrowser(url string) {
	var cmd string
	var args []string

	switch {
	case isCommandAvailable("open"):
		cmd = "open"
		args = []string{url}
	case isCommandAvailable("xdg-open"):
		cmd = "xdg-open"
		args = []string{url}
	case isCommandAvailable("cmd"):
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		fmt.Printf("Open %s in your browser\n", url)
		return
	}

	exec.Command(cmd, args...).Start()
}

// isCommandAvailable checks if a command is available in PATH.
func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runPagesWizard runs the interactive deployment wizard (bv-10g).
func runPagesWizard(issues []model.Issue, beadsPath string) error {
	wizard := export.NewWizard(beadsPath)

	// Run interactive wizard to collect configuration
	_, err := wizard.Run()
	if err != nil {
		return err
	}

	config := wizard.GetConfig()

	// Filter issues based on config
	exportIssues := issues
	if !config.IncludeClosed {
		var openIssues []model.Issue
		for _, issue := range issues {
			if issue.Status != model.StatusClosed {
				openIssues = append(openIssues, issue)
			}
		}
		exportIssues = openIssues
	}

	// Create temp directory for bundle
	bundlePath := config.OutputPath
	if bundlePath == "" {
		tmpDir, err := os.MkdirTemp("", "bv-pages-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		bundlePath = tmpDir
	}

	// Ensure output directory exists
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Perform export
	wizard.PerformExport(bundlePath)

	fmt.Println("Exporting static site...")
	fmt.Printf("  -> Loading %d issues\n", len(exportIssues))

	// Build graph and compute stats
	fmt.Println("  -> Running graph analysis...")
	analyzer := analysis.NewAnalyzer(exportIssues)
	stats := analyzer.AnalyzeAsync()
	stats.WaitForPhase2()

	// Compute triage
	fmt.Println("  -> Generating triage data...")
	triage := analysis.ComputeTriage(exportIssues)

	// Extract dependencies
	var deps []*model.Dependency
	for i := range exportIssues {
		issue := &exportIssues[i]
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			deps = append(deps, &model.Dependency{
				IssueID:     issue.ID,
				DependsOnID: dep.DependsOnID,
				Type:        dep.Type,
			})
		}
	}

	// Create exporter
	issuePointers := make([]*model.Issue, len(exportIssues))
	for i := range exportIssues {
		issuePointers[i] = &exportIssues[i]
	}
	exporter := export.NewSQLiteExporter(issuePointers, deps, stats, &triage)
	if config.Title != "" {
		exporter.Config.Title = config.Title
	}

	// Export SQLite database
	fmt.Println("  -> Writing database and JSON files...")
	if err := exporter.Export(bundlePath); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Copy viewer assets
	fmt.Println("  -> Copying viewer assets...")
	if err := copyViewerAssets(bundlePath, config.Title); err != nil {
		return fmt.Errorf("failed to copy assets: %w", err)
	}

	fmt.Printf("  -> Bundle created: %s\n", bundlePath)
	fmt.Println("")

	// Offer preview (if deploying to GitHub)
	if config.DeployTarget == "github" {
		_, err := wizard.OfferPreview()
		if err != nil {
			return err
		}

		// Perform deployment
		result, err := wizard.PerformDeploy()
		if err != nil {
			return err
		}

		wizard.PrintSuccess(result)
	} else {
		// Local export - just show success
		result := &export.WizardResult{
			BundlePath:   bundlePath,
			DeployTarget: "local",
		}
		wizard.PrintSuccess(result)
	}

	// Save config for next run
	export.SaveWizardConfig(config)

	return nil
}

// BurndownOutput represents the JSON output for --robot-burndown (bv-159)
type BurndownOutput struct {
	GeneratedAt       time.Time             `json:"generated_at"`
	SprintID          string                `json:"sprint_id"`
	SprintName        string                `json:"sprint_name"`
	StartDate         time.Time             `json:"start_date"`
	EndDate           time.Time             `json:"end_date"`
	TotalDays         int                   `json:"total_days"`
	ElapsedDays       int                   `json:"elapsed_days"`
	RemainingDays     int                   `json:"remaining_days"`
	TotalIssues       int                   `json:"total_issues"`
	CompletedIssues   int                   `json:"completed_issues"`
	RemainingIssues   int                   `json:"remaining_issues"`
	IdealBurnRate     float64               `json:"ideal_burn_rate"`
	ActualBurnRate    float64               `json:"actual_burn_rate"`
	ProjectedComplete *time.Time            `json:"projected_complete,omitempty"`
	OnTrack           bool                  `json:"on_track"`
	DailyPoints       []model.BurndownPoint `json:"daily_points"`
	IdealLine         []model.BurndownPoint `json:"ideal_line"`
	ScopeChanges      []ScopeChangeEvent    `json:"scope_changes,omitempty"`
}

// ScopeChangeEvent represents when issues were added/removed from sprint
type ScopeChangeEvent struct {
	Date       time.Time `json:"date"`
	IssueID    string    `json:"issue_id"`
	IssueTitle string    `json:"issue_title"`
	Action     string    `json:"action"` // "added" or "removed"
}

// calculateBurndown computes burndown data for a sprint (bv-159)
func calculateBurndown(sprint *model.Sprint, issues []model.Issue) BurndownOutput {
	now := time.Now()

	// Build issue map for sprint beads
	issueMap := make(map[string]model.Issue, len(issues))
	for _, iss := range issues {
		issueMap[iss.ID] = iss
	}

	// Count total and completed issues in sprint
	var sprintIssues []model.Issue
	for _, beadID := range sprint.BeadIDs {
		if iss, ok := issueMap[beadID]; ok {
			sprintIssues = append(sprintIssues, iss)
		}
	}

	totalIssues := len(sprintIssues)
	completedIssues := 0
	for _, iss := range sprintIssues {
		if iss.Status == model.StatusClosed {
			completedIssues++
		}
	}
	remainingIssues := totalIssues - completedIssues

	// Calculate days
	totalDays := 0
	elapsedDays := 0
	remainingDays := 0

	if !sprint.StartDate.IsZero() && !sprint.EndDate.IsZero() {
		totalDays = int(sprint.EndDate.Sub(sprint.StartDate).Hours()/24) + 1
		if now.Before(sprint.StartDate) {
			elapsedDays = 0
			remainingDays = totalDays
		} else if now.After(sprint.EndDate) {
			elapsedDays = totalDays
			remainingDays = 0
		} else {
			elapsedDays = int(now.Sub(sprint.StartDate).Hours()/24) + 1
			remainingDays = totalDays - elapsedDays
		}
	}

	// Calculate burn rates
	idealBurnRate := 0.0
	if totalDays > 0 {
		idealBurnRate = float64(totalIssues) / float64(totalDays)
	}

	actualBurnRate := 0.0
	if elapsedDays > 0 {
		actualBurnRate = float64(completedIssues) / float64(elapsedDays)
	}

	// Calculate projected completion
	var projectedComplete *time.Time
	onTrack := true
	if actualBurnRate > 0 && remainingIssues > 0 {
		daysToComplete := float64(remainingIssues) / actualBurnRate
		projected := now.AddDate(0, 0, int(daysToComplete)+1)
		projectedComplete = &projected
		onTrack = !projected.After(sprint.EndDate)
	} else if remainingIssues == 0 {
		// Already complete
		onTrack = true
	} else if elapsedDays > 0 && completedIssues == 0 {
		// No progress made
		onTrack = false
	}

	// Generate daily burndown points
	dailyPoints := generateDailyBurndown(sprint, sprintIssues, now)

	// Generate ideal line
	idealLine := generateIdealLine(sprint, totalIssues)

	return BurndownOutput{
		GeneratedAt:       now.UTC(),
		SprintID:          sprint.ID,
		SprintName:        sprint.Name,
		StartDate:         sprint.StartDate,
		EndDate:           sprint.EndDate,
		TotalDays:         totalDays,
		ElapsedDays:       elapsedDays,
		RemainingDays:     remainingDays,
		TotalIssues:       totalIssues,
		CompletedIssues:   completedIssues,
		RemainingIssues:   remainingIssues,
		IdealBurnRate:     idealBurnRate,
		ActualBurnRate:    actualBurnRate,
		ProjectedComplete: projectedComplete,
		OnTrack:           onTrack,
		DailyPoints:       dailyPoints,
		IdealLine:         idealLine,
		ScopeChanges:      []ScopeChangeEvent{}, // TODO: Track scope changes via git history
	}
}

// generateDailyBurndown creates actual burndown points based on issue closure dates
func generateDailyBurndown(sprint *model.Sprint, issues []model.Issue, now time.Time) []model.BurndownPoint {
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() {
		return nil
	}

	var points []model.BurndownPoint
	totalIssues := len(issues)

	// Iterate through each day of the sprint
	for d := sprint.StartDate; !d.After(sprint.EndDate) && !d.After(now); d = d.AddDate(0, 0, 1) {
		dayEnd := d.Add(24*time.Hour - time.Second)
		completed := 0

		for _, iss := range issues {
			if iss.Status == model.StatusClosed && iss.ClosedAt != nil && !iss.ClosedAt.After(dayEnd) {
				completed++
			}
		}

		points = append(points, model.BurndownPoint{
			Date:      d,
			Remaining: totalIssues - completed,
			Completed: completed,
		})
	}

	return points
}

// generateIdealLine creates the ideal burndown line
func generateIdealLine(sprint *model.Sprint, totalIssues int) []model.BurndownPoint {
	if sprint.StartDate.IsZero() || sprint.EndDate.IsZero() || totalIssues == 0 {
		return nil
	}

	var points []model.BurndownPoint
	totalDays := int(sprint.EndDate.Sub(sprint.StartDate).Hours()/24) + 1
	burnPerDay := float64(totalIssues) / float64(totalDays)

	for i := 0; i <= totalDays; i++ {
		d := sprint.StartDate.AddDate(0, 0, i)
		remaining := totalIssues - int(float64(i)*burnPerDay)
		if remaining < 0 {
			remaining = 0
		}
		points = append(points, model.BurndownPoint{
			Date:      d,
			Remaining: remaining,
			Completed: totalIssues - remaining,
		})
	}

	return points
}

// generateJQHelpers creates a markdown document with jq snippets for agent brief
func generateJQHelpers() string {
	return `# jq Helper Snippets

Quick reference for extracting data from the agent brief JSON files.

## triage.json

### Top Picks
` + "```bash" + `
# Get top 3 recommendations
jq '.quick_ref.top_picks[:3]' triage.json

# Get IDs of top picks
jq '.quick_ref.top_picks[].id' triage.json

# Get top pick with highest unblocks
jq '.quick_ref.top_picks | max_by(.unblocks)' triage.json
` + "```" + `

### Recommendations
` + "```bash" + `
# List all recommendations with scores
jq '.recommendations[] | {id, score, action}' triage.json

# Filter high-score items (score > 0.15)
jq '.recommendations[] | select(.score > 0.15)' triage.json

# Get breakdown metrics
jq '.recommendations[] | {id, pr: .breakdown.pagerank_norm, bw: .breakdown.betweenness_norm}' triage.json
` + "```" + `

### Quick Wins
` + "```bash" + `
# List quick wins
jq '.quick_wins[] | {id, title, reason}' triage.json

# Count quick wins
jq '.quick_wins | length' triage.json
` + "```" + `

### Blockers
` + "```bash" + `
# Get actionable blockers
jq '.blockers_to_clear[] | select(.actionable)' triage.json

# Sort by unblocks count
jq '.blockers_to_clear | sort_by(-.unblocks_count)' triage.json
` + "```" + `

## insights.json

### Graph Metrics
` + "```bash" + `
# Top PageRank issues
jq '.top_pagerank | to_entries | sort_by(-.value)[:5]' insights.json

# Top betweenness centrality
jq '.top_betweenness | to_entries | sort_by(-.value)[:5]' insights.json

# Find hub issues (high in-degree)
jq '.top_in_degree | to_entries | sort_by(-.value)[:3]' insights.json
` + "```" + `

### Project Health
` + "```bash" + `
# Get velocity metrics
jq '.velocity' insights.json

# List critical issues
jq '.critical_issues' insights.json
` + "```" + `

## Combining Files
` + "```bash" + `
# Cross-reference top picks with insights
jq -s '.[0].quick_ref.top_picks[0].id as $id | .[1].top_pagerank[$id] // 0' triage.json insights.json

# Export summary to CSV
jq -r '.recommendations[] | [.id, .score, .action] | @csv' triage.json
` + "```" + `
`
}
