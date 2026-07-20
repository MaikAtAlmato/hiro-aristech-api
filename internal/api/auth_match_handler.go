// internal/api/auth_match_handler.go
package api

import (
	"context"
	"errors"

	"bitbucket.org/almatoag/hiro-aristech-api/internal/identity"
	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

func (s *Server) handleAuthMatch(ctx context.Context, input *NameMatchInput) (*NameMatchOutput, error) {
	log.Info().
		Interface("firstName", input.Body.FirstName).
		Interface("lastName", input.Body.LastName).
		Str("domain", input.Body.Domain).
		Str("tickerMessageId", input.Body.TickerMessageID).
		Str("lastNameHint", input.Body.LastNameHint).
		Msg("auth/match request received")

	query := identity.NameMatchQuery{
		FirstName:    toSTTResult(input.Body.FirstName),
		LastName:     toSTTResult(input.Body.LastName),
		Domains:      parseDomains(input.Body.Domain),
		LastNameHint: input.Body.LastNameHint,
	}

	candidates, err := s.nameMatcher.Match(ctx, query)
	switch {
	case errors.Is(err, identity.ErrNoNameData):
		return nil, huma.Error400BadRequest("first or last name recognition result is required")
	case err != nil:
		log.Error().Err(err).Msg("name match lookup failed")
		return nil, huma.Error503ServiceUnavailable("name match lookup failed")
	}

	resp := &NameMatchOutput{}
	resp.Body.Candidates = make([]NameMatchCandidateDTO, 0, len(candidates))
	for _, c := range candidates {
		resp.Body.Candidates = append(resp.Body.Candidates, NameMatchCandidateDTO{
			Name:                  c.Name,
			FirstName:             c.FirstName,
			LastName:              c.LastName,
			FirstNamePhonetic:     c.FirstNamePhonetic,
			LastNamePhonetic:      c.LastNamePhonetic,
			ValuemationExternalID: c.ValuemationPersonXID,
			MsgraphExternalID:     c.MsgraphPersonXID,
			Confidence:            c.Confidence,
			StagePoints:           c.StagePoints,
			RepresentationPoints:  c.RepresentationPoints,
			UniquenessPoints:      c.UniquenessPoints,
		})
	}
	return resp, nil
}

func toSTTResult(c NameCandidate) identity.STTResult {
	return identity.STTResult{
		ResultRaw:        c.ResultRaw,
		ResultTagged:     c.ResultTagged,
		ResultSlotted:    c.ResultSlotted,
		ResultNlp:        c.ResultNlp,
		ResultStructured: c.ResultStructured,
	}
}
