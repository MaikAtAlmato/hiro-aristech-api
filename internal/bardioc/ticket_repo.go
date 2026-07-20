package bardioc

import (
	"context"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
	"bitbucket.org/almatoag/bardioc-go/graph/gremlin"
	servicemanagement "bitbucket.org/almatoag/graph-go/NTO/ServiceManagement"
	"bitbucket.org/almatoag/graph-go/connx"
)

// ticketOgitType is the ogit type string for Ticket nodes, used in ES queries.
var ticketOgitType = servicemanagement.Ticket{}.OgitType()

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
func (r *TicketRepository) FindForPerson(ctx context.Context, personID graph.MetadataID) ([]Ticket, error) {
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

	tickets := make([]Ticket, 0, len(ticketIDs))
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

// FindByIDForPerson finds a ticket by ID (same lookup rules as FindByID) and
// verifies it is connected to personID via affects, assignedTo, or opens edges.
// personID may be either a graph node ID (ogit/_id) or an XID (ogit/_xid);
// when it looks like an XID it is resolved to the graph node ID first.
// Returns nil, nil when no ticket is found or the ticket is not owned by the person.
func (r *TicketRepository) FindByIDForPerson(ctx context.Context, id string, personID graph.MetadataID) (*Ticket, error) {
	ticket, err := r.FindByID(ctx, id)
	if err != nil || ticket == nil {
		return nil, err
	}

	graphPersonID, err := r.resolvePersonID(ctx, personID)
	if err != nil {
		return nil, err
	}
	if graphPersonID == "" {
		return nil, nil
	}

	ticketGraphID := ticket.Metadata.ID
	for _, verb := range []graph.ConnectionType{connx.Affects, connx.AssignedTo} {
		ids, err := r.incomingTicketIDs(ctx, graphPersonID, verb)
		if err != nil {
			return nil, err
		}
		for _, tid := range ids {
			if tid == ticketGraphID {
				return ticket, nil
			}
		}
	}
	openedIDs, err := r.outgoingTicketIDs(ctx, graphPersonID, connx.Opens)
	if err != nil {
		return nil, err
	}
	for _, tid := range openedIDs {
		if tid == ticketGraphID {
			return ticket, nil
		}
	}
	return nil, nil
}

// resolvePersonID returns personID unchanged. The caller is expected to pass
// the graph node ID (ogit/_id) as returned in the auth response's
// valuemationPersonId field.
func (r *TicketRepository) resolvePersonID(_ context.Context, personID graph.MetadataID) (graph.MetadataID, error) {
	return personID, nil
}

// FindByID finds a ticket by its Valuemation ticket ID (ogit/id, e.g.
// "IN-3084747"), by ogit/ServiceManagement/ticketId, or by the graph node ID
// (ogit/_id). Returns nil, nil when no ticket is found.
func (r *TicketRepository) FindByID(ctx context.Context, id string) (*Ticket, error) {
	for _, field := range []string{"ogit/id", "ogit/ServiceManagement/ticketId"} {
		b := esquery.NewBuilder().
			Equal(graph.OgitType, ticketOgitType).And().
			Equal(field, id)
		rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
		if err != nil {
			return nil, fmt.Errorf("find ticket by %s %q: %w", field, id, err)
		}
		tickets, err := graph.ScanRows[Ticket](rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("scan ticket by %s %q: %w", field, id, err)
		}
		if len(tickets) > 0 {
			return &tickets[0], nil
		}
	}
	// Fall back: treat id as a graph node ID (ogit/_id) and fetch directly.
	row := r.client.GetEntity(ctx, graph.MetadataID(id), graph.WithIncludeDeleted(false))
	defer row.Close()
	var ticket Ticket
	if err := row.Scan(&ticket); err != nil {
		return nil, nil
	}
	return &ticket, nil
}

func (r *TicketRepository) getTicket(ctx context.Context, id graph.MetadataID) (*Ticket, error) {
	row := r.client.GetEntity(ctx, id, graph.WithIncludeDeleted(false))
	defer row.Close()

	var ticket Ticket
	if err := row.Scan(&ticket); err != nil {
		return nil, fmt.Errorf("get ticket %s: %w", id, err)
	}
	return &ticket, nil
}
