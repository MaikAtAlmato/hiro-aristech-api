package api

import (
	"context"
	"errors"
	"strings"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

func (s *Server) handleAuth(ctx context.Context, input *AuthInput) (*AuthOutput, error) {
	phone := strings.TrimSpace(input.Phone)
	name := strings.TrimSpace(input.Name)

	log.Info().Str("phone", phone).Str("name", name).Str("domain", input.Domain).Msg("auth request received")

	switch {
	case phone == "" && name == "":
		return nil, huma.Error400BadRequest("either phone or name is required")
	case phone != "" && name != "":
		return nil, huma.Error400BadRequest("provide either phone or name, not both")
	}

	id, err := s.resolver.Resolve(ctx, identity.Criterion{
		Phone:   phone,
		Name:    name,
		Domains: parseDomains(input.Domain),
	})
	switch {
	case errors.Is(err, identity.ErrNotFound):
		return nil, huma.Error401Unauthorized("no matching caller found")
	case errors.Is(err, identity.ErrAmbiguous):
		return nil, huma.Error409Conflict("multiple matching callers found")
	case err != nil:
		log.Error().Err(err).Msg("identity lookup failed")
		return nil, huma.Error503ServiceUnavailable("identity lookup failed")
	}

	var valuemationXID, msgraphXID string
	if id.ValuemationPersonID != nil {
		valuemationXID = id.ValuemationPersonID.String()
	}
	if id.MsgraphPersonID != nil {
		msgraphXID = id.MsgraphPersonID.String()
	}

	token, err := s.tokens.Issue(id.DisplayName, valuemationXID, msgraphXID)
	if err != nil {
		log.Error().Err(err).Msg("failed to issue token")
		return nil, huma.Error500InternalServerError("failed to issue token")
	}

	resp := &AuthOutput{}
	resp.Body.Token = token
	resp.Body.ExpiresIn = int(s.tokens.TTL().Seconds())
	resp.Body.Name = id.DisplayName
	resp.Body.ValuemationExternalID = id.ValuemationPersonXID
	resp.Body.MsgraphExternalID = id.MsgraphPersonXID

	if id.MsgraphPersonID != nil {
		account, err := s.accountRepo.FindForPerson(ctx, *id.MsgraphPersonID)
		if err != nil {
			log.Error().Err(err).Msg("account lookup failed")
		} else if account != nil {
			resp.Body.MsgraphAccountExternalID = externalID(account.XID, account.Metadata.ID)
			resp.Body.MsgraphAccountStatus = account.Status
		}
	}

	return resp, nil
}

// parseDomains splits a comma-separated domain list into a normalized slice:
// lowercase, whitespace-trimmed, empty entries dropped. Returns nil (no
// filter) when raw has no non-empty entries.
func parseDomains(raw string) []string {
	var domains []string
	for _, d := range strings.Split(raw, ",") {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// externalID returns xid if set, otherwise id's string form. Used because
// not every synced node has an external XID populated.
func externalID(xid string, id graph.MetadataID) string {
	if xid != "" {
		return xid
	}
	return id.String()
}
