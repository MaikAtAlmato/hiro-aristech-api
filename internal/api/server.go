package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/auth"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2"
)

const bearerPrefix = "Bearer "

// TicketFinder finds tickets connected to a resolved Valuemation Person node,
// or by ticket/graph ID.
type TicketFinder interface {
	FindForPerson(ctx context.Context, personID graph.MetadataID) ([]bardioc.Ticket, error)
	FindByID(ctx context.Context, id string) (*bardioc.Ticket, error)
	FindByIDForPerson(ctx context.Context, id string, personID graph.MetadataID) (*bardioc.Ticket, error)
}

// AccountFinder finds the Account hiro-conn-msgraph connects to a resolved
// MSGraph Person node.
type AccountFinder interface {
	FindForPerson(ctx context.Context, personID graph.MetadataID) (*bardioc.MsgraphAccount, error)
}

// IssueStore creates and monitors AutomationIssue nodes.
type IssueStore interface {
	Create(ctx context.Context, attributes map[string]string) (graph.MetadataID, error)
	Variables(ctx context.Context, id graph.MetadataID) (map[string]any, error)
}

// IntentFinder finds and lists Intent nodes.
type IntentFinder interface {
	Get(ctx context.Context, id graph.MetadataID) (*automation.Intent, error)
	List(ctx context.Context, intentTypes []string) ([]map[string]any, error)
}

// Server holds the dependencies for all HTTP endpoints.
type Server struct {
	resolver    *identity.Resolver
	nameMatcher *identity.Matcher
	tokens      *auth.TokenService
	ticketRepo  TicketFinder
	accountRepo AccountFinder
	issueRepo   IssueStore
	intentRepo  IntentFinder
	apiKey      string
}

// NewServer creates a new Server.
func NewServer(resolver *identity.Resolver, nameMatcher *identity.Matcher, tokens *auth.TokenService, ticketRepo TicketFinder, accountRepo AccountFinder, issueRepo IssueStore, intentRepo IntentFinder, apiKey string) *Server {
	return &Server{
		resolver:    resolver,
		nameMatcher: nameMatcher,
		tokens:      tokens,
		ticketRepo:  ticketRepo,
		accountRepo: accountRepo,
		issueRepo:   issueRepo,
		intentRepo:  intentRepo,
		apiKey:      apiKey,
	}
}

// verifyBearer extracts and validates the JWT from an "Authorization:
// Bearer <token>" header, shared by every endpoint that requires an
// authenticated caller.
func (s *Server) verifyBearer(authorization string) (*auth.Claims, error) {
	if !strings.HasPrefix(authorization, bearerPrefix) {
		return nil, huma.Error401Unauthorized("missing bearer token")
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix))
	if tokenString == "" {
		return nil, huma.Error401Unauthorized("missing bearer token")
	}
	claims, err := s.tokens.Verify(tokenString)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid or expired token")
	}
	return claims, nil
}

// Register registers the shared API-key middleware and all operations on
// the given huma API.
func (s *Server) Register(api huma.API) {
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		if subtle.ConstantTimeCompare([]byte(ctx.Header("X-Api-Key")), []byte(s.apiKey)) != 1 {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or missing API key")
			return
		}
		next(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "authenticate-caller",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth",
		Summary:     "Authenticate a caller by phone number or name",
		Tags:        []string{"auth"},
	}, s.handleAuth)

	huma.Register(api, huma.Operation{
		OperationID: "get-caller-tickets",
		Method:      http.MethodGet,
		Path:        "/api/v1/tickets",
		Summary:     "Get the authenticated caller's support tickets",
		Tags:        []string{"tickets"},
	}, s.handleTickets)

	huma.Register(api, huma.Operation{
		OperationID: "create-automation-issue",
		Method:      http.MethodPost,
		Path:        "/api/v1/issues",
		Summary:     "Create an AutomationIssue to trigger the Reasoning Engine",
		Tags:        []string{"issues"},
	}, s.handleCreateIssue)

	huma.Register(api, huma.Operation{
		OperationID: "get-automation-issue-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/issues/{issueId}/status",
		Summary:     "Get the current status of a previously created AutomationIssue",
		Tags:        []string{"issues"},
	}, s.handleIssueStatus)

	huma.Register(api, huma.Operation{
		OperationID: "match-caller-name",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/match",
		Summary:     "Score candidate callers for a possibly-incomplete STT name recognition result",
		Tags:        []string{"auth"},
	}, s.handleAuthMatch)

	huma.Register(api, huma.Operation{
		OperationID: "get-ticket-by-id",
		Method:      http.MethodGet,
		Path:        "/api/v1/tickets/{id}",
		Summary:     "Get a specific ticket by Valuemation ticket ID (e.g. IN-3084747) or graph node ID",
		Tags:        []string{"tickets"},
	}, s.handleTicketByID)

	huma.Register(api, huma.Operation{
		OperationID: "list-intents",
		Method:      http.MethodGet,
		Path:        "/api/v1/intents",
		Summary:     "List Intent nodes, optionally filtered by /IntentType",
		Tags:        []string{"intents"},
	}, s.handleListIntents)
}
