package api

import (
	"context"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

func (s *Server) handleTickets(ctx context.Context, input *TicketsInput) (*TicketsOutput, error) {
	claims, err := s.verifyBearer(input.Authorization)
	if err != nil {
		return nil, err
	}

	resp := &TicketsOutput{}
	resp.Body.Tickets = []TicketDTO{}

	if claims.ValuemationPersonID == "" {
		return resp, nil
	}

	tickets, err := s.ticketRepo.FindForPerson(ctx, graph.MetadataID(claims.ValuemationPersonID))
	if err != nil {
		log.Error().Err(err).Msg("ticket lookup failed")
		return nil, huma.Error503ServiceUnavailable("ticket lookup failed")
	}

	for _, t := range tickets {
		resp.Body.Tickets = append(resp.Body.Tickets, TicketDTO{
			ID:          t.Id,
			Subject:     t.Subject,
			Description: t.Description,
			Status:      t.Status,
			Priority:    t.Priority,
			CreatedAt:   t.CreatedAt,
			ClosedAt:    t.ClosedAt,
		})
	}
	return resp, nil
}
