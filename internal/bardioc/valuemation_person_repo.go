// internal/bardioc/valuemation_person_repo.go
package bardioc

import (
	"context"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
)

// ValuemationPersonRepository finds Person nodes written by hiro-conn-valuemation.
type ValuemationPersonRepository struct {
	client EdgeClient
}

// NewValuemationPersonRepository creates a new ValuemationPersonRepository.
func NewValuemationPersonRepository(client EdgeClient) *ValuemationPersonRepository {
	return &ValuemationPersonRepository{client: client}
}

// valuemationPhoneFields lists the fields that may hold a caller's phone
// number, in the shape synced by hiro-conn-valuemation.
var valuemationPhoneFields = []string{"/phoneNo", "ogit/officePhone"}

// FindByPhone returns every Valuemation Person whose /phoneNo or
// ogit/officePhone field equals phone. Each field is queried separately and
// results are deduplicated by node ID, avoiding any dependency on esquery's
// OR/AND grouping semantics.
func (r *ValuemationPersonRepository) FindByPhone(ctx context.Context, phone string) ([]ValuemationPerson, error) {
	seen := map[string]bool{}
	var results []ValuemationPerson

	for _, field := range valuemationPhoneFields {
		b := esquery.NewBuilder().
			Equal(graph.OgitType, "ogit/Person").And().
			Equal("/pFlag", PFlagValuemation).And().
			Equal(field, phone)

		rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
		if err != nil {
			return nil, fmt.Errorf("query valuemation person by %s: %w", field, err)
		}

		persons, err := graph.ScanRows[ValuemationPerson](rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("scan valuemation person by %s: %w", field, err)
		}

		for _, p := range persons {
			id := p.Metadata.ID.String()
			if !seen[id] {
				seen[id] = true
				results = append(results, p)
			}
		}
	}

	return results, nil
}

// FindByName returns every Valuemation Person with an exact, case-insensitive
// match on first and last name as stored in the graph.
func (r *ValuemationPersonRepository) FindByName(ctx context.Context, firstName, lastName string) ([]ValuemationPerson, error) {
	b := esquery.NewBuilder().
		Equal(graph.OgitType, "ogit/Person").And().
		Equal("/pFlag", PFlagValuemation).And().
		Equal("ogit/firstName", firstName, esquery.WithIgnoreCase()).And().
		Equal("ogit/lastName", lastName, esquery.WithIgnoreCase())

	rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query valuemation person by name: %w", err)
	}
	defer rows.Close()

	persons, err := graph.ScanRows[ValuemationPerson](rows)
	if err != nil {
		return nil, fmt.Errorf("scan valuemation person by name: %w", err)
	}
	return persons, nil
}

// FindByNamePrefix returns every Valuemation Person whose first and last
// name each start with the given prefix (case-insensitive). A prefix equal
// to the full name matches only that name (modulo casing), so this one
// method serves both full-name and initial-only lookups.
func (r *ValuemationPersonRepository) FindByNamePrefix(ctx context.Context, firstNamePrefix, lastNamePrefix string) ([]ValuemationPerson, error) {
	b := esquery.NewBuilder().
		Equal(graph.OgitType, "ogit/Person").And().
		Equal("/pFlag", PFlagValuemation).And().
		Equal("ogit/firstName", esquery.NewRegex(namePrefixPattern(firstNamePrefix), esquery.WithIgnoreCase())).And().
		Equal("ogit/lastName", esquery.NewRegex(namePrefixPattern(lastNamePrefix), esquery.WithIgnoreCase()))

	rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query valuemation person by name prefix: %w", err)
	}
	defer rows.Close()

	persons, err := graph.ScanRows[ValuemationPerson](rows)
	if err != nil {
		return nil, fmt.Errorf("scan valuemation person by name prefix: %w", err)
	}
	return persons, nil
}
