// internal/bardioc/msgraph_person_repo.go
package bardioc

import (
	"context"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/esquery"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
)

// msgraphPhoneFields lists the sgo.Person fields that may hold a caller's
// phone number, in the shape synced by hiro-conn-msgraph.
var msgraphPhoneFields = []string{"ogit/officePhone", "ogit/mobilePhone", "ogit/otherPhone"}

// MsgraphPersonRepository finds Person nodes written by hiro-conn-msgraph.
type MsgraphPersonRepository struct {
	client EdgeClient
}

// NewMsgraphPersonRepository creates a new MsgraphPersonRepository.
func NewMsgraphPersonRepository(client EdgeClient) *MsgraphPersonRepository {
	return &MsgraphPersonRepository{client: client}
}

// FindByPhone returns every MSGraph Person whose office, mobile, or other
// phone field equals phone. Each of the three fields is queried separately
// and results are deduplicated by node ID, avoiding any dependency on
// esquery's OR/AND grouping semantics.
func (r *MsgraphPersonRepository) FindByPhone(ctx context.Context, phone string) ([]sgo.Person, error) {
	seen := map[string]bool{}
	var results []sgo.Person

	for _, field := range msgraphPhoneFields {
		b := esquery.NewBuilder().
			Equal(graph.OgitType, "ogit/Person").And().
			Equal("/pFlag", PFlagMsgraph).And().
			Equal(field, phone)

		rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
		if err != nil {
			return nil, fmt.Errorf("query msgraph person by %s: %w", field, err)
		}

		persons, err := graph.ScanRows[sgo.Person](rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("scan msgraph person by %s: %w", field, err)
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

// FindByName returns every MSGraph Person with an exact, case-insensitive
// match on first and last name as stored in the graph.
func (r *MsgraphPersonRepository) FindByName(ctx context.Context, firstName, lastName string) ([]sgo.Person, error) {
	b := esquery.NewBuilder().
		Equal(graph.OgitType, "ogit/Person").And().
		Equal("/pFlag", PFlagMsgraph).And().
		Equal("ogit/firstName", firstName, esquery.WithIgnoreCase()).And().
		Equal("ogit/lastName", lastName, esquery.WithIgnoreCase())

	rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query msgraph person by name: %w", err)
	}
	defer rows.Close()

	persons, err := graph.ScanRows[sgo.Person](rows)
	if err != nil {
		return nil, fmt.Errorf("scan msgraph person by name: %w", err)
	}
	return persons, nil
}

// FindByNamePrefix returns every MSGraph Person whose first and last name
// each start with the given prefix (case-insensitive). A prefix equal to
// the full name matches only that name (modulo casing), so this one method
// serves both full-name and initial-only lookups.
func (r *MsgraphPersonRepository) FindByNamePrefix(ctx context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error) {
	b := esquery.NewBuilder().
		Equal(graph.OgitType, "ogit/Person").And().
		Equal("/pFlag", PFlagMsgraph).And().
		Equal("ogit/firstName", esquery.NewRegex(namePrefixPattern(firstNamePrefix), esquery.WithIgnoreCase())).And().
		Equal("ogit/lastName", esquery.NewRegex(namePrefixPattern(lastNamePrefix), esquery.WithIgnoreCase()))

	rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query msgraph person by name prefix: %w", err)
	}
	defer rows.Close()

	persons, err := graph.ScanRows[sgo.Person](rows)
	if err != nil {
		return nil, fmt.Errorf("scan msgraph person by name prefix: %w", err)
	}
	return persons, nil
}

// FindByLastNameSuffix returns every MSGraph Person whose first name starts
// with firstNamePrefix and whose last name matches any single leading
// character followed by lastNameSuffix (case-insensitive). Mirrors the
// Valuemation equivalent.
func (r *MsgraphPersonRepository) FindByLastNameSuffix(ctx context.Context, firstNamePrefix, lastNameSuffix string) ([]sgo.Person, error) {
	b := esquery.NewBuilder().
		Equal(graph.OgitType, "ogit/Person").And().
		Equal("/pFlag", PFlagMsgraph).And().
		Equal("ogit/firstName", esquery.NewRegex(namePrefixPattern(firstNamePrefix), esquery.WithIgnoreCase())).And().
		Equal("ogit/lastName", esquery.NewRegex(lastNameSuffixPattern(lastNameSuffix), esquery.WithIgnoreCase()))

	rows, err := r.client.QueryVertices(ctx, b, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query msgraph person by last-name suffix: %w", err)
	}
	defer rows.Close()

	persons, err := graph.ScanRows[sgo.Person](rows)
	if err != nil {
		return nil, fmt.Errorf("scan msgraph person by last-name suffix: %w", err)
	}
	return persons, nil
}
