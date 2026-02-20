package main

import (
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestFilterByRepo_CaseInsensitiveAndFlexibleSeparators(t *testing.T) {
	issues := []model.Issue{
		{ID: "api-AUTH-1", SourceRepo: "services/api"},
		{ID: "web:UI-2", SourceRepo: "apps/web"},
		{ID: "lib_UTIL_3", SourceRepo: "libs/util"},
		{ID: "misc-4", SourceRepo: "misc"},
	}

	tests := []struct {
		filter   string
		expected int
	}{
		{"API", 1},      // case-insensitive, matches api-
		{"web", 1},      // flexible with ':' separator
		{"lib", 1},      // flexible with '_' separator
		{"missing", 0},  // no match
		{"misc-", 1},    // exact prefix
		{"services", 1}, // matches SourceRepo when ID lacks prefix
	}

	for _, tt := range tests {
		got := filterByRepo(issues, tt.filter)
		if len(got) != tt.expected {
			t.Errorf("filterByRepo(%q) = %d issues, want %d", tt.filter, len(got), tt.expected)
		}
	}
}
