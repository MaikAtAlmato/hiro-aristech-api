// internal/api/auth_match_handler_test.go
package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

type stubMsgraphPrefixFinder struct {
	byPrefix map[string][]sgo.Person
	err      error
}

func (s stubMsgraphPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

func (s stubMsgraphPrefixFinder) FindByLastNameSuffix(_ context.Context, _ string, _ string) ([]sgo.Person, error) {
	if s.err != nil {
		return nil, s.err
	}
	return nil, nil
}

func newMatchTestServer(matcher *identity.Matcher) *Server {
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	return NewServer(resolver, matcher, tokens, stubTicketFinder{}, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey)
}

func TestAuthMatchHandler_FullNameMatch_ReturnsHighConfidenceCandidate(t *testing.T) {
	_, api := humatest.New(t)
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	matcher := &identity.Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
		"mai lan": {maik},
	}}}
	newMatchTestServer(matcher).Register(api)

	resp := api.Post("/api/v1/auth/match", map[string]any{
		"firstName": map[string]any{"resultRaw": "Maik"},
		"lastName":  map[string]any{"resultRaw": "Lander"},
	}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"name":"Maik Lander"`)
	// stagePoints=50 (full/full) + representationPoints=30 (1/1, only "raw" has
	// data on both sides here) + uniquenessPoints=20 (one distinct person) = 100
	require.Contains(t, resp.Body.String(), `"confidence":100`)
}

func TestAuthMatchHandler_NoNameData_Returns400(t *testing.T) {
	_, api := humatest.New(t)
	newMatchTestServer(&identity.Matcher{}).Register(api)

	resp := api.Post("/api/v1/auth/match", map[string]any{
		"firstName": map[string]any{},
		"lastName":  map[string]any{},
	}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestAuthMatchHandler_NoCandidatesFound_Returns200WithEmptyList(t *testing.T) {
	_, api := humatest.New(t)
	matcher := &identity.Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{}}}
	newMatchTestServer(matcher).Register(api)

	resp := api.Post("/api/v1/auth/match", map[string]any{
		"firstName": map[string]any{"resultRaw": "Maik"},
		"lastName":  map[string]any{"resultRaw": "Lander"},
	}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"candidates":[]`)
}

func TestAuthMatchHandler_QueryFailure_Returns503(t *testing.T) {
	_, api := humatest.New(t)
	matcher := &identity.Matcher{Msgraph: stubMsgraphPrefixFinder{err: errors.New("graph unavailable")}}
	newMatchTestServer(matcher).Register(api)

	resp := api.Post("/api/v1/auth/match", map[string]any{
		"firstName": map[string]any{"resultRaw": "Maik"},
		"lastName":  map[string]any{"resultRaw": "Lander"},
	}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestAuthMatchHandler_LastNameHint_ThreadsIntoMatchQuery(t *testing.T) {
	_, api := humatest.New(t)
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-maik")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	max := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-max")}},
		FirstName: "Max",
		LastName:  "Lehmann",
	}
	matcher := &identity.Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
		"m l": {maik, max},
	}}}
	newMatchTestServer(matcher).Register(api)

	resp := api.Post("/api/v1/auth/match", map[string]any{
		"firstName":    map[string]any{"resultRaw": "M"},
		"lastName":     map[string]any{"resultRaw": "L"},
		"lastNameHint": "Land",
	}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	// "Maik Lander" must be the first (index 0) candidate — proving the
	// lastNameHint field bound correctly and boosted it above "Max Lehmann",
	// which would otherwise narrowly lead (see
	// TestMatcher_Match_LastNameHint_ReordersAmbiguousCandidates in
	// internal/identity for the underlying arithmetic).
	require.Contains(t, resp.Body.String(), `"candidates":[{"name":"Maik Lander"`)
}
