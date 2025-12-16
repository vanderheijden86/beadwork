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

// ============================================================================
// Unified Triage Scoring (bv-147)
// Extends base impact scoring with triage-specific factors
// ============================================================================

// TriageScore represents a triage-specific score with factors applied
type TriageScore struct {
	IssueID        string         `json:"issue_id"`
	Title          string         `json:"title"`
	BaseScore      float64        `json:"base_score"`      // From ComputeImpactScores
	TriageScore    float64        `json:"triage_score"`    // Final triage-adjusted score
	Breakdown      ScoreBreakdown `json:"breakdown"`       // Original breakdown
	TriageFactors  TriageFactors  `json:"triage_factors"`  // Triage-specific factors
	FactorsApplied []string       `json:"factors_applied"` // Which factors were used
	FactorsPending []string       `json:"factors_pending"` // Which factors are not yet available
	Priority       int            `json:"priority"`
	Status         string         `json:"status"`
}

// TriageFactors holds the triage-specific score modifiers
type TriageFactors struct {
	UnblockBoost    float64 `json:"unblock_boost"`              // Boost for items that unblock many others
	QuickWinBoost   float64 `json:"quick_win_boost"`            // Boost for low-effort high-impact items
	LabelHealth     float64 `json:"label_health,omitempty"`     // Phase 2: Label health factor
	ClaimPenalty    float64 `json:"claim_penalty,omitempty"`    // Phase 3: Penalty for claimed items
	AttentionScore  float64 `json:"attention_score,omitempty"`  // Phase 4: Attention-weighted health
}

// TriageScoringOptions configures triage scoring behavior
type TriageScoringOptions struct {
	// Weight configuration
	BaseScoreWeight    float64 // Default 0.70
	UnblockBoostWeight float64 // Default 0.15
	QuickWinWeight     float64 // Default 0.15

	// Thresholds
	UnblockThreshold int     // Min unblocks to get full boost (default 5)
	QuickWinMaxDepth int     // Max dependency depth for quick win (default 2)

	// Feature flags (for graceful degradation)
	EnableLabelHealth    bool   // Phase 2 feature
	EnableClaimPenalty   bool   // Phase 3 feature
	EnableAttentionScore bool   // Phase 4 feature
	ClaimedByAgent       string // Current agent for claim penalty calculation
}

// DefaultTriageScoringOptions returns sensible defaults
func DefaultTriageScoringOptions() TriageScoringOptions {
	return TriageScoringOptions{
		BaseScoreWeight:    0.70,
		UnblockBoostWeight: 0.15,
		QuickWinWeight:     0.15,
		UnblockThreshold:   5,
		QuickWinMaxDepth:   2,
		// All optional features off by default (MVP mode)
		EnableLabelHealth:    false,
		EnableClaimPenalty:   false,
		EnableAttentionScore: false,
	}
}

// ComputeTriageScores calculates triage-optimized scores for all open issues
func ComputeTriageScores(issues []model.Issue) []TriageScore {
	return ComputeTriageScoresWithOptions(issues, DefaultTriageScoringOptions())
}

// ComputeTriageScoresWithOptions calculates triage scores with custom options
func ComputeTriageScoresWithOptions(issues []model.Issue, opts TriageScoringOptions) []TriageScore {
	if len(issues) == 0 {
		return nil
	}

	// Build analyzer for base scoring and graph analysis
	analyzer := NewAnalyzer(issues)
	baseScores := analyzer.ComputeImpactScores()

	// Build unblocks map for factor calculation
	unblocksMap := buildUnblocksMap(analyzer, issues)

	// Calculate max unblocks for normalization
	maxUnblocks := 0
	for _, unblocks := range unblocksMap {
		if len(unblocks) > maxUnblocks {
			maxUnblocks = len(unblocks)
		}
	}

	// Build triage scores
	triageScores := make([]TriageScore, 0, len(baseScores))
	for _, base := range baseScores {
		ts := computeSingleTriageScore(base, unblocksMap, maxUnblocks, analyzer, opts)
		triageScores = append(triageScores, ts)
	}

	// Sort by triage score descending
	sort.Slice(triageScores, func(i, j int) bool {
		if triageScores[i].TriageScore != triageScores[j].TriageScore {
			return triageScores[i].TriageScore > triageScores[j].TriageScore
		}
		return triageScores[i].IssueID < triageScores[j].IssueID
	})

	return triageScores
}

// computeSingleTriageScore calculates the triage score for a single issue
func computeSingleTriageScore(base ImpactScore, unblocksMap map[string][]string, maxUnblocks int, analyzer *Analyzer, opts TriageScoringOptions) TriageScore {
	factors := TriageFactors{}
	applied := []string{"base"}
	pending := []string{}

	// Calculate unblock boost
	unblocks := unblocksMap[base.IssueID]
	if len(unblocks) > 0 {
		// Normalize unblocks: items that unblock more get higher boost
		unblocksNorm := float64(len(unblocks)) / float64(maxOf(maxUnblocks, opts.UnblockThreshold))
		if unblocksNorm > 1.0 {
			unblocksNorm = 1.0
		}
		factors.UnblockBoost = unblocksNorm * opts.UnblockBoostWeight
		applied = append(applied, "unblock")
	}

	// Calculate quick-win boost
	// Quick wins are items with low blocker depth but high impact
	blockerDepth := analyzer.GetBlockerDepth(base.IssueID)
	if blockerDepth <= opts.QuickWinMaxDepth && blockerDepth >= 0 {
		// Lower depth = higher quick win potential
		depthFactor := 1.0 - float64(blockerDepth)/float64(opts.QuickWinMaxDepth+1)
		// Combine with base score for impact consideration
		factors.QuickWinBoost = depthFactor * base.Score * opts.QuickWinWeight
		if factors.QuickWinBoost > opts.QuickWinWeight {
			factors.QuickWinBoost = opts.QuickWinWeight // Cap at max weight
		}
		applied = append(applied, "quick_win")
	}

	// Track pending features
	if !opts.EnableLabelHealth {
		pending = append(pending, "label_health")
	}
	if !opts.EnableClaimPenalty {
		pending = append(pending, "claim_penalty")
	}
	if !opts.EnableAttentionScore {
		pending = append(pending, "attention_score")
	}

	// Calculate final triage score
	triageScore := base.Score*opts.BaseScoreWeight + factors.UnblockBoost + factors.QuickWinBoost

	// Future phases (when enabled):
	// Phase 2: triageScore += factors.LabelHealth * labelHealthWeight
	// Phase 3: if claimedByOther { triageScore *= 0.1 }
	// Phase 4: Replace label health with attention-weighted health

	return TriageScore{
		IssueID:        base.IssueID,
		Title:          base.Title,
		BaseScore:      base.Score,
		TriageScore:    triageScore,
		Breakdown:      base.Breakdown,
		TriageFactors:  factors,
		FactorsApplied: applied,
		FactorsPending: pending,
		Priority:       base.Priority,
		Status:         base.Status,
	}
}

// GetBlockerDepth returns the depth of the blocker chain for an issue
// Returns 0 if no blockers, 1 if blocked by one level, etc.
// Returns -1 if the issue is part of a cycle
func (a *Analyzer) GetBlockerDepth(issueID string) int {
	visited := make(map[string]bool)
	return a.getBlockerDepthRecursive(issueID, visited, 0)
}

func (a *Analyzer) getBlockerDepthRecursive(issueID string, visited map[string]bool, depth int) int {
	if visited[issueID] {
		return -1 // Cycle detected
	}
	visited[issueID] = true

	blockers := a.GetOpenBlockers(issueID)
	if len(blockers) == 0 {
		return depth
	}

	maxDepth := depth
	for _, blockerID := range blockers {
		blockerDepth := a.getBlockerDepthRecursive(blockerID, visited, depth+1)
		if blockerDepth == -1 {
			return -1 // Propagate cycle
		}
		if blockerDepth > maxDepth {
			maxDepth = blockerDepth
		}
	}

	return maxDepth
}

// maxOf returns the maximum of two integers
func maxOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetTopTriageScores returns the top N triage scores
func GetTopTriageScores(issues []model.Issue, n int) []TriageScore {
	scores := ComputeTriageScores(issues)
	if n > len(scores) {
		n = len(scores)
	}
	return scores[:n]
}

// ============================================================================
// Reason Generation (bv-148)
// Actionable, emoji-prefixed reasons for AI agents
// ============================================================================

// TriageReasonContext provides context for generating triage reasons
type TriageReasonContext struct {
	Issue           *model.Issue
	TriageScore     *TriageScore
	UnblocksIDs     []string
	BlockedByIDs    []string
	LabelHealth     map[string]int // Label -> health score (0-100)
	ClaimedByAgent  string         // Empty if unclaimed
	DaysSinceUpdate int
	IsQuickWin      bool
	BlockerDepth    int
}

// TriageReasons contains all generated reasons for an issue
type TriageReasons struct {
	Primary    string   `json:"primary"`    // Single most important reason
	All        []string `json:"all"`        // All reasons in priority order
	ActionHint string   `json:"action_hint"` // Suggested next action
}

// GenerateTriageReasons creates actionable reasons for a triage recommendation
// These are emoji-prefixed, human-readable explanations that tell agents what to DO
func GenerateTriageReasons(ctx TriageReasonContext) TriageReasons {
	var reasons []string
	primary := ""
	actionHint := "Start work on this issue"

	// 1. Unblock cascade (highest priority - most actionable)
	if len(ctx.UnblocksIDs) >= 3 {
		reason := fmt.Sprintf("🎯 Completing this unblocks %d downstream issues (%s)",
			len(ctx.UnblocksIDs), formatUnblockList(ctx.UnblocksIDs))
		reasons = append(reasons, reason)
		if primary == "" {
			primary = reason
		}
	} else if len(ctx.UnblocksIDs) > 0 {
		reason := fmt.Sprintf("🔓 Unblocks %d item(s): %s",
			len(ctx.UnblocksIDs), formatUnblockList(ctx.UnblocksIDs))
		reasons = append(reasons, reason)
	}

	// 2. Label health (shows context for labels needing attention)
	if len(ctx.LabelHealth) > 0 && ctx.Issue != nil {
		for _, label := range ctx.Issue.Labels {
			health, exists := ctx.LabelHealth[label]
			if exists && health < 60 {
				reason := fmt.Sprintf("⚠️ Label '%s' needs attention (health: %d/100)", label, health)
				reasons = append(reasons, reason)
			}
		}
	}

	// 3. Graph metrics (bottleneck/centrality)
	if ctx.TriageScore != nil {
		bd := ctx.TriageScore.Breakdown
		if bd.BetweennessNorm > 0.5 {
			reason := fmt.Sprintf("🔀 Critical path bottleneck (betweenness: %.0f%%)", bd.BetweennessNorm*100)
			reasons = append(reasons, reason)
			if primary == "" {
				primary = reason
			}
		}
		if bd.PageRankNorm > 0.3 {
			reason := fmt.Sprintf("📊 High centrality in dependency graph (PageRank: %.0f%%)", bd.PageRankNorm*100)
			reasons = append(reasons, reason)
		}
	}

	// 4. Staleness alert
	if ctx.DaysSinceUpdate > 14 {
		reason := fmt.Sprintf("🕐 No activity in %d days - may need review", ctx.DaysSinceUpdate)
		reasons = append(reasons, reason)
		if ctx.Issue != nil && ctx.Issue.Status == model.StatusInProgress {
			actionHint = "Check if this is stuck and needs help"
		}
	} else if ctx.DaysSinceUpdate > 7 {
		reason := fmt.Sprintf("📅 Last updated %d days ago", ctx.DaysSinceUpdate)
		reasons = append(reasons, reason)
	}

	// 5. Quick-win identification
	if ctx.IsQuickWin {
		reason := "⚡ Low effort, high impact - good starting point"
		reasons = append(reasons, reason)
		if primary == "" && len(ctx.UnblocksIDs) > 0 {
			primary = reason
		}
		actionHint = "Quick win - start here for fast progress"
	}

	// 6. Agent claim status
	if ctx.ClaimedByAgent == "" {
		reasons = append(reasons, "✅ Currently unclaimed - available for work")
	} else {
		reason := fmt.Sprintf("👤 Claimed by %s", ctx.ClaimedByAgent)
		reasons = append(reasons, reason)
		actionHint = fmt.Sprintf("Contact %s if you want to help", ctx.ClaimedByAgent)
	}

	// 7. Blocked status context
	if len(ctx.BlockedByIDs) > 0 {
		if len(ctx.BlockedByIDs) == 1 {
			reason := fmt.Sprintf("⏳ Blocked by %s - complete that first", ctx.BlockedByIDs[0])
			reasons = append(reasons, reason)
		} else {
			reason := fmt.Sprintf("⏳ Blocked by %d items - need to clear dependencies", len(ctx.BlockedByIDs))
			reasons = append(reasons, reason)
		}
		actionHint = fmt.Sprintf("Work on %s first to unblock this", ctx.BlockedByIDs[0])
	}

	// 8. Priority context
	if ctx.Issue != nil && ctx.Issue.Priority <= 1 {
		reason := fmt.Sprintf("🚨 High priority (P%d) - prioritize this work", ctx.Issue.Priority)
		reasons = append(reasons, reason)
	}

	// Default primary reason
	if primary == "" && len(reasons) > 0 {
		primary = reasons[0]
	} else if primary == "" {
		primary = "Good candidate for work"
		reasons = append(reasons, primary)
	}

	return TriageReasons{
		Primary:    primary,
		All:        reasons,
		ActionHint: actionHint,
	}
}

// formatUnblockList creates a comma-separated list of issue IDs, truncating if needed
func formatUnblockList(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
    if len(ids) <= 3 {
        // joinStrings is defined in diff.go
        return joinStrings(ids, ", ")
    }
	return fmt.Sprintf("%s, %s, +%d more", ids[0], ids[1], len(ids)-2)
}

// GenerateTriageReasonsForScore generates reasons from a TriageScore and Analyzer context
// This is a convenience function for common use cases
func GenerateTriageReasonsForScore(score TriageScore, analyzer *Analyzer, unblocksMap map[string][]string) TriageReasons {
	issue := analyzer.GetIssue(score.IssueID)

	daysSinceUpdate := 0
	if issue != nil && !issue.UpdatedAt.IsZero() {
		daysSinceUpdate = int(time.Since(issue.UpdatedAt).Hours() / 24)
	}

	// Determine if this is a quick win based on factors
	isQuickWin := score.TriageFactors.QuickWinBoost > 0.05

	ctx := TriageReasonContext{
		Issue:           issue,
		TriageScore:     &score,
		UnblocksIDs:     unblocksMap[score.IssueID],
		BlockedByIDs:    analyzer.GetOpenBlockers(score.IssueID),
		DaysSinceUpdate: daysSinceUpdate,
		IsQuickWin:      isQuickWin,
		BlockerDepth:    analyzer.GetBlockerDepth(score.IssueID),
	}

	return GenerateTriageReasons(ctx)
}

// EnhanceRecommendationWithTriageReasons updates a Recommendation with triage-specific reasons
func EnhanceRecommendationWithTriageReasons(rec *Recommendation, triageReasons TriageReasons) {
	if rec == nil {
		return
	}
	// Replace base reasons with enhanced triage reasons
	rec.Reasons = triageReasons.All
}
