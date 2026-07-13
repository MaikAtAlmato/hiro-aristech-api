// internal/bardioc/msgraph_person_repo_test.go
package bardioc

import (
	"context"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func personWithID(id, firstName, lastName string) sgo.Person {
	return sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(id)}},
		FirstName: firstName,
		LastName:  lastName,
	}
}

func TestMsgraphPersonRepository_FindByPhone_MatchesAnyPhoneField(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	match := personWithID("ms-1", "Max", "Mustermann")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Twice()

	got, err := repo.FindByPhone(context.Background(), "+491234567890")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ms-1", got[0].Metadata.ID.String())
}

func TestMsgraphPersonRepository_FindByPhone_DedupesAcrossFields(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	match := personWithID("ms-1", "Max", "Mustermann")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Times(3)

	got, err := repo.FindByPhone(context.Background(), "+491234567890")

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestMsgraphPersonRepository_FindByPhone_NoMatch(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Times(3)

	got, err := repo.FindByPhone(context.Background(), "+490000000000")

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestMsgraphPersonRepository_FindByName(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	match := personWithID("ms-2", "Erika", "Musterfrau")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByName(context.Background(), "Erika", "Musterfrau")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ms-2", got[0].Metadata.ID.String())
}

func TestMsgraphPersonRepository_FindByName_IsCaseInsensitive(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	match := personWithID("ms-2", "Erika", "Musterfrau")
	var capturedQuery string
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, b *esquery.Builder, _ ...graph.Param) {
			capturedQuery = b.Build()
		}).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByName(context.Background(), "eRiKa", "MUSTERFRAU")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Contains(t, capturedQuery, "[eE][rR][iI][kK][aA]")
	require.Contains(t, capturedQuery, "[mM][uU][sS][tT][eE][rR][fF][rR][aA][uU]")
}

func TestMsgraphPersonRepository_FindByNamePrefix(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphPersonRepository(client)

	match := personWithID("ms-3", "Maik", "Lander")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByNamePrefix(context.Background(), "M", "L")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ms-3", got[0].Metadata.ID.String())
}
