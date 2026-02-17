// Package ui provides the terminal user interface for beadwork.
// This file implements the DataSnapshot type for thread-safe UI rendering.
package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

type datasetTier int

const (
	datasetTierUnknown datasetTier = iota
	datasetTierSmall
	datasetTierMedium
	datasetTierLarge
	datasetTierHuge
)

func datasetTierForIssueCount(total int) datasetTier {
	switch {
	case total <= 0:
		return datasetTierUnknown
	case total < 1000:
		return datasetTierSmall
	case total < 5000:
		return datasetTierMedium
	case total < 20000:
		return datasetTierLarge
	default:
		return datasetTierHuge
	}
}

func (t datasetTier) String() string {
	switch t {
	case datasetTierSmall:
		return "small"
	case datasetTierMedium:
		return "medium"
	case datasetTierLarge:
		return "large"
	case datasetTierHuge:
		return "huge"
	default:
		return "unknown"
	}
}

func isClosedLikeStatus(status model.Status) bool {
	return status == model.StatusClosed || status == model.StatusTombstone
}

type snapshotBuildConfig struct {
	PrecomputeTree  bool
	PrecomputeBoard bool
}

func snapshotBuildConfigDefault() snapshotBuildConfig {
	return snapshotBuildConfig{
		PrecomputeTree:  true,
		PrecomputeBoard: true,
	}
}

func snapshotBuildConfigForTier(tier datasetTier) snapshotBuildConfig {
	cfg := snapshotBuildConfigDefault()
	switch tier {
	case datasetTierLarge, datasetTierHuge:
		cfg.PrecomputeTree = false
		cfg.PrecomputeBoard = false
	}
	return cfg
}

func compactCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%dm", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dk", n/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// DataSnapshot is an immutable, self-contained representation of all data
// the UI needs to render. Once created, it never changes - this is critical
// for thread safety when the background worker is building the next snapshot.
//
// The UI thread reads exclusively from its current snapshot pointer.
// When a new snapshot is ready, the UI swaps the pointer atomically.
type DataSnapshot struct {
	// Core data
	Issues   []model.Issue           // All issues (sorted)
	IssueMap map[string]*model.Issue // Lookup by ID
	// pooledIssues holds pooled backing structs used during parse.
	// It must be returned to the pool when the snapshot is replaced.
	pooledIssues []*model.Issue

	// Computed statistics
	CountOpen    int
	CountReady   int
	CountBlocked int
	CountClosed  int

	// Pre-computed UI data
	ListItems []IssueItem // Pre-built list items
	// TreeRoots and TreeNodeMap contain a pre-built parent/child tree for the Tree view.
	TreeRoots   []*IssueTreeNode
	TreeNodeMap map[string]*IssueTreeNode
	// BoardState contains pre-built Kanban board columns for each swimlane mode.
	BoardState *BoardState

	// Metadata
	CreatedAt time.Time // When this snapshot was built
	DataHash  string    // Hash of source data for cache validation
	// DatasetTier is a tiered performance mode for large datasets.
	DatasetTier datasetTier
	// SourceIssueCountHint is an approximate total issue count from the source file.
	SourceIssueCountHint int
	// LoadedOpenOnly indicates the snapshot intentionally excluded closed/tombstone
	// issues for performance (huge tier).
	LoadedOpenOnly bool
	// TruncatedCount is an approximate count of issues excluded by load policy.
	TruncatedCount int
	// LargeDatasetWarning is a short, user-facing warning to show in the footer.
	LargeDatasetWarning string
	// LoadWarningCount is the number of non-fatal parse warnings encountered while loading.
	LoadWarningCount int

	// Error state (for graceful degradation)
	LoadError    error     // Non-nil if last load had recoverable errors
	ErrorTime    time.Time // When error occurred
	StaleWarning bool      // True if data is from previous successful load
}

// BoardState contains precomputed Kanban columns for each swimlane mode.
type BoardState struct {
	ByStatus   [4][]model.Issue
	ByPriority [4][]model.Issue
	ByType     [4][]model.Issue
}

func (s *BoardState) ColumnsForMode(mode SwimLaneMode) [4][]model.Issue {
	if s == nil {
		return [4][]model.Issue{}
	}
	switch mode {
	case SwimByPriority:
		return s.ByPriority
	case SwimByType:
		return s.ByType
	default:
		return s.ByStatus
	}
}

// SnapshotBuilder constructs DataSnapshots from raw data.
type SnapshotBuilder struct {
	issues []model.Issue
	cfg    snapshotBuildConfig
}

// NewSnapshotBuilder creates a builder for constructing a DataSnapshot.
func NewSnapshotBuilder(issues []model.Issue) *SnapshotBuilder {
	return &SnapshotBuilder{
		issues: issues,
		cfg:    snapshotBuildConfigDefault(),
	}
}

func (b *SnapshotBuilder) WithBuildConfig(cfg snapshotBuildConfig) *SnapshotBuilder {
	b.cfg = cfg
	return b
}

// Build constructs the final immutable DataSnapshot.
func (b *SnapshotBuilder) Build() *DataSnapshot {
	issues := b.issues

	// Apply default sorting: creation date descending (newest first)
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].CreatedAt.After(issues[j].CreatedAt)
	})

	// Build lookup map
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}

	// Compute statistics
	cOpen, cReady, cBlocked, cClosed := 0, 0, 0, 0
	for i := range issues {
		issue := &issues[i]
		if isClosedLikeStatus(issue.Status) {
			cClosed++
			continue
		}

		cOpen++
		if issue.Status == model.StatusBlocked {
			cBlocked++
			continue
		}

		// Check if blocked by open dependencies
		isBlocked := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if blocker, exists := issueMap[dep.DependsOnID]; exists && !isClosedLikeStatus(blocker.Status) {
				isBlocked = true
				break
			}
		}
		if !isBlocked {
			cReady++
		}
	}

	// Build list items
	listItems := buildListItems(issues)

	var (
		treeRoots   []*IssueTreeNode
		treeNodeMap map[string]*IssueTreeNode
	)
	if b.cfg.PrecomputeTree {
		treeRoots, treeNodeMap = buildIssueTreeNodes(issues)
	}

	var boardState *BoardState
	if b.cfg.PrecomputeBoard {
		boardState = buildBoardState(issues)
	}

	return &DataSnapshot{
		Issues:       issues,
		IssueMap:     issueMap,
		CountOpen:    cOpen,
		CountReady:   cReady,
		CountBlocked: cBlocked,
		CountClosed:  cClosed,
		ListItems:    listItems,
		TreeRoots:    treeRoots,
		TreeNodeMap:  treeNodeMap,
		BoardState:   boardState,
		CreatedAt:    time.Now(),
	}
}

func buildListItems(issues []model.Issue) []IssueItem {
	listItems := make([]IssueItem, len(issues))
	for i := range issues {
		listItems[i] = IssueItem{
			Issue:      issues[i],
			RepoPrefix: ExtractRepoPrefix(issues[i].ID),
			DiffStatus: DiffStatusNone,
		}
	}
	return listItems
}

func buildBoardState(issues []model.Issue) *BoardState {
	if len(issues) == 0 {
		return nil
	}
	return &BoardState{
		ByStatus:   groupIssuesByMode(issues, SwimByStatus),
		ByPriority: groupIssuesByMode(issues, SwimByPriority),
		ByType:     groupIssuesByMode(issues, SwimByType),
	}
}

// IsEmpty returns true if the snapshot has no issues.
func (s *DataSnapshot) IsEmpty() bool {
	return s == nil || len(s.Issues) == 0
}

// GetIssue returns an issue by ID, or nil if not found.
func (s *DataSnapshot) GetIssue(id string) *model.Issue {
	if s == nil || s.IssueMap == nil {
		return nil
	}
	return s.IssueMap[id]
}

// Age returns how long ago this snapshot was created.
func (s *DataSnapshot) Age() time.Duration {
	if s == nil {
		return 0
	}
	return time.Since(s.CreatedAt)
}

// computeDataHash generates a deterministic hash of issue data.
// Issues are sorted by ID to ensure consistent hashing regardless of input order.
func computeDataHash(issues []model.Issue) string {
	if len(issues) == 0 {
		return "empty"
	}

	sorted := make([]model.Issue, len(issues))
	copy(sorted, issues)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	h := sha256.New()
	for _, issue := range sorted {
		h.Write([]byte(issue.ID))
		h.Write([]byte{0})
		h.Write([]byte(issue.Title))
		h.Write([]byte{0})
		h.Write([]byte(issue.Description))
		h.Write([]byte{0})
		h.Write([]byte(issue.Notes))
		h.Write([]byte{0})
		h.Write([]byte(issue.Design))
		h.Write([]byte{0})
		h.Write([]byte(issue.AcceptanceCriteria))
		h.Write([]byte{0})
		h.Write([]byte(issue.Assignee))
		h.Write([]byte{0})
		h.Write([]byte(issue.SourceRepo))
		h.Write([]byte{0})
		if issue.ExternalRef != nil {
			h.Write([]byte(*issue.ExternalRef))
		}
		h.Write([]byte{0})
		h.Write([]byte(issue.Status))
		h.Write([]byte{0})
		h.Write([]byte(issue.IssueType))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(issue.Priority)))
		h.Write([]byte{0})
		h.Write([]byte(issue.CreatedAt.UTC().Format(time.RFC3339Nano)))
		h.Write([]byte{0})
		h.Write([]byte(issue.UpdatedAt.UTC().Format(time.RFC3339Nano)))
		h.Write([]byte{0})
		if issue.ClosedAt != nil {
			h.Write([]byte(issue.ClosedAt.UTC().Format(time.RFC3339Nano)))
		}
		h.Write([]byte{0})

		if len(issue.Labels) > 0 {
			labels := append([]string(nil), issue.Labels...)
			sort.Strings(labels)
			for _, lbl := range labels {
				h.Write([]byte(lbl))
				h.Write([]byte{0})
			}
		}
		h.Write([]byte{0})

		if len(issue.Dependencies) > 0 {
			type depKey struct {
				dependsOn string
				depType   string
			}
			deps := make([]depKey, 0, len(issue.Dependencies))
			for _, dep := range issue.Dependencies {
				if dep == nil {
					continue
				}
				deps = append(deps, depKey{
					dependsOn: dep.DependsOnID,
					depType:   string(dep.Type),
				})
			}
			sort.Slice(deps, func(i, j int) bool {
				if deps[i].dependsOn != deps[j].dependsOn {
					return deps[i].dependsOn < deps[j].dependsOn
				}
				return deps[i].depType < deps[j].depType
			})
			for _, dep := range deps {
				h.Write([]byte(dep.dependsOn))
				h.Write([]byte{0})
				h.Write([]byte(dep.depType))
				h.Write([]byte{0})
			}
		}
		h.Write([]byte{0})

		if len(issue.Comments) > 0 {
			type commentKey struct {
				id   string
				text string
			}
			comments := make([]commentKey, 0, len(issue.Comments))
			for _, comment := range issue.Comments {
				if comment == nil {
					continue
				}
				comments = append(comments, commentKey{
					id:   strconv.FormatInt(comment.ID, 10),
					text: comment.Text,
				})
			}
			sort.Slice(comments, func(i, j int) bool {
				return comments[i].id < comments[j].id
			})
			for _, comment := range comments {
				h.Write([]byte(comment.id))
				h.Write([]byte{0})
				h.Write([]byte(comment.text))
				h.Write([]byte{0})
			}
		}

		h.Write([]byte{1}) // issue separator
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}
