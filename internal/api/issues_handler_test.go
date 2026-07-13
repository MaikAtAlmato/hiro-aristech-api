// internal/api/issues_handler_test.go
package api

import (
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

func newIssuesTestServer(issues *stubIssueStore, intents stubIntentFinder) (*Server, string) {
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	token, err := tokens.Issue("Max Mustermann", "vm-1", "")
	if err != nil {
		panic(err)
	}
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	server := NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, stubAccountFinder{}, issues, intents, testAPIKey)
	return server, token
}

func TestCreateIssue_DirectPayload_CreatesIssueWithGivenAttributes(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-1")}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{
			"subject":   "Drucker kaputt",
			"variables": map[string]string{"/userMail": "anrufer@example.com"},
		},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"issueId":"issue-1"`)
}

func TestCreateIssue_IntentID_MergesSystemVariablesBeforeUserVariables(t *testing.T) {
	_, humaAPI := humatest.New(t)
	intent := automation.Intent{
		SystemVariableNames:  "/dont_process_ticket, /ParloaIntent",
		SystemVariableValues: "true, CreateTicket",
	}
	intents := stubIntentFinder{byID: map[graph.MetadataID]*automation.Intent{"intent-1": &intent}}
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-2")}
	server, token := newIssuesTestServer(issues, intents)
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{
			"intentId":  "intent-1",
			"variables": map[string]string{"/userMail": "anrufer@example.com"},
		},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"issueId":"issue-2"`)
}

func TestCreateIssue_UnknownIntent_Returns404(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, token := newIssuesTestServer(&stubIssueStore{}, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{"intentId": "does-not-exist"},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestCreateIssue_MissingBearer_Returns401(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, _ := newIssuesTestServer(&stubIssueStore{}, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues", map[string]any{"subject": "x"}, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestIssueStatus_ReturnsCurrentStatus(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{status: "PROCESSING"}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/issues/issue-1/status", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"status":"PROCESSING"`)
}

func TestIssueStatus_NotFound_Returns404(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{statusErr: graph.ErrEntityNotFound}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/issues/issue-1/status", "Authorization: Bearer "+token, "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestIssueStatus_MissingBearer_Returns401(t *testing.T) {
	_, humaAPI := humatest.New(t)
	server, _ := newIssuesTestServer(&stubIssueStore{}, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Get("/api/v1/issues/issue-1/status", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestCreateIssue_OriginNode_SetsOriginNodeAttribute(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-3")}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{
			"subject":    "Test MFA",
			"originNode": "clofxb4tl0b1s01927tzfmzik_clolxbnjze5si0150m3yjtmtq",
		},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t,
		"clofxb4tl0b1s01927tzfmzik_clolxbnjze5si0150m3yjtmtq",
		issues.capturedAttributes["ogit/Automation/originNode"],
	)
}

func TestCreateIssue_Scope_SetsScopeAttribute(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-4")}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{
			"subject": "Test MFA",
			"scope":   "cln8sd29t04zz0186ataggjya_clofxb4tl0b1s01927tzfmzik",
		},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t,
		"cln8sd29t04zz0186ataggjya_clofxb4tl0b1s01927tzfmzik",
		issues.capturedAttributes["ogit/_scope"],
	)
}

func TestCreateIssue_NoOriginNodeOrScope_AttributesAbsent(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-5")}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{"subject": "Drucker kaputt"},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	_, hasOriginNode := issues.capturedAttributes["ogit/Automation/originNode"]
	_, hasScope := issues.capturedAttributes["ogit/_scope"]
	require.False(t, hasOriginNode)
	require.False(t, hasScope)
}

func TestCreateIssue_ScopeField_OverridesSameKeyInVariables(t *testing.T) {
	_, humaAPI := humatest.New(t)
	issues := &stubIssueStore{createdID: graph.MetadataID("issue-6")}
	server, token := newIssuesTestServer(issues, stubIntentFinder{})
	server.Register(humaAPI)

	resp := humaAPI.Post("/api/v1/issues",
		map[string]any{
			"subject": "Test MFA",
			"scope":   "new-scope",
			"variables": map[string]string{
				"ogit/_scope": "old-scope-from-variables",
			},
		},
		"Authorization: Bearer "+token,
		"X-Api-Key: "+testAPIKey,
	)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "new-scope", issues.capturedAttributes["ogit/_scope"])
}
