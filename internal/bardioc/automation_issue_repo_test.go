package bardioc

import (
	"context"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAutomationIssuePayload_MarshalJSON_FlattensAttributes(t *testing.T) {
	payload := AutomationIssuePayload{
		Entity:     graph.Entity{XID: "issue-xid-1"},
		Attributes: map[string]string{"ogit/subject": "Drucker kaputt", "/userMail": "anrufer@example.com"},
	}

	b, err := payload.MarshalJSON()

	require.NoError(t, err)
	require.JSONEq(t, `{"ogit/_xid":"issue-xid-1","ogit/subject":"Drucker kaputt","/userMail":"anrufer@example.com"}`, string(b))
}

func TestAutomationIssueRepository_Create_ReturnsNewIssueID(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewAutomationIssueRepository(client)

	created := automation.AutomationIssue{}
	created.Metadata = &graph.Metadata{ID: graph.MetadataID("issue-1")}
	created.Status = "UNPROCESSED"

	client.EXPECT().
		CreateEntity(mock.Anything, mock.Anything).
		Return(newSingleRow(created)).
		Once()

	id, err := repo.Create(context.Background(), map[string]string{"ogit/subject": "Drucker kaputt"})

	require.NoError(t, err)
	require.Equal(t, graph.MetadataID("issue-1"), id)
}

func TestAutomationIssueRepository_Status_ReturnsCurrentStatus(t *testing.T) {
	client := mocks.NewMockEdgeClient(t)
	repo := NewAutomationIssueRepository(client)

	issue := automation.AutomationIssue{}
	issue.Metadata = &graph.Metadata{ID: graph.MetadataID("issue-1")}
	issue.Status = "RESOLVED"

	client.EXPECT().
		GetEntity(mock.Anything, graph.MetadataID("issue-1"), mock.Anything).
		Return(newSingleRow(issue)).
		Once()

	status, err := repo.Status(context.Background(), graph.MetadataID("issue-1"))

	require.NoError(t, err)
	require.Equal(t, "RESOLVED", status)
}
