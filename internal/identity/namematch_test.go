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

func TestMatchQuality_ExactMatch_ReturnsOne(t *testing.T) {
	require.Equal(t, 1.0, matchQuality("Maik", "Maik"))
}

func TestMatchQuality_PrefixOnly_MatchesOldSpecificityRatio(t *testing.T) {
	// "M" is a true prefix of "Maik": levenshtein("m","maik")=3, maxLen=4,
	// fuzzyRatio=1-3/4=0.25 — no phonetic bonus ("m" and "maik" encode
	// differently). Same value the old specificity-ratio formula gave.
	require.InDelta(t, 0.25, matchQuality("M", "Maik"), 0.0001)
}

func TestMatchQuality_PhoneticMatchDifferentSpelling(t *testing.T) {
	// "Meyer" STT text against a stored "Maier": both encode to Kölner
	// Phonetik "67", so even though the spellings differ letter-for-letter,
	// the phonetic bonus pushes the combined score up.
	withoutPhoneticNeighbor := matchQuality("Meyer", "Schmidt")
	withPhoneticNeighbor := matchQuality("Meyer", "Maier")
	require.Greater(t, withPhoneticNeighbor, withoutPhoneticNeighbor)
}

func TestMatchQuality_TypoWithinEditDistance_SelnerAgainstSellner(t *testing.T) {
	// The originally reported real-world case: STT heard "Selner" for the
	// real name "Sellner". levenshtein("selner","sellner")=1, maxLen=7,
	// fuzzyRatio=1-1/7=0.8571. Both also encode to Kölner Phonetik "8567",
	// so +0.22 phonetic bonus applies: 0.8571+0.22=1.0771, clamped to 1.0.
	require.Equal(t, 1.0, matchQuality("Selner", "Sellner"))
}

func TestMatchQuality_NoSimilarity_ReturnsLowScore(t *testing.T) {
	require.Less(t, matchQuality("Meyer", "Zzzzzzzzzz"), 0.3)
}

func TestHintMatches(t *testing.T) {
	require.True(t, hintMatches("Land", "Lander"))
	require.True(t, hintMatches("LAND", "lander")) // case-insensitive via normalize
	require.False(t, hintMatches("Land", "Meyer"))
	require.False(t, hintMatches("", "Lander")) // empty hint never matches
}

func TestTopByStagePoints_KeepsHighestNSorted(t *testing.T) {
	groups := []personGroup{
		{stagePoints: 20}, {stagePoints: 50}, {stagePoints: 35}, {stagePoints: 45}, {stagePoints: 30},
	}

	got := topByStagePoints(groups, 3)

	require.Len(t, got, 3)
	require.Equal(t, 50, got[0].stagePoints)
	require.Equal(t, 45, got[1].stagePoints)
	require.Equal(t, 35, got[2].stagePoints)
}

func TestTopByStagePoints_FewerThanN_ReturnsAllSorted(t *testing.T) {
	groups := []personGroup{{stagePoints: 20}, {stagePoints: 50}}

	got := topByStagePoints(groups, 15)

	require.Len(t, got, 2)
	require.Equal(t, 50, got[0].stagePoints)
	require.Equal(t, 20, got[1].stagePoints)
}

func TestStagePoints(t *testing.T) {
	require.Equal(t, 50, stagePoints(1.0, 1.0))    // full first + full last
	require.Equal(t, 0, stagePoints(0.0, 0.0))     // no similarity, degenerate zero case
	require.Equal(t, 23, stagePoints(1.0, 0.1667)) // full first + initial last ("Maik"/"L" vs "Lander")
	// 0.35*1.0 + 0.65*0.1667 = 0.4583 → int(50*0.4583+0.5) = 23
}

func TestStagePoints_LastNameWeightedHigher(t *testing.T) {
	// Last name carries 65% of weight.
	// First name perfect (1.0), last name zero (0.0):
	// stageScore = 0.35 → stagePoints = int(50*0.35+0.5) = 18
	got := stagePoints(1.0, 0.0)
	require.Equal(t, 18, got)
}

func TestStagePoints_BothPerfect(t *testing.T) {
	// Both 1.0: stageScore = 1.0 → stagePoints = 50
	got := stagePoints(1.0, 1.0)
	require.Equal(t, 50, got)
}

func TestStagePoints_BothZero(t *testing.T) {
	// Both 0.0: stageScore = 0.0 → stagePoints = 0 (no floor)
	got := stagePoints(0.0, 0.0)
	require.Equal(t, 0, got)
}

func TestStagePoints_LastNamePerfectFirstZero(t *testing.T) {
	// Last name perfect (1.0), first name zero (0.0)
	// stageScore = 0.65 → int(50*0.65+0.5) = 33
	got := stagePoints(0.0, 1.0)
	require.Equal(t, 33, got)
}

func TestPhoneticBonus_Value(t *testing.T) {
	require.InDelta(t, 0.35, phoneticBonus, 0.0001)
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

func TestGroupRecords_SplitsIntoDistinctPeopleAndScoresMatchQuality(t *testing.T) {
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

	groups := groupRecords(records, repRaw, "M", "L", "")

	require.Len(t, groups, 2)
	for _, g := range groups {
		require.Equal(t, repRaw, g.repType)
		require.True(t, g.stagePoints >= 0 && g.stagePoints <= 50)
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

	groups := groupRecords(records, repRaw, "Maik", "Lander", "")

	require.Len(t, groups, 1)
	require.Len(t, groups[0].records, 2)
}

func TestGroupRecords_LastNameHintBoostsMatchingCandidateStagePoints(t *testing.T) {
	lander := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	records := []personRecord{{ms: &lander}}

	withoutHint := groupRecords(records, repRaw, "M", "L", "")
	withHint := groupRecords(records, repRaw, "M", "L", "Land")

	require.Len(t, withoutHint, 1)
	require.Len(t, withHint, 1)
	require.Equal(t, withoutHint[0].stagePoints+10, withHint[0].stagePoints)
}

func TestGroupRecords_LastNameHintBoostCapsAt50(t *testing.T) {
	lander := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	records := []personRecord{{ms: &lander}}

	// Exact full-name match already scores stagePoints=50 (the ceiling);
	// a matching hint must not push it above that ceiling.
	got := groupRecords(records, repRaw, "Maik", "Lander", "Land")

	require.Len(t, got, 1)
	require.Equal(t, 50, got[0].stagePoints)
}

func TestMergeGroups_MergesAcrossRepresentationTypesBySharedRecord(t *testing.T) {
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	rawGroups := groupRecords([]personRecord{{ms: &maik}}, repRaw, "Maik", "Lander", "")
	nlpGroups := groupRecords([]personRecord{{ms: &maik}}, repNlp, "Maik", "Lander", "")

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
	groups := groupRecords([]personRecord{{ms: &maik}, {ms: &max}}, repRaw, "M", "L", "")

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
	bySuffix map[string][]sgo.Person
}

func (s stubMsgraphPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error) {
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

func (s stubMsgraphPrefixFinder) FindByLastNameSuffix(_ context.Context, firstNamePrefix, lastNameSuffix string) ([]sgo.Person, error) {
	return s.bySuffix[firstNamePrefix+" "+lastNameSuffix], nil
}

type stubValuemationPrefixFinder struct {
	byPrefix map[string][]bardioc.ValuemationPerson
	bySuffix map[string][]bardioc.ValuemationPerson
}

func (s stubValuemationPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]bardioc.ValuemationPerson, error) {
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

func (s stubValuemationPrefixFinder) FindByLastNameSuffix(_ context.Context, firstNamePrefix, lastNameSuffix string) ([]bardioc.ValuemationPerson, error) {
	return s.bySuffix[firstNamePrefix+" "+lastNameSuffix], nil
}

// countingPrefixFinder wraps byPrefix like stubMsgraphPrefixFinder but
// also counts how many times FindByNamePrefix was called, so tests can
// assert on query fan-out (e.g. queryRecordsByPrefix issuing one query
// when normalization variants are identical, two when they differ).
type countingPrefixFinder struct {
	byPrefix map[string][]sgo.Person
	calls    *int
}

func (s countingPrefixFinder) FindByNamePrefix(_ context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error) {
	*s.calls++
	return s.byPrefix[firstNamePrefix+" "+lastNamePrefix], nil
}

func (s countingPrefixFinder) FindByLastNameSuffix(_ context.Context, _ string, _ string) ([]sgo.Person, error) {
	return nil, nil
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
		// Query key is shortPrefix(normalize("Maik"),3)+" "+shortPrefix(normalize("Lander"),3) —
		// queryRecordsByPrefix normalizes (lowercases) each name part before
		// taking its 3-rune prefix, so the key is lowercase even though the
		// candidate string itself is "Maik"/"Lander".
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"mai lan": {maik},
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
		// "m" and "l" are already <=3 runes, so shortPrefix leaves them
		// unchanged after normalize() lowercases them.
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"m l": {maik},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	// matchQuality("M","Maik"): "M" is a true prefix of "Maik", so
	// fuzzyRatio = len("M")/len("Maik") = 0.25 (no phonetic bonus: "m" and
	// "maik" encode differently). matchQuality("L","Lander") = 1/6 =
	// 0.1667 by the same true-prefix reasoning (no phonetic bonus either).
	// stageScore = 0.35*0.25 + 0.65*0.1667 = 0.1958 -> stagePoints=int(50*0.1958+0.5)=10.
	require.Equal(t, 10, got[0].StagePoints)
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
			"m l": {maik, max},
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

func TestMatcher_Match_LastNameHint_ReordersAmbiguousCandidates(t *testing.T) {
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
		// "max" before "maik" so that on a stagePoints tie (both score 10
		// with initials-only input), stable sort preserves "Max Lehmann" first.
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"m l": {max, maik},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	// Without a hint, both candidates score stagePoints=10 with initials-only
	// input; "Max Lehmann" leads by insertion order in the stub result.
	withoutHint, err := m.Match(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, withoutHint, 2)
	require.Equal(t, "Max Lehmann", withoutHint[0].Name)

	// With a "Land" hint, "Maik Lander" gets +10 stagePoints (10->20) and
	// takes the lead: 20+30+10=60 vs Lehmann's unchanged 10+30+10=50.
	q.LastNameHint = "Land"
	withHint, err := m.Match(context.Background(), q)
	require.NoError(t, err)
	require.Len(t, withHint, 2)
	require.Equal(t, "Maik Lander", withHint[0].Name)
	require.Equal(t, 20, withHint[0].StagePoints)
	require.Equal(t, 60, withHint[0].Confidence)
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
		Msgraph:     stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{"mai lan": {ms}}},
		Valuemation: stubValuemationPrefixFinder{byPrefix: map[string][]bardioc.ValuemationPerson{"mai lan": {vm}}},
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
	persons["m l"] = all
	m := &Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: persons}}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "M"},
		LastName:  STTResult{ResultRaw: "L"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 5)
}

func TestMatcher_Match_LargeCandidatePool_BestMatchSurvivesTopCaps(t *testing.T) {
	good := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-good")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	var pool []sgo.Person
	pool = append(pool, good)
	for i := 0; i < 19; i++ {
		pool = append(pool, sgo.Person{
			Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID(fmt.Sprintf("ms-bad-%d", i))}},
			FirstName: "Maik",
			LastName:  "Zzzzzzzzzz",
		})
	}
	m := &Matcher{Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
		"mai lan": pool,
	}}}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultRaw: "Lander"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 5) // outer top-5 cap, even though 20 candidates were queried
	// The best match survives both the top-15-per-representation cap and
	// the outer top-5 cap, ranking first.
	require.Equal(t, "Maik Lander", got[0].Name)
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
			"pau hop": {schleich, almato},
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

func TestMatcher_Match_TypoFoundViaPhoneticFuzzyPipeline(t *testing.T) {
	sellner := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Thomas",
		LastName:  "Sellner",
	}
	m := &Matcher{
		// shortPrefix(normalize("Thomas"),3)+" "+shortPrefix(normalize("Selner"),3)
		// = "tho sel" (normalize lowercases before the prefix is taken).
		// Unlike the old two-stage design, there is no separate exact key
		// to worry about — this IS the only query the pipeline ever runs.
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			"tho sel": {sellner},
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
	// matchQuality("Thomas","Thomas")=1.0. matchQuality("Selner","Sellner"):
	// fuzzyRatio=1-1/7=0.8571, both encode to Kölner Phonetik "8567" so
	// +0.35 phonetic bonus -> 1.2071, clamped to 1.0. Both qualities 1.0 ->
	// stageScore=1.0 -> stagePoints=int(50*1.0+0.5)=50.
	require.Equal(t, 50, got[0].StagePoints)
	require.Equal(t, 30, got[0].RepresentationPoints) // 1/1 usable representation ("raw") agreed
	require.Equal(t, 20, got[0].UniquenessPoints)
	require.Equal(t, 100, got[0].Confidence)
}

func TestMatcher_Match_UmlautMismatch_STTTransliteratedFindsStoredUmlautName(t *testing.T) {
	mueller := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Müller",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{byPrefix: map[string][]sgo.Person{
			// STT "Müller" normalizes to variant B "muller" -> shortPrefix 3
			// = "mul". Registering ONLY under the variant-B key proves the
			// fix's second query (not just the first, variant-A one) is
			// what finds this record.
			"mai mul": {mueller},
		}},
	}
	q := NameMatchQuery{
		FirstName: STTResult{ResultRaw: "Maik"},
		LastName:  STTResult{ResultRaw: "Müller"},
	}

	got, err := m.Match(context.Background(), q)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Maik Müller", got[0].Name)
}

func TestMatcher_QueryRecordsByPrefix_NoUmlaut_QueriesOnce(t *testing.T) {
	calls := 0
	m := &Matcher{
		Msgraph: countingPrefixFinder{
			calls: &calls,
			byPrefix: map[string][]sgo.Person{
				"mai lan": {{
					Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
					FirstName: "Maik",
					LastName:  "Lander",
				}},
			},
		},
	}

	records, err := m.queryRecordsByPrefix(context.Background(), "Maik", "Lander", nil)

	require.NoError(t, err)
	require.Len(t, records, 1)
	// "Maik" aliases "Mike" (mik) and "Marek" (mar), both differ from own prefix "mai",
	// so the alias stage issues two FindByNamePrefix calls: 1 main + 2 aliases.
	require.Equal(t, 3, calls)
}

func TestMatcher_QueryRecordsByPrefix_UmlautMismatch_QueriesBothVariantsAndDedupes(t *testing.T) {
	mueller := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Müller",
	}
	calls := 0
	m := &Matcher{
		Msgraph: countingPrefixFinder{
			calls: &calls,
			byPrefix: map[string][]sgo.Person{
				// STT text "Müller" normalizes to variant A "mueller" (prefix
				// "mue") and variant B "muller" (prefix "mul") -- these
				// differ, so both are queried. Registering the same record
				// under both keys simulates a real prefix search returning
				// it either way, so this test proves dedup collapses the
				// two hits into one.
				"mai mue": {mueller},
				"mai mul": {mueller},
			},
		},
	}

	records, err := m.queryRecordsByPrefix(context.Background(), "Maik", "Müller", nil)

	require.NoError(t, err)
	require.Len(t, records, 1) // deduped by record ID despite matching both variant queries
	// variants differ ("mueller" vs "muller") -> 2 prefix queries, plus 2 alias queries
	// for "Mike" (mik) and "Marek" (mar) (firstNameAliasPrefixes("maik") = ["mik","mar"]) -> 4 total.
	require.Equal(t, 4, calls)
}

func TestMatcher_QueryRecordsByPrefix_WrongFirstLetter_FoundViaSuffix(t *testing.T) {
	// STT heard "Fellner" but the correct name is "Sellner".
	// F and S are not in the same phonetic group, so the prefix query ("fel")
	// would return nothing. The suffix query ("ellner") uses the pattern
	// ".ellner.*" which matches "Sellner" and any other name with that suffix.
	sellner := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Thomas",
		LastName:  "Sellner",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{
			byPrefix: map[string][]sgo.Person{},
			// normalize("Fellner") = "fellner", runes[1:] = "ellner"
			// firstPrefix = shortPrefix(normalize("Thomas"), 3) = "tho"
			bySuffix: map[string][]sgo.Person{
				"tho ellner": {sellner},
			},
		},
	}

	records, err := m.queryRecordsByPrefix(context.Background(), "Thomas", "Fellner", nil)

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "ms:ms-1", records[0].recordID())
}

func TestMatcher_QueryRecordsByPrefix_FirstNameAlias_FoundViaMaik(t *testing.T) {
	// STT said "Mike" but the stored name is "Maik".
	// normalize("Mike") = "mike"; firstNameAliasPrefixes("mike") = ["mai"].
	// The alias query uses prefix "mai" for firstName and "lan" for lastName.
	// The main prefix query ("mik" + "lan") returns nothing; the alias query finds the record.
	maik := sgo.Person{
		Entity:    graph.Entity{Metadata: &graph.Metadata{ID: graph.MetadataID("ms-1")}},
		FirstName: "Maik",
		LastName:  "Lander",
	}
	m := &Matcher{
		Msgraph: stubMsgraphPrefixFinder{
			byPrefix: map[string][]sgo.Person{
				"mai lan": {maik},
			},
		},
	}

	records, err := m.queryRecordsByPrefix(context.Background(), "Mike", "Lander", nil)

	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "ms:ms-1", records[0].recordID())
}
