package analysis

import (
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/topo"
)

// findCyclesSafe finds a limited number of cycles in the graph without exponential blowup.
// It uses Tarjan's SCC algorithm to identify cyclic components and extracts one cycle per component.
func findCyclesSafe(g graph.Directed, limit int) [][]graph.Node {
	sccs := topo.TarjanSCC(g)
	var cycles [][]graph.Node

	for _, scc := range sccs {
		if len(cycles) >= limit {
			break
		}

		if len(scc) == 1 {
			// Check for self-loop
			n := scc[0]
			if g.HasEdgeFromTo(n.ID(), n.ID()) {
				cycles = append(cycles, []graph.Node{n, n})
			}
			continue
		}

		// Find a cycle within this non-trivial SCC
		if cycle := findOneCycleInSCC(g, scc); len(cycle) > 0 {
			cycles = append(cycles, cycle)
		}
	}

	// Sort cycles for determinism
	// 1. By length (ascending - shortest cycles are more interesting/fixable)
	// 2. By ID of first node (numerical)
	sort.Slice(cycles, func(i, j int) bool {
		if len(cycles[i]) != len(cycles[j]) {
			return len(cycles[i]) < len(cycles[j])
		}
		// Compare first node IDs as tie-breaker (numerical)
		return cycles[i][0].ID() < cycles[j][0].ID()
	})

	return cycles
}

// findOneCycleInSCC finds a single cycle within a Strongly Connected Component.
func findOneCycleInSCC(g graph.Directed, scc []graph.Node) []graph.Node {
	// Sort SCC nodes for deterministic DFS starting point
	sort.Slice(scc, func(i, j int) bool {
		return scc[i].ID() < scc[j].ID()
	})

	// Build a set for fast containment check
	inSCC := make(map[int64]bool, len(scc))
	for _, n := range scc {
		inSCC[n.ID()] = true
	}

	// Pre-compute and sort adjacency lists for nodes in SCC
	// This avoids repeated filtering and sorting during recursion
	adj := make(map[int64][]graph.Node, len(scc))
	for _, u := range scc {
		to := g.To(u.ID())
		var neighbors []graph.Node
		for to.Next() {
			n := to.Node()
			if inSCC[n.ID()] {
				neighbors = append(neighbors, n)
			}
		}
		sort.Slice(neighbors, func(i, j int) bool {
			return neighbors[i].ID() < neighbors[j].ID()
		})
		adj[u.ID()] = neighbors
	}

	// DFS state
	visited := make(map[int64]bool)
	stack := make([]graph.Node, 0)
	onStack := make(map[int64]bool)

	var dfs func(u graph.Node) []graph.Node
	dfs = func(u graph.Node) []graph.Node {
		visited[u.ID()] = true
		stack = append(stack, u)
		onStack[u.ID()] = true

		// Iterate pre-sorted neighbors
		for _, v := range adj[u.ID()] {
			if onStack[v.ID()] {
				// Cycle found! Reconstruct path from v to u then close with v
				var cycle []graph.Node
				// Find index of v in stack
				idx := -1
				for i, n := range stack {
					if n.ID() == v.ID() {
						idx = i
						break
					}
				}
				if idx != -1 {
					cycle = append(cycle, stack[idx:]...)
					cycle = append(cycle, v) // Close the loop
					return cycle
				}
			}

			if !visited[v.ID()] {
				if res := dfs(v); res != nil {
					return res
				}
			}
		}

		// Backtrack
		stack = stack[:len(stack)-1]
		onStack[u.ID()] = false
		return nil
	}

	// Start DFS from the first node in SCC (guaranteed to be part of at least one cycle in a non-trivial SCC)
	if len(scc) > 0 {
		return dfs(scc[0])
	}
	return nil
}