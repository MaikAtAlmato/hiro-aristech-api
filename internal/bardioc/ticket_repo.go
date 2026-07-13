package bardioc

import (
	"context"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/gremlin"
	servicemanagement "bitbucket.org/almatoag/graph-go/NTO/ServiceManagement"
	"bitbucket.org/almatoag/graph-go/connx"
)

// TicketRepository finds Ticket nodes connected to a Valuemation Person node.
type TicketRepository struct {
	client EdgeClient
}

// NewTicketRepository creates a new TicketRepository.
func NewTicketRepository(client EdgeClient) *TicketRepository {
	return &TicketRepository{client: client}
}

// FindForPerson returns every Ticket connected to personID via the
// affects, assignedTo (both Ticket -> Person, so incoming from the
// Person's side), or opens (Person -> Ticket, outgoing) edges, deduplicated
// by ticket ID.
func (r *TicketRepository) FindForPerson(ctx context.Context, personID graph.MetadataID) ([]servicemanagement.Ticket, error) {
	ticketIDs := map[graph.MetadataID]bool{}

	for _, verb := range []graph.ConnectionType{connx.Affects, connx.AssignedTo} {
		ids, err := r.incomingTicketIDs(ctx, personID, verb)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			ticketIDs[id] = true
		}
	}

	openedIDs, err := r.outgoingTicketIDs(ctx, personID, connx.Opens)
	if err != nil {
		return nil, err
	}
	for _, id := range openedIDs {
		ticketIDs[id] = true
	}

	tickets := make([]servicemanagement.Ticket, 0, len(ticketIDs))
	for id := range ticketIDs {
		ticket, err := r.getTicket(ctx, id)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, *ticket)
	}
	return tickets, nil
}

func (r *TicketRepository) incomingTicketIDs(ctx context.Context, personID graph.MetadataID, verb graph.ConnectionType) ([]graph.MetadataID, error) {
	builder := gremlin.NewBuilder().InE(string(verb))
	rows, err := r.client.QueryGremlin(ctx, builder, personID, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query incoming %s edges: %w", verb, err)
	}
	defer rows.Close()

	verbs, err := graph.ScanRows[graph.Verb](rows)
	if err != nil {
		return nil, fmt.Errorf("scan incoming %s edges: %w", verb, err)
	}

	ids := make([]graph.MetadataID, 0, len(verbs))
	for _, v := range verbs {
		ids = append(ids, v.OutID)
	}
	return ids, nil
}

func (r *TicketRepository) outgoingTicketIDs(ctx context.Context, personID graph.MetadataID, verb graph.ConnectionType) ([]graph.MetadataID, error) {
	builder := gremlin.NewBuilder().OutE(string(verb))
	rows, err := r.client.QueryGremlin(ctx, builder, personID, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query outgoing %s edges: %w", verb, err)
	}
	defer rows.Close()

	verbs, err := graph.ScanRows[graph.Verb](rows)
	if err != nil {
		return nil, fmt.Errorf("scan outgoing %s edges: %w", verb, err)
	}

	ids := make([]graph.MetadataID, 0, len(verbs))
	for _, v := range verbs {
		ids = append(ids, v.InID)
	}
	return ids, nil
}

func (r *TicketRepository) getTicket(ctx context.Context, id graph.MetadataID) (*servicemanagement.Ticket, error) {
	row := r.client.GetEntity(ctx, id, graph.WithIncludeDeleted(false))
	defer row.Close()

	var ticket servicemanagement.Ticket
	if err := row.Scan(&ticket); err != nil {
		return nil, fmt.Errorf("get ticket %s: %w", id, err)
	}
	return &ticket, nil
}
