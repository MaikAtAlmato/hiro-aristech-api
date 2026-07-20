package api

import (
	"context"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
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
		resp.Body.Tickets = append(resp.Body.Tickets, ticketToDTO(t))
	}
	return resp, nil
}

func (s *Server) handleTicketByID(ctx context.Context, input *TicketByIDInput) (*TicketByIDOutput, error) {
	if _, err := s.verifyBearer(input.Authorization); err != nil {
		return nil, err
	}

	var t *bardioc.Ticket
	var err error
	t, err = s.ticketRepo.FindByIDForPerson(ctx, input.ID, graph.MetadataID(input.PersonID))
	if err != nil {
		log.Error().Err(err).Str("id", input.ID).Msg("ticket lookup by ID failed")
		return nil, huma.Error503ServiceUnavailable("ticket lookup failed")
	}
	if t == nil {
		return nil, huma.Error404NotFound("ticket not found")
	}

	return &TicketByIDOutput{Body: ticketToDTO(*t)}, nil
}

// ticketToDTO maps a Ticket to TicketDTO, preferring CleanDescription over Description.
func ticketToDTO(t bardioc.Ticket) TicketDTO {
	desc := t.CleanDescription
	if desc == "" {
		desc = t.Description
	}
	return TicketDTO{
		ID:           t.Id,
		XID:          t.XID,
		TicketID:     t.TicketId,
		Subject:      t.Subject,
		Description:  desc,
		Status:       t.Status,
		SourceStatus: t.SourceStatus,
		Type:         t.Type,
		Class:        t.Class,
		Urgency:      t.Urgency,
		HLQ1Value:    t.HLQ1Value,
		DateFinished: t.DateFinished,
		KnownError:   t.KnownError,
		Priority:     t.Priority,
		CreatedAt:    t.CreatedAt,
		ClosedAt:     t.ClosedAt,
	}
}
