package bardioc

import (
	"context"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func msgraphAccount(id, xid, status string) MsgraphAccount {
	var a MsgraphAccount
	a.Entity = graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(id)}, XID: xid}
	a.PFlag = PFlagMsgraph
	a.Status = status
	return a
}

func TestMsgraphAccountRepository_FindForPerson_ReturnsConnectedAccount(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphAccountRepository(client)

	personID := graph.MetadataID("ms-1")
	connectsVerb := graph.Verb{OutID: graph.MetadataID("acct-1")}

	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, personID, mock.Anything, mock.Anything).
		Return(newFakeRows(connectsVerb), nil).
		Once()
	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("acct-1"), mock.Anything).
		Return(newSingleRow(msgraphAccount("acct-1", "acct-xid-1", "active"))).
		Once()

	got, err := repo.FindForPerson(context.Background(), personID)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "acct-xid-1", got.XID)
	require.Equal(t, "active", got.Status)
}

func TestMsgraphAccountRepository_FindForPerson_NoConnectedAccount(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphAccountRepository(client)

	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(newFakeRows(), nil).
		Once()

	got, err := repo.FindForPerson(context.Background(), graph.MetadataID("ms-2"))

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestMsgraphAccountRepository_FindForPerson_SkipsAccountsFromOtherConnectors(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewMsgraphAccountRepository(client)

	personID := graph.MetadataID("ms-3")
	connectsVerb := graph.Verb{OutID: graph.MetadataID("acct-2")}

	other := msgraphAccount("acct-2", "acct-xid-2", "active")
	other.PFlag = "some-other-connector"

	client.EXPECT().
		QueryGremlin(mock.Anything, mock.Anything, personID, mock.Anything, mock.Anything).
		Return(newFakeRows(connectsVerb), nil).
		Once()
	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("acct-2"), mock.Anything).
		Return(newSingleRow(other)).
		Once()

	got, err := repo.FindForPerson(context.Background(), personID)

	require.NoError(t, err)
	require.Nil(t, got)
}
