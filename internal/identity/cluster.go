package identity

// clusterIndices groups the indices 0..n-1 into clusters using union-find,
// where linked(i, j) (called only for i < j) decides whether candidates i
// and j belong to the same cluster. linked need not be transitive on its
// own — union-find closes the relation transitively, so if 0 links to 1 and
// 1 links to 2, all three end up in one cluster even if 0 and 2 don't
// directly link.
func clusterIndices(n int, linked func(i, j int) bool) [][]int {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}

	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}

	union := func(x, y int) {
		rx, ry := find(x), find(y)
		if rx != ry {
			parent[rx] = ry
		}
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if linked(i, j) {
				union(i, j)
			}
		}
	}

	groups := map[int][]int{}
	for i := 0; i < n; i++ {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	clusters := make([][]int, 0, len(groups))
	for _, g := range groups {
		clusters = append(clusters, g)
	}
	return clusters
}
