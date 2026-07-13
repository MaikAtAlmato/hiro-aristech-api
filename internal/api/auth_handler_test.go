package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
	servicemanagement "bitbucket.org/almatoag/graph-go/NTO/ServiceManagement"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

type stubMsgraphFinder struct {
	byPhone map[string][]sgo.Person
	byName  map[string][]sgo.Person
}

func (s stubMsgraphFinder) FindByPhone(_ context.Context, phone string) ([]sgo.Person, error) {
	return s.byPhone[phone], nil
}

func (s stubMsgraphFinder) FindByName(_ context.Context, firstName, lastName string) ([]sgo.Person, error) {
	return s.byName[firstName+" "+lastName], nil
}

type stubValuemationFinder struct {
	byPhone map[string][]bardioc.ValuemationPerson
	byName  map[string][]bardioc.ValuemationPerson
}

func (s stubValuemationFinder) FindByPhone(_ context.Context, phone string) ([]bardioc.ValuemationPerson, error) {
	return s.byPhone[phone], nil
}

func (s stubValuemationFinder) FindByName(_ context.Context, firstName, lastName string) ([]bardioc.ValuemationPerson, error) {
	return s.byName[firstName+" "+lastName], nil
}

type stubTicketFinder struct{}

func (stubTicketFinder) FindForPerson(_ context.Context, _ graph.MetadataID) ([]servicemanagement.Ticket, error) {
	return nil, nil
}

type stubAccountFinder struct {
	byPersonID map[graph.MetadataID]*bardioc.MsgraphAccount
}

func (s stubAccountFinder) FindForPerson(_ context.Context, personID graph.MetadataID) (*bardioc.MsgraphAccount, error) {
	return s.byPersonID[personID], nil
}

type stubIssueStore struct {
	createdID          graph.MetadataID
	createErr          error
	status             string
	statusErr          error
	capturedAttributes map[string]string
}

func (s *stubIssueStore) Create(_ context.Context, attributes map[string]string) (graph.MetadataID, error) {
	s.capturedAttributes = attributes
	return s.createdID, s.createErr
}

func (s stubIssueStore) Status(_ context.Context, _ graph.MetadataID) (string, error) {
	return s.status, s.statusErr
}

type stubIntentFinder struct {
	byID map[graph.MetadataID]*automation.Intent
}

func (s stubIntentFinder) Get(_ context.Context, id graph.MetadataID) (*automation.Intent, error) {
	intent, ok := s.byID[id]
	if !ok {
		return nil, graph.ErrEntityNotFound
	}
	return intent, nil
}

func (s stubIntentFinder) List(_ context.Context, _ []string) ([]map[string]any, error) {
	return nil, nil
}

func newTestServer() *Server {
	vm := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-1")}},
			FirstName: "Max",
			LastName:  "Mustermann",
		},
		PhoneNo: "+491234567890",
	}
	resolver := &identity.Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491234567890": {vm}}},
	}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	return NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey)
}

func TestAuthHandler_UniquePhoneMatch_ReturnsToken(t *testing.T) {
	_, api := humatest.New(t)
	newTestServer().Register(api)

	resp := api.Get("/api/v1/auth?phone=%2B491234567890", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"token"`)
	require.Contains(t, resp.Body.String(), `"name":"Max Mustermann"`)
	require.Contains(t, resp.Body.String(), `"valuemationExternalId":"vm-1"`)
	require.NotContains(t, resp.Body.String(), `"msgraphExternalId"`)
}

func TestAuthHandler_MsgraphPersonWithConnectedAccount_IncludesAccountInfo(t *testing.T) {
	_, humaAPI := humatest.New(t)

	ms := sgo.Person{
		Entity:      graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName:   "Erika",
		LastName:    "Musterfrau",
		OfficePhone: "+491111111111",
	}
	account := &bardioc.MsgraphAccount{
		Account: sgo.Account{
			Entity: graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("acct-1")}, XID: "acct-xid-1"},
		},
		Status: "active",
	}
	resolver := &identity.Resolver{
		Msgraph:     stubMsgraphFinder{byPhone: map[string][]sgo.Person{"+491111111111": {ms}}},
		Valuemation: stubValuemationFinder{},
	}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	accountRepo := stubAccountFinder{byPersonID: map[graph.MetadataID]*bardioc.MsgraphAccount{"ms-1": account}}
	server := NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, accountRepo, &stubIssueStore{}, stubIntentFinder{}, testAPIKey)
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/auth?phone=%2B491111111111", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"msgraphAccountExternalId":"acct-xid-1"`)
	require.Contains(t, resp.Body.String(), `"msgraphAccountStatus":"active"`)
}

func TestAuthHandler_NoMatch_Returns401(t *testing.T) {
	_, api := humatest.New(t)
	newTestServer().Register(api)

	resp := api.Get("/api/v1/auth?phone=%2B490000000000", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestAuthHandler_MissingParams_Returns400(t *testing.T) {
	_, api := humatest.New(t)
	newTestServer().Register(api)

	resp := api.Get("/api/v1/auth", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestAuthHandler_BothParams_Returns400(t *testing.T) {
	_, api := humatest.New(t)
	newTestServer().Register(api)

	resp := api.Get("/api/v1/auth?phone=%2B491234567890&name=Max%20Mustermann", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestParseDomains(t *testing.T) {
	require.Nil(t, parseDomains(""))
	require.Equal(t, []string{"almato.com"}, parseDomains("almato.com"))
	require.Equal(t, []string{"almato.com", "datagroup.de"}, parseDomains("almato.com,datagroup.de"))
	require.Equal(t, []string{"almato.com", "datagroup.de"}, parseDomains(" Almato.com , DataGroup.de "))
	require.Nil(t, parseDomains(" , , "))
}

func TestAuthHandler_DomainFilterResolvesAmbiguousName(t *testing.T) {
	_, api := humatest.New(t)

	schleich := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-1")}},
			FirstName: "Paul",
			LastName:  "Hoppe",
			Email:     "paul.hoppe@schleichfigurines.onmicrosoft.com",
		},
		PhoneNo: "0",
	}
	almato := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-2")}},
			FirstName: "Paul",
			LastName:  "Hoppe",
			Email:     "Paul.Hoppe@almato.com",
		},
		PhoneNo: "+4971211478772",
	}
	resolver := &identity.Resolver{
		Msgraph: stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{
			"Paul Hoppe": {schleich, almato},
		}},
	}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	server := NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey)
	server.Register(api)

	ambiguous := api.Get("/api/v1/auth?name=Paul%20Hoppe", "X-Api-Key: "+testAPIKey)
	require.Equal(t, http.StatusConflict, ambiguous.Code)

	resolved := api.Get("/api/v1/auth?name=Paul%20Hoppe&domain=almato.com", "X-Api-Key: "+testAPIKey)
	require.Equal(t, http.StatusOK, resolved.Code)
	require.Contains(t, resolved.Body.String(), `"valuemationExternalId":"vm-2"`)
}
