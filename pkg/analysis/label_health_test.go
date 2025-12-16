package analysis

import (
	"fmt"
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

func TestHealthLevelFromScore(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, HealthLevelHealthy},
		{70, HealthLevelHealthy},
		{69, HealthLevelWarning},
		{40, HealthLevelWarning},
		{39, HealthLevelCritical},
		{0, HealthLevelCritical},
	}

	for _, tt := range tests {
		result := HealthLevelFromScore(tt.score)
		if result != tt.expected {
			t.Errorf("HealthLevelFromScore(%d) = %s, want %s", tt.score, result, tt.expected)
		}
	}
}

func TestComputeCompositeHealth(t *testing.T) {
	cfg := DefaultLabelHealthConfig()

	// All components at 100 should give 100
	score := ComputeCompositeHealth(100, 100, 100, 100, cfg)
	if score != 100 {
		t.Errorf("All 100s should give 100, got %d", score)
	}

	// All components at 0 should give 0
	score = ComputeCompositeHealth(0, 0, 0, 0, cfg)
	if score != 0 {
		t.Errorf("All 0s should give 0, got %d", score)
	}

	// All components at 50 should give 50
	score = ComputeCompositeHealth(50, 50, 50, 50, cfg)
	if score != 50 {
		t.Errorf("All 50s should give 50, got %d", score)
	}

	// Test weighted average
	// velocity=100, freshness=0, flow=100, criticality=0
	// With equal weights: (100*0.25 + 0*0.25 + 100*0.25 + 0*0.25) = 50
	score = ComputeCompositeHealth(100, 0, 100, 0, cfg)
	if score != 50 {
		t.Errorf("Expected 50 for alternating 100/0, got %d", score)
	}
}

func TestDefaultLabelHealthConfig(t *testing.T) {
	cfg := DefaultLabelHealthConfig()

	// Check weights sum to 1.0
	totalWeight := cfg.VelocityWeight + cfg.FreshnessWeight + cfg.FlowWeight + cfg.CriticalityWeight
	if totalWeight != 1.0 {
		t.Errorf("Weights should sum to 1.0, got %f", totalWeight)
	}

	// Check reasonable defaults
	if cfg.StaleThresholdDays != 14 {
		t.Errorf("Expected stale threshold of 14 days, got %d", cfg.StaleThresholdDays)
	}

	if cfg.MinIssuesForHealth != 1 {
		t.Errorf("Expected min issues of 1, got %d", cfg.MinIssuesForHealth)
	}
}

func TestNewLabelHealth(t *testing.T) {
	health := NewLabelHealth("test-label")

	if health.Label != "test-label" {
		t.Errorf("Expected label 'test-label', got '%s'", health.Label)
	}

	if health.Health != 100 {
		t.Errorf("New label should start with health 100, got %d", health.Health)
	}

	if health.HealthLevel != HealthLevelHealthy {
		t.Errorf("New label should be healthy, got %s", health.HealthLevel)
	}

	if health.Velocity.TrendDirection != "stable" {
		t.Errorf("Expected stable trend, got %s", health.Velocity.TrendDirection)
	}

	if health.Freshness.StaleThresholdDays != DefaultStaleThresholdDays {
		t.Errorf("Expected stale threshold %d, got %d", DefaultStaleThresholdDays, health.Freshness.StaleThresholdDays)
	}
}

func TestNeedsAttention(t *testing.T) {
	healthyLabel := LabelHealth{Health: 80}
	warningLabel := LabelHealth{Health: 50}
	criticalLabel := LabelHealth{Health: 30}

	if NeedsAttention(healthyLabel) {
		t.Error("Healthy label (80) should not need attention")
	}

	if !NeedsAttention(warningLabel) {
		t.Error("Warning label (50) should need attention")
	}

	if !NeedsAttention(criticalLabel) {
		t.Error("Critical label (30) should need attention")
	}
}

func TestLabelHealthTypes(t *testing.T) {
	// Test that all types can be instantiated and have expected structure
	velocity := VelocityMetrics{
		ClosedLast7Days:  5,
		ClosedLast30Days: 20,
		AvgDaysToClose:   3.5,
		TrendDirection:   "improving",
		TrendPercent:     15.0,
		VelocityScore:    80,
	}

	if velocity.ClosedLast7Days != 5 {
		t.Errorf("VelocityMetrics field mismatch")
	}

	freshness := FreshnessMetrics{
		AvgDaysSinceUpdate: 5.5,
		StaleCount:         2,
		StaleThresholdDays: 14,
		FreshnessScore:     70,
	}

	if freshness.StaleCount != 2 {
		t.Errorf("FreshnessMetrics field mismatch")
	}

	flow := FlowMetrics{
		IncomingDeps:      3,
		OutgoingDeps:      2,
		IncomingLabels:    []string{"api", "core"},
		OutgoingLabels:    []string{"ui"},
		BlockedByExternal: 1,
		BlockingExternal:  1,
		FlowScore:         85,
	}

	if len(flow.IncomingLabels) != 2 {
		t.Errorf("FlowMetrics labels mismatch")
	}

	criticality := CriticalityMetrics{
		AvgPageRank:       0.05,
		AvgBetweenness:    0.15,
		MaxBetweenness:    0.35,
		CriticalPathCount: 3,
		BottleneckCount:   1,
		CriticalityScore:  75,
	}

	if criticality.BottleneckCount != 1 {
		t.Errorf("CriticalityMetrics field mismatch")
	}
}

func TestCrossLabelFlowTypes(t *testing.T) {
	dep := LabelDependency{
		FromLabel:  "api",
		ToLabel:    "ui",
		IssueCount: 3,
		IssueIDs:   []string{"bv-1", "bv-2", "bv-3"},
		BlockingPairs: []BlockingPair{
			{BlockerID: "bv-1", BlockedID: "bv-4", BlockerLabel: "api", BlockedLabel: "ui"},
		},
	}

	if dep.FromLabel != "api" {
		t.Errorf("LabelDependency FromLabel mismatch")
	}

	if len(dep.BlockingPairs) != 1 {
		t.Errorf("Expected 1 blocking pair, got %d", len(dep.BlockingPairs))
	}

	flow := CrossLabelFlow{
		Labels:              []string{"api", "ui", "core"},
		FlowMatrix:          [][]int{{0, 3, 1}, {0, 0, 2}, {0, 0, 0}},
		Dependencies:        []LabelDependency{dep},
		BottleneckLabels:    []string{"api"},
		TotalCrossLabelDeps: 6,
	}

	if len(flow.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(flow.Labels))
	}

	if flow.TotalCrossLabelDeps != 6 {
		t.Errorf("Expected 6 cross-label deps, got %d", flow.TotalCrossLabelDeps)
	}
}

func TestComputeCrossLabelFlow(t *testing.T) {
	now := time.Now().UTC()
	cfg := DefaultLabelHealthConfig()
	issues := []model.Issue{
		{ID: "A", Labels: []string{"api"}, Status: model.StatusOpen},
		{ID: "B", Labels: []string{"ui"}, Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
		{ID: "C", Labels: []string{"api", "core"}, Status: model.StatusOpen},
		{ID: "D", Labels: []string{"ui", "core"}, Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "C", Type: model.DepBlocks}}},
		{ID: "E", Labels: []string{"api"}, Status: model.StatusClosed, Dependencies: []*model.Dependency{{DependsOnID: "A", Type: model.DepBlocks}}},
	}

	flow := ComputeCrossLabelFlow(issues, cfg)

	if flow.TotalCrossLabelDeps != 4 { // A->B (api->ui) plus C->D cross-product (api->ui, api->core, core->ui)
		t.Fatalf("expected 4 cross-label deps, got %d", flow.TotalCrossLabelDeps)
	}

	if len(flow.Labels) == 0 || flow.FlowMatrix == nil {
		t.Fatalf("expected labels and flow matrix to be populated")
	}

	// Ensure bottlenecks include api (highest outgoing)
	foundAPI := false
	for _, l := range flow.BottleneckLabels {
		if l == "api" {
			foundAPI = true
			break
		}
	}
	if !foundAPI {
		t.Fatalf("expected api in bottleneck labels")
	}

	// Ensure closed issue E is ignored in flow counts
	apiIdx := -1
	uiIdx := -1
	for i, l := range flow.Labels {
		if l == "api" {
			apiIdx = i
		}
		if l == "ui" {
			uiIdx = i
		}
	}
	if apiIdx == -1 || uiIdx == -1 {
		t.Fatalf("missing api/ui labels in flow")
	}
	if flow.FlowMatrix[apiIdx][uiIdx] != 2 { // A->B and C->D (api part) count
		t.Fatalf("expected api->ui count 2, got %d", flow.FlowMatrix[apiIdx][uiIdx])
	}

	_ = now // suppress unused if future additions use time
}

func TestLabelPath(t *testing.T) {
	path := LabelPath{
		Labels:      []string{"core", "api", "ui"},
		Length:      2,
		IssueCount:  5,
		TotalWeight: 12.5,
	}

	if path.Length != 2 {
		t.Errorf("Expected length 2, got %d", path.Length)
	}

	if len(path.Labels) != 3 {
		t.Errorf("Expected 3 labels in path, got %d", len(path.Labels))
	}
}

func TestLabelAnalysisResult(t *testing.T) {
	result := LabelAnalysisResult{
		TotalLabels:     5,
		HealthyCount:    3,
		WarningCount:    1,
		CriticalCount:   1,
		AttentionNeeded: []string{"blocked-label", "stale-label"},
	}

	if result.TotalLabels != 5 {
		t.Errorf("Expected 5 total labels, got %d", result.TotalLabels)
	}

	total := result.HealthyCount + result.WarningCount + result.CriticalCount
	if total != result.TotalLabels {
		t.Errorf("Health counts (%d) don't sum to total (%d)", total, result.TotalLabels)
	}

	if len(result.AttentionNeeded) != 2 {
		t.Errorf("Expected 2 labels needing attention, got %d", len(result.AttentionNeeded))
	}
}

// ============================================================================
// Label Extraction Tests (bv-101)
// ============================================================================

func TestExtractLabelsEmpty(t *testing.T) {
	result := ExtractLabels([]model.Issue{})

	if result.LabelCount != 0 {
		t.Errorf("Expected 0 labels for empty input, got %d", result.LabelCount)
	}
	if result.IssueCount != 0 {
		t.Errorf("Expected 0 issues for empty input, got %d", result.IssueCount)
	}
	if len(result.Stats) != 0 {
		t.Errorf("Expected empty stats map, got %d entries", len(result.Stats))
	}
}

func TestExtractLabelsBasic(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api", "bug"}, Status: model.StatusOpen, Priority: 1},
		{ID: "bv-2", Labels: []string{"api", "feature"}, Status: model.StatusClosed, Priority: 2},
		{ID: "bv-3", Labels: []string{"ui"}, Status: model.StatusInProgress, Priority: 1},
		{ID: "bv-4", Labels: []string{}, Status: model.StatusOpen, Priority: 3}, // No labels
	}

	result := ExtractLabels(issues)

	// Check counts
	if result.IssueCount != 4 {
		t.Errorf("Expected 4 issues, got %d", result.IssueCount)
	}
	if result.UnlabeledCount != 1 {
		t.Errorf("Expected 1 unlabeled issue, got %d", result.UnlabeledCount)
	}
	if result.LabelCount != 4 {
		t.Errorf("Expected 4 unique labels, got %d", result.LabelCount)
	}

	// Check labels are sorted
	expectedLabels := []string{"api", "bug", "feature", "ui"}
	for i, label := range expectedLabels {
		if result.Labels[i] != label {
			t.Errorf("Label %d: expected %s, got %s", i, label, result.Labels[i])
		}
	}

	// Check api label stats
	apiStats := result.Stats["api"]
	if apiStats == nil {
		t.Fatal("api label stats missing")
	}
	if apiStats.TotalCount != 2 {
		t.Errorf("api: expected 2 total, got %d", apiStats.TotalCount)
	}
	if apiStats.OpenCount != 1 {
		t.Errorf("api: expected 1 open, got %d", apiStats.OpenCount)
	}
	if apiStats.ClosedCount != 1 {
		t.Errorf("api: expected 1 closed, got %d", apiStats.ClosedCount)
	}

	// Check ui label stats
	uiStats := result.Stats["ui"]
	if uiStats == nil {
		t.Fatal("ui label stats missing")
	}
	if uiStats.InProgress != 1 {
		t.Errorf("ui: expected 1 in_progress, got %d", uiStats.InProgress)
	}

	// Check top labels (should be api first with 2 issues)
	if len(result.TopLabels) < 1 || result.TopLabels[0] != "api" {
		t.Errorf("Expected api as top label, got %v", result.TopLabels)
	}
}

func TestExtractLabelsDuplicateLabelsOnIssue(t *testing.T) {
	// Edge case: same label appears twice on an issue (shouldn't happen, but handle gracefully)
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api", "api"}, Status: model.StatusOpen}, // Duplicate
	}

	result := ExtractLabels(issues)

	// Both occurrences should be counted (total reflects raw label count per issue)
	if result.LabelCount != 1 {
		t.Errorf("Expected 1 unique label, got %d", result.LabelCount)
	}

	apiStats := result.Stats["api"]
	if apiStats.TotalCount != 2 {
		t.Errorf("Expected 2 counts for duplicate label, got %d", apiStats.TotalCount)
	}
}

func TestExtractLabelsEmptyLabelString(t *testing.T) {
	// Edge case: empty string label (should be skipped)
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"", "api", ""}, Status: model.StatusOpen},
	}

	result := ExtractLabels(issues)

	if result.LabelCount != 1 {
		t.Errorf("Expected 1 label (empty strings skipped), got %d", result.LabelCount)
	}
	if result.Labels[0] != "api" {
		t.Errorf("Expected api label, got %s", result.Labels[0])
	}
}

func TestGetLabelIssues(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api", "bug"}},
		{ID: "bv-2", Labels: []string{"api"}},
		{ID: "bv-3", Labels: []string{"ui"}},
	}

	apiIssues := GetLabelIssues(issues, "api")
	if len(apiIssues) != 2 {
		t.Errorf("Expected 2 api issues, got %d", len(apiIssues))
	}

	uiIssues := GetLabelIssues(issues, "ui")
	if len(uiIssues) != 1 {
		t.Errorf("Expected 1 ui issue, got %d", len(uiIssues))
	}

	noIssues := GetLabelIssues(issues, "nonexistent")
	if len(noIssues) != 0 {
		t.Errorf("Expected 0 issues for nonexistent label, got %d", len(noIssues))
	}
}

func TestGetLabelsForIssue(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api", "bug"}},
		{ID: "bv-2", Labels: []string{"ui"}},
	}

	labels := GetLabelsForIssue(issues, "bv-1")
	if len(labels) != 2 {
		t.Errorf("Expected 2 labels for bv-1, got %d", len(labels))
	}

	labels = GetLabelsForIssue(issues, "bv-999")
	if labels != nil {
		t.Errorf("Expected nil for nonexistent issue, got %v", labels)
	}
}

func TestGetCommonLabels(t *testing.T) {
	set1 := []string{"api", "bug", "feature"}
	set2 := []string{"api", "feature", "ui"}
	set3 := []string{"api", "core"}

	// Common to all three: only "api"
	common := GetCommonLabels(set1, set2, set3)
	if len(common) != 1 || common[0] != "api" {
		t.Errorf("Expected [api], got %v", common)
	}

	// Common to two: "api" and "feature"
	common = GetCommonLabels(set1, set2)
	if len(common) != 2 {
		t.Errorf("Expected 2 common labels, got %d", len(common))
	}

	// Empty input
	common = GetCommonLabels()
	if common != nil {
		t.Errorf("Expected nil for empty input, got %v", common)
	}
}

func TestGetLabelCooccurrence(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api", "bug"}},     // api+bug
		{ID: "bv-2", Labels: []string{"api", "bug"}},     // api+bug again
		{ID: "bv-3", Labels: []string{"api", "feature"}}, // api+feature
		{ID: "bv-4", Labels: []string{"ui"}},             // single label, no co-occurrence
	}

	cooc := GetLabelCooccurrence(issues)

	// api+bug should appear twice
	if cooc["api"]["bug"] != 2 {
		t.Errorf("Expected api+bug co-occurrence of 2, got %d", cooc["api"]["bug"])
	}
	if cooc["bug"]["api"] != 2 {
		t.Errorf("Expected bug+api co-occurrence of 2, got %d", cooc["bug"]["api"])
	}

	// api+feature should appear once
	if cooc["api"]["feature"] != 1 {
		t.Errorf("Expected api+feature co-occurrence of 1, got %d", cooc["api"]["feature"])
	}

	// ui has no co-occurrences
	if len(cooc["ui"]) != 0 {
		t.Errorf("Expected no co-occurrences for ui, got %v", cooc["ui"])
	}
}

func TestSortLabelsByCount(t *testing.T) {
	stats := map[string]*LabelStats{
		"api":     {Label: "api", TotalCount: 10},
		"bug":     {Label: "bug", TotalCount: 5},
		"feature": {Label: "feature", TotalCount: 10}, // Same as api
		"ui":      {Label: "ui", TotalCount: 3},
	}

	sorted := sortLabelsByCount(stats)

	// Should be sorted by count descending, then alphabetically for ties
	expected := []string{"api", "feature", "bug", "ui"}
	for i, label := range expected {
		if sorted[i] != label {
			t.Errorf("Position %d: expected %s, got %s", i, label, sorted[i])
		}
	}
}

// ============================================================================
// Velocity Metrics Tests (bv-102)
// ============================================================================

func TestComputeVelocityMetricsEmpty(t *testing.T) {
	now := time.Now()
	v := ComputeVelocityMetrics([]model.Issue{}, now)

	if v.ClosedLast7Days != 0 {
		t.Errorf("Expected 0 closed last 7 days, got %d", v.ClosedLast7Days)
	}
	if v.ClosedLast30Days != 0 {
		t.Errorf("Expected 0 closed last 30 days, got %d", v.ClosedLast30Days)
	}
	if v.AvgDaysToClose != 0 {
		t.Errorf("Expected 0 avg days to close, got %f", v.AvgDaysToClose)
	}
	if v.TrendDirection != "stable" {
		t.Errorf("Expected stable trend, got %s", v.TrendDirection)
	}
	if v.VelocityScore != 0 {
		t.Errorf("Expected velocity score 0, got %d", v.VelocityScore)
	}
}

func TestComputeVelocityMetricsWithClosures(t *testing.T) {
	now := time.Now()
	threeDaysAgo := now.Add(-3 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	twentyDaysAgo := now.Add(-20 * 24 * time.Hour)

	issues := []model.Issue{
		{ID: "1", CreatedAt: twentyDaysAgo, ClosedAt: &threeDaysAgo, Status: model.StatusClosed},  // Closed 3 days ago
		{ID: "2", CreatedAt: twentyDaysAgo, ClosedAt: &tenDaysAgo, Status: model.StatusClosed},    // Closed 10 days ago
		{ID: "3", CreatedAt: twentyDaysAgo, ClosedAt: &twentyDaysAgo, Status: model.StatusClosed}, // Closed 20 days ago
		{ID: "4", Status: model.StatusOpen}, // Open, no closure
	}

	v := ComputeVelocityMetrics(issues, now)

	// 1 closed in last 7 days
	if v.ClosedLast7Days != 1 {
		t.Errorf("Expected 1 closed last 7 days, got %d", v.ClosedLast7Days)
	}

	// 3 closed in last 30 days
	if v.ClosedLast30Days != 3 {
		t.Errorf("Expected 3 closed last 30 days, got %d", v.ClosedLast30Days)
	}

	// Velocity score should be positive
	if v.VelocityScore <= 0 {
		t.Errorf("Expected positive velocity score, got %d", v.VelocityScore)
	}
}

func TestComputeVelocityMetricsTrendImproving(t *testing.T) {
	now := time.Now()
	// Current week: 5 closures
	// Previous week: 2 closures
	// Should show improving trend

	var issues []model.Issue
	// 5 closures in current week (days 1-6)
	for i := 1; i <= 5; i++ {
		closedAt := now.Add(time.Duration(-i) * 24 * time.Hour)
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("cur-%d", i),
			CreatedAt: now.Add(-30 * 24 * time.Hour),
			ClosedAt:  &closedAt,
			Status:    model.StatusClosed,
		})
	}
	// 2 closures in previous week (days 8-10)
	for i := 8; i <= 9; i++ {
		closedAt := now.Add(time.Duration(-i) * 24 * time.Hour)
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("prev-%d", i),
			CreatedAt: now.Add(-30 * 24 * time.Hour),
			ClosedAt:  &closedAt,
			Status:    model.StatusClosed,
		})
	}

	v := ComputeVelocityMetrics(issues, now)

	if v.TrendDirection != "improving" {
		t.Errorf("Expected improving trend (5 vs 2), got %s", v.TrendDirection)
	}
	if v.TrendPercent <= 0 {
		t.Errorf("Expected positive trend percent, got %f", v.TrendPercent)
	}
}

func TestComputeVelocityMetricsTrendDeclining(t *testing.T) {
	now := time.Now()
	// Current week: 1 closure
	// Previous week: 5 closures
	// Should show declining trend

	var issues []model.Issue
	// 1 closure in current week
	closedAt := now.Add(-2 * 24 * time.Hour)
	issues = append(issues, model.Issue{
		ID:        "cur-1",
		CreatedAt: now.Add(-30 * 24 * time.Hour),
		ClosedAt:  &closedAt,
		Status:    model.StatusClosed,
	})
	// 5 closures in previous week
	for i := 8; i <= 12; i++ {
		closedAt := now.Add(time.Duration(-i) * 24 * time.Hour)
		issues = append(issues, model.Issue{
			ID:        fmt.Sprintf("prev-%d", i),
			CreatedAt: now.Add(-30 * 24 * time.Hour),
			ClosedAt:  &closedAt,
			Status:    model.StatusClosed,
		})
	}

	v := ComputeVelocityMetrics(issues, now)

	if v.TrendDirection != "declining" {
		t.Errorf("Expected declining trend (1 vs 5), got %s", v.TrendDirection)
	}
	if v.TrendPercent >= 0 {
		t.Errorf("Expected negative trend percent, got %f", v.TrendPercent)
	}
}

func TestComputeVelocityMetricsAvgDaysToClose(t *testing.T) {
	now := time.Now()
	// Create issues with known time to close
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	fifteenDaysAgo := now.Add(-15 * 24 * time.Hour)

	issues := []model.Issue{
		{ID: "1", CreatedAt: tenDaysAgo, ClosedAt: &fiveDaysAgo, Status: model.StatusClosed},     // 5 days to close
		{ID: "2", CreatedAt: fifteenDaysAgo, ClosedAt: &fiveDaysAgo, Status: model.StatusClosed}, // 10 days to close
	}

	v := ComputeVelocityMetrics(issues, now)

	// Average should be (5 + 10) / 2 = 7.5 days
	expectedAvg := 7.5
	if v.AvgDaysToClose < expectedAvg-0.1 || v.AvgDaysToClose > expectedAvg+0.1 {
		t.Errorf("Expected avg days to close ~%.1f, got %.1f", expectedAvg, v.AvgDaysToClose)
	}
}

// ============================================================================
// Freshness Metrics Tests (bv-102)
// ============================================================================

func TestComputeFreshnessMetricsEmpty(t *testing.T) {
	now := time.Now()
	f := ComputeFreshnessMetrics([]model.Issue{}, now, 14)

	if f.StaleCount != 0 {
		t.Errorf("Expected 0 stale count, got %d", f.StaleCount)
	}
	if f.AvgDaysSinceUpdate != 0 {
		t.Errorf("Expected 0 avg days since update, got %f", f.AvgDaysSinceUpdate)
	}
	if f.StaleThresholdDays != 14 {
		t.Errorf("Expected stale threshold 14, got %d", f.StaleThresholdDays)
	}
}

func TestComputeFreshnessMetricsWithUpdates(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.Add(-1 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	twentyDaysAgo := now.Add(-20 * 24 * time.Hour)

	issues := []model.Issue{
		{ID: "1", UpdatedAt: oneDayAgo, Status: model.StatusOpen},     // Fresh
		{ID: "2", UpdatedAt: tenDaysAgo, Status: model.StatusOpen},    // Not stale (< 14 days)
		{ID: "3", UpdatedAt: twentyDaysAgo, Status: model.StatusOpen}, // Stale (> 14 days)
	}

	f := ComputeFreshnessMetrics(issues, now, 14)

	// 1 stale issue (20 days > 14 days threshold)
	if f.StaleCount != 1 {
		t.Errorf("Expected 1 stale issue, got %d", f.StaleCount)
	}

	// Most recent should be the 1-day-ago update
	if !f.MostRecentUpdate.Equal(oneDayAgo) {
		t.Errorf("Expected most recent update %v, got %v", oneDayAgo, f.MostRecentUpdate)
	}

	// Freshness score should be > 0 (not all stale)
	if f.FreshnessScore <= 0 {
		t.Errorf("Expected positive freshness score, got %d", f.FreshnessScore)
	}
}

func TestComputeFreshnessMetricsOldestOpen(t *testing.T) {
	now := time.Now()
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	twentyDaysAgo := now.Add(-20 * 24 * time.Hour)

	issues := []model.Issue{
		{ID: "1", CreatedAt: fiveDaysAgo, UpdatedAt: fiveDaysAgo, Status: model.StatusOpen},
		{ID: "2", CreatedAt: twentyDaysAgo, UpdatedAt: tenDaysAgo, Status: model.StatusOpen}, // Oldest open
		{ID: "3", CreatedAt: tenDaysAgo, UpdatedAt: tenDaysAgo, Status: model.StatusClosed},  // Closed, shouldn't count
	}

	f := ComputeFreshnessMetrics(issues, now, 14)

	// Oldest open should be the 20-day-old issue
	if !f.OldestOpenIssue.Equal(twentyDaysAgo) {
		t.Errorf("Expected oldest open %v, got %v", twentyDaysAgo, f.OldestOpenIssue)
	}
}

func TestComputeFreshnessMetricsDefaultThreshold(t *testing.T) {
	now := time.Now()
	// Pass 0 or negative threshold - should use default
	f := ComputeFreshnessMetrics([]model.Issue{}, now, 0)

	if f.StaleThresholdDays != DefaultStaleThresholdDays {
		t.Errorf("Expected default threshold %d, got %d", DefaultStaleThresholdDays, f.StaleThresholdDays)
	}

	f = ComputeFreshnessMetrics([]model.Issue{}, now, -5)
	if f.StaleThresholdDays != DefaultStaleThresholdDays {
		t.Errorf("Expected default threshold for negative input, got %d", f.StaleThresholdDays)
	}
}

func TestComputeFreshnessMetricsScoreCapping(t *testing.T) {
	now := time.Now()
	// Very fresh issues should give high score
	justNow := now.Add(-1 * time.Hour)
	issues := []model.Issue{
		{ID: "1", UpdatedAt: justNow, Status: model.StatusOpen},
	}

	f := ComputeFreshnessMetrics(issues, now, 14)

	// Score should be close to 100 for very fresh
	if f.FreshnessScore < 90 {
		t.Errorf("Expected high freshness score for fresh issue, got %d", f.FreshnessScore)
	}

	// Very stale issues should give low score
	veryOld := now.Add(-60 * 24 * time.Hour)
	staleIssues := []model.Issue{
		{ID: "1", UpdatedAt: veryOld, Status: model.StatusOpen},
	}

	f = ComputeFreshnessMetrics(staleIssues, now, 14)

	// Score should be 0 for very stale (> 2x threshold)
	if f.FreshnessScore != 0 {
		t.Errorf("Expected 0 freshness score for very stale issue, got %d", f.FreshnessScore)
	}
}

// ============================================================================
// Label Subgraph Extraction Tests (bv-113)
// ============================================================================

func TestComputeLabelSubgraphEmpty(t *testing.T) {
	// Empty issues
	sg := ComputeLabelSubgraph([]model.Issue{}, "api")
	if !sg.IsEmpty() {
		t.Errorf("Expected empty subgraph for empty issues, got %d issues", sg.IssueCount)
	}

	// Empty label
	issues := []model.Issue{{ID: "bv-1", Labels: []string{"api"}}}
	sg = ComputeLabelSubgraph(issues, "")
	if !sg.IsEmpty() {
		t.Errorf("Expected empty subgraph for empty label, got %d issues", sg.IssueCount)
	}
}

func TestComputeLabelSubgraphSingleLabel(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{ID: "bv-2", Labels: []string{"api"}},
		{ID: "bv-3", Labels: []string{"ui"}}, // Different label, not included
	}

	sg := ComputeLabelSubgraph(issues, "api")

	if sg.CoreCount != 2 {
		t.Errorf("Expected 2 core issues, got %d", sg.CoreCount)
	}
	if sg.IssueCount != 2 {
		t.Errorf("Expected 2 total issues (no deps), got %d", sg.IssueCount)
	}
	if len(sg.DependencyIssues) != 0 {
		t.Errorf("Expected 0 dependency issues, got %d", len(sg.DependencyIssues))
	}
}

func TestComputeLabelSubgraphWithDependencies(t *testing.T) {
	// bv-1 (api) is blocked by bv-3 (core)
	// bv-2 (api) blocks bv-4 (ui)
	issues := []model.Issue{
		{
			ID:     "bv-1",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-3", Type: model.DepBlocks},
			},
		},
		{ID: "bv-2", Labels: []string{"api"}},
		{ID: "bv-3", Labels: []string{"core"}}, // Blocker, not in api label
		{
			ID:     "bv-4",
			Labels: []string{"ui"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-2", Type: model.DepBlocks}, // Blocked by bv-2 (api)
			},
		},
	}

	sg := ComputeLabelSubgraph(issues, "api")

	// Core issues: bv-1, bv-2
	if sg.CoreCount != 2 {
		t.Errorf("Expected 2 core issues, got %d", sg.CoreCount)
	}
	expectedCore := []string{"bv-1", "bv-2"}
	for i, id := range expectedCore {
		if sg.CoreIssues[i] != id {
			t.Errorf("CoreIssues[%d]: expected %s, got %s", i, id, sg.CoreIssues[i])
		}
	}

	// Dependency issues: bv-3 (blocker of bv-1), bv-4 (blocked by bv-2)
	if len(sg.DependencyIssues) != 2 {
		t.Errorf("Expected 2 dependency issues, got %d: %v", len(sg.DependencyIssues), sg.DependencyIssues)
	}

	// Total: 4 issues in subgraph
	if sg.IssueCount != 4 {
		t.Errorf("Expected 4 total issues, got %d", sg.IssueCount)
	}

	// Edge: bv-3 -> bv-1 (bv-3 blocks bv-1)
	if sg.OutDegree["bv-3"] != 1 {
		t.Errorf("Expected bv-3 out-degree 1, got %d", sg.OutDegree["bv-3"])
	}
	if sg.InDegree["bv-1"] != 1 {
		t.Errorf("Expected bv-1 in-degree 1, got %d", sg.InDegree["bv-1"])
	}

	// Edge: bv-2 -> bv-4 (bv-2 blocks bv-4)
	if sg.OutDegree["bv-2"] != 1 {
		t.Errorf("Expected bv-2 out-degree 1, got %d", sg.OutDegree["bv-2"])
	}
	if sg.InDegree["bv-4"] != 1 {
		t.Errorf("Expected bv-4 in-degree 1, got %d", sg.InDegree["bv-4"])
	}
}

func TestComputeLabelSubgraphRootsAndLeaves(t *testing.T) {
	// Chain: bv-1 -> bv-2 -> bv-3 (all api)
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "bv-3",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-2", Type: model.DepBlocks},
			},
		},
	}

	sg := ComputeLabelSubgraph(issues, "api")

	// bv-1 has no blockers (root)
	roots := sg.GetSubgraphRoots()
	if len(roots) != 1 || roots[0] != "bv-1" {
		t.Errorf("Expected roots [bv-1], got %v", roots)
	}

	// bv-3 doesn't block anything (leaf)
	leaves := sg.GetSubgraphLeaves()
	if len(leaves) != 1 || leaves[0] != "bv-3" {
		t.Errorf("Expected leaves [bv-3], got %v", leaves)
	}
}

func TestComputeLabelSubgraphAdjacency(t *testing.T) {
	// bv-1 blocks bv-2 and bv-3
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "bv-3",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
	}

	sg := ComputeLabelSubgraph(issues, "api")

	// Check adjacency: bv-1 -> [bv-2, bv-3]
	adj := sg.Adjacency["bv-1"]
	if len(adj) != 2 {
		t.Errorf("Expected bv-1 to have 2 adjacencies, got %d", len(adj))
	}
	// Adjacency list should be sorted
	if adj[0] != "bv-2" || adj[1] != "bv-3" {
		t.Errorf("Expected bv-1 adjacency [bv-2, bv-3], got %v", adj)
	}

	// Total edge count
	if sg.EdgeCount != 2 {
		t.Errorf("Expected 2 edges, got %d", sg.EdgeCount)
	}
}

func TestComputeLabelSubgraphCoreIssueSet(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{ID: "bv-2", Labels: []string{"api"}},
		{ID: "bv-3", Labels: []string{"ui"}},
	}

	sg := ComputeLabelSubgraph(issues, "api")
	coreSet := sg.GetCoreIssueSet()

	if !coreSet["bv-1"] {
		t.Error("Expected bv-1 in core set")
	}
	if !coreSet["bv-2"] {
		t.Error("Expected bv-2 in core set")
	}
	if coreSet["bv-3"] {
		t.Error("bv-3 should not be in core set")
	}
}

func TestComputeLabelSubgraphNonBlockingDeps(t *testing.T) {
	// Non-blocking dependencies should not be included in adjacency
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepRelated}, // Related, not blocking
			},
		},
	}

	sg := ComputeLabelSubgraph(issues, "api")

	// No edges since dependency is "related" not "blocks"
	if sg.EdgeCount != 0 {
		t.Errorf("Expected 0 edges for non-blocking deps, got %d", sg.EdgeCount)
	}
}

func TestHasLabel(t *testing.T) {
	issue := model.Issue{
		ID:     "bv-1",
		Labels: []string{"api", "bug", "urgent"},
	}

	if !HasLabel(issue, "api") {
		t.Error("Expected HasLabel to return true for 'api'")
	}
	if !HasLabel(issue, "bug") {
		t.Error("Expected HasLabel to return true for 'bug'")
	}
	if HasLabel(issue, "feature") {
		t.Error("Expected HasLabel to return false for 'feature'")
	}
	if HasLabel(issue, "") {
		t.Error("Expected HasLabel to return false for empty string")
	}
}

// ============================================================================
// Label-Specific PageRank Tests (bv-114)
// ============================================================================

func TestComputeLabelPageRankEmpty(t *testing.T) {
	// Empty subgraph
	sg := ComputeLabelSubgraph([]model.Issue{}, "api")
	result := ComputeLabelPageRank(sg)

	if result.IssueCount != 0 {
		t.Errorf("Expected 0 issues, got %d", result.IssueCount)
	}
	if len(result.Scores) != 0 {
		t.Errorf("Expected empty scores, got %d", len(result.Scores))
	}
	if len(result.TopIssues) != 0 {
		t.Errorf("Expected empty top issues, got %d", len(result.TopIssues))
	}
}

func TestComputeLabelPageRankSingleNode(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
	}

	result := ComputeLabelPageRankFromIssues(issues, "api")

	if result.IssueCount != 1 {
		t.Errorf("Expected 1 issue, got %d", result.IssueCount)
	}
	if result.CoreCount != 1 {
		t.Errorf("Expected 1 core issue, got %d", result.CoreCount)
	}
	if len(result.Scores) != 1 {
		t.Errorf("Expected 1 score, got %d", len(result.Scores))
	}
	// Single node should have PageRank of 1.0 (all probability mass)
	if result.Scores["bv-1"] < 0.9 {
		t.Errorf("Expected single node PageRank ~1.0, got %f", result.Scores["bv-1"])
	}
}

func TestComputeLabelPageRankChain(t *testing.T) {
	// Chain: bv-1 blocks bv-2 blocks bv-3 (all api)
	// In PageRank, nodes with more incoming links have higher scores
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "bv-3",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-2", Type: model.DepBlocks},
			},
		},
	}

	result := ComputeLabelPageRankFromIssues(issues, "api")

	if result.IssueCount != 3 {
		t.Errorf("Expected 3 issues, got %d", result.IssueCount)
	}

	// All three should have scores
	if len(result.Scores) != 3 {
		t.Errorf("Expected 3 scores, got %d", len(result.Scores))
	}

	// Top issues should be sorted by score
	if len(result.TopIssues) != 3 {
		t.Errorf("Expected 3 top issues, got %d", len(result.TopIssues))
	}
	// First rank should be 1
	if result.TopIssues[0].Rank != 1 {
		t.Errorf("Expected first issue to have rank 1, got %d", result.TopIssues[0].Rank)
	}
}

func TestComputeLabelPageRankNormalized(t *testing.T) {
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
	}

	result := ComputeLabelPageRankFromIssues(issues, "api")

	// Normalized scores should be between 0 and 1
	for id, norm := range result.Normalized {
		if norm < 0 || norm > 1 {
			t.Errorf("Normalized score for %s out of range: %f", id, norm)
		}
	}

	// With varying scores, one should be at 1.0 (max) and one at 0.0 (min)
	if result.MaxScore != result.MinScore {
		foundMax := false
		foundMin := false
		for _, norm := range result.Normalized {
			if norm == 1.0 {
				foundMax = true
			}
			if norm == 0.0 {
				foundMin = true
			}
		}
		if !foundMax {
			t.Error("Expected one normalized score to be 1.0")
		}
		if !foundMin {
			t.Error("Expected one normalized score to be 0.0")
		}
	}
}

func TestComputeLabelPageRankCoreVsDep(t *testing.T) {
	// bv-1 (api) is blocked by bv-2 (core - not api label)
	issues := []model.Issue{
		{
			ID:     "bv-1",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-2", Type: model.DepBlocks},
			},
		},
		{ID: "bv-2", Labels: []string{"core"}}, // Dependency, not core
	}

	result := ComputeLabelPageRankFromIssues(issues, "api")

	// Should have 2 issues total (1 core + 1 dep)
	if result.IssueCount != 2 {
		t.Errorf("Expected 2 issues, got %d", result.IssueCount)
	}
	if result.CoreCount != 1 {
		t.Errorf("Expected 1 core issue, got %d", result.CoreCount)
	}

	// CoreOnly should only have bv-1
	if len(result.CoreOnly) != 1 {
		t.Errorf("Expected 1 core-only score, got %d", len(result.CoreOnly))
	}
	if _, ok := result.CoreOnly["bv-1"]; !ok {
		t.Error("Expected bv-1 in CoreOnly")
	}
	if _, ok := result.CoreOnly["bv-2"]; ok {
		t.Error("bv-2 should not be in CoreOnly")
	}

	// TopIssues should mark IsCore correctly
	for _, ri := range result.TopIssues {
		if ri.ID == "bv-1" && !ri.IsCore {
			t.Error("bv-1 should be marked as IsCore")
		}
		if ri.ID == "bv-2" && ri.IsCore {
			t.Error("bv-2 should not be marked as IsCore")
		}
	}
}

func TestComputeLabelPageRankGetTopCoreIssues(t *testing.T) {
	// 3 core issues, 1 dependency
	issues := []model.Issue{
		{ID: "bv-1", Labels: []string{"api"}},
		{
			ID:     "bv-2",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-1", Type: model.DepBlocks},
			},
		},
		{
			ID:     "bv-3",
			Labels: []string{"api"},
			Dependencies: []*model.Dependency{
				{DependsOnID: "bv-4", Type: model.DepBlocks},
			},
		},
		{ID: "bv-4", Labels: []string{"core"}}, // Not api
	}

	result := ComputeLabelPageRankFromIssues(issues, "api")

	// Get top 2 core issues
	topCore := result.GetTopCoreIssues(2)
	if len(topCore) != 2 {
		t.Errorf("Expected 2 top core issues, got %d", len(topCore))
	}

	// All returned should be core
	for _, ri := range topCore {
		if !ri.IsCore {
			t.Errorf("Expected IsCore=true for %s", ri.ID)
		}
	}

	// Get all core issues (more than exist)
	allCore := result.GetTopCoreIssues(10)
	if len(allCore) != 3 {
		t.Errorf("Expected 3 core issues when asking for 10, got %d", len(allCore))
	}
}
