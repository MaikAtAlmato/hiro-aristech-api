// internal/identity/resolver_test.go
package identity

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"github.com/stretchr/testify/require"
)

type stubMsgraphFinder struct {
	byPhone map[string][]sgo.Person
	byName  map[string][]sgo.Person
}

func (s stubMsgraphFinder) FindByPhone(_ context.Context, phone string) ([]sgo.Person, error) {
	return s.byPhone[phone], nil
}

func (s stubMsgraphFinder) FindByName(_ context.Context, firstName, lastName string) ([]sgo.Person, error) {
	return s.byName[firstName+" "+lastName], nil
}

type stubValuemationFinder struct {
	byPhone map[string][]bardioc.ValuemationPerson
	byName  map[string][]bardioc.ValuemationPerson
}

func (s stubValuemationFinder) FindByPhone(_ context.Context, phone string) ([]bardioc.ValuemationPerson, error) {
	return s.byPhone[phone], nil
}

func (s stubValuemationFinder) FindByName(_ context.Context, firstName, lastName string) ([]bardioc.ValuemationPerson, error) {
	return s.byName[firstName+" "+lastName], nil
}

func msPerson(id, first, last, email, phone string) sgo.Person {
	return sgo.Person{
		Entity:      graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(id)}},
		FirstName:   first,
		LastName:    last,
		Email:       email,
		MobilePhone: phone,
	}
}

func vmPerson(id, first, last, email, phone string) bardioc.ValuemationPerson {
	return bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(id)}},
			FirstName: first,
			LastName:  last,
			Email:     email,
		},
		PhoneNo: phone,
	}
}

func TestResolve_BothSourcesMatchByEmail(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "max@example.com", "+491111")
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+492222")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byPhone: map[string][]sgo.Person{"+491111": {ms}}},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.NotNil(t, id.MsgraphPersonID)
	require.Equal(t, "ms-1", id.MsgraphPersonID.String())
	require.NotNil(t, id.ValuemationPersonID)
	require.Equal(t, "vm-1", id.ValuemationPersonID.String())
	require.Equal(t, "Max Mustermann", id.DisplayName)
}

func TestResolve_OnlyValuemationMatch(t *testing.T) {
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.Nil(t, id.MsgraphPersonID)
	require.NotNil(t, id.ValuemationPersonID)
	require.Equal(t, "vm-1", id.ValuemationPersonID.String())
}

func TestResolve_OnlyMsgraphMatch(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "max@example.com", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byPhone: map[string][]sgo.Person{"+491111": {ms}}},
		Valuemation: stubValuemationFinder{},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.NotNil(t, id.MsgraphPersonID)
	require.Nil(t, id.ValuemationPersonID)
}

func TestResolve_NoMatchAnywhere(t *testing.T) {
	r := &Resolver{Msgraph: stubMsgraphFinder{}, Valuemation: stubValuemationFinder{}}

	_, err := r.Resolve(context.Background(), Criterion{Phone: "+490000"})

	require.True(t, errors.Is(err, ErrNotFound))
}

func TestResolve_AmbiguousWithinOneSource(t *testing.T) {
	vm1 := vmPerson("vm-1", "Max", "Mustermann", "max1@example.com", "+491111")
	vm2 := vmPerson("vm-2", "Max", "Mustermann", "max2@example.com", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm1, vm2}}},
	}

	_, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.True(t, errors.Is(err, ErrAmbiguous))
}

func TestResolve_ConflictingCrossSourceMatch(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "max@example.com", "+491111")
	vm := vmPerson("vm-1", "Someone", "Else", "someone@example.com", "+499999")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byName: map[string][]sgo.Person{"Max Mustermann": {ms}}},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Max Mustermann": {vm}}},
	}

	_, err := r.Resolve(context.Background(), Criterion{Name: "Max Mustermann"})

	require.True(t, errors.Is(err, ErrAmbiguous))
}

func TestResolve_MatchesByPhoneWhenEmailMissing(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "", "+491111")
	vm := vmPerson("vm-1", "Max", "Mustermann", "", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byPhone: map[string][]sgo.Person{"+491111": {ms}}},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.NotNil(t, id.MsgraphPersonID)
	require.NotNil(t, id.ValuemationPersonID)
}

func TestResolve_PhoneMismatchIsAmbiguousDespiteMatchingName(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "", "+491111")
	vm := vmPerson("vm-1", "Max", "Mustermann", "", "+499999")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byName: map[string][]sgo.Person{"Max Mustermann": {ms}}},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Max Mustermann": {vm}}},
	}

	_, err := r.Resolve(context.Background(), Criterion{Name: "Max Mustermann"})

	require.True(t, errors.Is(err, ErrAmbiguous))
}

func TestResolve_MatchesByNameWhenEmailAndPhoneMissing(t *testing.T) {
	ms := msPerson("ms-1", "Max", "Mustermann", "", "")
	vm := vmPerson("vm-1", "Max", "Mustermann", "", "")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byName: map[string][]sgo.Person{"Max Mustermann": {ms}}},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Max Mustermann": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Name: "Max Mustermann"})

	require.NoError(t, err)
	require.NotNil(t, id.MsgraphPersonID)
	require.NotNil(t, id.ValuemationPersonID)
}

func TestResolve_SplitsNameOnFirstSpace(t *testing.T) {
	vm := vmPerson("vm-1", "Anna", "von der Heide", "", "")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Anna von der Heide": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Name: "Anna von der Heide"})

	require.NoError(t, err)
	require.NotNil(t, id.ValuemationPersonID)
}

func TestResolve_ExternalIDPrefersXIDOverMetadataID(t *testing.T) {
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+491111")
	vm.XID = "valuemation-ext-1"

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.Equal(t, "valuemation-ext-1", id.ValuemationPersonXID)
}

func TestResolve_ExternalIDFallsBackToMetadataIDWhenXIDEmpty(t *testing.T) {
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Phone: "+491111"})

	require.NoError(t, err)
	require.Equal(t, "vm-1", id.ValuemationPersonXID)
}

func TestResolve_MergesValuemationSyncDuplicates(t *testing.T) {
	dup1 := vmPerson("vm-1", "Thomas", "Sellner", "thomas@almato.com", "+4971211478731")
	dup2 := vmPerson("vm-2", "Thomas", "Sellner", "thomas@almato.com", "+4971211478731")
	dup2.XID = "valuemation-ext-thomas"

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Thomas Sellner": {dup1, dup2}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Name: "Thomas Sellner"})

	require.NoError(t, err)
	require.Equal(t, "valuemation-ext-thomas", id.ValuemationPersonXID)
}

func TestResolve_DisjointDuplicateGroupsStayAmbiguous(t *testing.T) {
	schleich1 := vmPerson("vm-1", "Thomas", "Sellner", "thomas.sellner@schleichfigurines.onmicrosoft.com", "0")
	schleich2 := vmPerson("vm-2", "Thomas", "Sellner", "thomas.sellner@schleichfigurines.onmicrosoft.com", "0")
	almato1 := vmPerson("vm-3", "Thomas", "Sellner", "thomas.sellner@almato.com", "+4971211478731")
	almato2 := vmPerson("vm-4", "Thomas", "Sellner", "thomas.sellner@almato.com", "+4971211478731")

	r := &Resolver{
		Msgraph: stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{
			"Thomas Sellner": {schleich1, schleich2, almato1, almato2},
		}},
	}

	_, err := r.Resolve(context.Background(), Criterion{Name: "Thomas Sellner"})

	require.True(t, errors.Is(err, ErrAmbiguous))
}

func TestResolve_CrossSourceMatchAcceptsAnyPairAcrossDuplicates(t *testing.T) {
	// msA and msB are sync duplicates of one MSGraph person (linked by
	// shared phone). msA alone would NOT match vm via samePerson (different
	// email, different phone) — only msB does. The cross-source check must
	// try every pair in both clusters, not just the chosen representative,
	// or this resolves to ErrAmbiguous instead of a merged identity.
	msA := sgo.Person{
		Entity:      graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName:   "Max",
		LastName:    "Mustermann",
		Email:       "unrelated@example.com",
		MobilePhone: "+491111",
	}
	msB := sgo.Person{
		Entity:      graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-2")}},
		FirstName:   "Max",
		LastName:    "Mustermann",
		Email:       "max@example.com",
		MobilePhone: "+491111",
	}
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+499999")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{byName: map[string][]sgo.Person{"Max Mustermann": {msA, msB}}},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{"Max Mustermann": {vm}}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Name: "Max Mustermann"})

	require.NoError(t, err)
	require.NotNil(t, id.MsgraphPersonID)
	require.NotNil(t, id.ValuemationPersonID)
}

func TestResolve_DomainFilterNarrowsAmbiguousNameMatch(t *testing.T) {
	schleich := vmPerson("vm-1", "Paul", "Hoppe", "paul.hoppe@schleichfigurines.onmicrosoft.com", "0")
	almato := vmPerson("vm-2", "Paul", "Hoppe", "Paul.Hoppe@almato.com", "+4971211478772")

	r := &Resolver{
		Msgraph: stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{
			"Paul Hoppe": {schleich, almato},
		}},
	}

	id, err := r.Resolve(context.Background(), Criterion{Name: "Paul Hoppe", Domains: []string{"almato.com"}})

	require.NoError(t, err)
	require.Equal(t, "vm-2", id.ValuemationPersonID.String())
}

func TestResolve_DomainFilterWithBothDomainsStaysAmbiguous(t *testing.T) {
	schleich := vmPerson("vm-1", "Paul", "Hoppe", "paul.hoppe@schleichfigurines.onmicrosoft.com", "0")
	almato := vmPerson("vm-2", "Paul", "Hoppe", "Paul.Hoppe@almato.com", "+4971211478772")

	r := &Resolver{
		Msgraph: stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byName: map[string][]bardioc.ValuemationPerson{
			"Paul Hoppe": {schleich, almato},
		}},
	}

	_, err := r.Resolve(context.Background(), Criterion{
		Name:    "Paul Hoppe",
		Domains: []string{"almato.com", "schleichfigurines.onmicrosoft.com"},
	})

	require.True(t, errors.Is(err, ErrAmbiguous))
}

func TestResolve_DomainFilterExcludesAllCandidatesReturnsNotFound(t *testing.T) {
	vm := vmPerson("vm-1", "Max", "Mustermann", "max@example.com", "+491111")

	r := &Resolver{
		Msgraph:     stubMsgraphFinder{},
		Valuemation: stubValuemationFinder{byPhone: map[string][]bardioc.ValuemationPerson{"+491111": {vm}}},
	}

	_, err := r.Resolve(context.Background(), Criterion{Phone: "+491111", Domains: []string{"other.com"}})

	require.True(t, errors.Is(err, ErrNotFound))
}
