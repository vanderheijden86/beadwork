package analysis

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// StartupProfile captures detailed timing information for startup diagnostics.
// Use AnalyzeWithProfile to populate this structure.
type StartupProfile struct {
	// Data characteristics
	NodeCount int     `json:"node_count"`
	EdgeCount int     `json:"edge_count"`
	Density   float64 `json:"density"`

	// Phase 1 timings
	BuildGraph time.Duration `json:"build_graph"`
	Degree     time.Duration `json:"degree"`
	TopoSort   time.Duration `json:"topo_sort"`
	Phase1     time.Duration `json:"phase1_total"`

	// Phase 2 timings (zero if skipped)
	PageRank      time.Duration `json:"pagerank"`
	PageRankTO    bool          `json:"pagerank_timeout"`
	Betweenness   time.Duration `json:"betweenness"`
	BetweennessTO bool          `json:"betweenness_timeout"`
	Eigenvector   time.Duration `json:"eigenvector"`
	HITS          time.Duration `json:"hits"`
	HITSTO        bool          `json:"hits_timeout"`
	CriticalPath  time.Duration `json:"critical_path"`
	Cycles        time.Duration `json:"cycles"`
	CyclesTO      bool          `json:"cycles_timeout"`
	CycleCount    int           `json:"cycle_count"`
	KCore         time.Duration `json:"kcore"`        // bv-85
	Articulation  time.Duration `json:"articulation"` // bv-85
	Slack         time.Duration `json:"slack"`        // bv-85
	Phase2        time.Duration `json:"phase2_total"`

	// Configuration used
	Config AnalysisConfig `json:"config"`

	// Totals
	Total time.Duration `json:"total"`
}

// GraphStats holds the results of graph analysis.
// Phase 1 fields (OutDegree, InDegree, TopologicalOrder, Density) are populated
// immediately and can be read without synchronization after AnalyzeAsync returns.
// Phase 2 fields (centrality metrics, cycles) are computed in background and
// must be accessed via thread-safe accessor methods.
type GraphStats struct {
	// Phase 1 - Available immediately after AnalyzeAsync returns (read-only after init)
	OutDegree        map[string]int // Number of dependencies this issue has (edges out)
	InDegree         map[string]int // Number of issues that depend on this issue (edges in)
	TopologicalOrder []string
	Density          float64
	NodeCount        int // Number of nodes in graph
	EdgeCount        int // Number of edges in graph

	// Configuration used for this analysis (read-only after init)
	Config AnalysisConfig

	// Phase 2 - Computed in background, access via thread-safe methods only
	mu                sync.RWMutex
	phase2Ready       bool
	phase2Done        chan struct{} // Closed when Phase 2 completes
	pageRank          map[string]float64
	betweenness       map[string]float64
	eigenvector       map[string]float64
	hubs              map[string]float64
	authorities       map[string]float64
	criticalPathScore map[string]float64
	coreNumber        map[string]int
	articulation      map[string]bool
	slack             map[string]float64
	cycles            [][]string

	// Phase 2 status flags for robot visibility
	status MetricStatus
}

// metricStatus captures per-metric computation outcome for transparency.
type MetricStatus struct {
	PageRank     statusEntry
	Betweenness  statusEntry
	Eigenvector  statusEntry
	HITS         statusEntry
	Critical     statusEntry
	Cycles       statusEntry
	KCore        statusEntry // bv-85: k-core decomposition
	Articulation statusEntry // bv-85: articulation points (cut vertices)
	Slack        statusEntry // bv-85: longest-path slack per node
}

// statusEntry records computation state for a single metric.
type statusEntry struct {
	State   string        `json:"state"`            // computed|approx|timeout|skipped
	Reason  string        `json:"reason,omitempty"` // explanation when skipped/timeout/approx
	Sample  int           `json:"sample,omitempty"` // sample size when approximate
	Elapsed time.Duration `json:"ms,omitempty"`     // elapsed time
}

// IsPhase2Ready returns true if Phase 2 metrics have been computed.
func (s *GraphStats) IsPhase2Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.phase2Ready
}

// Status returns a copy of metric status flags.
func (s *GraphStats) Status() MetricStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// stateFromTiming converts config flags/timeouts to a user-facing state string.
func stateFromTiming(enabled bool, timedOut bool) string {
	switch {
	case !enabled:
		return "skipped"
	case timedOut:
		return "timeout"
	default:
		return "computed"
	}
}

func betweennessReason(cfg AnalysisConfig, isApprox bool) string {
	if cfg.BetweennessSkipReason != "" {
		return cfg.BetweennessSkipReason
	}
	if cfg.BetweennessMode == BetweennessApproximate || isApprox {
		return "approximate"
	}
	return ""
}

// WaitForPhase2 blocks until Phase 2 computation completes.
func (s *GraphStats) WaitForPhase2() {
	if s.phase2Done != nil {
		<-s.phase2Done
	}
}

// GetPageRankScore returns the PageRank score for a single issue.
// Returns 0 if Phase 2 is not yet complete or if the issue is not found.
func (s *GraphStats) GetPageRankScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pageRank == nil {
		return 0
	}
	return s.pageRank[id]
}

// GetBetweennessScore returns the betweenness centrality for a single issue.
func (s *GraphStats) GetBetweennessScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.betweenness == nil {
		return 0
	}
	return s.betweenness[id]
}

// GetEigenvectorScore returns the eigenvector centrality for a single issue.
func (s *GraphStats) GetEigenvectorScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.eigenvector == nil {
		return 0
	}
	return s.eigenvector[id]
}

// GetHubScore returns the hub score for a single issue.
func (s *GraphStats) GetHubScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.hubs == nil {
		return 0
	}
	return s.hubs[id]
}

// GetAuthorityScore returns the authority score for a single issue.
func (s *GraphStats) GetAuthorityScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.authorities == nil {
		return 0
	}
	return s.authorities[id]
}

// GetCriticalPathScore returns the critical path score for a single issue.
func (s *GraphStats) GetCriticalPathScore(id string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.criticalPathScore == nil {
		return 0
	}
	return s.criticalPathScore[id]
}

// PageRank returns a copy of the PageRank map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) PageRank() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.pageRank == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.pageRank))
	for k, v := range s.pageRank {
		cp[k] = v
	}
	return cp
}

// Betweenness returns a copy of the Betweenness map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) Betweenness() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.betweenness == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.betweenness))
	for k, v := range s.betweenness {
		cp[k] = v
	}
	return cp
}

// Eigenvector returns a copy of the Eigenvector map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) Eigenvector() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.eigenvector == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.eigenvector))
	for k, v := range s.eigenvector {
		cp[k] = v
	}
	return cp
}

// Hubs returns a copy of the Hubs map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) Hubs() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.hubs == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.hubs))
	for k, v := range s.hubs {
		cp[k] = v
	}
	return cp
}

// Authorities returns a copy of the Authorities map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) Authorities() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.authorities == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.authorities))
	for k, v := range s.authorities {
		cp[k] = v
	}
	return cp
}

// CriticalPathScore returns a copy of the CriticalPathScore map. Safe for concurrent iteration.
// Returns an empty map if Phase 2 is not yet complete.
func (s *GraphStats) CriticalPathScore() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.criticalPathScore == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.criticalPathScore))
	for k, v := range s.criticalPathScore {
		cp[k] = v
	}
	return cp
}

// CoreNumber returns k-core numbers per node (undirected view).
func (s *GraphStats) CoreNumber() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.coreNumber == nil {
		return nil
	}
	cp := make(map[string]int, len(s.coreNumber))
	for k, v := range s.coreNumber {
		cp[k] = v
	}
	return cp
}

// ArticulationPoints returns articulation points (cut vertices) detected on the undirected view.
func (s *GraphStats) ArticulationPoints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.articulation == nil {
		return nil
	}
	var points []string
	for k, v := range s.articulation {
		if v {
			points = append(points, k)
		}
	}
	sort.Strings(points)
	return points
}

// Slack returns per-node slack (longest-path slack; 0 on critical path).
func (s *GraphStats) Slack() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.slack == nil {
		return nil
	}
	cp := make(map[string]float64, len(s.slack))
	for k, v := range s.slack {
		cp[k] = v
	}
	return cp
}

// Cycles returns a copy of detected cycles. Safe for concurrent iteration.
// Returns nil if Phase 2 is not yet complete.
func (s *GraphStats) Cycles() [][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cycles == nil {
		return nil
	}
	cp := make([][]string, len(s.cycles))
	for i, c := range s.cycles {
		cp[i] = append([]string(nil), c...)
	}
	return cp
}

// NewGraphStatsForTest creates a GraphStats with the given data for testing.
// This allows tests to create GraphStats with specific values without needing
// to run the full analyzer.
func NewGraphStatsForTest(
	pageRank, betweenness, eigenvector, hubs, authorities, criticalPathScore map[string]float64,
	outDegree, inDegree map[string]int,
	cycles [][]string,
	density float64,
	topologicalOrder []string,
) *GraphStats {
	stats := &GraphStats{
		OutDegree:         outDegree,
		InDegree:          inDegree,
		TopologicalOrder:  topologicalOrder,
		Density:           density,
		phase2Done:        make(chan struct{}),
		pageRank:          pageRank,
		betweenness:       betweenness,
		eigenvector:       eigenvector,
		hubs:              hubs,
		authorities:       authorities,
		criticalPathScore: criticalPathScore,
		cycles:            cycles,
		phase2Ready:       true,
	}
	close(stats.phase2Done)
	return stats
}

// Analyzer encapsulates the graph logic
type Analyzer struct {
	g        *simple.DirectedGraph
	idToNode map[string]int64
	nodeToID map[int64]string
	issueMap map[string]model.Issue
	config   *AnalysisConfig // Optional custom config, nil means use size-based defaults
}

// SetConfig sets a custom analysis configuration.
// Pass nil to use size-based automatic configuration.
func (a *Analyzer) SetConfig(config *AnalysisConfig) {
	a.config = config
}

func NewAnalyzer(issues []model.Issue) *Analyzer {
	g := simple.NewDirectedGraph()
	// Pre-allocate maps for efficiency
	idToNode := make(map[string]int64, len(issues))
	nodeToID := make(map[int64]string, len(issues))
	issueMap := make(map[string]model.Issue, len(issues))

	// 1. Add Nodes
	for _, issue := range issues {
		issueMap[issue.ID] = issue
		n := g.NewNode()
		g.AddNode(n)
		idToNode[issue.ID] = n.ID()
		nodeToID[n.ID()] = issue.ID
	}

	// 2. Add Edges (Dependency Direction)
	// We only model *blocking* relationships in the analysis graph. Non-blocking
	// links such as "related" should not influence centrality metrics or cycle
	// detection because they do not gate execution order.
	for _, issue := range issues {
		u, ok := idToNode[issue.ID]
		if !ok {
			continue
		}

		for _, dep := range issue.Dependencies {
			if dep == nil {
				continue
			}

			// Only model blocking relationships in the analysis graph
			if !dep.Type.IsBlocking() {
				continue
			}

			v, exists := idToNode[dep.DependsOnID]
			if exists {
				// Issue (u) depends on v → edge u -> v
				g.SetEdge(g.NewEdge(g.Node(u), g.Node(v)))
			}
		}
	}

	return &Analyzer{
		g:        g,
		idToNode: idToNode,
		nodeToID: nodeToID,
		issueMap: issueMap,
	}
}

// AnalyzeAsync performs graph analysis in two phases for fast startup.
// Phase 1 (instant): Degree centrality, topological order, density
// Phase 2 (background): PageRank, Betweenness, Eigenvector, HITS, Cycles
// Returns immediately with Phase 1 data. Use IsPhase2Ready() or WaitForPhase2() for Phase 2.
//
// If SetConfig was called, uses that config. Otherwise uses ConfigForSize() to
// automatically select appropriate algorithms based on graph size.
func (a *Analyzer) AnalyzeAsync() *GraphStats {
	var config AnalysisConfig
	if a.config != nil {
		config = *a.config
	} else {
		nodeCount := len(a.issueMap)
		edgeCount := a.g.Edges().Len()
		config = ConfigForSize(nodeCount, edgeCount)
	}
	return a.AnalyzeAsyncWithConfig(config)
}

// AnalyzeAsyncWithConfig performs graph analysis with a custom configuration.
// This allows callers to override the default size-based algorithm selection.
func (a *Analyzer) AnalyzeAsyncWithConfig(config AnalysisConfig) *GraphStats {
	nodeCount := len(a.issueMap)
	edgeCount := a.g.Edges().Len()

	stats := &GraphStats{
		OutDegree:         make(map[string]int),
		InDegree:          make(map[string]int),
		NodeCount:         nodeCount,
		EdgeCount:         edgeCount,
		Config:            config,
		phase2Done:        make(chan struct{}),
		pageRank:          make(map[string]float64),
		betweenness:       make(map[string]float64),
		eigenvector:       make(map[string]float64),
		hubs:              make(map[string]float64),
		authorities:       make(map[string]float64),
		criticalPathScore: make(map[string]float64),
		status: MetricStatus{
			PageRank:    statusEntry{State: "pending"},
			Betweenness: statusEntry{State: "pending"},
			Eigenvector: statusEntry{State: "pending"},
			HITS:        statusEntry{State: "pending"},
			Critical:    statusEntry{State: "pending"},
			Cycles:      statusEntry{State: "pending"},
		},
	}

	// Handle empty graph - mark phase 2 ready immediately
	if nodeCount == 0 {
		stats.phase2Ready = true
		close(stats.phase2Done)
		return stats
	}

	// Phase 1: Fast metrics (degree centrality, topo sort, density)
	a.computePhase1(stats)

	// Phase 2: Expensive metrics in background goroutine
	go a.computePhase2(stats, config)

	return stats
}

// Analyze performs synchronous graph analysis (for backward compatibility).
// Blocks until all metrics are computed.
func (a *Analyzer) Analyze() GraphStats {
	stats := a.AnalyzeAsync()
	stats.WaitForPhase2()
	// Return a copy with public fields populated for backward compatibility
	return GraphStats{
		OutDegree:         stats.OutDegree,
		InDegree:          stats.InDegree,
		TopologicalOrder:  stats.TopologicalOrder,
		Density:           stats.Density,
		NodeCount:         stats.NodeCount,
		EdgeCount:         stats.EdgeCount,
		Config:            stats.Config,
		pageRank:          stats.pageRank,
		betweenness:       stats.betweenness,
		eigenvector:       stats.eigenvector,
		hubs:              stats.hubs,
		authorities:       stats.authorities,
		criticalPathScore: stats.criticalPathScore,
		coreNumber:        stats.coreNumber,
		articulation:      stats.articulation,
		slack:             stats.slack,
		cycles:            stats.cycles,
		phase2Ready:       true,
		status:            stats.status,
	}
}

// AnalyzeWithConfig performs synchronous graph analysis with a custom configuration.
func (a *Analyzer) AnalyzeWithConfig(config AnalysisConfig) GraphStats {
	stats := a.AnalyzeAsyncWithConfig(config)
	stats.WaitForPhase2()
	return GraphStats{
		OutDegree:         stats.OutDegree,
		InDegree:          stats.InDegree,
		TopologicalOrder:  stats.TopologicalOrder,
		Density:           stats.Density,
		NodeCount:         stats.NodeCount,
		EdgeCount:         stats.EdgeCount,
		Config:            stats.Config,
		pageRank:          stats.pageRank,
		betweenness:       stats.betweenness,
		eigenvector:       stats.eigenvector,
		hubs:              stats.hubs,
		authorities:       stats.authorities,
		criticalPathScore: stats.criticalPathScore,
		coreNumber:        stats.coreNumber,
		articulation:      stats.articulation,
		slack:             stats.slack,
		cycles:            stats.cycles,
		phase2Ready:       true,
		status:            stats.status,
	}
}

// AnalyzeWithProfile performs synchronous graph analysis and returns detailed timing profile.
// This is intended for diagnostics and the --profile-startup CLI flag.
func (a *Analyzer) AnalyzeWithProfile(config AnalysisConfig) (*GraphStats, *StartupProfile) {
	profile := &StartupProfile{
		Config: config,
	}

	totalStart := time.Now()

	nodeCount := len(a.issueMap)
	edgeCount := a.g.Edges().Len()

	profile.NodeCount = nodeCount
	profile.EdgeCount = edgeCount

	stats := &GraphStats{
		OutDegree:         make(map[string]int),
		InDegree:          make(map[string]int),
		NodeCount:         nodeCount,
		EdgeCount:         edgeCount,
		Config:            config,
		phase2Done:        make(chan struct{}),
		pageRank:          make(map[string]float64),
		betweenness:       make(map[string]float64),
		eigenvector:       make(map[string]float64),
		hubs:              make(map[string]float64),
		authorities:       make(map[string]float64),
		criticalPathScore: make(map[string]float64),
	}

	// Handle empty graph
	if nodeCount == 0 {
		stats.phase2Ready = true
		close(stats.phase2Done)
		profile.Total = time.Since(totalStart)
		return stats, profile
	}

	// Phase 1: Fast metrics with timing
	phase1Start := time.Now()
	a.computePhase1WithProfile(stats, profile)
	profile.Phase1 = time.Since(phase1Start)

	profile.Density = stats.Density

	// Phase 2: Expensive metrics synchronously with timing
	phase2Start := time.Now()
	a.computePhase2WithProfile(stats, config, profile)
	profile.Phase2 = time.Since(phase2Start)

	stats.phase2Ready = true
	close(stats.phase2Done)

	profile.Total = time.Since(totalStart)
	return stats, profile
}

// computePhase1WithProfile calculates fast metrics with timing instrumentation.
func (a *Analyzer) computePhase1WithProfile(stats *GraphStats, profile *StartupProfile) {
	// Degree centrality
	degreeStart := time.Now()
	nodes := a.g.Nodes()
	for nodes.Next() {
		n := nodes.Node()
		id := a.nodeToID[n.ID()]
		to := a.g.To(n.ID())
		stats.InDegree[id] = to.Len()
		from := a.g.From(n.ID())
		stats.OutDegree[id] = from.Len()
	}
	profile.Degree = time.Since(degreeStart)

	// Topological Sort
	topoStart := time.Now()
	sorted, err := topo.Sort(a.g)
	if err == nil {
		for i := len(sorted) - 1; i >= 0; i-- {
			stats.TopologicalOrder = append(stats.TopologicalOrder, a.nodeToID[sorted[i].ID()])
		}
	}
	profile.TopoSort = time.Since(topoStart)

	// Density
	n := float64(len(a.issueMap))
	e := float64(a.g.Edges().Len())
	if n > 1 {
		stats.Density = e / (n * (n - 1))
	}
}

// computePhase2WithProfile calculates expensive metrics with timing instrumentation.
func (a *Analyzer) computePhase2WithProfile(stats *GraphStats, config AnalysisConfig, profile *StartupProfile) {
	localPageRank := make(map[string]float64)
	localBetweenness := make(map[string]float64)
	localEigenvector := make(map[string]float64)
	localHubs := make(map[string]float64)
	localAuthorities := make(map[string]float64)
	localCriticalPath := make(map[string]float64)
	var localCore map[string]int
	var localArticulation map[string]bool
	var localSlack map[string]float64
	var localCycles [][]string

	betweennessIsApprox := false
	actualBetweennessSample := 0
	cyclesTruncated := false

	// PageRank
	if config.ComputePageRank {
		prStart := time.Now()
		prDone := make(chan map[int64]float64, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Panic -> implicitly causes timeout in parent
				}
			}()
			prDone <- network.PageRank(a.g, 0.85, 1e-6)
		}()

		timer := time.NewTimer(config.PageRankTimeout)
		select {
		case pr := <-prDone:
			timer.Stop()
			for id, score := range pr {
				localPageRank[a.nodeToID[id]] = score
			}
		case <-timer.C:
			profile.PageRankTO = true
			uniform := 1.0 / float64(len(a.issueMap))
			for id := range a.issueMap {
				localPageRank[id] = uniform
			}
		}
		profile.PageRank = time.Since(prStart)
	}

	// Betweenness
	if config.ComputeBetweenness {
		bwStart := time.Now()
		bwDone := make(chan BetweennessResult, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Panic -> implicitly causes timeout in parent
				}
			}()
			// Choose algorithm based on mode
			if config.BetweennessMode == BetweennessApproximate && config.BetweennessSampleSize > 0 {
				bwDone <- ApproxBetweenness(a.g, config.BetweennessSampleSize, 1)
			} else {
				// Exact mode or mode not set (default to exact)
				exact := network.Betweenness(a.g)
				bwDone <- BetweennessResult{
					Scores:     exact,
					Mode:       BetweennessExact,
					TotalNodes: a.g.Nodes().Len(),
				}
			}
		}()

		timer := time.NewTimer(config.BetweennessTimeout)
		select {
		case result := <-bwDone:
			timer.Stop()
			for id, score := range result.Scores {
				localBetweenness[a.nodeToID[id]] = score
			}
			// Track if approximation was used
			if result.Mode == BetweennessApproximate {
				betweennessIsApprox = true
				actualBetweennessSample = result.SampleSize
			}
		case <-timer.C:
			profile.BetweennessTO = true
		}
		profile.Betweenness = time.Since(bwStart)
	}

	// Eigenvector
	if config.ComputeEigenvector {
		evStart := time.Now()
		for id, score := range computeEigenvector(a.g) {
			localEigenvector[a.nodeToID[id]] = score
		}
		profile.Eigenvector = time.Since(evStart)
	}

	// HITS
	if config.ComputeHITS && a.g.Edges().Len() > 0 {
		hitsStart := time.Now()
		hitsDone := make(chan map[int64]network.HubAuthority, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Panic -> implicitly causes timeout in parent
				}
			}()
			hitsDone <- network.HITS(a.g, 1e-3)
		}()

		timer := time.NewTimer(config.HITSTimeout)
		select {
		case hubAuth := <-hitsDone:
			timer.Stop()
			for id, ha := range hubAuth {
				localHubs[a.nodeToID[id]] = ha.Hub
				localAuthorities[a.nodeToID[id]] = ha.Authority
			}
		case <-timer.C:
			profile.HITSTO = true
		}
		profile.HITS = time.Since(hitsStart)
	}

	// Critical Path
	if config.ComputeCriticalPath {
		cpStart := time.Now()
		sorted, err := topo.Sort(a.g)
		if err == nil {
			localCriticalPath = a.computeHeights(sorted)
		}
		profile.CriticalPath = time.Since(cpStart)
	}

	// Cycles
	if config.ComputeCycles {
		cyclesStart := time.Now()
		maxCycles := config.MaxCyclesToStore
		if maxCycles == 0 {
			maxCycles = 100
		}

		sccs := topo.TarjanSCC(a.g)
		hasCycles := false
		for _, scc := range sccs {
			if len(scc) > 1 {
				hasCycles = true
				break
			}
		}

		if hasCycles {
			cyclesDone := make(chan [][]graph.Node, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Panic -> implicitly causes timeout in parent
					}
				}()
				cyclesDone <- findCyclesSafe(a.g, maxCycles)
			}()

			timer := time.NewTimer(config.CyclesTimeout)
			select {
			case cycles := <-cyclesDone:
				timer.Stop()
				profile.CycleCount = len(cycles)
				cyclesToProcess := cycles
				if len(cyclesToProcess) > maxCycles {
					cyclesToProcess = cyclesToProcess[:maxCycles]
					cyclesTruncated = true
				}

				for _, cycle := range cyclesToProcess {
					var cycleIDs []string
					for _, n := range cycle {
						cycleIDs = append(cycleIDs, a.nodeToID[n.ID()])
					}
					localCycles = append(localCycles, cycleIDs)
				}
			case <-timer.C:
				profile.CyclesTO = true
			}
		}
		profile.Cycles = time.Since(cyclesStart)
	}

	// Advanced graph signals: k-core, articulation points (undirected), slack (bv-85)
	kcoreStart := time.Now()
	localCore, localArticulation = a.computeCoreAndArticulation()
	profile.KCore = time.Since(kcoreStart)
	profile.Articulation = 0 // Computed together with k-core

	slackStart := time.Now()
	localSlack = a.computeSlack()
	profile.Slack = time.Since(slackStart)

	// Atomic assignment
	stats.mu.Lock()
	stats.pageRank = localPageRank
	stats.betweenness = localBetweenness
	stats.eigenvector = localEigenvector
	stats.hubs = localHubs
	stats.authorities = localAuthorities
	stats.criticalPathScore = localCriticalPath
	stats.coreNumber = localCore
	stats.articulation = localArticulation
	stats.slack = localSlack
	stats.cycles = localCycles
	stats.phase2Ready = true

	cycleReason := config.CyclesSkipReason
	if cyclesTruncated {
		if cycleReason != "" {
			cycleReason += "; "
		}
		cycleReason += "truncated"
	}

	// record status snapshot
	stats.status = MetricStatus{
		PageRank: statusEntry{State: stateFromTiming(config.ComputePageRank, profile.PageRankTO), Elapsed: profile.PageRank},
		Betweenness: statusEntry{
			State:   stateFromTiming(config.ComputeBetweenness, profile.BetweennessTO),
			Reason:  betweennessReason(config, betweennessIsApprox),
			Sample:  actualBetweennessSample,
			Elapsed: profile.Betweenness,
		},
		Eigenvector:  statusEntry{State: stateFromTiming(config.ComputeEigenvector, false), Elapsed: profile.Eigenvector},
		HITS:         statusEntry{State: stateFromTiming(config.ComputeHITS, profile.HITSTO), Reason: config.HITSSkipReason, Elapsed: profile.HITS},
		Critical:     statusEntry{State: stateFromTiming(config.ComputeCriticalPath, false), Elapsed: profile.CriticalPath},
		Cycles:       statusEntry{State: stateFromTiming(config.ComputeCycles, profile.CyclesTO), Reason: cycleReason, Elapsed: profile.Cycles},
		KCore:        statusEntry{State: "computed", Elapsed: profile.KCore},        // bv-85: always computed (fast)
		Articulation: statusEntry{State: "computed", Elapsed: profile.Articulation}, // bv-85: computed with k-core
		Slack:        statusEntry{State: "computed", Elapsed: profile.Slack},        // bv-85: always computed (fast)
	}
	stats.mu.Unlock()
}

// computePhase1 calculates fast metrics synchronously.
func (a *Analyzer) computePhase1(stats *GraphStats) {
	nodes := a.g.Nodes()

	// Basic Degree Centrality
	for nodes.Next() {
		n := nodes.Node()
		id := a.nodeToID[n.ID()]

		// Edge direction: dependent -> dependency (A -> B means A depends on B)
		// To(n) = nodes pointing TO n = issues that depend on n = n blocks them
		to := a.g.To(n.ID())
		stats.InDegree[id] = to.Len() // Issues depending on me

		// From(n) = nodes n points TO = issues n depends on
		from := a.g.From(n.ID())
		stats.OutDegree[id] = from.Len() // Issues I depend on
	}

	// Topological Sort (fast for DAGs)
	sorted, err := topo.Sort(a.g)
	if err == nil {
		for i := len(sorted) - 1; i >= 0; i-- {
			stats.TopologicalOrder = append(stats.TopologicalOrder, a.nodeToID[sorted[i].ID()])
		}
	}

	// Density
	n := float64(len(a.issueMap))
	e := float64(a.g.Edges().Len())
	if n > 1 {
		stats.Density = e / (n * (n - 1))
	}
}

// computePhase2 calculates expensive metrics in background.
// Computes to local variables first, then atomically assigns under lock.
// Respects the config to skip expensive algorithms for large graphs.
func (a *Analyzer) computePhase2(stats *GraphStats, config AnalysisConfig) {
	defer close(stats.phase2Done)

	// Recover from panics to prevent crashing the entire application
	defer func() {
		if r := recover(); r != nil {
			stats.mu.Lock()
			defer stats.mu.Unlock()

			reason := fmt.Sprintf("panic: %v", r)
			failEntry := statusEntry{State: "panic", Reason: reason}

			// Mark all as failed so UI knows
			stats.status = MetricStatus{
				PageRank:     failEntry,
				Betweenness:  failEntry,
				Eigenvector:  failEntry,
				HITS:         failEntry,
				Critical:     failEntry,
				Cycles:       failEntry,
				KCore:        failEntry,
				Articulation: failEntry,
				Slack:        failEntry,
			}
			stats.phase2Ready = true
		}
	}()

	// Use the profiled version logic to avoid duplication
	// We discard the profile data as this is the standard run
	dummyProfile := &StartupProfile{}
	a.computePhase2WithProfile(stats, config, dummyProfile)
}

func (a *Analyzer) computeHeights(sorted []graph.Node) map[string]float64 {
	heights := make(map[int64]float64)
	impactScores := make(map[string]float64)

	for _, n := range sorted {
		nid := n.ID()
		maxParentHeight := 0.0

		to := a.g.To(nid)
		for to.Next() {
			p := to.Node()
			if h, ok := heights[p.ID()]; ok {
				if h > maxParentHeight {
					maxParentHeight = h
				}
			}
		}
		heights[nid] = 1.0 + maxParentHeight
		impactScores[a.nodeToID[nid]] = heights[nid]
	}

	return impactScores
}

// computeCoreAndArticulation builds an undirected view to derive k-core numbers and articulation points.
func (a *Analyzer) computeCoreAndArticulation() (map[string]int, map[string]bool) {
	u := simple.NewUndirectedGraph()

	// Add nodes
	nodes := a.g.Nodes()
	for nodes.Next() {
		n := nodes.Node()
		u.AddNode(simple.Node(n.ID()))
	}

	// Add undirected edges for each directed edge
	edges := a.g.Edges()
	for edges.Next() {
		e := edges.Edge()
		u.SetEdge(u.NewEdge(u.Node(e.From().ID()), u.Node(e.To().ID())))
	}

	core := computeKCore(u)
	art := findArticulationPoints(u)

	coreByID := make(map[string]int, len(core))
	artByID := make(map[string]bool, len(art))
	for id, val := range core {
		coreByID[a.nodeToID[id]] = val
	}
	for id := range art {
		artByID[a.nodeToID[id]] = true
	}
	return coreByID, artByID
}

// computeSlack calculates longest-path slack per node (0 on critical path).
// Edges are interpreted in execution order (prereq -> dependent), i.e., reversed from the stored direction.
func (a *Analyzer) computeSlack() map[string]float64 {
	if len(a.issueMap) == 0 {
		return nil
	}

	// Topological order (dependencies first)
	var order []string
	sorted, err := topo.Sort(a.g)
	if err == nil {
		for i := len(sorted) - 1; i >= 0; i-- {
			order = append(order, a.nodeToID[sorted[i].ID()])
		}
	}
	if len(order) == 0 {
		return nil
	}

	distFromStart := make(map[string]int, len(order))
	distToEnd := make(map[string]int, len(order))
	for _, id := range order {
		distFromStart[id] = 0
		distToEnd[id] = 0
	}

	// Helper for prerequisites (g.From) - items this node depends on
	prereqDeps := func(id string) []int64 {
		nID := a.idToNode[id]
		from := a.g.From(nID)
		var res []int64
		for from.Next() {
			res = append(res, from.Node().ID())
		}
		return res
	}

	// Forward pass: longest distance from any start to each node
	// Propagate from u to v (u -> v): dist[v] = max(dist[v], dist[u] + 1)
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		for _, dep := range prereqDeps(id) {
			depID := a.nodeToID[dep]
			if distFromStart[depID] < distFromStart[id]+1 {
				distFromStart[depID] = distFromStart[id] + 1
			}
		}
	}

	// Reverse pass: longest distance from node to any end
	// Propagate from v to u (u -> v): dist[u] = max(dist[u], dist[v] + 1)
	for _, id := range order {
		for _, dep := range prereqDeps(id) {
			depID := a.nodeToID[dep]
			if distToEnd[id] < distToEnd[depID]+1 {
				distToEnd[id] = distToEnd[depID] + 1
			}
		}
	}

	longest := 0
	for _, id := range order {
		if d := distFromStart[id] + distToEnd[id]; d > longest {
			longest = d
		}
	}

	slack := make(map[string]float64, len(order))
	for _, id := range order {
		slack[id] = float64(longest - distFromStart[id] - distToEnd[id])
	}
	return slack
}

// computeKCore returns core numbers using iterative k peeling (handles isolated nodes and preserves correct cores).
func computeKCore(g *simple.UndirectedGraph) map[int64]int {
	// Build adjacency and degrees
	deg := make(map[int64]int)
	adj := make(map[int64][]int64)
	nodes := g.Nodes()
	for nodes.Next() {
		n := nodes.Node()
		it := g.From(n.ID())
		for it.Next() {
			adj[n.ID()] = append(adj[n.ID()], it.Node().ID())
		}
		deg[n.ID()] = len(adj[n.ID()])
	}

	core := make(map[int64]int, len(deg))
	removed := make(map[int64]bool, len(deg))

	maxDeg := 0
	for _, d := range deg {
		if d > maxDeg {
			maxDeg = d
		}
	}

	for k := 1; k <= maxDeg; k++ {
		queue := make([]int64, 0)
		for id, d := range deg {
			if !removed[id] && d < k {
				queue = append(queue, id)
			}
		}

		for len(queue) > 0 {
			v := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			if removed[v] {
				continue
			}
			removed[v] = true
			// Highest core they failed to meet is k-1
			core[v] = k - 1
			for _, nbr := range adj[v] {
				if removed[nbr] {
					continue
				}
				deg[nbr]--
				if deg[nbr] < k {
					queue = append(queue, nbr)
				}
			}
		}
	}

	// Any nodes never removed get maxDeg
	for id := range deg {
		if !removed[id] {
			core[id] = maxDeg
		}
	}

	return core
}

// findArticulationPoints runs Tarjan to find cut vertices in an undirected graph.
func findArticulationPoints(g *simple.UndirectedGraph) map[int64]bool {
	var timeIdx int
	disc := make(map[int64]int)
	low := make(map[int64]int)
	parent := make(map[int64]int64)
	ap := make(map[int64]bool)

	const noParent int64 = -1

	var dfs func(v int64)
	dfs = func(v int64) {
		timeIdx++
		disc[v] = timeIdx
		low[v] = timeIdx
		childCount := 0

		it := g.From(v)
		for it.Next() {
			u := it.Node().ID()
			if disc[u] == 0 {
				parent[u] = v
				childCount++
				dfs(u)
				low[v] = min(low[v], low[u])

				// Root with >1 child OR low[u] >= disc[v]
				if parent[v] == noParent && childCount > 1 {
					ap[v] = true
				}
				if parent[v] != noParent && low[u] >= disc[v] {
					ap[v] = true
				}
			} else if u != parent[v] {
				low[v] = min(low[v], disc[u])
			}
		}
	}

	nodes := g.Nodes()
	for nodes.Next() {
		id := nodes.Node().ID()
		if disc[id] == 0 {
			parent[id] = noParent
			dfs(id)
		}
	}
	return ap
}

// GetActionableIssues returns issues that can be worked on immediately.
// An issue is actionable if:
// 1. It is not closed
// 2. All its blocking dependencies (type "blocks") are closed
// Missing blockers don't block (graceful degradation).
// Returns list sorted by ID for determinism.
func (a *Analyzer) GetActionableIssues() []model.Issue {
	var actionable []model.Issue

	// Collect IDs first to sort for deterministic iteration
	var ids []string
	for id := range a.issueMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		issue := a.issueMap[id]
		if issue.Status == model.StatusClosed {
			continue
		}

		isBlocked := false
		for _, dep := range issue.Dependencies {
			if !dep.Type.IsBlocking() {
				continue
			}

			blocker, exists := a.issueMap[dep.DependsOnID]
			if !exists {
				continue
			}

			if blocker.Status != model.StatusClosed {
				isBlocked = true
				break
			}
		}

		if !isBlocked {
			actionable = append(actionable, issue)
		}
	}

	return actionable
}

// GetIssue returns a single issue by ID, or nil if not found
func (a *Analyzer) GetIssue(id string) *model.Issue {
	if issue, ok := a.issueMap[id]; ok {
		return &issue
	}
	return nil
}

// GetBlockers returns the IDs of issues that block the given issue
func (a *Analyzer) GetBlockers(issueID string) []string {
	issue, ok := a.issueMap[issueID]
	if !ok {
		return nil
	}

	var blockers []string
	for _, dep := range issue.Dependencies {
		if dep.Type.IsBlocking() {
			if _, exists := a.issueMap[dep.DependsOnID]; exists {
				blockers = append(blockers, dep.DependsOnID)
			}
		}
	}
	return blockers
}

// GetOpenBlockers returns the IDs of non-closed issues that block the given issue
func (a *Analyzer) GetOpenBlockers(issueID string) []string {
	issue, ok := a.issueMap[issueID]
	if !ok {
		return nil
	}

	var openBlockers []string
	for _, dep := range issue.Dependencies {
		if dep.Type.IsBlocking() {
			if blocker, exists := a.issueMap[dep.DependsOnID]; exists {
				if blocker.Status != model.StatusClosed {
					openBlockers = append(openBlockers, dep.DependsOnID)
				}
			}
		}
	}
	return openBlockers
}

// computeEigenvector runs a simple power-iteration to estimate eigenvector centrality.
func computeEigenvector(g graph.Directed) map[int64]float64 {
	nodes := g.Nodes()
	var nodeList []graph.Node
	for nodes.Next() {
		nodeList = append(nodeList, nodes.Node())
	}
	n := len(nodeList)
	if n == 0 {
		return nil
	}

	// Sort nodes by ID for deterministic iteration order
	sort.Slice(nodeList, func(i, j int) bool {
		return nodeList[i].ID() < nodeList[j].ID()
	})

	// Pre-calculate and sort incoming neighbors for every node
	// This avoids allocating and sorting inside the hot loop
	incomingMap := make(map[int64][]int, n)
	index := make(map[int64]int, n)
	for i, node := range nodeList {
		index[node.ID()] = i
	}

	for _, node := range nodeList {
		var neighbors []graph.Node
		incoming := g.To(node.ID())
		for incoming.Next() {
			neighbors = append(neighbors, incoming.Node())
		}
		// Deterministic order for summation
		sort.Slice(neighbors, func(a, b int) bool {
			return neighbors[a].ID() < neighbors[b].ID()
		})

		// Store indices directly to avoid map lookups in loop
		indices := make([]int, len(neighbors))
		for k, neighbor := range neighbors {
			indices[k] = index[neighbor.ID()]
		}
		incomingMap[node.ID()] = indices
	}

	vec := make([]float64, n)
	for i := range vec {
		vec[i] = 1.0 / float64(n)
	}
	work := make([]float64, n)

	const iterations = 50
	for iter := 0; iter < iterations; iter++ {
		for i := range work {
			work[i] = 0
		}
		for i, node := range nodeList {
			// Use pre-calculated sorted indices
			for _, j := range incomingMap[node.ID()] {
				work[i] += vec[j]
			}
		}
		sum := 0.0
		for _, v := range work {
			sum += v * v
		}
		if sum == 0 {
			break
		}
		norm := 1 / math.Sqrt(sum)
		for i := range work {
			vec[i] = work[i] * norm
		}
	}

	res := make(map[int64]float64, n)
	for i, node := range nodeList {
		res[node.ID()] = vec[i]
	}
	return res
}
