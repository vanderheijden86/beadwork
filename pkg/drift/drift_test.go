package drift

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beads_viewer/pkg/baseline"
	"gopkg.in/yaml.v3"
)

func TestCalculatorNoDrift(t *testing.T) {
	bl := &baseline.Baseline{
		Version:   1,
		CreatedAt: time.Now(),
		Stats: baseline.GraphStats{
			NodeCount:       100,
			EdgeCount:       200,
			Density:         0.02,
			OpenCount:       50,
			ClosedCount:     40,
			BlockedCount:    10,
			CycleCount:      0,
			ActionableCount: 40,
		},
	}

	// Current matches baseline
	current := &baseline.Baseline{
		Version:   1,
		CreatedAt: time.Now(),
		Stats:     bl.Stats,
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	if result.HasDrift {
		t.Errorf("expected no drift, got %d alerts", len(result.Alerts))
	}
}

func TestCalculatorNewCycle(t *testing.T) {
	bl := &baseline.Baseline{
		Stats:  baseline.GraphStats{NodeCount: 10, EdgeCount: 15},
		Cycles: [][]string{},
	}

	current := &baseline.Baseline{
		Stats:  bl.Stats,
		Cycles: [][]string{{"A", "B", "C", "A"}},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	if !result.HasDrift {
		t.Error("expected drift to be detected")
	}

	if result.CriticalCount != 1 {
		t.Errorf("expected 1 critical alert, got %d", result.CriticalCount)
	}

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertNewCycle {
			found = true
			if alert.Severity != SeverityCritical {
				t.Errorf("new cycle should be critical, got %s", alert.Severity)
			}
		}
	}
	if !found {
		t.Error("expected new_cycle alert")
	}
}

func TestCalculatorDensityGrowth(t *testing.T) {
	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			EdgeCount: 200,
			Density:   0.02,
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			EdgeCount: 400,
			Density:   0.04, // 100% increase - definitely above 50% warning threshold
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			found = true
			if alert.Severity != SeverityWarning {
				t.Errorf("100%% density increase should be warning, got %s", alert.Severity)
			}
		}
	}
	if !found {
		t.Error("expected density_growth alert")
	}
}

func TestCalculatorBlockedIncrease(t *testing.T) {
	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:    100,
			BlockedCount: 5,
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:    100,
			BlockedCount: 15, // +10
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertBlockedIncrease {
			found = true
			if alert.Severity != SeverityWarning {
				t.Errorf("blocked increase should be warning, got %s", alert.Severity)
			}
		}
	}
	if !found {
		t.Error("expected blocked_increase alert")
	}
}

func TestCalculatorPageRankChange(t *testing.T) {
	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{NodeCount: 100},
		TopMetrics: baseline.TopMetrics{
			PageRank: []baseline.MetricItem{
				{ID: "TASK-1", Value: 0.2},
				{ID: "TASK-2", Value: 0.15},
			},
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{NodeCount: 100},
		TopMetrics: baseline.TopMetrics{
			PageRank: []baseline.MetricItem{
				{ID: "TASK-1", Value: 0.35}, // 75% increase
				{ID: "TASK-3", Value: 0.18}, // New entry
			},
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertPageRankChange {
			found = true
		}
	}
	if !found {
		t.Error("expected pagerank_change alert")
	}
}

func TestResultSummary(t *testing.T) {
	result := &Result{
		HasDrift: true,
		Alerts: []Alert{
			{Type: AlertNewCycle, Severity: SeverityCritical, Message: "New cycle"},
			{Type: AlertDensityGrowth, Severity: SeverityWarning, Message: "Density up"},
		},
		CriticalCount: 1,
		WarningCount:  1,
	}

	summary := result.Summary()

	if !strings.Contains(summary, "CRITICAL") {
		t.Error("summary should mention critical")
	}
	if !strings.Contains(summary, "WARNING") {
		t.Error("summary should mention warning")
	}
}

func TestResultExitCode(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		expected int
	}{
		{"no drift", &Result{}, 0},
		{"info only", &Result{HasDrift: true, InfoCount: 1}, 0},
		{"warning", &Result{HasDrift: true, WarningCount: 1}, 2},
		{"critical", &Result{HasDrift: true, CriticalCount: 1}, 1},
		{"critical and warning", &Result{HasDrift: true, CriticalCount: 1, WarningCount: 1}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.ExitCode(); got != tt.expected {
				t.Errorf("ExitCode() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestResultHasCritical(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{"no alerts", &Result{}, false},
		{"info only", &Result{InfoCount: 5}, false},
		{"warning only", &Result{WarningCount: 3}, false},
		{"critical", &Result{CriticalCount: 1}, true},
		{"critical with others", &Result{CriticalCount: 2, WarningCount: 1, InfoCount: 3}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasCritical(); got != tt.expected {
				t.Errorf("HasCritical() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResultHasWarnings(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{"no alerts", &Result{}, false},
		{"info only", &Result{InfoCount: 5}, false},
		{"warning only", &Result{WarningCount: 1}, true},
		{"critical only", &Result{CriticalCount: 1}, true},
		{"warning and info", &Result{WarningCount: 2, InfoCount: 3}, true},
		{"critical and warning", &Result{CriticalCount: 1, WarningCount: 2}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasWarnings(); got != tt.expected {
				t.Errorf("HasWarnings() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExampleConfig(t *testing.T) {
	example := ExampleConfig()

	// Should not be empty
	if example == "" {
		t.Error("ExampleConfig() returned empty string")
	}

	// Should be valid YAML that can be parsed
	var config Config
	if err := yaml.Unmarshal([]byte(example), &config); err != nil {
		t.Errorf("ExampleConfig() returned invalid YAML: %v", err)
	}

	// Should contain expected keys
	expectedKeys := []string{
		"density_warning_pct",
		"density_info_pct",
		"blocked_increase_threshold",
		"pagerank_change_warning_pct",
	}
	for _, key := range expectedKeys {
		if !strings.Contains(example, key) {
			t.Errorf("ExampleConfig() should contain %q", key)
		}
	}

	// Parsed config should have reasonable values
	if config.DensityWarningPct <= 0 {
		t.Error("ExampleConfig() density_warning_pct should be positive")
	}
	if config.BlockedIncreaseThreshold < 0 {
		t.Error("ExampleConfig() blocked_increase_threshold should be non-negative")
	}
}

func TestConfigLoadDefault(t *testing.T) {
	tmpDir := t.TempDir()

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should return defaults
	if config.DensityWarningPct != 50 {
		t.Errorf("expected default density_warning_pct=50, got %f", config.DensityWarningPct)
	}
}

func TestConfigLoadCustom(t *testing.T) {
	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `
density_warning_pct: 75
blocked_increase_threshold: 10
`
	if err := os.WriteFile(filepath.Join(bvDir, "drift.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.DensityWarningPct != 75 {
		t.Errorf("expected density_warning_pct=75, got %f", config.DensityWarningPct)
	}
	if config.BlockedIncreaseThreshold != 10 {
		t.Errorf("expected blocked_increase_threshold=10, got %d", config.BlockedIncreaseThreshold)
	}
}

func TestConfigLoadInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Invalid config: negative density warning
	configContent := `density_warning_pct: -50`
	if err := os.WriteFile(filepath.Join(bvDir, "drift.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	}
}

// TestConfigLoadInvalidYAML tests loading a file with invalid YAML syntax
func TestConfigLoadInvalidYAML(t *testing.T) {
	t.Log("Testing LoadConfig with invalid YAML syntax")

	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write invalid YAML (bad indentation/syntax)
	invalidYAML := `density_warning_pct: 50
  bad_indentation: true
    this_is_invalid`
	configPath := filepath.Join(bvDir, "drift.yaml")
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Created invalid YAML file at: %s", configPath)

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
		if !strings.Contains(err.Error(), "parsing") {
			t.Errorf("error should mention parsing, got: %v", err)
		}
	}
}

// TestConfigLoadPermissionError tests LoadConfig with an unreadable file
func TestConfigLoadPermissionError(t *testing.T) {
	// Skip on systems where we can't reliably test permissions
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	t.Log("Testing LoadConfig with permission denied")

	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create file with no read permissions
	configPath := filepath.Join(bvDir, "drift.yaml")
	if err := os.WriteFile(configPath, []byte("density_warning_pct: 50"), 0000); err != nil {
		t.Fatal(err)
	}
	t.Logf("Created unreadable file at: %s", configPath)

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for unreadable file, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
		if !strings.Contains(err.Error(), "reading") {
			t.Errorf("error should mention reading, got: %v", err)
		}
	}

	// Cleanup: restore permissions so temp dir can be removed
	os.Chmod(configPath, 0644)
}

// TestConfigSaveInvalidConfig tests SaveConfig with a config that fails validation
func TestConfigSaveInvalidConfig(t *testing.T) {
	t.Log("Testing SaveConfig with invalid config")

	tmpDir := t.TempDir()

	// Create invalid config
	invalidConfig := &Config{
		DensityWarningPct: -100, // Invalid: negative
	}

	err := SaveConfig(tmpDir, invalidConfig)
	if err == nil {
		t.Error("expected error for invalid config, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("error should mention invalid, got: %v", err)
		}
	}

	// Verify no file was created
	configPath := filepath.Join(tmpDir, ".bv", "drift.yaml")
	if _, err := os.Stat(configPath); err == nil {
		t.Error("config file should not have been created for invalid config")
	}
}

// TestConfigSaveMkdirError tests SaveConfig when .bv cannot be created (file exists)
func TestConfigSaveMkdirError(t *testing.T) {
	t.Log("Testing SaveConfig when .bv is a file instead of directory")

	tmpDir := t.TempDir()
	bvPath := filepath.Join(tmpDir, ".bv")

	// Create a FILE named .bv where a directory is expected
	if err := os.WriteFile(bvPath, []byte("blocking file"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("Created blocking file at: %s", bvPath)

	config := DefaultConfig()
	err := SaveConfig(tmpDir, config)
	if err == nil {
		t.Error("expected error when .bv is a file, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
		if !strings.Contains(err.Error(), "creating config directory") {
			t.Errorf("error should mention creating config directory, got: %v", err)
		}
	}
}

// TestConfigSavePermissionError tests SaveConfig when directory is not writable
func TestConfigSavePermissionError(t *testing.T) {
	// Skip on systems where we can't reliably test permissions
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	t.Log("Testing SaveConfig with permission denied")

	tmpDir := t.TempDir()
	bvDir := filepath.Join(tmpDir, ".bv")
	if err := os.MkdirAll(bvDir, 0555); err != nil { // Read-only directory
		t.Fatal(err)
	}
	t.Logf("Created read-only directory at: %s", bvDir)

	config := DefaultConfig()
	err := SaveConfig(tmpDir, config)
	if err == nil {
		t.Error("expected error for read-only directory, got nil")
	} else {
		t.Logf("Got expected error: %v", err)
	}

	// Cleanup: restore permissions
	os.Chmod(bvDir, 0755)
}

func TestConfigSave(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		DensityWarningPct:        80,
		BlockedIncreaseThreshold: 3,
	}

	if err := SaveConfig(tmpDir, config); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, ".bv", "drift.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{"valid default", DefaultConfig(), false},
		{"negative density warning", &Config{DensityWarningPct: -10}, true},
		{"info > warning", &Config{DensityWarningPct: 10, DensityInfoPct: 20}, true},
		{"negative blocked", &Config{DensityWarningPct: 50, BlockedIncreaseThreshold: -1}, true},
		{"negative node growth", &Config{DensityWarningPct: 50, NodeGrowthInfoPct: -5}, true},
		{"negative edge growth", &Config{DensityWarningPct: 50, EdgeGrowthInfoPct: -5}, true},
		{"actionable decrease > 100", &Config{DensityWarningPct: 50, ActionableDecreaseWarningPct: 150}, true},
		{"negative actionable increase", &Config{DensityWarningPct: 50, ActionableIncreaseInfoPct: -10}, true},
		{"negative pagerank change", &Config{DensityWarningPct: 50, PageRankChangeWarningPct: -20}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCycleKey(t *testing.T) {
	// Same cycle represented identically should match
	key1 := cycleKey([]string{"A", "B", "C", "A"})
	key2 := cycleKey([]string{"A", "B", "C", "A"})

	if key1 != key2 {
		t.Errorf("identical cycles should match: %s vs %s", key1, key2)
	}

	// Different cycle should not match
	key3 := cycleKey([]string{"X", "Y", "Z", "X"})
	if key1 == key3 {
		t.Error("different cycles should have different keys")
	}

	// Empty cycle
	key4 := cycleKey([]string{})
	if key4 != "" {
		t.Errorf("empty cycle should have empty key, got %s", key4)
	}
}

// =============================================================================
// Calculator Method Branch Coverage Tests (bv-cam.3)
// =============================================================================

// TestCheckActionable_BaselineZero tests that actionable=0 baseline skips calculation
func TestCheckActionable_BaselineZero(t *testing.T) {
	t.Log("Testing checkActionable when baseline actionable=0 (skip case)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 0, // Zero baseline should skip calculation
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 50, // Huge increase but should be ignored
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	// Should not have actionable alert when baseline is 0
	for _, alert := range result.Alerts {
		if alert.Type == AlertActionableChange {
			t.Errorf("should not generate actionable alert when baseline is 0, got: %s", alert.Message)
		}
	}
	t.Log("Correctly skipped actionable check with zero baseline")
}

// TestCheckActionable_InfoIncrease tests actionable increase triggers info alert
func TestCheckActionable_InfoIncrease(t *testing.T) {
	t.Log("Testing checkActionable with 25% increase (info alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 100,
		},
	}

	// 25% increase should trigger info (default threshold is 20%)
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 125,
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertActionableChange {
			found = true
			if alert.Severity != SeverityInfo {
				t.Errorf("25%% increase should be info, got %s", alert.Severity)
			}
			t.Logf("Got expected alert: %s", alert.Message)
		}
	}
	if !found {
		t.Error("expected actionable_change info alert for 25% increase")
	}
}

// TestCheckActionable_DecreaseWarning tests large decrease triggers warning
func TestCheckActionable_DecreaseWarning(t *testing.T) {
	t.Log("Testing checkActionable with 35% decrease (warning alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 100,
		},
	}

	// 35% decrease should trigger warning (default threshold is 30%)
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 65,
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertActionableChange {
			found = true
			if alert.Severity != SeverityWarning {
				t.Errorf("35%% decrease should be warning, got %s", alert.Severity)
			}
			t.Logf("Got expected alert: %s", alert.Message)
		}
	}
	if !found {
		t.Error("expected actionable_change warning alert for 35% decrease")
	}
}

// TestCheckActionable_InfoDecrease tests moderate decrease yields info alert (not warning)
func TestCheckActionable_InfoDecrease(t *testing.T) {
	t.Log("Testing checkActionable with 25% decrease (info alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 80,
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 60, // 25% decrease (warning threshold is 30%)
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertActionableChange {
			found = true
			if alert.Severity != SeverityInfo {
				t.Errorf("25%% decrease should be info, got %s", alert.Severity)
			}
		}
	}
	if !found {
		t.Fatal("expected actionable_change info alert for 25% decrease")
	}
}

// TestCheckCycles_BaselineHasCycles tests that removing cycles doesn't alert
func TestCheckCycles_BaselineHasCycles(t *testing.T) {
	t.Log("Testing checkCycles when baseline has cycles but current removes them")

	bl := &baseline.Baseline{
		Stats:  baseline.GraphStats{NodeCount: 10},
		Cycles: [][]string{{"A", "B", "C", "A"}, {"X", "Y", "X"}},
	}

	// Current has fewer cycles - should NOT alert (only new cycles alert)
	current := &baseline.Baseline{
		Stats:  bl.Stats,
		Cycles: [][]string{{"A", "B", "C", "A"}},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertNewCycle {
			t.Errorf("should not alert when cycles are removed, got: %s", alert.Message)
		}
	}
	t.Log("Correctly did not alert when cycles were removed")
}

// TestCheckCycles_NewCycles tests that new cycles trigger critical alerts
func TestCheckCycles_NewCycles(t *testing.T) {
	t.Log("Testing checkCycles when current snapshot adds new cycles")

	bl := &baseline.Baseline{
		Stats:  baseline.GraphStats{NodeCount: 10},
		Cycles: [][]string{}, // No cycles in baseline
	}

	current := &baseline.Baseline{
		Stats:  bl.Stats,
		Cycles: [][]string{{"A", "B", "A"}},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertNewCycle {
			found = true
			if alert.Severity != SeverityCritical {
				t.Errorf("new cycles should be critical, got %s", alert.Severity)
			}
			if alert.Delta != 1 {
				t.Errorf("expected delta 1 new cycle, got %.0f", alert.Delta)
			}
		}
	}
	if !found {
		t.Fatal("expected critical new_cycle alert when cycles are added")
	}
}

// TestCheckCycles_SameCycles tests that identical cycles don't alert
func TestCheckCycles_SameCycles(t *testing.T) {
	t.Log("Testing checkCycles when baseline and current have same cycles")

	cycles := [][]string{{"A", "B", "C", "A"}, {"X", "Y", "X"}}

	bl := &baseline.Baseline{
		Stats:  baseline.GraphStats{NodeCount: 10},
		Cycles: cycles,
	}

	current := &baseline.Baseline{
		Stats:  bl.Stats,
		Cycles: cycles,
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertNewCycle {
			t.Errorf("should not alert when cycles are identical, got: %s", alert.Message)
		}
	}
	t.Log("Correctly did not alert when cycles are identical")
}

// TestCheckCycles_BothEmpty tests that empty cycles in both don't alert
func TestCheckCycles_BothEmpty(t *testing.T) {
	t.Log("Testing checkCycles when both baseline and current have no cycles")

	bl := &baseline.Baseline{
		Stats:  baseline.GraphStats{NodeCount: 10},
		Cycles: [][]string{},
	}

	current := &baseline.Baseline{
		Stats:  bl.Stats,
		Cycles: [][]string{},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertNewCycle {
			t.Errorf("should not alert when both have empty cycles, got: %s", alert.Message)
		}
	}
	t.Log("Correctly did not alert when both have empty cycles")
}

// TestCheckDensity_InfoLevel tests density increase at info level (not warning)
func TestCheckDensity_InfoLevel(t *testing.T) {
	t.Log("Testing checkDensity with 30% increase (info level, not warning)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.02,
		},
	}

	// 30% increase: above 20% info threshold, below 50% warning threshold
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.026, // 30% increase
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			found = true
			if alert.Severity != SeverityInfo {
				t.Errorf("30%% density increase should be info, got %s", alert.Severity)
			}
			t.Logf("Got expected alert: %s", alert.Message)
		}
	}
	if !found {
		t.Error("expected density_growth info alert for 30% increase")
	}
}

// TestCheckDensity_Decrease tests that density decrease doesn't alert
func TestCheckDensity_Decrease(t *testing.T) {
	t.Log("Testing checkDensity when density decreases (no alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.05,
		},
	}

	// Density decreased - should NOT alert
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.02, // 60% decrease
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			t.Errorf("should not alert when density decreases, got: %s", alert.Message)
		}
	}
	t.Log("Correctly did not alert when density decreased")
}

// TestCheckDensity_WarningLevel tests density increase crossing warning threshold
func TestCheckDensity_WarningLevel(t *testing.T) {
	t.Log("Testing checkDensity with 75% increase (warning level)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.02,
		},
	}

	// 75% increase: above 50% warning threshold
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.035, // (0.035-0.02)/0.02 = 75%
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	found := false
	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			found = true
			if alert.Severity != SeverityWarning {
				t.Errorf("75%% density increase should be warning, got %s", alert.Severity)
			}
			t.Logf("Got expected warning: %s", alert.Message)
		}
	}
	if !found {
		t.Fatal("expected density_growth warning alert for 75% increase")
	}
}

// TestCheckDensity_BaselineZero tests that zero baseline density skips check
func TestCheckDensity_BaselineZero(t *testing.T) {
	t.Log("Testing checkDensity when baseline density=0 (skip case)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0, // Zero baseline should skip
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.05, // Would be infinite increase but should be skipped
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			t.Errorf("should not alert when baseline density is 0, got: %s", alert.Message)
		}
	}
	t.Log("Correctly skipped density check with zero baseline")
}

// TestCheckActionable_SmallChanges tests that small changes don't trigger alerts
func TestCheckActionable_SmallChanges(t *testing.T) {
	t.Log("Testing checkActionable with small changes (no alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 100,
		},
	}

	// 10% increase (threshold 20%) -> No Alert
	currentInc := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 110,
		},
	}

	calcInc := NewCalculator(bl, currentInc, nil)
	resultInc := calcInc.Calculate()
	for _, alert := range resultInc.Alerts {
		if alert.Type == AlertActionableChange {
			t.Errorf("10%% increase should not alert, got: %s", alert.Message)
		}
	}

	// 10% decrease (threshold 20% for info) -> No Alert
	currentDec := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       100,
			ActionableCount: 90,
		},
	}

	calcDec := NewCalculator(bl, currentDec, nil)
	resultDec := calcDec.Calculate()
	for _, alert := range resultDec.Alerts {
		if alert.Type == AlertActionableChange {
			t.Errorf("10%% decrease should not alert, got: %s", alert.Message)
		}
	}
}

// TestCheckDensity_SmallIncrease tests that small density increase doesn't alert
func TestCheckDensity_SmallIncrease(t *testing.T) {
	t.Log("Testing checkDensity with 10% increase (no alert)")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.02,
		},
	}

	// 10% increase (threshold 20%)
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.022,
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	for _, alert := range result.Alerts {
		if alert.Type == AlertDensityGrowth {
			t.Errorf("10%% density increase should not alert, got: %s", alert.Message)
		}
	}
}

func TestCalculatorZeroBaseline(t *testing.T) {
	t.Log("Testing calculator with zero-value baseline")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{}, // All zero
	}

	// Current has some values
	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount:       10,
			EdgeCount:       20,
			Density:         0.1,
			BlockedCount:    2,
			ActionableCount: 8,
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	// Should not crash.
	// With zero baseline, most percentage calc checks should be skipped or handle div-by-zero safely.
	// Node/Edge growth checks check "if blNodes > 0".
	// Density check checks "if blDensity == 0".
	// Actionable check checks "if blAction > 0".
	// Blocked check is absolute difference, so 2 - 0 = 2. Threshold is 5. So no alert.

	if result.HasDrift {
		t.Errorf("expected no drift with zero baseline, got %d alerts", len(result.Alerts))
		for _, a := range result.Alerts {
			t.Logf("Alert: %s", a.Message)
		}
	}
}

func TestCalculatorBoundaryThresholds(t *testing.T) {
	t.Log("Testing exact boundary conditions")

	cfg := &Config{
		DensityWarningPct: 50.0,
	}

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.50,
		},
	}

	// Case 1: Exactly 50% increase (0.50 -> 0.75)
	// Should trigger warning (using exact float representations)
	currentExact := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.75,
		},
	}

	calcExact := NewCalculator(bl, currentExact, cfg)
	resExact := calcExact.Calculate()

	found := false
	for _, a := range resExact.Alerts {
		if a.Type == AlertDensityGrowth && a.Severity == SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Error("Exact 50% density increase should trigger warning")
		for _, a := range resExact.Alerts {
			t.Logf("Found alert: [%s] %s (Delta: %f)", a.Severity, a.Message, a.Delta)
		}
	}

	// Case 2: Just below 50% increase (0.10 -> 0.1499)
	// Should NOT trigger warning (assuming no info threshold or low info threshold)
	// Default info is 20%, so it might trigger Info. Let's set Info to 49.9 to be safe or ignore info alerts.
	// Let's explicitly check it does NOT trigger Warning.
	currentBelow := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 100,
			Density:   0.1499,
		},
	}

	calcBelow := NewCalculator(bl, currentBelow, cfg)
	resBelow := calcBelow.Calculate()

	for _, a := range resBelow.Alerts {
		if a.Type == AlertDensityGrowth && a.Severity == SeverityWarning {
			t.Errorf("49.9%% density increase should NOT trigger warning, got: %s", a.Message)
		}
	}
}

func TestCalculatorEmptyMetrics(t *testing.T) {
	t.Log("Testing with empty metric slices")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{NodeCount: 10},
		TopMetrics: baseline.TopMetrics{
			PageRank: []baseline.MetricItem{}, // Empty
		},
		Cycles: [][]string{},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{NodeCount: 10},
		TopMetrics: baseline.TopMetrics{
			PageRank: []baseline.MetricItem{}, // Empty
		},
		Cycles: [][]string{},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	if result.HasDrift {
		t.Error("Empty metrics should not trigger drift")
	}
}

func TestCalculatorLargeValues(t *testing.T) {
	t.Log("Testing with large values to ensure no overflow/panic")

	bl := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 1000000,
			EdgeCount: 5000000,
			Density:   0.5,
		},
	}

	current := &baseline.Baseline{
		Stats: baseline.GraphStats{
			NodeCount: 1000000, // No change
			EdgeCount: 5000000,
			Density:   0.5,
		},
	}

	calc := NewCalculator(bl, current, nil)
	result := calc.Calculate()

	if result.HasDrift {
		t.Error("Large stable values should not trigger drift")
	}
}
