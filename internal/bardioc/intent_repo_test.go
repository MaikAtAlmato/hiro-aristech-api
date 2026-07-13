package bardioc

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIntentRepository_Get_ReturnsIntent(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewIntentRepository(client)

	intent := automation.Intent{
		Description:          "Ticket erstellen (allgemein)",
		SystemVariableNames:  "/dont_process_ticket, /ParloaIntent, /ForwardTicketIn",
		SystemVariableValues: "true, CreateTicket, true",
	}
	intent.Metadata = &graph.Metadata{ID: graph.MetadataID("intent-1")}

	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("intent-1"), mock.Anything).
		Return(newSingleRow(intent)).
		Once()

	got, err := repo.Get(context.Background(), graph.MetadataID("intent-1"))

	require.NoError(t, err)
	require.Equal(t, "Ticket erstellen (allgemein)", got.Description)
}

func TestSystemVariables_PairsNamesAndValuesPositionally(t *testing.T) {
	intent := automation.Intent{
		SystemVariableNames:  "/dont_process_ticket, /ParloaIntent, /ForwardTicketIn",
		SystemVariableValues: "true, CreateTicket, true",
	}

	vars := SystemVariables(intent)

	require.Equal(t, map[string]string{
		"/dont_process_ticket": "true",
		"/ParloaIntent":        "CreateTicket",
		"/ForwardTicketIn":     "true",
	}, vars)
}

func TestSystemVariables_MissingTrailingValueDefaultsToEmpty(t *testing.T) {
	intent := automation.Intent{
		SystemVariableNames:  "/a, /b",
		SystemVariableValues: "1",
	}

	vars := SystemVariables(intent)

	require.Equal(t, map[string]string{"/a": "1", "/b": ""}, vars)
}

func TestSystemVariables_EmptyIntentReturnsEmptyMap(t *testing.T) {
	vars := SystemVariables(automation.Intent{})

	require.Empty(t, vars)
}

func TestIntentRepository_List_NoFilter_ReturnsEverything(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewIntentRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(
			map[string]any{"ogit/_id": "intent-1", "ogit/description": "Passwort zurücksetzen"},
			map[string]any{"ogit/_id": "intent-2", "ogit/description": "Ticket erstellen"},
		), nil).
		Once()

	got, err := repo.List(context.Background(), nil)

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "intent-1", got[0]["ogit/_id"])
}

func TestIntentRepository_List_SingleType_FiltersByIntentType(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewIntentRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(
			map[string]any{"ogit/_id": "intent-1", "/IntentType": "Mainintents"},
		), nil).
		Once()

	got, err := repo.List(context.Background(), []string{"Mainintents"})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Mainintents", got[0]["/IntentType"])
}

func TestIntentRepository_List_MultipleTypes_QueriesEachAndDedupes(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewIntentRepository(client)

	shared := map[string]any{"ogit/_id": "intent-1", "/IntentType": "Mainintents"}
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(shared), nil).
		Once()
	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(
			shared, // same node returned again for the second type — must be deduped
			map[string]any{"ogit/_id": "intent-2", "/IntentType": "Subintents"},
		), nil).
		Once()

	got, err := repo.List(context.Background(), []string{"Mainintents", "Subintents"})

	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestIntentRepository_List_QueryError_Wrapped(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewIntentRepository(client)

	client.EXPECT().
		QueryVertices(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("graph unavailable")).
		Once()

	_, err := repo.List(context.Background(), nil)

	require.Error(t, err)
}
