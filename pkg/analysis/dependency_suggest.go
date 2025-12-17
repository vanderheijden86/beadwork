package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// DependencySuggestionConfig configures dependency suggestion generation
type DependencySuggestionConfig struct {
	// MinKeywordOverlap is the minimum number of shared keywords to suggest
	// Default: 2
	MinKeywordOverlap int

	// ExactMatchBonus is the confidence bonus for exact keyword matches
	// Default: 0.15
	ExactMatchBonus float64

	// LabelOverlapBonus is the confidence bonus per shared label
	// Default: 0.1
	LabelOverlapBonus float64

	// MinConfidence is the minimum confidence to report
	// Default: 0.5
	MinConfidence float64

	// MaxSuggestions limits the number of suggestions
	// Default: 20
	MaxSuggestions int

	// IgnoreExistingDeps skips pairs that already have dependencies
	// Default: true
	IgnoreExistingDeps bool
}

// DefaultDependencySuggestionConfig returns sensible defaults
func DefaultDependencySuggestionConfig() DependencySuggestionConfig {
	return DependencySuggestionConfig{
		MinKeywordOverlap:  2,
		ExactMatchBonus:    0.15,
		LabelOverlapBonus:  0.1,
		MinConfidence:      0.5,
		MaxSuggestions:     20,
		IgnoreExistingDeps: true,
	}
}

// DependencyMatch represents a potential dependency relationship
type DependencyMatch struct {
	From           string   `json:"from"`
	To             string   `json:"to"`
	Confidence     float64  `json:"confidence"`
	SharedKeywords []string `json:"shared_keywords"`
	SharedLabels   []string `json:"shared_labels,omitempty"`
	Reason         string   `json:"reason"`
}

// DetectMissingDependencies analyzes issues for potential missing dependencies
func DetectMissingDependencies(issues []model.Issue, config DependencySuggestionConfig) []Suggestion {
	if len(issues) < 2 {
		return nil
	}

	// Extract keywords and build maps
	issueKeywords := make(map[string]map[string]bool, len(issues))
	issueLabels := make(map[string]map[string]bool, len(issues))
	existingDeps := make(map[string]map[string]bool)

	for _, issue := range issues {
		// Keywords
		kw := extractKeywords(issue.Title, issue.Description)
		kwMap := make(map[string]bool, len(kw))
		for _, k := range kw {
			kwMap[k] = true
		}
		issueKeywords[issue.ID] = kwMap

		// Labels
		lblMap := make(map[string]bool, len(issue.Labels))
		for _, l := range issue.Labels {
			lblMap[strings.ToLower(l)] = true
		}
		issueLabels[issue.ID] = lblMap

		// Existing deps
		if existingDeps[issue.ID] == nil {
			existingDeps[issue.ID] = make(map[string]bool)
		}
		for _, dep := range issue.Dependencies {
			if dep != nil {
				existingDeps[issue.ID][dep.DependsOnID] = true
			}
		}
	}

	var matches []DependencyMatch

	// Compare all pairs
	for i := 0; i < len(issues); i++ {
		for j := i + 1; j < len(issues); j++ {
			issue1 := &issues[i]
			issue2 := &issues[j]

			// Skip if already have a dependency either direction
			if config.IgnoreExistingDeps {
				if existingDeps[issue1.ID][issue2.ID] || existingDeps[issue2.ID][issue1.ID] {
					continue
				}
			}

			// Skip if both are closed
			if issue1.Status == model.StatusClosed && issue2.Status == model.StatusClosed {
				continue
			}

			// Find shared keywords
			kw1 := issueKeywords[issue1.ID]
			kw2 := issueKeywords[issue2.ID]
			sharedKW := findSharedKeys(kw1, kw2)

			if len(sharedKW) < config.MinKeywordOverlap {
				continue
			}

			// Find shared labels
			lbl1 := issueLabels[issue1.ID]
			lbl2 := issueLabels[issue2.ID]
			sharedLabels := findSharedKeys(lbl1, lbl2)

			// Calculate confidence
			baseConf := float64(len(sharedKW)) * 0.1
			if baseConf > 0.5 {
				baseConf = 0.5
			}

			// Check for exact title mentions
			title2Lower := strings.ToLower(issue2.Title)
			id1Lower := strings.ToLower(issue1.ID)
			id2Lower := strings.ToLower(issue2.ID)
			desc1Lower := strings.ToLower(issue1.Description)
			desc2Lower := strings.ToLower(issue2.Description)

			// ID mentioned in other issue
			if strings.Contains(desc2Lower, id1Lower) || strings.Contains(desc1Lower, id2Lower) {
				baseConf += config.ExactMatchBonus * 2
			}

			// Title words of issue1 mentioned in issue2's title
			for word := range kw1 {
				if len(word) >= 5 && strings.Contains(title2Lower, word) {
					baseConf += config.ExactMatchBonus
					break
				}
			}

			// Label overlap
			baseConf += float64(len(sharedLabels)) * config.LabelOverlapBonus

			// Cap confidence
			if baseConf > 0.95 {
				baseConf = 0.95
			}

			if baseConf < config.MinConfidence {
				continue
			}

			// Determine direction: older/lower priority tends to be dependency
			var from, to *model.Issue
			if issue1.CreatedAt.Before(issue2.CreatedAt) || issue1.Priority < issue2.Priority {
				from, to = issue2, issue1
			} else {
				from, to = issue1, issue2
			}

			reason := fmt.Sprintf("%d shared keywords", len(sharedKW))
			if len(sharedLabels) > 0 {
				reason += fmt.Sprintf(", %d shared labels", len(sharedLabels))
			}

			matches = append(matches, DependencyMatch{
				From:           from.ID,
				To:             to.ID,
				Confidence:     baseConf,
				SharedKeywords: sharedKW,
				SharedLabels:   sharedLabels,
				Reason:         reason,
			})
		}
	}

	// Sort by confidence and limit
	sortMatchesByConfidence(matches)
	if len(matches) > config.MaxSuggestions {
		matches = matches[:config.MaxSuggestions]
	}

	// Convert to suggestions
	suggestions := make([]Suggestion, 0, len(matches))
	for _, match := range matches {
		sug := NewSuggestion(
			SuggestionMissingDependency,
			match.From,
			fmt.Sprintf("May depend on %s", match.To),
			match.Reason,
			match.Confidence,
		).WithRelatedBead(match.To).
			WithAction(fmt.Sprintf("bd dep add %s %s", match.From, match.To)).
			WithMetadata("shared_keywords", match.SharedKeywords)

		if len(match.SharedLabels) > 0 {
			sug = sug.WithMetadata("shared_labels", match.SharedLabels)
		}

		suggestions = append(suggestions, sug)
	}

	return suggestions
}

// findSharedKeys returns keys present in both maps
func findSharedKeys(m1, m2 map[string]bool) []string {
	var shared []string
	for k := range m1 {
		if m2[k] {
			shared = append(shared, k)
		}
	}
	return shared
}

// sortMatchesByConfidence sorts matches by confidence (highest first)
// Uses sort.Slice for O(n log n) performance instead of bubble sort O(n²)
func sortMatchesByConfidence(matches []DependencyMatch) {
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Confidence > matches[j].Confidence
	})
}

// DependencySuggestionDetector provides stateful dependency suggestion detection
type DependencySuggestionDetector struct {
	config DependencySuggestionConfig
}

// NewDependencySuggestionDetector creates a new detector with the given config
func NewDependencySuggestionDetector(config DependencySuggestionConfig) *DependencySuggestionDetector {
	return &DependencySuggestionDetector{
		config: config,
	}
}

// Detect finds missing dependency suggestions
func (d *DependencySuggestionDetector) Detect(issues []model.Issue) []Suggestion {
	return DetectMissingDependencies(issues, d.config)
}
