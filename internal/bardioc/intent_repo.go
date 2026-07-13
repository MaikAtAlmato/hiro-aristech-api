// internal/bardioc/intent_repo.go
package bardioc

import (
	"context"
	"fmt"
	"strings"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
)

// intentOgitType is the graph vertex type of every Intent node.
const intentOgitType = "ogit/Automation/Intent"

// IntentRepository finds Intent nodes by graph ID and lists them.
type IntentRepository struct {
	client EdgeClient
}

// NewIntentRepository creates a new IntentRepository.
func NewIntentRepository(client EdgeClient) *IntentRepository {
	return &IntentRepository{client: client}
}

// Get returns the Intent with the given graph ID.
func (r *IntentRepository) Get(ctx context.Context, id graph.MetadataID) (*automation.Intent, error) {
	row := r.client.GetEntity(ctx, id, graph.WithIncludeDeleted(false))
	defer row.Close()

	var intent automation.Intent
	if err := row.Scan(&intent); err != nil {
		return nil, fmt.Errorf("get intent %s: %w", id, err)
	}
	return &intent, nil
}

// List returns every Intent node, optionally restricted to nodes whose
// /IntentType exactly matches one of the given values. Intent nodes carry
// dynamic /-prefixed fields (IntentType, output, subject, ...) that aren't
// part of the fixed automation.Intent ontology struct, so results are
// returned as raw maps rather than a typed struct — this keeps the endpoint
// forward-compatible with new fields added to Intent nodes later.
//
// With no filter, a single unfiltered query runs. With one or more types,
// one query per type runs and results are deduplicated by ogit/_id — the
// same per-value-query-then-dedup approach as
// MsgraphPersonRepository.FindByPhone, avoiding any dependency on esquery's
// OR/grouping semantics.
//
// graph.WithLimit(-1) is required on every query: QueryVertices defaults to
// a server-side limit of 20 results otherwise, which would silently
// truncate the intent catalog (18 entries today) once it grows past that.
func (r *IntentRepository) List(ctx context.Context, intentTypes []string) ([]map[string]any, error) {
	filters := intentTypes
	if len(filters) == 0 {
		filters = []string{""}
	}

	seen := map[string]bool{}
	var results []map[string]any
	for _, t := range filters {
		b := esquery.NewBuilder().Equal(graph.OgitType, intentOgitType)
		if t != "" {
			b = b.And().Equal("/IntentType", t)
		}

		rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false), graph.WithLimit(-1))
		if err != nil {
			return nil, fmt.Errorf("query intents: %w", err)
		}

		items, err := graph.ScanRows[map[string]any](rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("scan intents: %w", err)
		}

		for _, item := range items {
			id, _ := item[graph.OgitID].(string)
			if !seen[id] {
				seen[id] = true
				results = append(results, item)
			}
		}
	}
	return results, nil
}

// SystemVariables pairs intent's SystemVariableNames with
// SystemVariableValues positionally (the n-th name maps to the n-th
// value) — these are the Intent's fixed automation attributes, unrelated
// to whatever the caller says. Intent nodes have no graph edges; this
// comma-separated pair of attributes is the only place the mapping lives.
func SystemVariables(intent automation.Intent) map[string]string {
	var names []string
	for _, n := range strings.Split(intent.SystemVariableNames, ",") {
		if t := strings.TrimSpace(n); t != "" {
			names = append(names, t)
		}
	}

	var values []string
	for _, v := range strings.Split(intent.SystemVariableValues, ",") {
		values = append(values, strings.TrimSpace(v))
	}

	vars := make(map[string]string, len(names))
	for i, name := range names {
		if i < len(values) {
			vars[name] = values[i]
		} else {
			vars[name] = ""
		}
	}
	return vars
}
