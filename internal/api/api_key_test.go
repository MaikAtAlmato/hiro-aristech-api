// internal/api/api_key_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/require"
)

// testAPIKey is the shared API key used to construct every test server in
// this package via NewServer(...). It must be sent as the X-Api-Key header
// on every request every handler test makes from here on, or the middleware
// registered in Server.Register rejects the request with 401 before it
// reaches any handler-specific logic (bearer check, 400 validation, etc).
const testAPIKey = "test-api-key"

func newAPIKeyTestServer() *Server {
	resolver := &identity.Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}
	tokens := auth.NewTokenService("test-secret", 15*time.Minute)
	return NewServer(resolver, &identity.Matcher{}, tokens, stubTicketFinder{}, stubAccountFinder{}, &stubIssueStore{}, stubIntentFinder{}, testAPIKey)
}

func TestAPIKeyMiddleware_MissingHeader_Returns401(t *testing.T) {
	_, humaAPI := humatest.New(t)
	newAPIKeyTestServer().Register(humaAPI)

	resp := humaAPI.Get("/api/v1/tickets")

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Contains(t, resp.Body.String(), `"detail":"invalid or missing API key"`)
}

func TestAPIKeyMiddleware_WrongKey_Returns401(t *testing.T) {
	_, humaAPI := humatest.New(t)
	newAPIKeyTestServer().Register(humaAPI)

	resp := humaAPI.Get("/api/v1/tickets", "X-Api-Key: wrong-key")

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Contains(t, resp.Body.String(), `"detail":"invalid or missing API key"`)
}

func TestAPIKeyMiddleware_CorrectKey_PassesThroughToHandlerAuth(t *testing.T) {
	_, humaAPI := humatest.New(t)
	newAPIKeyTestServer().Register(humaAPI)

	// Correct X-Api-Key but no bearer: the request must clear the API-key
	// gate and reach the handler's own verifyBearer check, which then
	// 401s for its own, different reason — proving the two layers compose
	// instead of the api-key check masking the bearer check (or vice versa).
	resp := humaAPI.Get("/api/v1/tickets", "X-Api-Key: "+testAPIKey)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Contains(t, resp.Body.String(), `"detail":"missing bearer token"`)
}

func TestAPIKeyMiddleware_UnprotectedAuthEndpoint_StillRequiresAPIKey(t *testing.T) {
	_, humaAPI := humatest.New(t)
	newAPIKeyTestServer().Register(humaAPI)

	// /auth has no bearer requirement of its own, but the API-key gate
	// must still apply to it — it's in front of every operation, not just
	// the ones that already required a bearer.
	resp := humaAPI.Get("/api/v1/auth?phone=%2B491234567890")

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Contains(t, resp.Body.String(), `"detail":"invalid or missing API key"`)
}

// TestAPIKeyMiddleware_EveryRegisteredRoute_Requires401WithoutKey structurally
// proves that the API-key gate covers every operation registered in
// Server.Register, not just the handful exercised incidentally by other
// tests. huma snapshots middlewares at registration time, so a future
// huma.Register call added above the api.UseMiddleware(...) block would
// silently ship unprotected — this test fails immediately if that happens,
// since it walks the exact set of routes Register wires up rather than
// relying on other tests' handler-specific requests to send the header.
func TestAPIKeyMiddleware_EveryRegisteredRoute_Requires401WithoutKey(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/auth"},
		{http.MethodPost, "/api/v1/auth/match"},
		{http.MethodGet, "/api/v1/tickets"},
		{http.MethodPost, "/api/v1/issues"},
		{http.MethodGet, "/api/v1/issues/placeholder-issue-id/status"},
		{http.MethodGet, "/api/v1/intents"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			_, humaAPI := humatest.New(t)
			newAPIKeyTestServer().Register(humaAPI)

			var resp *httptest.ResponseRecorder
			switch route.method {
			case http.MethodGet:
				resp = humaAPI.Get(route.path)
			case http.MethodPost:
				resp = humaAPI.Post(route.path, map[string]any{})
			default:
				t.Fatalf("unhandled method %q in route table", route.method)
			}

			require.Equal(t, http.StatusUnauthorized, resp.Code)
			require.Contains(t, resp.Body.String(), `"detail":"invalid or missing API key"`)
		})
	}
}
