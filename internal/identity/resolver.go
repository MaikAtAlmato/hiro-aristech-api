// Package identity resolves a phone number or name into a caller identity
// by cross-referencing the MSGraph- and Valuemation-sourced Person nodes in
// Bardioc.
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
)

// ErrNotFound means no unambiguous identity could be resolved from either source.
var ErrNotFound = errors.New("identity: no matching caller found")

// ErrAmbiguous means more than one candidate matched, or the two sources
// matched conflicting people.
var ErrAmbiguous = errors.New("identity: ambiguous caller match")

// Criterion is the voicebot-supplied lookup input. Exactly one of Phone or
// Name must be non-empty; callers of Resolve are responsible for enforcing
// that. Domains, if non-empty, restricts matching to persons whose email
// domain is in the list (already normalized to lowercase by the caller —
// see internal/api's parseDomains, added in a later task).
type Criterion struct {
	Phone   string
	Name    string
	Domains []string
}

// Identity is the resolved caller, referencing whichever graph nodes were found.
type Identity struct {
	ValuemationPersonID  *graph.MetadataID
	MsgraphPersonID      *graph.MetadataID
	ValuemationPersonXID string
	MsgraphPersonXID     string
	DisplayName          string
}

// MsgraphPersonFinder looks up MSGraph-sourced Person nodes.
type MsgraphPersonFinder interface {
	FindByPhone(ctx context.Context, phone string) ([]sgo.Person, error)
	FindByName(ctx context.Context, firstName, lastName string) ([]sgo.Person, error)
}

// ValuemationPersonFinder looks up Valuemation-sourced Person nodes.
type ValuemationPersonFinder interface {
	FindByPhone(ctx context.Context, phone string) ([]bardioc.ValuemationPerson, error)
	FindByName(ctx context.Context, firstName, lastName string) ([]bardioc.ValuemationPerson, error)
}

// Resolver consolidates MSGraph and Valuemation Person lookups into a single Identity.
type Resolver struct {
	Msgraph     MsgraphPersonFinder
	Valuemation ValuemationPersonFinder
}

// Resolve looks up c in both sources and consolidates the result. Each
// source's raw candidates are first clustered into "same real person"
// groups (see clusterMsgraphPersons/clusterValuemationPersons) so that sync
// duplicates don't cause a false ambiguous result. Resolve then succeeds
// when exactly one source has exactly one cluster (the other may have
// zero), or when both sources have exactly one cluster and those clusters
// are confirmed to be the same person (any pair across both clusters
// matching via email, then phone, then name). Any other combination is an
// error.
func (r *Resolver) Resolve(ctx context.Context, c Criterion) (*Identity, error) {
	var msPersons []sgo.Person
	var vmPersons []bardioc.ValuemationPerson
	var err error
	byPhone := c.Phone != ""

	if byPhone {
		if msPersons, err = r.Msgraph.FindByPhone(ctx, c.Phone); err != nil {
			return nil, fmt.Errorf("msgraph phone lookup: %w", err)
		}
		if vmPersons, err = r.Valuemation.FindByPhone(ctx, c.Phone); err != nil {
			return nil, fmt.Errorf("valuemation phone lookup: %w", err)
		}
	} else {
		firstName, lastName := splitName(c.Name)
		if msPersons, err = r.Msgraph.FindByName(ctx, firstName, lastName); err != nil {
			return nil, fmt.Errorf("msgraph name lookup: %w", err)
		}
		if vmPersons, err = r.Valuemation.FindByName(ctx, firstName, lastName); err != nil {
			return nil, fmt.Errorf("valuemation name lookup: %w", err)
		}
	}

	msClusters := clusterMsgraphPersons(filterMsgraphPersonsByDomain(msPersons, c.Domains), byPhone)
	vmClusters := clusterValuemationPersons(filterValuemationPersonsByDomain(vmPersons, c.Domains), byPhone)

	switch {
	case len(msClusters) > 1 || len(vmClusters) > 1:
		return nil, ErrAmbiguous
	case len(msClusters) == 1 && len(vmClusters) == 1:
		if !anyPairSamePerson(msClusters[0], vmClusters[0]) {
			return nil, ErrAmbiguous
		}
		ms := msgraphRepresentative(msClusters[0])
		vm := valuemationRepresentative(vmClusters[0])
		return identityFrom(&ms, &vm), nil
	case len(msClusters) == 1:
		ms := msgraphRepresentative(msClusters[0])
		return identityFrom(&ms, nil), nil
	case len(vmClusters) == 1:
		vm := valuemationRepresentative(vmClusters[0])
		return identityFrom(nil, &vm), nil
	default:
		return nil, ErrNotFound
	}
}

// anyPairSamePerson reports whether any MSGraph record in msCluster and any
// Valuemation record in vmCluster are confirmed the same person. Checking
// every pair (not just the chosen representatives) matters because
// duplicate records within a cluster may have different fields populated.
func anyPairSamePerson(msCluster []sgo.Person, vmCluster []bardioc.ValuemationPerson) bool {
	for _, ms := range msCluster {
		for _, vm := range vmCluster {
			if samePerson(ms, vm) {
				return true
			}
		}
	}
	return false
}

// samePerson confirms two candidates are the same real person, checking
// email, then phone, then name — the first field populated on both sides
// decides the comparison.
func samePerson(ms sgo.Person, vm bardioc.ValuemationPerson) bool {
	if ms.Email != "" && vm.Email != "" {
		return strings.EqualFold(ms.Email, vm.Email)
	}
	if vm.PhoneNo != "" && (ms.OfficePhone != "" || ms.MobilePhone != "" || ms.OtherPhone != "") {
		for _, phone := range []string{ms.OfficePhone, ms.MobilePhone, ms.OtherPhone} {
			if phone != "" && phone == vm.PhoneNo {
				return true
			}
		}
		return false
	}
	return strings.EqualFold(ms.FirstName, vm.FirstName) && strings.EqualFold(ms.LastName, vm.LastName)
}

// splitName splits "First Last-with-spaces" into first and last name on the
// first space. This is an MVP simplification: multi-word first names are
// not supported.
func splitName(name string) (firstName, lastName string) {
	name = strings.TrimSpace(name)
	parts := strings.SplitN(name, " ", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.TrimSpace(parts[1])
}

// externalID returns xid if set, otherwise id's string form. Used because
// not every synced Person node has an external XID populated.
func externalID(xid string, id graph.MetadataID) string {
	if xid != "" {
		return xid
	}
	return id.String()
}

func identityFrom(ms *sgo.Person, vm *bardioc.ValuemationPerson) *Identity {
	id := &Identity{}
	if ms != nil {
		msID := ms.Metadata.ID
		id.MsgraphPersonID = &msID
		id.MsgraphPersonXID = externalID(ms.XID, msID)
		id.DisplayName = strings.TrimSpace(ms.FirstName + " " + ms.LastName)
	}
	if vm != nil {
		vmID := vm.Metadata.ID
		id.ValuemationPersonID = &vmID
		id.ValuemationPersonXID = externalID(vm.XID, vmID)
		if id.DisplayName == "" {
			id.DisplayName = strings.TrimSpace(vm.FirstName + " " + vm.LastName)
		}
	}
	return id
}
