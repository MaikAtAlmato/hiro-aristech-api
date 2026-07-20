package bardioc

import (
	"context"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func ticketEntity(id, subject string) Ticket {
	t := Ticket{}
	t.Metadata = &graph.Metadata{ID: graph.MetadataID(id)}
	t.Id = id
	t.Subject = subject
	return t
}

func TestTicketRepository_FindForPerson_CollectsAndDedupes(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewTicketRepository(client)

	personID := graph.MetadataID("vm-1")

	// affects (incoming) -> ticket-1
	affectsVerb := graph.Verb{OutID: graph.MetadataID("ticket-1")}
	// assignedTo (incoming) -> ticket-1 again (must dedupe)
	assignedVerb := graph.Verb{OutID: graph.MetadataID("ticket-1")}
	// opens (outgoing) -> ticket-2
	opensVerb := graph.Verb{InID: graph.MetadataID("ticket-2")}

	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, personID, mock.Anything, mock.Anything).
		Return(newFakeRows(affectsVerb), nil).
		Once()
	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, personID, mock.Anything, mock.Anything).
		Return(newFakeRows(assignedVerb), nil).
		Once()
	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, personID, mock.Anything, mock.Anything).
		Return(newFakeRows(opensVerb), nil).
		Once()

	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("ticket-1"), mock.Anything).
		Return(newSingleRow(ticketEntity("ticket-1", "VPN broken"))).
		Once()
	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("ticket-2"), mock.Anything).
		Return(newSingleRow(ticketEntity("ticket-2", "Password reset"))).
		Once()

	got, err := repo.FindForPerson(context.Background(), personID)

	require.NoError(t, err)
	require.Len(t, got, 2)
	subjects := []string{got[0].Subject, got[1].Subject}
	require.ElementsMatch(t, []string{"VPN broken", "Password reset"}, subjects)
}

func TestTicketRepository_FindForPerson_NoEdges(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewTicketRepository(client)

	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Times(3)

	got, err := repo.FindForPerson(context.Background(), graph.MetadataID("vm-2"))

	require.NoError(t, err)
	require.Empty(t, got)
}
