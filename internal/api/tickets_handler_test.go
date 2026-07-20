// internal/api/tickets_handler_test.go
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

type fakeTicketFinder struct {
	byPerson map[string][]bardioc.Ticket
	byID     map[string]*bardioc.Ticket
}

func (f fakeTicketFinder) FindForPerson(_ context.Context, personID graph.MetadataID) ([]bardioc.Ticket, error) {
	return f.byPerson[personID.String()], nil
}

func (f fakeTicketFinder) FindByID(_ context.Context, id string) (*bardioc.Ticket, error) {
	return f.byID[id], nil
}

func (f fakeTicketFinder) FindByIDForPerson(_ context.Context, id string, _ graph.MetadataID) (*bardioc.Ticket, error) {
	return f.byID[id], nil
}

func newTestServerWithTickets(tickets fakeTicketFinder) (*Server, *auth.TokenService) {
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	return NewServer(resolver, &identity.Matcher{}, tokens, tickets, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey), tokens
}

func TestTicketsHandler_ValidToken_ReturnsTickets(t *testing.T) {
	ticket := bardioc.Ticket{}
	ticket.Id = "ticket-1"
	ticket.Subject = "VPN broken"

	server, tokens := newTestServerWithTickets(fakeTicketFinder{
		byPerson: map[string][]bardioc.Ticket{"vm-1": {ticket}},
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

func TestTicketByIDHandler_Found_ReturnsTicket(t *testing.T) {
	ticket := bardioc.Ticket{}
	ticket.Id = "IN-3084747"
	ticket.Subject = "VPN broken"

	server, tokens := newTestServerWithTickets(fakeTicketFinder{
		byID: map[string]*bardioc.Ticket{"IN-3084747": &ticket},
	})
	token, err := tokens.Issue("Max Mustermann", "vm-1", "")
	require.NoError(t, err)

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets/IN-3084747", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "VPN broken")
	require.Contains(t, resp.Body.String(), "IN-3084747")
}

func TestTicketByIDHandler_NotFound_Returns404(t *testing.T) {
	server, tokens := newTestServerWithTickets(fakeTicketFinder{byID: map[string]*bardioc.Ticket{}})
	token, err := tokens.Issue("Max Mustermann", "vm-1", "")
	require.NoError(t, err)

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets/IN-9999999", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestTicketByIDHandler_MissingAuth_Returns401(t *testing.T) {
	server, _ := newTestServerWithTickets(fakeTicketFinder{})

	_, api := humatest.New(t)
	server.Register(api)

	resp := api.Get("/api/v1/tickets/IN-3084747", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}
