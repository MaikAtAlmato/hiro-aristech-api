// internal/identity/dedup_test.go
package identity

import (
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"github.com/stretchr/testify/require"
)

func TestClusterValuemationPersons_LinksByEmail(t *testing.T) {
	a := vmPerson("vm-1", "Thomas", "Sellner", "thomas@example.com", "0")
	b := vmPerson("vm-2", "Thomas", "Sellner", "thomas@example.com", "0")

	clusters := clusterValuemationPersons([]bardioc.ValuemationPerson{a, b}, false)

	require.Len(t, clusters, 1)
	require.Len(t, clusters[0], 2)
}

func TestClusterValuemationPersons_LinksByPhone(t *testing.T) {
	a := vmPerson("vm-1", "Thomas", "Sellner", "thomas.a@example.com", "+4971211478731")
	b := vmPerson("vm-2", "Thomas", "Sellner", "thomas.b@example.com", "+4971211478731")

	clusters := clusterValuemationPersons([]bardioc.ValuemationPerson{a, b}, false)

	require.Len(t, clusters, 1)
	require.Len(t, clusters[0], 2)
}

func TestClusterValuemationPersons_PlaceholderPhoneDoesNotLink(t *testing.T) {
	a := vmPerson("vm-1", "Thomas", "Sellner", "thomas.a@example.com", "0")
	b := vmPerson("vm-2", "Thomas", "Sellner", "thomas.b@example.com", "0")

	clusters := clusterValuemationPersons([]bardioc.ValuemationPerson{a, b}, false)

	require.Len(t, clusters, 2)
}

func TestClusterValuemationPersons_DisjointGroupsLikeThomasSellner(t *testing.T) {
	schleich1 := vmPerson("vm-1", "Thomas", "Sellner", "thomas.sellner@schleichfigurines.onmicrosoft.com", "0")
	schleich2 := vmPerson("vm-2", "Thomas", "Sellner", "thomas.sellner@schleichfigurines.onmicrosoft.com", "0")
	almato1 := vmPerson("vm-3", "Thomas", "Sellner", "thomas.sellner@almato.com", "+4971211478731")
	almato2 := vmPerson("vm-4", "Thomas", "Sellner", "thomas.sellner@almato.com", "+4971211478731")

	clusters := clusterValuemationPersons([]bardioc.ValuemationPerson{schleich1, schleich2, almato1, almato2}, false)

	require.Len(t, clusters, 2)
}

func TestClusterValuemationPersons_ExcludesPhoneWhenSearchWasByPhone(t *testing.T) {
	// Same phone (the search criterion) but different emails: two different
	// people who happen to share e.g. a team phone line must NOT be merged.
	a := vmPerson("vm-1", "Anna", "A", "anna@example.com", "+491111")
	b := vmPerson("vm-2", "Berta", "B", "berta@example.com", "+491111")

	clusters := clusterValuemationPersons([]bardioc.ValuemationPerson{a, b}, true)

	require.Len(t, clusters, 2)
}

func TestValuemationRepresentative_PrefersXID(t *testing.T) {
	withoutXID := vmPerson("vm-1", "Thomas", "Sellner", "thomas@example.com", "0")
	withXID := vmPerson("vm-2", "Thomas", "Sellner", "thomas@example.com", "0")
	withXID.XID = "valuemation-external-42"

	rep := valuemationRepresentative([]bardioc.ValuemationPerson{withoutXID, withXID})

	require.Equal(t, "valuemation-external-42", rep.XID)
}

func TestValuemationRepresentative_PrefersHigherFieldCount(t *testing.T) {
	sparse := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-2")}},
			FirstName: "Thomas",
			LastName:  "Sellner",
		},
	}
	rich := vmPerson("vm-1", "Thomas", "Sellner", "thomas@example.com", "+4971211478731")

	rep := valuemationRepresentative([]bardioc.ValuemationPerson{sparse, rich})

	require.Equal(t, "vm-1", rep.Metadata.ID.String())
}

func TestValuemationRepresentative_TieBreaksOnLowestIDWhenScoresEqual(t *testing.T) {
	a := vmPerson("vm-2", "Thomas", "Sellner", "thomas@example.com", "+4971211478731")
	b := vmPerson("vm-1", "Thomas", "Sellner", "thomas@example.com", "+4971211478731")

	rep := valuemationRepresentative([]bardioc.ValuemationPerson{a, b})

	require.Equal(t, "vm-1", rep.Metadata.ID.String())
}

func TestClusterMsgraphPersons_LinksByAnyPhoneField(t *testing.T) {
	a := msPerson("ms-1", "Thomas", "Sellner", "", "+4971211478731")
	b := sgo.Person{
		Entity:      graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-2")}},
		FirstName:   "Thomas",
		LastName:    "Sellner",
		OfficePhone: "+4971211478731",
	}

	clusters := clusterMsgraphPersons([]sgo.Person{a, b}, false)

	require.Len(t, clusters, 1)
}

func TestMsgraphRepresentative_PrefersXID(t *testing.T) {
	withoutXID := msPerson("ms-1", "Thomas", "Sellner", "thomas@example.com", "")
	withXID := msPerson("ms-2", "Thomas", "Sellner", "thomas@example.com", "")
	withXID.XID = "msgraph-external-7"

	rep := msgraphRepresentative([]sgo.Person{withoutXID, withXID})

	require.Equal(t, "msgraph-external-7", rep.XID)
}

func TestFilterValuemationPersonsByDomain_NoDomainsPassesThrough(t *testing.T) {
	a := vmPerson("vm-1", "Paul", "Hoppe", "paul@almato.com", "0")

	got := filterValuemationPersonsByDomain([]bardioc.ValuemationPerson{a}, nil)

	require.Len(t, got, 1)
}

func TestFilterValuemationPersonsByDomain_KeepsMatchingDomain(t *testing.T) {
	almato := vmPerson("vm-1", "Paul", "Hoppe", "paul.hoppe@almato.com", "0")
	other := vmPerson("vm-2", "Paul", "Hoppe", "paul.hoppe@schleichfigurines.onmicrosoft.com", "0")

	got := filterValuemationPersonsByDomain([]bardioc.ValuemationPerson{almato, other}, []string{"almato.com"})

	require.Len(t, got, 1)
	require.Equal(t, "vm-1", got[0].Metadata.ID.String())
}

func TestFilterValuemationPersonsByDomain_UnionOfMultipleDomains(t *testing.T) {
	almato := vmPerson("vm-1", "Paul", "Hoppe", "paul.hoppe@almato.com", "0")
	datagroup := vmPerson("vm-2", "Paul", "Hoppe", "paul.hoppe@datagroup.de", "0")
	other := vmPerson("vm-3", "Paul", "Hoppe", "paul.hoppe@schleichfigurines.onmicrosoft.com", "0")

	got := filterValuemationPersonsByDomain(
		[]bardioc.ValuemationPerson{almato, datagroup, other},
		[]string{"almato.com", "datagroup.de"},
	)

	require.Len(t, got, 2)
}

func TestFilterValuemationPersonsByDomain_DropsMissingOrMalformedEmail(t *testing.T) {
	noEmail := vmPerson("vm-1", "Paul", "Hoppe", "", "0")
	noAt := vmPerson("vm-2", "Paul", "Hoppe", "not-an-email", "0")

	got := filterValuemationPersonsByDomain([]bardioc.ValuemationPerson{noEmail, noAt}, []string{"almato.com"})

	require.Empty(t, got)
}

func TestFilterValuemationPersonsByDomain_CaseInsensitive(t *testing.T) {
	a := vmPerson("vm-1", "Paul", "Hoppe", "Paul.Hoppe@ALMATO.COM", "0")

	got := filterValuemationPersonsByDomain([]bardioc.ValuemationPerson{a}, []string{"almato.com"})

	require.Len(t, got, 1)
}

func TestFilterMsgraphPersonsByDomain_KeepsMatchingDomain(t *testing.T) {
	almato := msPerson("ms-1", "Paul", "Hoppe", "paul.hoppe@almato.com", "")
	other := msPerson("ms-2", "Paul", "Hoppe", "paul.hoppe@example.com", "")

	got := filterMsgraphPersonsByDomain([]sgo.Person{almato, other}, []string{"almato.com"})

	require.Len(t, got, 1)
	require.Equal(t, "ms-1", got[0].Metadata.ID.String())
}
