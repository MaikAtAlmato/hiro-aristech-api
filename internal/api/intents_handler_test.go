// internal/api/intents_handler_test.go
package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

// stubIntentLister satisfies IntentFinder: Get is a no-op stub (no test in
// this file exercises it — that's covered by stubIntentFinder in
// auth_handler_test.go / issues_handler_test.go), List is the one under test.
type stubIntentLister struct {
	byType map[string][]map[string]any
	all    []map[string]any
	err    error
}

func (s stubIntentLister) Get(_ context.Context, _ graph.MetadataID) (*automation.Intent, error) {
	return nil, graph.ErrEntityNotFound
}

func (s stubIntentLister) List(_ context.Context, intentTypes []string) ([]map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(intentTypes) == 0 {
		return s.all, nil
	}
	seen := map[string]bool{}
	var results []map[string]any
	for _, t := range intentTypes {
		for _, item := range s.byType[t] {
			id, _ := item["ogit/_id"].(string)
			if !seen[id] {
				seen[id] = true
				results = append(results, item)
			}
		}
	}
	return results, nil
}

func newIntentsTestServer(intents IntentFinder) (*Server, string) {
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	server := NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, stubAccountFinder{}, &stubIssueStore{}, intents, testAPIKey)
	token, _ := tokens.Issue("Test Caller", "", "")
	return server, token
}

func TestListIntents_MissingBearer_Returns401(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, _ := newIntentsTestServer(stubIntentLister{})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestListIntents_NoFilter_ReturnsAllIntents(t *testing.T) {
	_, humaAPI := humatest.New(t)
	intents := stubIntentLister{all: []map[string]any{
		{"ogit/_id": "intent-1", "ogit/description": "Passwort zurücksetzen"},
		{"ogit/_id": "intent-2", "ogit/description": "Ticket erstellen"},
	}}
	server, token := newIntentsTestServer(intents)
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"ogit/_id":"intent-1"`)
	require.Contains(t, resp.Body.String(), `"ogit/_id":"intent-2"`)
}

func TestListIntents_NoMatches_ReturnsEmptyArrayNotNull(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, token := newIntentsTestServer(stubIntentLister{})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"intents":[]`)
	require.NotContains(t, resp.Body.String(), `"intents":null`)
}

func TestListIntents_TypeFilter_ReturnsOnlyMatchingIntents(t *testing.T) {
	_, humaAPI := humatest.New(t)
	intents := stubIntentLister{byType: map[string][]map[string]any{
		"Mainintents": {{"ogit/_id": "intent-1", "/IntentType": "Mainintents"}},
		"Subintents":  {{"ogit/_id": "intent-2", "/IntentType": "Subintents"}},
	}}
	server, token := newIntentsTestServer(intents)
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents?intentType=Mainintents", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"ogit/_id":"intent-1"`)
	require.NotContains(t, resp.Body.String(), `"ogit/_id":"intent-2"`)
}

func TestListIntents_MultiTypeCommaFilter_ReturnsUnion(t *testing.T) {
	_, humaAPI := humatest.New(t)
	intents := stubIntentLister{byType: map[string][]map[string]any{
		"Mainintents": {{"ogit/_id": "intent-1", "/IntentType": "Mainintents"}},
		"Subintents":  {{"ogit/_id": "intent-2", "/IntentType": "Subintents"}},
	}}
	server, token := newIntentsTestServer(intents)
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents?intentType=Mainintents,Subintents", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"ogit/_id":"intent-1"`)
	require.Contains(t, resp.Body.String(), `"ogit/_id":"intent-2"`)
}

func TestListIntents_RepoError_Returns503(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, token := newIntentsTestServer(stubIntentLister{err: errors.New("graph unavailable")})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/intents", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestParseIntentTypes(t *testing.T) {
	require.Nil(t, parseIntentTypes(""))
	require.Equal(t, []string{"Mainintents"}, parseIntentTypes("Mainintents"))
	require.Equal(t, []string{"Mainintents", "Subintents"}, parseIntentTypes("Mainintents,Subintents"))
	require.Equal(t, []string{"Mainintents", "Subintents"}, parseIntentTypes(" Mainintents , Subintents "))
	require.Equal(t, []string{"MainIntents"}, parseIntentTypes("MainIntents"), "must not lowercase — /IntentType matching is case-sensitive")
	require.Nil(t, parseIntentTypes(" , , "))
}
