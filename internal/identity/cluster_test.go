package identity

import (
	"reflect"
	"sort"
	"testing"
)

func sortedClusters(clusters [][]int) [][]int {
	out := make([][]int, len(clusters))
	for i, c := range clusters {
		cp := append([]int(nil), c...)
		sort.Ints(cp)
		out[i] = cp
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func TestClusterIndices_NoCandidates(t *testing.T) {
	clusters := clusterIndices(0, func(i, j int) bool { return false })
	if len(clusters) != 0 {
		t.Fatalf("expected no clusters, got %v", clusters)
	}
}

func TestClusterIndices_AllDisjoint(t *testing.T) {
	clusters := clusterIndices(3, func(i, j int) bool { return false })
	got := sortedClusters(clusters)
	want := [][]int{{0}, {1}, {2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestClusterIndices_AllLinked(t *testing.T) {
	clusters := clusterIndices(3, func(i, j int) bool { return true })
	got := sortedClusters(clusters)
	want := [][]int{{0, 1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestClusterIndices_TransitiveChain(t *testing.T) {
	// 0-1 linked, 1-2 linked, 0-2 NOT directly linked: still one cluster.
	linked := func(i, j int) bool {
		return (i == 0 && j == 1) || (i == 1 && j == 2)
	}
	clusters := clusterIndices(3, linked)
	got := sortedClusters(clusters)
	want := [][]int{{0, 1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestClusterIndices_TwoDisjointGroups(t *testing.T) {
	// {0,1} linked, {2,3} linked, no cross-link.
	linked := func(i, j int) bool {
		return (i == 0 && j == 1) || (i == 2 && j == 3)
	}
	clusters := clusterIndices(4, linked)
	got := sortedClusters(clusters)
	want := [][]int{{0, 1}, {2, 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
