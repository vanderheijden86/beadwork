package analysis

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// Package-level compiled regex for performance (avoids recompilation on each call)
var nonWordRegex = regexp.MustCompile(`[^\w\s]`)

// Package-level stop words map for performance (avoids recreation on each call)
var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true,
	"this": true, "that": true, "from": true, "are": true,
	"was": true, "were": true, "been": true, "have": true,
	"has": true, "had": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "can": true, "not": true,
	"all": true, "any": true, "some": true, "each": true,
	"when": true, "where": true, "what": true, "which": true,
	"how": true, "why": true, "who": true, "its": true,
	"also": true, "just": true, "only": true, "more": true,
	"than": true, "then": true, "now": true, "here": true,
	"there": true, "these": true, "those": true, "such": true,
	"into": true, "over": true, "after": true, "before": true,
	"being": true, "other": true, "about": true, "like": true,
	"very": true, "most": true, "make": true, "use": true,
}

// DuplicateConfig configures duplicate detection behavior
type DuplicateConfig struct {
	// JaccardThreshold is the minimum similarity score (0.0-1.0)
	// Default: 0.7
	JaccardThreshold float64

	// MinKeywords is the minimum number of keywords needed to compare
	// Default: 2
	MinKeywords int

	// IgnoreClosedVsOpen skips pairs where one is closed and one is open
	// Default: true
	IgnoreClosedVsOpen bool

	// MaxSuggestions limits the number of duplicate suggestions
	// Default: 20
	MaxSuggestions int
}

// DefaultDuplicateConfig returns sensible defaults
func DefaultDuplicateConfig() DuplicateConfig {
	return DuplicateConfig{
		JaccardThreshold:   0.7,
		MinKeywords:        2,
		IgnoreClosedVsOpen: true,
		MaxSuggestions:     20,
	}
}

// DuplicatePair represents a potential duplicate pair
type DuplicatePair struct {
	Issue1     string  `json:"issue1"`
	Issue2     string  `json:"issue2"`
	Similarity float64 `json:"similarity"`
	Method     string  `json:"method"` // "jaccard" or "semantic"
	Keywords   []string `json:"common_keywords,omitempty"`
}

// DetectDuplicates finds potential duplicate issues using keyword-based Jaccard similarity
func DetectDuplicates(issues []model.Issue, config DuplicateConfig) []Suggestion {
	if len(issues) < 2 {
		return nil
	}

	// Extract keywords for each issue
	issueKeywords := make(map[string][]string, len(issues))
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issue := &issues[i]
		issueMap[issue.ID] = issue
		issueKeywords[issue.ID] = extractKeywords(issue.Title, issue.Description)
	}

	var pairs []DuplicatePair

	// Compare all pairs (O(n^2) but typically small issue counts)
	for i := 0; i < len(issues); i++ {
		for j := i + 1; j < len(issues); j++ {
			issue1 := &issues[i]
			issue2 := &issues[j]

			// Skip closed vs open pairs if configured
			if config.IgnoreClosedVsOpen {
				if (issue1.Status == model.StatusClosed) != (issue2.Status == model.StatusClosed) {
					continue
				}
			}

			kw1 := issueKeywords[issue1.ID]
			kw2 := issueKeywords[issue2.ID]

			// Skip if not enough keywords
			if len(kw1) < config.MinKeywords || len(kw2) < config.MinKeywords {
				continue
			}

			similarity, common := jaccardSimilarity(kw1, kw2)
			if similarity >= config.JaccardThreshold {
				pairs = append(pairs, DuplicatePair{
					Issue1:     issue1.ID,
					Issue2:     issue2.ID,
					Similarity: similarity,
					Method:     "jaccard",
					Keywords:   common,
				})
			}
		}
	}

	// Sort by similarity (highest first) and limit
	sortPairsBySimilarity(pairs)
	if len(pairs) > config.MaxSuggestions {
		pairs = pairs[:config.MaxSuggestions]
	}

	// Convert to suggestions
	suggestions := make([]Suggestion, 0, len(pairs))
	for _, pair := range pairs {
		issue1 := issueMap[pair.Issue1]
		issue2 := issueMap[pair.Issue2]

		sug := NewSuggestion(
			SuggestionPotentialDuplicate,
			pair.Issue1,
			fmt.Sprintf("Potential duplicate of %s", pair.Issue2),
			fmt.Sprintf("%.0f%% keyword similarity; common: %s",
				pair.Similarity*100,
				strings.Join(truncateStringSlice(pair.Keywords, 5), ", ")),
			pair.Similarity,
		).WithRelatedBead(pair.Issue2).WithMetadata("method", pair.Method)

		// Add action command if both are open
		if issue1.Status != model.StatusClosed && issue2.Status != model.StatusClosed {
			sug = sug.WithAction(fmt.Sprintf("bd dep add %s %s --type=related", pair.Issue1, pair.Issue2))
		}

		suggestions = append(suggestions, sug)
	}

	return suggestions
}

// extractKeywords extracts meaningful keywords from text
func extractKeywords(title, description string) []string {
	text := strings.ToLower(title + " " + description)

	// Remove common markdown/code artifacts (uses package-level compiled regex)
	text = nonWordRegex.ReplaceAllString(text, " ")

	words := strings.Fields(text)

	// Filter out stop words and short words
	keywords := make([]string, 0, len(words)/2)
	seen := make(map[string]bool)

	for _, word := range words {
		// Skip short words
		if len(word) < 3 {
			continue
		}

		// Skip common stop words (uses package-level map)
		if stopWords[word] {
			continue
		}

		// Deduplicate
		if seen[word] {
			continue
		}
		seen[word] = true
		keywords = append(keywords, word)
	}

	return keywords
}

// jaccardSimilarity computes Jaccard similarity between two keyword sets
// Returns similarity score and the common keywords
func jaccardSimilarity(set1, set2 []string) (float64, []string) {
	if len(set1) == 0 || len(set2) == 0 {
		return 0, nil
	}

	// Build maps for O(1) lookup
	map1 := make(map[string]bool, len(set1))
	for _, k := range set1 {
		map1[k] = true
	}

	map2 := make(map[string]bool, len(set2))
	for _, k := range set2 {
		map2[k] = true
	}

	// Compute intersection
	var intersection []string
	for k := range map1 {
		if map2[k] {
			intersection = append(intersection, k)
		}
	}

	// Compute union size
	union := len(set1)
	for k := range map2 {
		if !map1[k] {
			union++
		}
	}

	if union == 0 {
		return 0, nil
	}

	return float64(len(intersection)) / float64(union), intersection
}

// sortPairsBySimilarity sorts duplicate pairs by similarity (highest first)
// Uses sort.Slice for O(n log n) performance instead of bubble sort O(n²)
func sortPairsBySimilarity(pairs []DuplicatePair) {
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Similarity > pairs[j].Similarity
	})
}

// truncateStringSlice truncates a string slice to max length
func truncateStringSlice(s []string, max int) []string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// DuplicateDetector provides stateful duplicate detection with caching
type DuplicateDetector struct {
	config     DuplicateConfig
	lastRun    time.Time
	lastResult []Suggestion
}

// NewDuplicateDetector creates a new detector with the given config
func NewDuplicateDetector(config DuplicateConfig) *DuplicateDetector {
	return &DuplicateDetector{
		config: config,
	}
}

// Detect finds potential duplicates
func (d *DuplicateDetector) Detect(issues []model.Issue) []Suggestion {
	d.lastRun = time.Now()
	d.lastResult = DetectDuplicates(issues, d.config)
	return d.lastResult
}

// LastRun returns when detection was last performed
func (d *DuplicateDetector) LastRun() time.Time {
	return d.lastRun
}
