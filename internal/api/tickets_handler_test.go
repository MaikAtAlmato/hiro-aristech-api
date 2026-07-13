// internal/api/tickets_handler_test.go
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/almatoag/bardioc-go/graph"
	servicemanagement "bitbucket.org/almatoag/graph-go/NTO/ServiceManagement"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

type fakeTicketFinder struct {
	byPerson map[string][]servicemanagement.Ticket
}

func (f fakeTicketFinder) FindForPerson(_ context.Context, personID graph.MetadataID) ([]servicemanagement.Ticket, error) {
	return f.byPerson[personID.String()], nil
}

func newTestServerWithTickets(tickets fakeTicketFinder) (*Server, *auth.TokenService) {
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	return NewServer(resolver, &identity.Matcher{}, tokens, tickets, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey), tokens
}

func TestTicketsHandler_ValidToken_ReturnsTickets(t *testing.T) {
	ticket := servicemanagement.Ticket{}
	ticket.Id = "ticket-1"
	ticket.Subject = "VPN broken"

	server, tokens := newTestServerWithTickets(fakeTicketFinder{
		byPerson: map[string][]servicemanagement.Ticket{"vm-1": {ticket}},
	})
	token, err := tokens.Issue("Max Mustermann", "vm-1", "")
	require.NoError(t, err)

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "VPN broken")
}

func TestTicketsHandler_NoValuemationIdentity_ReturnsEmptyList(t *testing.T) {
	server, tokens := newTestServerWithTickets(fakeTicketFinder{})
	token, err := tokens.Issue("Max Mustermann", "", "ms-1")
	require.NoError(t, err)

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"tickets":[]`)
}

func TestTicketsHandler_MissingAuthHeader_Returns401(t *testing.T) {
	server, _ := newTestServerWithTickets(fakeTicketFinder{})

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTicketsHandler_InvalidToken_Returns401(t *testing.T) {
	server, _ := newTestServerWithTickets(fakeTicketFinder{})

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets", "Authorization: Bearer not-a-real-token", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestTicketsHandler_MissingBearerPrefix_Returns401(t *testing.T) {
	server, tokens := newTestServerWithTickets(fakeTicketFinder{})
	token, err := tokens.Issue("Max Mustermann", "vm-1", "")
	require.NoError(t, err)

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets", fmt.Sprintf("Authorization: %s", token), "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}
