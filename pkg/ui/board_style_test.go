package ui

import (
	"strings"
	"testing"
	"time"
)

func TestSelectedCardTextColor_HighContrast(t *testing.T) {
	if selectedCardTextColor.Dark != "#101010" {
		t.Fatalf("selectedCardTextColor.Dark = %q, want %q", selectedCardTextColor.Dark, "#101010")
	}
	if selectedCardTextColor.Light != "#101010" {
		t.Fatalf("selectedCardTextColor.Light = %q, want %q", selectedCardTextColor.Light, "#101010")
	}
}

func TestBuildBoardColumnHeaderText_Narrow(t *testing.T) {
	got := buildBoardColumnHeaderText("OPEN (3)", ColumnStats{
		Total:     3,
		P0Count:   1,
		P1Count:   2,
		OldestAge: 10 * 24 * time.Hour,
	}, 80, SwimByStatus, ColOpen)

	if got != "OPEN (3)" {
		t.Fatalf("header = %q, want %q", got, "OPEN (3)")
	}
}

func TestBuildBoardColumnHeaderText_Medium(t *testing.T) {
	got := buildBoardColumnHeaderText("OPEN (4)", ColumnStats{
		Total:   4,
		P0Count: 2,
		P1Count: 1,
	}, 120, SwimByStatus, ColOpen)

	if !strings.Contains(got, "P0:2") || !strings.Contains(got, "P1:1") {
		t.Fatalf("header = %q, want P0/P1 tokens", got)
	}
	if strings.ContainsAny(got, "🔴🟡⚠️⏱") {
		t.Fatalf("header should use plain text tokens, got %q", got)
	}
}

func TestBuildBoardColumnHeaderText_WideWithBlockedAndAge(t *testing.T) {
	got := buildBoardColumnHeaderText("IN PROGRESS (5)", ColumnStats{
		Total:        5,
		P0Count:      1,
		P1Count:      2,
		BlockedCount: 3,
		OldestAge:    14 * 24 * time.Hour,
	}, 160, SwimByStatus, ColInProgress)

	for _, token := range []string{"P0:1", "P1:2", "BLK:3", "AGE:2w"} {
		if !strings.Contains(got, token) {
			t.Fatalf("header = %q, missing %q", got, token)
		}
	}
	if strings.ContainsAny(got, "🔴🟡⚠️⏱") {
		t.Fatalf("header should use plain text tokens, got %q", got)
	}
}
