// internal/bardioc/valuemation_person_repo_test.go
package bardioc

import (
	"context"
	"strings"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func valuemationPersonWithID(id, firstName, lastName, phone string) ValuemationPerson {
	return ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(id)}},
			FirstName: firstName,
			LastName:  lastName,
		},
		PhoneNo: phone,
	}
}

func TestValuemationPersonRepository_FindByPhone(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	match := valuemationPersonWithID("vm-1", "Max", "Mustermann", "+491234567890")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	got, err := repo.FindByPhone(context.Background(), "+491234567890")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "vm-1", got[0].Metadata.ID.String())
	require.Equal(t, "+491234567890", got[0].PhoneNo)
}

func TestValuemationPersonRepository_FindByPhone_MatchesOfficePhone(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	match := valuemationPersonWithID("vm-3", "Max", "Mustermann", "")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByPhone(context.Background(), "+491234567890")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "vm-3", got[0].Metadata.ID.String())
}

func TestValuemationPersonRepository_FindByPhone_NoMatch(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Twice()

	got, err := repo.FindByPhone(context.Background(), "+490000000000")

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestValuemationPersonRepository_FindByName(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	match := valuemationPersonWithID("vm-2", "Erika", "Musterfrau", "+499999999999")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByName(context.Background(), "Erika", "Musterfrau")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "vm-2", got[0].Metadata.ID.String())
}

func TestValuemationPersonRepository_FindByName_IsCaseInsensitive(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	match := valuemationPersonWithID("vm-2", "Erika", "Musterfrau", "+499999999999")
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

func TestValuemationPersonRepository_FindByNamePrefix(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	match := valuemationPersonWithID("vm-3", "Maik", "Lander", "")
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(match), nil).
		Once()

	got, err := repo.FindByNamePrefix(context.Background(), "M", "L")

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "vm-3", got[0].Metadata.ID.String())
}

func TestValuemationPersonRepository_FindByName_ContainsDGFilter(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.MatchedBy(func(b *esquery.Builder) bool {
			q := b.Build()
			return strings.Contains(q, `\/hlq1Value:DG`) && strings.Contains(q, `ogit\/status:ACTIVE`)
		}), mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	_, _ = repo.FindByName(context.Background(), "Max", "Mustermann")
}

func TestValuemationPersonRepository_FindByNamePrefix_ContainsDGFilter(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.MatchedBy(func(b *esquery.Builder) bool {
			q := b.Build()
			return strings.Contains(q, `\/hlq1Value:DG`) && strings.Contains(q, `ogit\/status:ACTIVE`)
		}), mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	_, _ = repo.FindByNamePrefix(context.Background(), "Max", "Mus")
}

func TestValuemationPersonRepository_FindByPhone_ContainsDGFilter(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.MatchedBy(func(b *esquery.Builder) bool {
			q := b.Build()
			return strings.Contains(q, `\/hlq1Value:DG`) && strings.Contains(q, `ogit\/status:ACTIVE`)
		}), mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Twice()

	_, _ = repo.FindByPhone(context.Background(), "+491234567890")
}

func TestValuemationPersonRepository_FindByNamePrefix_PhoneticVariants(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	// prefix "Sel" → variants ["Cel","Sel","Zel"]; esquery expands each to [xX] bracket form.
	// Assert OR is present and all three letter-class patterns appear in the query.
	client.EXPECT().
		QueryVertices(mock.Anything, mock.MatchedBy(func(b *esquery.Builder) bool {
			q := b.Build()
			return strings.Contains(q, " OR ") &&
				strings.Contains(q, "[cC][eE][lL]") &&
				strings.Contains(q, "[sS][eE][lL]") &&
				strings.Contains(q, "[zZ][eE][lL]")
		}), mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	_, _ = repo.FindByNamePrefix(context.Background(), "Thomas", "Sel")
}

func TestValuemationPersonRepository_FindByNamePrefix_SingleVariant_NoOR(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewValuemationPersonRepository(client)

	// prefix "Lan" → only ["Lan"] (L has no equivalents); no OR, no group wrapping.
	// esquery expands "Lan" to [lL][aA][nN] bracket form; OpenGroup/CloseGroup emit "(" and ")".
	client.EXPECT().
		QueryVertices(mock.Anything, mock.MatchedBy(func(b *esquery.Builder) bool {
			q := b.Build()
			return strings.Contains(q, "[lL][aA][nN]") &&
				!strings.Contains(q, " OR ") &&
				!strings.Contains(q, "(ogit\\/lastName")
		}), mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	_, _ = repo.FindByNamePrefix(context.Background(), "Thomas", "Lan")
}
