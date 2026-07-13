package bardioc

import (
	"testing"

	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Name string `json:"name"`
}

func TestFakeRows_IteratesAndScans(t *testing.T) {
	rows := newFakeRows(sample{Name: "a"}, sample{Name: "b"})

	var got []string
	for rows.Next() {
		var s sample
		require.NoError(t, rows.Scan(&s))
		got = append(got, s.Name)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"a", "b"}, got)
	require.False(t, rows.Next())
}

func TestFakeRows_ScanPastEnd(t *testing.T) {
	rows := newFakeRows()
	require.False(t, rows.Next())
	var s sample
	require.Error(t, rows.Scan(&s))
}

func TestMockEdgeClient_SatisfiesInterface(t *testing.T) {
	var _ EdgeClient = newMockEdgeClientForTest(t)
}

func newMockEdgeClientForTest(t *testing.T) *mocks.MockEdgeClient {
	t.Helper()
	return mocks.NewMockEdgeClient(t)
}
