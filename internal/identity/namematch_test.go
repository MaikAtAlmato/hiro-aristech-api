package identity

import (
	"context"
	"fmt"
	"testing"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"github.com/stretchr/testify/require"
)

func TestStripTags(t *testing.T) {
	require.Equal(t, "Maik", stripTags("<firstname>Maik</firstname>"))
	require.Equal(t, "Maik Lander", stripTags("<main>Maik <last>Lander</last></main>"))
	require.Equal(t, "", stripTags("<empty></empty>"))
	require.Equal(t, "plain", stripTags("plain"))
}

func TestParseStructuredWords(t *testing.T) {
	raw := `[{"word":"Maik","start":0,"end":1,"confidence":0.9},{"word":"Lander","start":1,"end":2,"confidence":0.8}]`
	got, ok := parseStructuredWords(raw)
	require.True(t, ok)
	require.Equal(t, "Maik Lander", got)

	_, ok = parseStructuredWords("")
	require.False(t, ok)

	_, ok = parseStructuredWords("not json")
	require.False(t, ok)

	_, ok = parseStructuredWords("[]")
	require.False(t, ok)
}

func TestCandidateString(t *testing.T) {
	r := STTResult{
		ResultRaw:        " Maik ",
		ResultNlp:        "Maik",
		ResultTagged:     "<firstname>Maik</firstname>",
		ResultSlotted:    "<main>Maik</main>",
		ResultStructured: `[{"word":"Maik"}]`,
	}

	s, ok := candidateString(repRaw, r)
	require.True(t, ok)
	require.Equal(t, "Maik", s)

	s, ok = candidateString(repTagged, r)
	require.True(t, ok)
	require.Equal(t, "Maik", s)

	s, ok = candidateString(repStructured, r)
	require.True(t, ok)
	require.Equal(t, "Maik", s)

	_, ok = candidateString(repRaw, STTResult{})
	require.False(t, ok)
}

func TestExtractCandidates(t *testing.T) {
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik", ResultNlp: "Maik"},
		LastName:  STTResult{ResultRaw: "Lander"}, // no ResultNlp on this side
	}

	got := extractCandidates(q)

	require.Len(t, got, 1) // only "raw" has data on BOTH sides
	require.Equal(t, repRaw, got[0].rtype)
	require.Equal(t, "Maik", got[0].firstName)
	require.Equal(t, "Lander", got[0].lastName)
}

func TestExtractCandidates_NoData_ReturnsEmpty(t *testing.T) {
	require.Empty(t, extractCandidates(NameMatchQuery{}))
}

func TestExtractCandidates_MismatchedTypesAcrossSides_ReturnsEmpty(t *testing.T) {
	// Both sides have data, but under different representation types only —
	// no type has data on both FirstName and LastName, so nothing is usable,
	// even though neither side is literally empty.
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultNlp: "Lander"},
	}

	require.Empty(t, extractCandidates(q))
}

func TestSpecificity(t *testing.T) {
	require.Equal(t, 1.0, specificity("Maik", "Maik"))
	require.Equal(t, 0.25, specificity("M", "Maik"))
	require.Equal(t, 1.0, specificity("Maikael", "Maik")) // longer than matched name clamps to 1.0
	require.Equal(t, 0.0, specificity("M", ""))           // empty matched name
}

func TestStagePoints(t *testing.T) {
	require.Equal(t, 50, stagePoints(1.0, 1.0))   // full first + full last
	require.Equal(t, 20, stagePoints(0.0, 0.0))   // initial + initial, degenerate zero case
	require.Equal(t, 38, stagePoints(1.0, 0.1667)) // full first + initial last ("Maik"/"L" vs "Lander")
}

func TestRepresentationPoints(t *testing.T) {
	require.Equal(t, 30, representationPoints(4, 4))
	require.Equal(t, 20, representationPoints(3, 4))
	require.Equal(t, 10, representationPoints(2, 4))
	require.Equal(t, 0, representationPoints(1, 4))
	require.Equal(t, 0, representationPoints(0, 0))
	require.Equal(t, 30, representationPoints(1, 1))
}

func TestUniquenessPoints(t *testing.T) {
	require.Equal(t, 20, uniquenessPoints(1))
	require.Equal(t, 10, uniquenessPoints(2))
	require.Equal(t, 0, uniquenessPoints(3))
	require.Equal(t, 0, uniquenessPoints(5))
}

func TestGroupRecords_SplitsIntoDistinctPeopleAndScoresSpecificity(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-maik")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	max := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-max")}},
		FirstName: "Max",
		LastName:  "Lehmann",
	}
	records := []personRecord{{ms: &maik}, {ms: &max}}

	groups := groupRecords(records, repRaw, "M", "L")

	require.Len(t, groups, 2)
	for _, g := range groups {
		require.Equal(t, repRaw, g.repType)
		require.True(t, g.stagePoints >= 20 && g.stagePoints <= 50)
	}
}

func TestGroupRecords_MergesCrossSourceSamePerson(t *testing.T) {
	ms := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
		Email:     "maik.lander@almato.com",
	}
	vm := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-1")}},
			FirstName: "Maik",
			LastName:  "Lander",
			Email:     "maik.lander@almato.com",
		},
	}
	records := []personRecord{{ms: &ms}, {vm: &vm}}

	groups := groupRecords(records, repRaw, "Maik", "Lander")

	require.Len(t, groups, 1)
	require.Len(t, groups[0].records, 2)
}

func TestMergeGroups_MergesAcrossRepresentationTypesBySharedRecord(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	rawGroups := groupRecords([]personRecord{{ms: &maik}}, repRaw, "Maik", "Lander")
	nlpGroups := groupRecords([]personRecord{{ms: &maik}}, repNlp, "Maik", "Lander")

	finals := mergeGroups([][]personGroup{rawGroups, nlpGroups})

	require.Len(t, finals, 1)
	require.Len(t, finals[0].repTypes, 2)
	require.True(t, finals[0].repTypes[repRaw])
	require.True(t, finals[0].repTypes[repNlp])
}

func TestMergeGroups_KeepsDistinctPeopleSeparate(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-maik")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	max := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-max")}},
		FirstName: "Max",
		LastName:  "Lehmann",
	}
	groups := groupRecords([]personRecord{{ms: &maik}, {ms: &max}}, repRaw, "M", "L")

	finals := mergeGroups([][]personGroup{groups})

	require.Len(t, finals, 2)
}

func TestMergeGroups_TransitivelyMergesBridgingGroups(t *testing.T) {
	g1 := personGroup{recordIDs: map[string]bool{"a": true, "b": true}, repType: repRaw, stagePoints: 40}
	g2 := personGroup{recordIDs: map[string]bool{"c": true, "d": true}, repType: repRaw, stagePoints: 30}
	g3 := personGroup{recordIDs: map[string]bool{"b": true, "c": true}, repType: repNlp, stagePoints: 50}

	finals := mergeGroups([][]personGroup{{g1, g2}, {g3}})

	require.Len(t, finals, 1)
	require.True(t, finals[0].recordIDs["a"])
	require.True(t, finals[0].recordIDs["b"])
	require.True(t, finals[0].recordIDs["c"])
	require.True(t, finals[0].recordIDs["d"])
	require.Len(t, finals[0].repTypes, 2)
	require.True(t, finals[0].repTypes[repRaw])
	require.True(t, finals[0].repTypes[repNlp])
}

func TestCandidateFromCluster_PopulatesFieldsFromRepresentative(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}, XID: "ms-xid-1"},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	fc := &finalCluster{
		recordIDs:   map[string]bool{"ms:ms-1": true},
		records:     []personRecord{{ms: &maik}},
		repTypes:    map[representationType]bool{repRaw: true},
		stagePoints: 50,
	}

	c := candidateFromCluster(fc, 30, 20)

	require.Equal(t, "Maik Lander", c.Name)
	require.Equal(t, "Maik", c.FirstName)
	require.Equal(t, "Lander", c.LastName)
	require.Equal(t, "ms-xid-1", c.MsgraphPersonXID)
	require.Equal(t, "", c.ValuemationPersonXID)
	require.Equal(t, 50, c.StagePoints)
	require.Equal(t, 30, c.RepresentationPoints)
	require.Equal(t, 20, c.UniquenessPoints)
	require.Equal(t, 100, c.Confidence)
}

type stubMsgraphPrefixFinder struct {
	byPrefix map[string][]sgo.Person
}

func (s stubMsgraphPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error) {
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

type stubValuemationPrefixFinder struct {
	byPrefix map[string][]bardioc.ValuemationPerson
}

func (s stubValuemationPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]bardioc.ValuemationPerson, error) {
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

func TestMatcher_Match_NoNameData_ReturnsErrNoNameData(t *testing.T) {
	m := &Matcher{}
	_, err := m.Match(context.Background(), NameMatchQuery{})
	require.ErrorIs(t, err, ErrNoNameData)
}

func TestMatcher_Match_MismatchedTypesAcrossSides_ReturnsErrNoNameData(t *testing.T) {
	// Both sides carry data, but under different representation types only
	// (FirstName has raw, LastName has nlp) — no shared type means nothing
	// is usable, so this is the same 400 boundary as fully-empty input, not
	// a partial-match attempt.
	m := &Matcher{}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultNlp: "Lander"},
	}

	_, err := m.Match(context.Background(), q)

	require.ErrorIs(t, err, ErrNoNameData)
}

func TestMatcher_Match_FullNameTwoRepresentations_HighConfidence(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"Maik Lander": {maik},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik", ResultNlp: "Maik"},
		LastName:  STTResult{ResultRaw: "Lander", ResultNlp: "Lander"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 100, got[0].Confidence)
	require.Equal(t, 50, got[0].StagePoints)
	require.Equal(t, 30, got[0].RepresentationPoints)
	require.Equal(t, 20, got[0].UniquenessPoints)
	require.Equal(t, "Maik Lander", got[0].Name)
}

func TestMatcher_Match_InitialsOnlySingleRepresentation_LowerConfidence(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"M L": {maik},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	// specificity("M","Maik")=0.25, specificity("L","Lander")=1/6=0.1667
	// stageScore=(0.25+0.1667)/2=0.2083 -> stagePoints=int(20+30*0.2083+0.5)=int(26.75)=26
	require.Equal(t, 26, got[0].StagePoints)
	require.Equal(t, 30, got[0].RepresentationPoints) // 1/1 available representation agreed
	require.Equal(t, 20, got[0].UniquenessPoints)
}

func TestMatcher_Match_AmbiguousAcrossPeople_SplitsUniquenessPoints(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-maik")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	max := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-max")}},
		FirstName: "Max",
		LastName:  "Lehmann",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"M L": {maik, max},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, c := range got {
		require.Equal(t, 10, c.UniquenessPoints)
	}
}

func TestMatcher_Match_CrossSourceSamePerson_MergesXIDs(t *testing.T) {
	ms := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}, XID: "ms-xid-1"},
		FirstName: "Maik",
		LastName:  "Lander",
		Email:     "maik.lander@almato.com",
	}
	vm := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-1")}, XID: "vm-xid-1"},
			FirstName: "Maik",
			LastName:  "Lander",
			Email:     "maik.lander@almato.com",
		},
	}
	m := &Matcher{
		Msgraph:     stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{"Maik Lander": {ms}}},
		Valuemation: stubValuemationPrefixFinder{byPrefix: map[string][]bardioc.ValuemationPerson{"Maik Lander": {vm}}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultRaw: "Lander"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ms-xid-1", got[0].MsgraphPersonXID)
	require.Equal(t, "vm-xid-1", got[0].ValuemationPersonXID)
}

func TestMatcher_Match_CapsAtFiveCandidates(t *testing.T) {
	persons := map[string][]sgo.Person{}
	var all []sgo.Person
	for i := 0; i < 7; i++ {
		p := sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(fmt.Sprintf("ms-%d", i))}},
			FirstName: "Maik",
			LastName:  fmt.Sprintf("Lastname%d", i),
		}
		all = append(all, p)
	}
	persons["M L"] = all
	m := &Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: persons}}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 5)
}

func TestMatcher_Match_DomainFilterNarrowsAmbiguousResult(t *testing.T) {
	schleich := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-1")}},
			FirstName: "Paul",
			LastName:  "Hoppe",
			Email:     "paul.hoppe@schleichfigurines.onmicrosoft.com",
		},
	}
	almato := bardioc.ValuemationPerson{
		Person: sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("vm-2")}},
			FirstName: "Paul",
			LastName:  "Hoppe",
			Email:     "Paul.Hoppe@almato.com",
		},
	}
	m := &Matcher{
		Valuemation: stubValuemationPrefixFinder{byPrefix: map[string][]bardioc.ValuemationPerson{
			"Paul Hoppe": {schleich, almato},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Paul"},
		LastName:  STTResult{ResultRaw: "Hoppe"},
		Domains:   []string{"almato.com"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "vm-2", got[0].ValuemationPersonXID) // no XID set -> falls back to node ID
}

func TestApplyFuzzyMalus_SubtractsAndFloorsAt20(t *testing.T) {
	groups := []personGroup{{stagePoints: 25}, {stagePoints: 50}, {stagePoints: 20}}

	got := applyFuzzyMalus(groups)

	require.Equal(t, 20, got[0].stagePoints) // 25-15=10 -> floored to 20
	require.Equal(t, 35, got[1].stagePoints) // 50-15=35, above floor
	require.Equal(t, 20, got[2].stagePoints) // 20-15=5 -> floored to 20
}

func TestMatcher_FuzzyQueryRecords_ShortNamePart_SkipsLookupEntirely(t *testing.T) {
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			// Deliberately registered so the test fails loudly if this ever
			// gets queried — firstName "T" is only 1 rune, below
			// minFuzzyNameLength, so fuzzyQueryRecords must return before
			// querying anything.
			"T Sel": {{Entity: graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}}, FirstName: "Thomas", LastName: "Sellner"}},
		}},
	}

	records, err := m.fuzzyQueryRecords(context.Background(), "T", "Selner", nil)

	require.NoError(t, err)
	require.Empty(t, records)
}

func TestMatcher_FuzzyQueryRecords_FiltersCandidatesBeyondThreshold(t *testing.T) {
	closeMatch := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-close")}},
		FirstName: "Thomas",
		LastName:  "Sellner",
	}
	farAway := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-far")}},
		FirstName: "Thomas",
		LastName:  "Schneider",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"Tho Sel": {closeMatch, farAway},
		}},
	}

	records, err := m.fuzzyQueryRecords(context.Background(), "Thomas", "Selner", nil)

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "Sellner", records[0].ms.LastName)
}

func TestMatcher_Match_FuzzyFallback_FindsTypoAfterExactMatchFails(t *testing.T) {
	sellner := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Thomas",
		LastName:  "Sellner",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			// Widened fuzzy query key (shortPrefix("Thomas",3)+" "+shortPrefix("Selner",3)).
			// The exact key "Thomas Selner" is deliberately NOT registered —
			// the exact-prefix path must find nothing so this test proves
			// the fuzzy fallback is what finds the person.
			"Tho Sel": {sellner},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Thomas"},
		LastName:  STTResult{ResultRaw: "Selner"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Thomas Sellner", got[0].Name)
	// specificity("Thomas","Thomas")=1.0, specificity("Selner","Sellner")=6/7=0.8571
	// stageScore=(1.0+0.8571)/2=0.9286 -> stagePoints=int(20+30*0.9286+0.5)=48
	// fuzzy malus: 48-15=33 (above floor 20)
	require.Equal(t, 33, got[0].StagePoints)
	require.Equal(t, 30, got[0].RepresentationPoints) // 1/1 usable representation ("raw") agreed
	require.Equal(t, 20, got[0].UniquenessPoints)
	require.Equal(t, 83, got[0].Confidence)
}

func TestMatcher_Match_ExactMatchFound_FuzzyFallbackNotTriggered(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"Maik Lander": {maik},
			// If the fuzzy fallback ran despite the exact match already
			// succeeding, its widened query (shortPrefix("Maik",3)+"
			// "+shortPrefix("Lander",3) = "Mai Lan") would ALSO match this
			// different, wrong person — proving fuzzy did not run.
			"Mai Lan": {{
				Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-wrong")}},
				FirstName: "Maike",
				LastName:  "Lahn",
			}},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultRaw: "Lander"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Maik Lander", got[0].Name)
	require.Equal(t, 50, got[0].StagePoints) // exact full match, no fuzzy malus applied
}
