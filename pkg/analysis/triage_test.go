package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestComputeTriage_Empty(t *testing.T) {
	triage := ComputeTriage(nil)

	if triage.Meta.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", triage.Meta.Version)
	}
	if triage.QuickRef.OpenCount != 0 {
		t.Errorf("expected 0 open count, got %d", triage.QuickRef.OpenCount)
	}
	if len(triage.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(triage.Recommendations))
	}
}

func TestComputeTriage_BasicIssues(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "test-1",
			Title:     "First issue",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:        "test-2",
			Title:     "Second issue",
			Status:    model.StatusOpen,
			Priority:  2,
			IssueType: model.TypeBug,
			UpdatedAt: time.Now().Add(-48 * time.Hour),
		},
		{
			ID:        "test-3",
			Title:     "Closed issue",
			Status:    model.StatusClosed,
			Priority:  1,
			IssueType: model.TypeTask,
		},
	}

	triage := ComputeTriage(issues)

	// Check counts
	if triage.QuickRef.OpenCount != 2 {
		t.Errorf("expected 2 open, got %d", triage.QuickRef.OpenCount)
	}
	if triage.ProjectHealth.Counts.Closed != 1 {
		t.Errorf("expected 1 closed, got %d", triage.ProjectHealth.Counts.Closed)
	}
	if triage.ProjectHealth.Counts.Total != 3 {
		t.Errorf("expected 3 total, got %d", triage.ProjectHealth.Counts.Total)
	}

	// Should have recommendations for open issues
	if len(triage.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// Commands should be populated
	if triage.Commands.ListReady != "bd ready" {
		t.Errorf("expected 'bd ready' command, got %s", triage.Commands.ListReady)
	}
}

func TestComputeTriage_WithDependencies(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "blocker",
			Title:     "Blocker issue",
			Status:    model.StatusOpen,
			Priority:  0,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "blocked",
			Title:     "Blocked issue",
			Status:    model.StatusOpen,
			Priority:  1,
			IssueType: model.TypeTask,
			UpdatedAt: time.Now(),
			Dependencies: []*model.Dependency{
				{DependsOnID: "blocker", Type: model.DepBlocks},
			},
		},
	}

	triage := ComputeTriage(issues)

	// One should be blocked
	if triage.QuickRef.BlockedCount != 1 {
		t.Errorf("expected 1 blocked, got %d", triage.QuickRef.BlockedCount)
	}
	if triage.QuickRef.ActionableCount != 1 {
		t.Errorf("expected 1 actionable, got %d", triage.QuickRef.ActionableCount)
	}

	// Blocker should appear in blockers_to_clear
	found := false
	for _, b := range triage.BlockersToClear {
		if b.ID == "blocker" && b.UnblocksCount == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected blocker to appear in blockers_to_clear")
	}
}

func TestComputeTriage_TopPicks(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "A", Status: model.StatusOpen, Priority: 2, UpdatedAt: time.Now()},
		{ID: "b", Title: "B", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
		{ID: "c", Title: "C", Status: model.StatusOpen, Priority: 0, UpdatedAt: time.Now()},
		{ID: "d", Title: "D", Status: model.StatusOpen, Priority: 3, UpdatedAt: time.Now()},
	}

	triage := ComputeTriage(issues)

	// Should have top picks
	if len(triage.QuickRef.TopPicks) == 0 {
		t.Error("expected top picks")
	}
	if len(triage.QuickRef.TopPicks) > 3 {
		t.Errorf("expected max 3 top picks, got %d", len(triage.QuickRef.TopPicks))
	}
}

func TestComputeTriageWithOptions(t *testing.T) {
	issues := make([]model.Issue, 20)
	for i := 0; i < 20; i++ {
		issues[i] = model.Issue{
			ID:        string(rune('a' + i)),
			Title:     "Issue " + string(rune('A'+i)),
			Status:    model.StatusOpen,
			Priority:  i % 4,
			UpdatedAt: time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		}
	}

	opts := TriageOptions{
		TopN:       5,
		QuickWinN:  3,
		BlockerN:   2,
	}

	triage := ComputeTriageWithOptions(issues, opts)

	if len(triage.Recommendations) > 5 {
		t.Errorf("expected max 5 recommendations, got %d", len(triage.Recommendations))
	}
	if len(triage.QuickWins) > 3 {
		t.Errorf("expected max 3 quick wins, got %d", len(triage.QuickWins))
	}
}

func TestTriageRecommendation_Action(t *testing.T) {
	// Issue in progress for a long time should suggest review
	issues := []model.Issue{
		{
			ID:        "stale-wip",
			Title:     "Stale work in progress",
			Status:    model.StatusInProgress,
			Priority:  1,
			UpdatedAt: time.Now().Add(-20 * 24 * time.Hour), // 20 days old
		},
	}

	triage := ComputeTriage(issues)

	if len(triage.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}

	rec := triage.Recommendations[0]
	if rec.Action != "review" {
		t.Errorf("expected action 'review' for stale in_progress, got %s", rec.Action)
	}
}

func TestTriageHealthCounts(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Status: model.StatusOpen, Priority: 0, IssueType: model.TypeBug},
		{ID: "2", Status: model.StatusOpen, Priority: 1, IssueType: model.TypeBug},
		{ID: "3", Status: model.StatusInProgress, Priority: 1, IssueType: model.TypeTask},
		{ID: "4", Status: model.StatusClosed, Priority: 2, IssueType: model.TypeFeature},
		{ID: "5", Status: model.StatusBlocked, Priority: 2, IssueType: model.TypeTask},
	}

	triage := ComputeTriage(issues)
	counts := triage.ProjectHealth.Counts

	if counts.ByType["bug"] != 2 {
		t.Errorf("expected 2 bugs, got %d", counts.ByType["bug"])
	}
	if counts.ByType["task"] != 2 {
		t.Errorf("expected 2 tasks, got %d", counts.ByType["task"])
	}
	if counts.ByPriority[1] != 2 {
		t.Errorf("expected 2 P1, got %d", counts.ByPriority[1])
	}
}

func TestTriageGraphHealth(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen},
		{ID: "b", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}}},
		{ID: "c", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "b", Type: model.DepBlocks}}},
	}

	triage := ComputeTriage(issues)
	graph := triage.ProjectHealth.Graph

	if graph.NodeCount != 3 {
		t.Errorf("expected 3 nodes, got %d", graph.NodeCount)
	}
	if graph.EdgeCount != 2 {
		t.Errorf("expected 2 edges, got %d", graph.EdgeCount)
	}
	if graph.HasCycles {
		t.Error("expected no cycles")
	}
}

func TestTriageWithCycles(t *testing.T) {
	// Create a cycle: a -> b -> a
	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "b", Type: model.DepBlocks}}},
		{ID: "b", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}}},
	}

	opts := TriageOptions{WaitForPhase2: true}
	triage := ComputeTriageWithOptions(issues, opts)
	graph := triage.ProjectHealth.Graph

	if !graph.HasCycles {
		t.Error("expected cycles to be detected")
	}
	if graph.CycleCount == 0 {
		t.Error("expected cycle count > 0")
	}
}

func TestTriageEmptyCommands(t *testing.T) {
	// When there are no open issues, commands should be gracefully handled
	issues := []model.Issue{
		{ID: "closed-1", Status: model.StatusClosed},
	}

	triage := ComputeTriage(issues)

	// Should not have "bd update  --status" (empty ID)
	if triage.Commands.ClaimTop == "bd update  --status=in_progress" {
		t.Error("ClaimTop should not have empty ID")
	}
	// Should have a fallback message
	if triage.Commands.ClaimTop == "" {
		t.Error("ClaimTop should not be empty")
	}
}

func TestTriageNoRecommendationsCommands(t *testing.T) {
	// Empty project
	triage := ComputeTriage(nil)

	// Commands should be valid even with no recommendations
	if triage.Commands.ListReady != "bd ready" {
		t.Errorf("expected 'bd ready', got %s", triage.Commands.ListReady)
	}
	// ClaimTop should have fallback, not empty ID
	if triage.Commands.ClaimTop == "bd update  --status=in_progress" {
		t.Error("ClaimTop should not have empty ID in command")
	}
}

func TestTriageInProgressAction(t *testing.T) {
	// Test the different staleness thresholds for in_progress items
	tests := []struct {
		name           string
		daysOld        int
		expectedAction string
	}{
		{"fresh in_progress", 5, "work"},      // < 9 days (0.3 * 30)
		{"moderate in_progress", 12, "review"}, // > 9 days, < 15 days
		{"stale in_progress", 20, "review"},    // > 15 days (0.5 * 30)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := []model.Issue{
				{
					ID:        "wip",
					Title:     tt.name,
					Status:    model.StatusInProgress,
					Priority:  1,
					UpdatedAt: time.Now().Add(-time.Duration(tt.daysOld) * 24 * time.Hour),
				},
			}

			triage := ComputeTriage(issues)
			if len(triage.Recommendations) == 0 {
				t.Fatal("expected recommendations")
			}

			rec := triage.Recommendations[0]
			if rec.Action != tt.expectedAction {
				t.Errorf("expected action %q, got %q (staleness: %.2f)",
					tt.expectedAction, rec.Action, rec.Breakdown.StalenessNorm)
			}
		})
	}
}

// ============================================================================
// Tests for bv-147 Triage Scoring
// ============================================================================

func TestComputeTriageScores_Empty(t *testing.T) {
	scores := ComputeTriageScores(nil)
	if scores != nil {
		t.Errorf("expected nil for empty issues, got %d scores", len(scores))
	}
}

func TestComputeTriageScores_BasicIssues(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "High priority", Status: model.StatusOpen, Priority: 0, UpdatedAt: time.Now()},
		{ID: "b", Title: "Medium priority", Status: model.StatusOpen, Priority: 2, UpdatedAt: time.Now()},
		{ID: "c", Title: "Low priority", Status: model.StatusOpen, Priority: 4, UpdatedAt: time.Now()},
	}

	scores := ComputeTriageScores(issues)

	if len(scores) != 3 {
		t.Errorf("expected 3 scores, got %d", len(scores))
	}

	// Should be sorted by triage score descending
	for i := 0; i < len(scores)-1; i++ {
		if scores[i].TriageScore < scores[i+1].TriageScore {
			t.Errorf("scores not sorted descending: %f < %f", scores[i].TriageScore, scores[i+1].TriageScore)
		}
	}

	// Check factors applied includes base and quick_win (no unblock for isolated issues)
	for _, score := range scores {
		hasBase := false
		for _, f := range score.FactorsApplied {
			if f == "base" {
				hasBase = true
			}
		}
		if !hasBase {
			t.Errorf("score for %s missing 'base' factor", score.IssueID)
		}
	}
}

func TestComputeTriageScores_WithUnblocks(t *testing.T) {
	// Create a chain: blocker -> blocked
	issues := []model.Issue{
		{ID: "blocker", Title: "Blocker", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
		{
			ID:       "blocked",
			Title:    "Blocked",
			Status:   model.StatusOpen,
			Priority: 0,
			UpdatedAt: time.Now(),
			Dependencies: []*model.Dependency{
				{DependsOnID: "blocker", Type: model.DepBlocks},
			},
		},
	}

	scores := ComputeTriageScores(issues)

	// Find the blocker score
	var blockerScore *TriageScore
	for i := range scores {
		if scores[i].IssueID == "blocker" {
			blockerScore = &scores[i]
			break
		}
	}

	if blockerScore == nil {
		t.Fatal("blocker score not found")
	}

	// Blocker should have unblock boost
	if blockerScore.TriageFactors.UnblockBoost <= 0 {
		t.Errorf("blocker should have positive unblock boost, got %f", blockerScore.TriageFactors.UnblockBoost)
	}

	// Check unblock is in factors applied
	hasUnblock := false
	for _, f := range blockerScore.FactorsApplied {
		if f == "unblock" {
			hasUnblock = true
		}
	}
	if !hasUnblock {
		t.Error("blocker should have 'unblock' in factors applied")
	}
}

func TestComputeTriageScores_QuickWin(t *testing.T) {
	// Issue with no blockers should get quick win boost
	issues := []model.Issue{
		{ID: "easy", Title: "Easy win", Status: model.StatusOpen, Priority: 2, UpdatedAt: time.Now()},
	}

	scores := ComputeTriageScores(issues)

	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}

	// Should have quick_win factor
	hasQuickWin := false
	for _, f := range scores[0].FactorsApplied {
		if f == "quick_win" {
			hasQuickWin = true
		}
	}
	if !hasQuickWin {
		t.Error("isolated issue should have 'quick_win' in factors applied")
	}
}

func TestComputeTriageScores_PendingFactors(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "Test", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
	}

	scores := ComputeTriageScores(issues)

	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}

	// Should have pending factors for features not yet enabled
	expectedPending := []string{"label_health", "claim_penalty", "attention_score"}
	for _, expected := range expectedPending {
		found := false
		for _, p := range scores[0].FactorsPending {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected '%s' in factors pending", expected)
		}
	}
}

func TestComputeTriageScoresWithOptions_CustomWeights(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "Test", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
	}

	opts := TriageScoringOptions{
		BaseScoreWeight:    0.5,
		UnblockBoostWeight: 0.25,
		QuickWinWeight:     0.25,
		UnblockThreshold:   3,
		QuickWinMaxDepth:   1,
	}

	scores := ComputeTriageScoresWithOptions(issues, opts)

	if len(scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(scores))
	}

	// Triage score should be different from base score (due to quick win)
	if scores[0].TriageScore == scores[0].BaseScore {
		t.Error("triage score should differ from base score when quick_win applied")
	}
}

func TestGetBlockerDepth_NoBlockers(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "No blockers", Status: model.StatusOpen},
	}
	analyzer := NewAnalyzer(issues)

	depth := analyzer.GetBlockerDepth("a")
	if depth != 0 {
		t.Errorf("expected depth 0 for issue with no blockers, got %d", depth)
	}
}

func TestGetBlockerDepth_OneLevel(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Title: "Blocker", Status: model.StatusOpen},
		{
			ID:           "b",
			Title:        "Blocked",
			Status:       model.StatusOpen,
			Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}},
		},
	}
	analyzer := NewAnalyzer(issues)

	depth := analyzer.GetBlockerDepth("b")
	if depth != 1 {
		t.Errorf("expected depth 1 for issue blocked by one, got %d", depth)
	}
}

func TestGetBlockerDepth_Cycle(t *testing.T) {
	issues := []model.Issue{
		{
			ID:           "a",
			Title:        "A",
			Status:       model.StatusOpen,
			Dependencies: []*model.Dependency{{DependsOnID: "b", Type: model.DepBlocks}},
		},
		{
			ID:           "b",
			Title:        "B",
			Status:       model.StatusOpen,
			Dependencies: []*model.Dependency{{DependsOnID: "a", Type: model.DepBlocks}},
		},
	}
	analyzer := NewAnalyzer(issues)

	depth := analyzer.GetBlockerDepth("a")
	if depth != -1 {
		t.Errorf("expected depth -1 for cyclic dependency, got %d", depth)
	}
}

func TestGetTopTriageScores(t *testing.T) {
	issues := []model.Issue{
		{ID: "a", Status: model.StatusOpen, Priority: 0, UpdatedAt: time.Now()},
		{ID: "b", Status: model.StatusOpen, Priority: 1, UpdatedAt: time.Now()},
		{ID: "c", Status: model.StatusOpen, Priority: 2, UpdatedAt: time.Now()},
		{ID: "d", Status: model.StatusOpen, Priority: 3, UpdatedAt: time.Now()},
		{ID: "e", Status: model.StatusOpen, Priority: 4, UpdatedAt: time.Now()},
	}

	top3 := GetTopTriageScores(issues, 3)

	if len(top3) != 3 {
		t.Errorf("expected 3 top scores, got %d", len(top3))
	}

	// Request more than available
	top10 := GetTopTriageScores(issues, 10)
	if len(top10) != 5 {
		t.Errorf("expected 5 scores when requesting 10 from 5, got %d", len(top10))
	}
}

func TestDefaultTriageScoringOptions(t *testing.T) {
	opts := DefaultTriageScoringOptions()

	// Weights should sum to 1.0
	totalWeight := opts.BaseScoreWeight + opts.UnblockBoostWeight + opts.QuickWinWeight
	if totalWeight != 1.0 {
		t.Errorf("weights should sum to 1.0, got %f", totalWeight)
	}

	// MVP mode: all optional features off
	if opts.EnableLabelHealth || opts.EnableClaimPenalty || opts.EnableAttentionScore {
		t.Error("MVP mode should have all optional features disabled")
	}
}

// ============================================================================
// Tests for bv-148 Reason Generation
// ============================================================================

func TestGenerateTriageReasons_EmptyContext(t *testing.T) {
	ctx := TriageReasonContext{}
	reasons := GenerateTriageReasons(ctx)

	if reasons.Primary == "" {
		t.Error("expected non-empty primary reason")
	}
	if len(reasons.All) == 0 {
		t.Error("expected at least one reason")
	}
	if reasons.ActionHint == "" {
		t.Error("expected non-empty action hint")
	}
}

func TestGenerateTriageReasons_UnblockCascade(t *testing.T) {
	ctx := TriageReasonContext{
		UnblocksIDs: []string{"bv-1", "bv-2", "bv-3", "bv-4", "bv-5"},
	}
	reasons := GenerateTriageReasons(ctx)

	// Should have unblock cascade as primary (>=3 unblocks)
	if reasons.Primary == "" {
		t.Error("expected primary reason for unblock cascade")
	}
	if len(reasons.All) < 2 {
		t.Errorf("expected at least 2 reasons (unblock + unclaimed), got %d", len(reasons.All))
	}

	// Check that primary contains unblock info
	foundUnblock := false
	for _, r := range reasons.All {
		if contains(r, "unblocks") || contains(r, "Unblocks") {
			foundUnblock = true
			break
		}
	}
	if !foundUnblock {
		t.Error("expected a reason about unblocking")
	}
}

func TestGenerateTriageReasons_LabelHealth(t *testing.T) {
	issue := &model.Issue{
		ID:     "test-1",
		Labels: []string{"backend", "database"},
	}
	ctx := TriageReasonContext{
		Issue: issue,
		LabelHealth: map[string]int{
			"backend":  45, // Below threshold
			"frontend": 80, // Above threshold
		},
	}
	reasons := GenerateTriageReasons(ctx)

	// Should have label attention reason for backend (health < 60)
	foundLabelReason := false
	for _, r := range reasons.All {
		if contains(r, "backend") && contains(r, "attention") {
			foundLabelReason = true
			break
		}
	}
	if !foundLabelReason {
		t.Error("expected reason about label 'backend' needing attention")
	}
}

func TestGenerateTriageReasons_Staleness(t *testing.T) {
	ctx := TriageReasonContext{
		DaysSinceUpdate: 15,
	}
	reasons := GenerateTriageReasons(ctx)

	// Should have staleness reason
	foundStale := false
	for _, r := range reasons.All {
		if contains(r, "15 days") || contains(r, "activity") {
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Error("expected reason about staleness")
	}
}

func TestGenerateTriageReasons_QuickWin(t *testing.T) {
	ctx := TriageReasonContext{
		IsQuickWin: true,
	}
	reasons := GenerateTriageReasons(ctx)

	// Should have quick win reason
	foundQuickWin := false
	for _, r := range reasons.All {
		if contains(r, "Low effort") || contains(r, "quick win") {
			foundQuickWin = true
			break
		}
	}
	if !foundQuickWin {
		t.Error("expected reason about quick win")
	}

	// Action hint should mention quick win
	if !contains(reasons.ActionHint, "Quick win") {
		t.Errorf("expected action hint to mention quick win, got: %s", reasons.ActionHint)
	}
}

func TestGenerateTriageReasons_ClaimStatus(t *testing.T) {
	// Unclaimed
	ctx := TriageReasonContext{
		ClaimedByAgent: "",
	}
	reasons := GenerateTriageReasons(ctx)

	foundUnclaimed := false
	for _, r := range reasons.All {
		if contains(r, "unclaimed") {
			foundUnclaimed = true
			break
		}
	}
	if !foundUnclaimed {
		t.Error("expected reason about being unclaimed")
	}

	// Claimed
	ctx2 := TriageReasonContext{
		ClaimedByAgent: "OtherAgent",
	}
	reasons2 := GenerateTriageReasons(ctx2)

	foundClaimed := false
	for _, r := range reasons2.All {
		if contains(r, "OtherAgent") {
			foundClaimed = true
			break
		}
	}
	if !foundClaimed {
		t.Error("expected reason about being claimed by OtherAgent")
	}
}

func TestGenerateTriageReasons_BlockedBy(t *testing.T) {
	ctx := TriageReasonContext{
		BlockedByIDs: []string{"bv-10", "bv-11"},
	}
	reasons := GenerateTriageReasons(ctx)

	foundBlocked := false
	for _, r := range reasons.All {
		if contains(r, "Blocked by") {
			foundBlocked = true
			break
		}
	}
	if !foundBlocked {
		t.Error("expected reason about being blocked")
	}

	// Action hint should mention working on blocker
	if !contains(reasons.ActionHint, "bv-10") {
		t.Errorf("expected action hint to mention first blocker, got: %s", reasons.ActionHint)
	}
}

func TestGenerateTriageReasons_HighPriority(t *testing.T) {
	ctx := TriageReasonContext{
		Issue: &model.Issue{
			ID:       "test-1",
			Priority: 0,
		},
	}
	reasons := GenerateTriageReasons(ctx)

	foundPriority := false
	for _, r := range reasons.All {
		if contains(r, "P0") || contains(r, "High priority") {
			foundPriority = true
			break
		}
	}
	if !foundPriority {
		t.Error("expected reason about high priority")
	}
}

func TestGenerateTriageReasons_GraphMetrics(t *testing.T) {
	ctx := TriageReasonContext{
		TriageScore: &TriageScore{
			Breakdown: ScoreBreakdown{
				PageRankNorm:    0.5,  // Above 0.3 threshold
				BetweennessNorm: 0.7,  // Above 0.5 threshold
			},
		},
	}
	reasons := GenerateTriageReasons(ctx)

	foundBottleneck := false
	foundCentrality := false
	for _, r := range reasons.All {
		if contains(r, "bottleneck") {
			foundBottleneck = true
		}
		if contains(r, "centrality") || contains(r, "PageRank") {
			foundCentrality = true
		}
	}
	if !foundBottleneck {
		t.Error("expected reason about being a bottleneck")
	}
	if !foundCentrality {
		t.Error("expected reason about high centrality")
	}
}

func TestFormatUnblockList_Empty(t *testing.T) {
	result := formatUnblockList(nil)
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

func TestFormatUnblockList_Short(t *testing.T) {
	result := formatUnblockList([]string{"bv-1", "bv-2"})
	if result != "bv-1, bv-2" {
		t.Errorf("expected 'bv-1, bv-2', got %q", result)
	}
}

func TestFormatUnblockList_Long(t *testing.T) {
	result := formatUnblockList([]string{"bv-1", "bv-2", "bv-3", "bv-4", "bv-5"})
	if !contains(result, "+3 more") {
		t.Errorf("expected '+3 more' in result, got %q", result)
	}
}

func TestGenerateTriageReasonsForScore(t *testing.T) {
	issues := []model.Issue{
		{
			ID:        "blocker",
			Title:     "Blocker",
			Status:    model.StatusOpen,
			Priority:  1,
			UpdatedAt: time.Now().Add(-10 * 24 * time.Hour), // 10 days old
		},
		{
			ID:       "blocked",
			Title:    "Blocked",
			Status:   model.StatusOpen,
			Priority: 2,
			UpdatedAt: time.Now(),
			Dependencies: []*model.Dependency{
				{DependsOnID: "blocker", Type: model.DepBlocks},
			},
		},
	}

	analyzer := NewAnalyzer(issues)
	unblocksMap := buildUnblocksMap(analyzer, issues)

	// Get triage scores
	scores := ComputeTriageScores(issues)

	// Find blocker score
	var blockerScore TriageScore
	for _, s := range scores {
		if s.IssueID == "blocker" {
			blockerScore = s
			break
		}
	}

	reasons := GenerateTriageReasonsForScore(blockerScore, analyzer, unblocksMap)

	// Should have reasons
	if len(reasons.All) == 0 {
		t.Error("expected at least one reason")
	}
	if reasons.Primary == "" {
		t.Error("expected non-empty primary reason")
	}
	if reasons.ActionHint == "" {
		t.Error("expected non-empty action hint")
	}
}

func TestEnhanceRecommendationWithTriageReasons(t *testing.T) {
	rec := &Recommendation{
		ID:      "test-1",
		Title:   "Test",
		Reasons: []string{"old reason"},
	}

	triageReasons := TriageReasons{
		Primary:    "🎯 Primary reason",
		All:        []string{"🎯 Primary reason", "📊 Secondary reason"},
		ActionHint: "Do this",
	}

	EnhanceRecommendationWithTriageReasons(rec, triageReasons)

	if len(rec.Reasons) != 2 {
		t.Errorf("expected 2 reasons after enhancement, got %d", len(rec.Reasons))
	}
	if rec.Reasons[0] != "🎯 Primary reason" {
		t.Errorf("expected first reason to be primary, got %s", rec.Reasons[0])
	}
}

func TestEnhanceRecommendationWithTriageReasons_NilRec(t *testing.T) {
	// Should not panic
	EnhanceRecommendationWithTriageReasons(nil, TriageReasons{})
}

// Helper function for tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
