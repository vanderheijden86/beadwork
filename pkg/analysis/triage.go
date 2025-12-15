package analysis

import (
	"fmt"
	"sort"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// TriageResult is the unified output for --robot-triage
// Designed as a single entry point for AI agents to get everything they need
type TriageResult struct {
	Meta            TriageMeta         `json:"meta"`
	QuickRef        QuickRef           `json:"quick_ref"`
	Recommendations []Recommendation   `json:"recommendations"`
	QuickWins       []QuickWin         `json:"quick_wins"`
	BlockersToClear []BlockerItem      `json:"blockers_to_clear"`
	ProjectHealth   ProjectHealth      `json:"project_health"`
	Alerts          []Alert            `json:"alerts,omitempty"`
	Commands        CommandHelpers     `json:"commands"`
}

// TriageMeta contains metadata about the triage computation
type TriageMeta struct {
	Version      string    `json:"version"`
	GeneratedAt  time.Time `json:"generated_at"`
	Phase2Ready  bool      `json:"phase2_ready"`
	IssueCount   int       `json:"issue_count"`
	ComputeTimeMs int64    `json:"compute_time_ms"`
}

// QuickRef provides at-a-glance summary for fast decisions
type QuickRef struct {
	OpenCount       int       `json:"open_count"`
	ActionableCount int       `json:"actionable_count"`
	BlockedCount    int       `json:"blocked_count"`
	InProgressCount int       `json:"in_progress_count"`
	TopPicks        []TopPick `json:"top_picks"` // Top 3 recommended items
}

// TopPick is a condensed recommendation for quick reference
type TopPick struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Score    float64  `json:"score"`
	Reasons  []string `json:"reasons"`
	Unblocks int      `json:"unblocks"` // How many items this unblocks
}

// Recommendation is an actionable item with full context
type Recommendation struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	Priority    int            `json:"priority"`
	Labels      []string       `json:"labels"`
	Score       float64        `json:"score"`
	Breakdown   ScoreBreakdown `json:"breakdown"`
	Action      string         `json:"action"`   // "work", "review", "unblock"
	Reasons     []string       `json:"reasons"`
	UnblocksIDs []string       `json:"unblocks_ids,omitempty"`
	BlockedBy   []string       `json:"blocked_by,omitempty"`
}

// QuickWin represents a low-effort, high-impact item
type QuickWin struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Score       float64 `json:"score"`
	Reason      string  `json:"reason"`
	UnblocksIDs []string `json:"unblocks_ids,omitempty"`
}

// BlockerItem represents an item that blocks significant downstream work
type BlockerItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	UnblocksCount int      `json:"unblocks_count"`
	UnblocksIDs   []string `json:"unblocks_ids"`
	Actionable    bool     `json:"actionable"` // Can we work on this now?
	BlockedBy     []string `json:"blocked_by,omitempty"`
}

// ProjectHealth provides overall project status
type ProjectHealth struct {
	Counts     HealthCounts `json:"counts"`
	Graph      GraphHealth  `json:"graph"`
	Velocity   *Velocity    `json:"velocity,omitempty"`   // nil until labels view ready
	Staleness  *Staleness   `json:"staleness,omitempty"`  // nil until history ready
}

// HealthCounts is basic issue statistics
type HealthCounts struct {
	Total      int            `json:"total"`
	Open       int            `json:"open"`
	Closed     int            `json:"closed"`
	Blocked    int            `json:"blocked"`
	Actionable int            `json:"actionable"`
	ByStatus   map[string]int `json:"by_status"`
	ByType     map[string]int `json:"by_type"`
	ByPriority map[int]int    `json:"by_priority"`
}

// GraphHealth summarizes dependency graph metrics
type GraphHealth struct {
	NodeCount    int     `json:"node_count"`
	EdgeCount    int     `json:"edge_count"`
	Density      float64 `json:"density"`
	HasCycles    bool    `json:"has_cycles"`
	CycleCount   int     `json:"cycle_count,omitempty"`
	Phase2Ready  bool    `json:"phase2_ready"`
}

// Velocity tracks work completion rate (future: from labels view)
type Velocity struct {
	ClosedLast7Days  int     `json:"closed_last_7_days"`
	ClosedLast30Days int     `json:"closed_last_30_days"`
	AvgDaysToClose   float64 `json:"avg_days_to_close"`
}

// Staleness tracks stale issues (future: from history)
type Staleness struct {
	StaleCount       int      `json:"stale_count"`        // Issues with no activity > threshold
	StalestIssueID   string   `json:"stalest_issue_id"`
	StalestIssueDays int      `json:"stalest_issue_days"`
	ThresholdDays    int      `json:"threshold_days"`
}

// Alert represents a proactive warning (future: from alerts engine)
type Alert struct {
	Type     string `json:"type"`     // "stale", "velocity_drop", "cycle", "duplicate"
	Severity string `json:"severity"` // "info", "warning", "error"
	Message  string `json:"message"`
	IssueID  string `json:"issue_id,omitempty"`
	IssueIDs []string `json:"issue_ids,omitempty"`
}

// CommandHelpers provides copy-paste commands for common actions
type CommandHelpers struct {
	ClaimTop       string `json:"claim_top"`        // bd update <id> --status=in_progress
	ShowTop        string `json:"show_top"`         // bd show <id>
	ListReady      string `json:"list_ready"`       // bd ready
	ListBlocked    string `json:"list_blocked"`     // bd blocked
	RefreshTriage  string `json:"refresh_triage"`   // bv --robot-triage
}

// ComputeTriage generates a unified triage result from issues
func ComputeTriage(issues []model.Issue) TriageResult {
	return ComputeTriageWithOptions(issues, TriageOptions{})
}

// TriageOptions configures triage computation
type TriageOptions struct {
	TopN           int  // Number of recommendations (default 10)
	QuickWinN      int  // Number of quick wins (default 5)
	BlockerN       int  // Number of blockers to show (default 5)
	WaitForPhase2  bool // Block until Phase 2 metrics ready
}

// ComputeTriageWithOptions generates triage with custom options
func ComputeTriageWithOptions(issues []model.Issue, opts TriageOptions) TriageResult {
	start := time.Now()

	// Set defaults
	if opts.TopN <= 0 {
		opts.TopN = 10
	}
	if opts.QuickWinN <= 0 {
		opts.QuickWinN = 5
	}
	if opts.BlockerN <= 0 {
		opts.BlockerN = 5
	}

	// Build analyzer
	analyzer := NewAnalyzer(issues)
	stats := analyzer.AnalyzeAsync()

	if opts.WaitForPhase2 {
		stats.WaitForPhase2()
	}

	// Compute impact scores
	impactScores := analyzer.ComputeImpactScores()

	// Get execution plan for unblock analysis (currently unused but kept for future phases)
	_ = analyzer.GetExecutionPlan()

	// Build unblocks map
	unblocksMap := buildUnblocksMap(analyzer, issues)

	// Compute counts
	counts := computeCounts(issues, analyzer)

	// Build recommendations
	recommendations := buildRecommendations(impactScores, analyzer, unblocksMap, opts.TopN)

	// Build quick wins
	quickWins := buildQuickWins(impactScores, unblocksMap, opts.QuickWinN)

	// Build blockers to clear
	blockersToClear := buildBlockersToClear(analyzer, unblocksMap, opts.BlockerN)

	// Build top picks for quick ref
	topPicks := buildTopPicks(recommendations, 3)

	// Determine top issue for commands
	topID := ""
	if len(recommendations) > 0 {
		topID = recommendations[0].ID
	}

	elapsed := time.Since(start)

	return TriageResult{
		Meta: TriageMeta{
			Version:       "1.0.0",
			GeneratedAt:   time.Now(),
			Phase2Ready:   stats.IsPhase2Ready(),
			IssueCount:    len(issues),
			ComputeTimeMs: elapsed.Milliseconds(),
		},
		QuickRef: QuickRef{
			OpenCount:       counts.Open,
			ActionableCount: counts.Actionable,
			BlockedCount:    counts.Blocked,
			InProgressCount: counts.ByStatus["in_progress"],
			TopPicks:        topPicks,
		},
		Recommendations: recommendations,
		QuickWins:       quickWins,
		BlockersToClear: blockersToClear,
		ProjectHealth: ProjectHealth{
			Counts: counts,
			Graph:  buildGraphHealth(stats),
			// Velocity and Staleness are nil until those features are implemented
		},
		Commands: buildCommands(topID),
	}
}

// buildUnblocksMap computes what each issue unblocks
func buildUnblocksMap(analyzer *Analyzer, issues []model.Issue) map[string][]string {
	unblocksMap := make(map[string][]string)
	for _, issue := range issues {
		if issue.Status == model.StatusClosed {
			continue
		}
		unblocksMap[issue.ID] = analyzer.computeUnblocks(issue.ID)
	}
	return unblocksMap
}

// computeCounts tallies issues by various dimensions
func computeCounts(issues []model.Issue, analyzer *Analyzer) HealthCounts {
	counts := HealthCounts{
		Total:      len(issues),
		ByStatus:   make(map[string]int),
		ByType:     make(map[string]int),
		ByPriority: make(map[int]int),
	}

	actionable := analyzer.GetActionableIssues()
	actionableSet := make(map[string]bool, len(actionable))
	for _, a := range actionable {
		actionableSet[a.ID] = true
	}

	for _, issue := range issues {
		counts.ByStatus[string(issue.Status)]++
		counts.ByType[string(issue.IssueType)]++
		counts.ByPriority[issue.Priority]++

		if issue.Status == model.StatusClosed {
			counts.Closed++
		} else {
			counts.Open++
			if actionableSet[issue.ID] {
				counts.Actionable++
			} else {
				counts.Blocked++
			}
		}
	}

	return counts
}

// buildRecommendations creates detailed recommendations from impact scores
func buildRecommendations(scores []ImpactScore, analyzer *Analyzer, unblocksMap map[string][]string, limit int) []Recommendation {
	if len(scores) > limit {
		scores = scores[:limit]
	}

	recommendations := make([]Recommendation, 0, len(scores))
	for _, score := range scores {
		issue := analyzer.GetIssue(score.IssueID)
		if issue == nil {
			continue
		}

		// Determine action and reasons
		action, reasons := determineAction(score, unblocksMap[score.IssueID], issue)

		// Get labels (already strings in model.Issue)
		labels := issue.Labels

		// Get blocked by
		blockedBy := analyzer.GetOpenBlockers(score.IssueID)

		rec := Recommendation{
			ID:          score.IssueID,
			Title:       score.Title,
			Type:        string(issue.IssueType),
			Status:      score.Status,
			Priority:    score.Priority,
			Labels:      labels,
			Score:       score.Score,
			Breakdown:   score.Breakdown,
			Action:      action,
			Reasons:     reasons,
			UnblocksIDs: unblocksMap[score.IssueID],
		}
		if len(blockedBy) > 0 {
			rec.BlockedBy = blockedBy
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// determineAction decides what action to take and why
func determineAction(score ImpactScore, unblocks []string, issue *model.Issue) (string, []string) {
	var reasons []string
	action := "work"

	// High PageRank = central to project
	if score.Breakdown.PageRankNorm > 0.3 {
		reasons = append(reasons, fmt.Sprintf("High centrality (PageRank: %.2f)", score.Breakdown.PageRankNorm))
	}

	// High Betweenness = bottleneck
	if score.Breakdown.BetweennessNorm > 0.5 {
		reasons = append(reasons, fmt.Sprintf("Critical bottleneck (Betweenness: %.2f)", score.Breakdown.BetweennessNorm))
	}

	// High blocker ratio = unblocks many
	if len(unblocks) >= 3 {
		reasons = append(reasons, fmt.Sprintf("Unblocks %d downstream items", len(unblocks)))
		action = "unblock" // Priority action
	} else if len(unblocks) > 0 {
		reasons = append(reasons, fmt.Sprintf("Unblocks %d item(s)", len(unblocks)))
	}

	// Staleness - check if issue is stale
	isStale := score.Breakdown.StalenessNorm > 0.5
	if isStale {
		days := int(score.Breakdown.StalenessNorm * 30)
		reasons = append(reasons, fmt.Sprintf("Stale for %d+ days", days))
	}

	// In progress items may need review
	if issue.Status == model.StatusInProgress {
		if isStale {
			// Very stale in_progress - definitely needs review
			action = "review"
			reasons = append(reasons, "In progress but appears stuck")
		} else if score.Breakdown.StalenessNorm > 0.3 {
			// Moderately stale in_progress - might need attention
			action = "review"
			reasons = append(reasons, "In progress - may need attention")
		}
	}

	// Priority consideration
	if score.Priority <= 1 {
		reasons = append(reasons, fmt.Sprintf("High priority (P%d)", score.Priority))
	}

	// Default reason if none
	if len(reasons) == 0 {
		reasons = append(reasons, "Good candidate for work")
	}

	return action, reasons
}

// buildQuickWins finds low-complexity, high-impact items
func buildQuickWins(scores []ImpactScore, unblocksMap map[string][]string, limit int) []QuickWin {
	// Quick wins: high score but likely simple (no deep dependency chains)
	// Heuristic: items that unblock others but have low blocker ratio themselves

	type candidate struct {
		score   ImpactScore
		unblocks []string
		quickWinScore float64
	}

	var candidates []candidate
	for _, score := range scores {
		unblocks := unblocksMap[score.IssueID]
		// Quick win score: benefits unblocking, penalizes complexity
		// - High unblock count = good (helps project progress)
		// - Low BlockerRatioNorm = few things depend on this = safer to work on
		// - High priority number (P3, P4) = likely simpler tasks
		qwScore := float64(len(unblocks)) * 0.5
		if score.Breakdown.BlockerRatioNorm < 0.3 {
			qwScore += 0.3 // Bonus: not a critical bottleneck (fewer downstream deps)
		}
		if score.Priority >= 3 {
			qwScore += 0.2 // Bonus: lower priority often means simpler
		}
		candidates = append(candidates, candidate{score, unblocks, qwScore})
	}

	// Sort by quick win score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].quickWinScore > candidates[j].quickWinScore
	})

	quickWins := make([]QuickWin, 0, limit)
	for i := 0; i < len(candidates) && i < limit; i++ {
		c := candidates[i]
		reason := "Low complexity"
		if len(c.unblocks) > 0 {
			reason = fmt.Sprintf("Unblocks %d items", len(c.unblocks))
		}

		quickWins = append(quickWins, QuickWin{
			ID:          c.score.IssueID,
			Title:       c.score.Title,
			Score:       c.quickWinScore,
			Reason:      reason,
			UnblocksIDs: c.unblocks,
		})
	}

	return quickWins
}

// buildBlockersToClear finds items that block the most downstream work
func buildBlockersToClear(analyzer *Analyzer, unblocksMap map[string][]string, limit int) []BlockerItem {
	type blocker struct {
		id       string
		title    string
		unblocks []string
	}

	actionable := analyzer.GetActionableIssues()
	actionableSet := make(map[string]bool, len(actionable))
	for _, a := range actionable {
		actionableSet[a.ID] = true
	}

	var blockers []blocker
	for id, unblocks := range unblocksMap {
		if len(unblocks) == 0 {
			continue
		}
		issue := analyzer.GetIssue(id)
		if issue == nil || issue.Status == model.StatusClosed {
			continue
		}
		blockers = append(blockers, blocker{
			id:       id,
			title:    issue.Title,
			unblocks: unblocks,
		})
	}

	// Sort by unblocks count descending
	sort.Slice(blockers, func(i, j int) bool {
		return len(blockers[i].unblocks) > len(blockers[j].unblocks)
	})

	result := make([]BlockerItem, 0, limit)
	for i := 0; i < len(blockers) && i < limit; i++ {
		b := blockers[i]
		item := BlockerItem{
			ID:            b.id,
			Title:         b.title,
			UnblocksCount: len(b.unblocks),
			UnblocksIDs:   b.unblocks,
			Actionable:    actionableSet[b.id],
		}
		if !item.Actionable {
			item.BlockedBy = analyzer.GetOpenBlockers(b.id)
		}
		result = append(result, item)
	}

	return result
}

// buildTopPicks creates condensed top picks from recommendations
func buildTopPicks(recommendations []Recommendation, limit int) []TopPick {
	if len(recommendations) > limit {
		recommendations = recommendations[:limit]
	}

	picks := make([]TopPick, 0, len(recommendations))
	for _, rec := range recommendations {
		picks = append(picks, TopPick{
			ID:       rec.ID,
			Title:    rec.Title,
			Score:    rec.Score,
			Reasons:  rec.Reasons,
			Unblocks: len(rec.UnblocksIDs),
		})
	}

	return picks
}

// buildGraphHealth constructs graph health metrics from stats
func buildGraphHealth(stats *GraphStats) GraphHealth {
	// Call Cycles() once to avoid duplicate work (it makes a copy each time)
	cycles := stats.Cycles()
	cycleCount := 0
	if cycles != nil {
		cycleCount = len(cycles)
	}

	return GraphHealth{
		NodeCount:   stats.NodeCount,
		EdgeCount:   stats.EdgeCount,
		Density:     stats.Density,
		HasCycles:   cycleCount > 0,
		CycleCount:  cycleCount,
		Phase2Ready: stats.IsPhase2Ready(),
	}
}

// buildCommands constructs helper commands, handling empty topID gracefully
func buildCommands(topID string) CommandHelpers {
	claimTop := "bd ready  # No top pick available"
	showTop := "bd ready  # No top pick available"
	if topID != "" {
		claimTop = fmt.Sprintf("bd update %s --status=in_progress", topID)
		showTop = fmt.Sprintf("bd show %s", topID)
	}

	return CommandHelpers{
		ClaimTop:      claimTop,
		ShowTop:       showTop,
		ListReady:     "bd ready",
		ListBlocked:   "bd blocked",
		RefreshTriage: "bv --robot-triage",
	}
}
