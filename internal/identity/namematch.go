// internal/identity/namematch.go
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
)

// STTResult is one Aristech STT recognition result for a single name part
// (first or last name), as documented in
// docs/AST-Spracherkennung-STT.odt.
type STTResult struct {
	ResultRaw        string
	ResultTagged     string
	ResultSlotted    string
	ResultNlp        string
	ResultStructured string
}

// NameMatchQuery is the input to Matcher.Match.
type NameMatchQuery struct {
	FirstName STTResult
	LastName  STTResult
	Domains   []string
	// LastNameHint is an optional caller-supplied prefix of the last name
	// (e.g. its first 4 letters), re-asked by Aristech's own dialog logic
	// when the initial match was ambiguous or empty. Boosts matching
	// candidates' stagePoints (see hintMatches); never filters
	// non-matching candidates out.
	LastNameHint string
}

// NameMatchCandidate is one resolved person with its confidence breakdown,
// returned by Matcher.Match.
type NameMatchCandidate struct {
	Name                 string
	FirstName            string
	LastName             string
	FirstNamePhonetic    string
	LastNamePhonetic     string
	ValuemationPersonXID string
	MsgraphPersonXID     string
	Confidence           int
	StagePoints          int
	RepresentationPoints int
	UniquenessPoints     int
}

// representationType identifies one of the five text representations the
// Aristech STT module can produce for a name part.
type representationType string

const (
	repRaw        representationType = "raw"
	repNlp        representationType = "nlp"
	repTagged     representationType = "tagged"
	repSlotted    representationType = "slotted"
	repStructured representationType = "structured"
)

var allRepresentationTypes = []representationType{repRaw, repNlp, repTagged, repSlotted, repStructured}

var tagRe = regexp.MustCompile(`<[^>]*>`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

// stripTags removes XML/tag markers, e.g.
// "<firstname>Maik</firstname>" -> "Maik".
func stripTags(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// structuredWord is one element of a result_structured JSON array, per
// docs/AST-Spracherkennung-STT.odt. Only the field this endpoint needs is
// decoded.
type structuredWord struct {
	Word string `json:"word"`
}

// parseStructuredWords parses a result_structured JSON array into its word
// strings, joined with spaces. Returns ok=false if raw is empty, not valid
// JSON, or has no non-empty words — this is not a hard error; the caller
// simply treats this representation type as having no data.
func parseStructuredWords(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	var words []structuredWord
	if err := json.Unmarshal([]byte(raw), &words); err != nil {
		return "", false
	}
	parts := make([]string, 0, len(words))
	for _, w := range words {
		if w.Word != "" {
			parts = append(parts, w.Word)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " "), true
}

// candidateString derives the plain-text candidate for one representation
// type from an STTResult. ok is false when that type has no usable data.
func candidateString(rt representationType, r STTResult) (string, bool) {
	switch rt {
	case repRaw:
		s := strings.TrimSpace(r.ResultRaw)
		return s, s != ""
	case repNlp:
		s := strings.TrimSpace(r.ResultNlp)
		return s, s != ""
	case repTagged:
		s := stripTags(r.ResultTagged)
		return s, s != ""
	case repSlotted:
		s := stripTags(r.ResultSlotted)
		return s, s != ""
	case repStructured:
		return parseStructuredWords(r.ResultStructured)
	default:
		return "", false
	}
}

// usableRepresentation is one representation type where both name parts
// produced a non-empty candidate string.
type usableRepresentation struct {
	rtype     representationType
	firstName string
	lastName  string
}

// extractCandidates returns one usableRepresentation per representation
// type that has data on both q.FirstName and q.LastName. A type missing
// data on either side is skipped entirely — it does not count as
// "checked and disagreed", it counts as "not available".
func extractCandidates(q NameMatchQuery) []usableRepresentation {
	var out []usableRepresentation
	for _, rt := range allRepresentationTypes {
		first, firstOK := candidateString(rt, q.FirstName)
		last, lastOK := candidateString(rt, q.LastName)
		if firstOK && lastOK {
			out = append(out, usableRepresentation{rtype: rt, firstName: first, lastName: last})
		}
	}
	return out
}

// matchQuality returns a 0-1 score for how well sttText matches
// storedName, combining fuzzy edit distance (checked across all four
// combinations of the two normalization variants — see normalize) with a
// Kölner Phonetik bonus. This is what feeds stagePoints below, replacing
// the old pure length-ratio specificity check: it also rewards
// phonetically-similar spellings (e.g. STT text "Meyer" against a stored
// "Maier"), not just prefix completeness.
func matchQuality(sttText, storedName string) float64 {
	sttA, sttB := normalize(sttText)
	nameA, nameB := normalize(storedName)

	quality := bestFuzzyRatio(sttA, sttB, nameA, nameB)

	if sttB != "" && nameB != "" && koelnerPhonetik(sttB) == koelnerPhonetik(nameB) {
		quality += phoneticBonus
	}

	if quality > 1 {
		quality = 1
	}
	if quality < 0 {
		quality = 0
	}
	return quality
}

// phoneticBonus is added to matchQuality when two name parts' Kölner
// Phonetik codes are equal.
const phoneticBonus = 0.35

// bestFuzzyRatio computes fuzzyRatio for all four combinations of
// {sttA, sttB} × {nameA, nameB} and returns the highest ratio found.
func bestFuzzyRatio(sttA, sttB, nameA, nameB string) float64 {
	best := 0.0
	for _, stt := range [2]string{sttA, sttB} {
		for _, name := range [2]string{nameA, nameB} {
			if r := fuzzyRatio(stt, name); r > best {
				best = r
			}
		}
	}
	return best
}

// fuzzyRatio returns 1 - levenshtein(a,b)/maxLen(a,b), clamped to [0,1].
// Returns 0 if both strings are empty (maxLen would be 0).
func fuzzyRatio(a, b string) float64 {
	maxLen := len([]rune(a))
	if bl := len([]rune(b)); bl > maxLen {
		maxLen = bl
	}
	if maxLen == 0 {
		return 0
	}
	ratio := 1 - float64(levenshtein(a, b))/float64(maxLen)
	if ratio < 0 {
		ratio = 0
	}
	return ratio
}

// hintMatches reports whether storedLastName's normalized form (variant A
// or B) starts with hint's equally normalized form. Used only as a
// stagePoints boost (see groupRecords), never as a filter. An empty hint
// never matches.
func hintMatches(hint, storedLastName string) bool {
	if strings.TrimSpace(hint) == "" {
		return false
	}
	hintA, hintB := normalize(hint)
	nameA, nameB := normalize(storedLastName)
	return (hintA != "" && strings.HasPrefix(nameA, hintA)) || (hintB != "" && strings.HasPrefix(nameB, hintB))
}

// lastNameHintBoost is added to a candidate's stagePoints when
// hintMatches its stored last name.
const lastNameHintBoost = 10

// stagePoints scores how specific/complete a matched name was: 0 (no
// similarity) to 50 (both full, well-matching names). Last name carries 65%
// of the weight (first name 35%) — last names are more distinctive and STT
// errors in first names (transpositions, similar sounds) are more forgivable.
func stagePoints(firstQuality, lastQuality float64) int {
	stageScore := firstQuality*0.35 + lastQuality*0.65
	return int(50*stageScore + 0.5) // round half up; inputs are always >= 0
}

// representationPoints scores how many of the available STT
// representations agreed on the same candidate: 0-30.
func representationPoints(hits, total int) int {
	if total == 0 {
		return 0
	}
	ratio := float64(hits) / float64(total)
	switch {
	case ratio >= 1.0:
		return 30
	case ratio >= 0.75:
		return 20
	case ratio >= 0.5:
		return 10
	default:
		return 0
	}
}

// uniquenessPoints scores how many distinct people a whole match run
// found: 20 for a single unambiguous result, down to 0 for 3 or more.
func uniquenessPoints(distinctPeople int) int {
	switch distinctPeople {
	case 1:
		return 20
	case 2:
		return 10
	default:
		return 0
	}
}

// personRecord is one MSGraph or Valuemation Person candidate, wrapped so
// clusterIndices can group them together regardless of source. Exactly one
// of ms/vm is non-nil.
type personRecord struct {
	ms *sgo.Person
	vm *bardioc.ValuemationPerson
}

// recordID returns a stable identifier for a record's underlying graph
// node, used to detect when two separately-built groups (from different
// representation types) actually share a member.
func (r personRecord) recordID() string {
	if r.ms != nil {
		return "ms:" + r.ms.Metadata.ID.String()
	}
	return "vm:" + r.vm.Metadata.ID.String()
}

// recordsLinked reports whether a and b are the same real person, reusing
// the same-source clustering rules from dedup.go and the cross-source
// samePerson check from resolver.go.
func recordsLinked(a, b personRecord) bool {
	switch {
	case a.ms != nil && b.ms != nil:
		return sameEmail(a.ms.Email, b.ms.Email) || sharesAny(msgraphPhones(*a.ms), msgraphPhones(*b.ms))
	case a.vm != nil && b.vm != nil:
		return sameEmail(a.vm.Email, b.vm.Email) ||
			(isUsablePhone(a.vm.PhoneNo) && isUsablePhone(b.vm.PhoneNo) && a.vm.PhoneNo == b.vm.PhoneNo)
	case a.ms != nil && b.vm != nil:
		return samePerson(*a.ms, *b.vm)
	case a.vm != nil && b.ms != nil:
		return samePerson(*b.ms, *a.vm)
	default:
		return false
	}
}

// groupRepresentatives picks the best MSGraph and/or Valuemation record
// from members (reusing the existing tie-break rules), for display and
// scoring.
func groupRepresentatives(members []personRecord) (msRep *sgo.Person, vmRep *bardioc.ValuemationPerson) {
	var msMembers []sgo.Person
	var vmMembers []bardioc.ValuemationPerson
	for _, m := range members {
		if m.ms != nil {
			msMembers = append(msMembers, *m.ms)
		}
		if m.vm != nil {
			vmMembers = append(vmMembers, *m.vm)
		}
	}
	if len(msMembers) > 0 {
		rep := msgraphRepresentative(msMembers)
		msRep = &rep
	}
	if len(vmMembers) > 0 {
		rep := valuemationRepresentative(vmMembers)
		vmRep = &rep
	}
	return msRep, vmRep
}

// personGroup is one distinct real person found by a single prefix query
// (one representation type), with the stagePoints of the match that found
// them.
type personGroup struct {
	records     []personRecord
	recordIDs   map[string]bool
	stagePoints int
	repType     representationType
}

// groupRecords clusters records (all persons returned by one
// representation type's prefix query, across both sources) into distinct
// real people, scoring each group's stagePoints against its own best
// representative's actual name. lastNameHint, if non-empty, boosts a
// group's stagePoints when it matches (see hintMatches) — it never
// filters groups out.
func groupRecords(records []personRecord, rtype representationType, firstCandidate, lastCandidate, lastNameHint string) []personGroup {
	linked := func(i, j int) bool { return recordsLinked(records[i], records[j]) }
	clusters := clusterIndices(len(records), linked)

	groups := make([]personGroup, 0, len(clusters))
	for _, idxs := range clusters {
		members := make([]personRecord, 0, len(idxs))
		for _, idx := range idxs {
			members = append(members, records[idx])
		}

		msRep, vmRep := groupRepresentatives(members)
		var firstName, lastName string
		switch {
		case msRep != nil:
			firstName, lastName = msRep.FirstName, msRep.LastName
		case vmRep != nil:
			firstName, lastName = vmRep.FirstName, vmRep.LastName
		}

		ids := map[string]bool{}
		for _, m := range members {
			ids[m.recordID()] = true
		}

		sp := stagePoints(matchQuality(firstCandidate, firstName), matchQuality(lastCandidate, lastName))
		if hintMatches(lastNameHint, lastName) {
			sp += lastNameHintBoost
			if sp > 50 {
				sp = 50
			}
		}

		groups = append(groups, personGroup{
			records:     members,
			recordIDs:   ids,
			stagePoints: sp,
			repType:     rtype,
		})
	}
	return groups
}

// topByStagePoints returns the n highest-stagePoints groups from groups,
// sorted descending. If groups has n or fewer entries, all are returned
// (still sorted).
func topByStagePoints(groups []personGroup, n int) []personGroup {
	sorted := make([]personGroup, len(groups))
	copy(sorted, groups)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].stagePoints > sorted[j].stagePoints })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// finalCluster accumulates evidence for one distinct real person across
// every representation type's query.
type finalCluster struct {
	recordIDs   map[string]bool
	records     []personRecord
	repTypes    map[representationType]bool
	stagePoints int // highest stagePoints seen across contributing queries
}

func newFinalCluster(g personGroup) *finalCluster {
	fc := &finalCluster{
		recordIDs: map[string]bool{},
		repTypes:  map[representationType]bool{},
	}
	fc.merge(g)
	return fc
}

func (fc *finalCluster) merge(g personGroup) {
	for id := range g.recordIDs {
		fc.recordIDs[id] = true
	}
	fc.records = append(fc.records, g.records...)
	fc.repTypes[g.repType] = true
	if g.stagePoints > fc.stagePoints {
		fc.stagePoints = g.stagePoints
	}
}

// mergeGroups folds every representation type's person groups into final
// clusters using transitive closure over shared record IDs, via the same
// clusterIndices union-find used elsewhere in this package. Transitive
// closure matters because recordsLinked itself need not be transitive —
// a duplicate record can bridge two others via different identifying
// fields (e.g. shared email vs. shared phone) — and merging into only the
// first matching final would leave such bridges unresolved, producing
// two finals that both claim the same underlying record.
func mergeGroups(allGroups [][]personGroup) []*finalCluster {
	var flat []personGroup
	for _, groups := range allGroups {
		flat = append(flat, groups...)
	}
	if len(flat) == 0 {
		return nil
	}

	linked := func(i, j int) bool {
		for id := range flat[i].recordIDs {
			if flat[j].recordIDs[id] {
				return true
			}
		}
		return false
	}
	clusters := clusterIndices(len(flat), linked)

	finals := make([]*finalCluster, 0, len(clusters))
	for _, idxs := range clusters {
		fc := newFinalCluster(flat[idxs[0]])
		for _, idx := range idxs[1:] {
			fc.merge(flat[idx])
		}
		finals = append(finals, fc)
	}
	return finals
}

// candidateFromCluster builds the final scored NameMatchCandidate for one
// distinct person, given the representationPoints and uniquenessPoints
// already computed for the whole match run.
func candidateFromCluster(fc *finalCluster, representationPts, uniquenessPts int) NameMatchCandidate {
	msRep, vmRep := groupRepresentatives(fc.records)

	var firstName, lastName, msgraphXID, valuemationXID string
	if msRep != nil {
		firstName, lastName = msRep.FirstName, msRep.LastName
		msgraphXID = externalID(msRep.XID, msRep.Metadata.ID)
	}
	if vmRep != nil {
		if firstName == "" {
			firstName, lastName = vmRep.FirstName, vmRep.LastName
		}
		valuemationXID = externalID(vmRep.XID, vmRep.Metadata.ID)
	}

	_, firstB := normalize(firstName)
	_, lastB := normalize(lastName)

	return NameMatchCandidate{
		Name:                 strings.TrimSpace(firstName + " " + lastName),
		FirstName:            firstName,
		LastName:             lastName,
		FirstNamePhonetic:    koelnerPhonetik(firstB),
		LastNamePhonetic:     koelnerPhonetik(lastB),
		ValuemationPersonXID: valuemationXID,
		MsgraphPersonXID:     msgraphXID,
		StagePoints:          fc.stagePoints,
		RepresentationPoints: representationPts,
		UniquenessPoints:     uniquenessPts,
		Confidence:           fc.stagePoints + representationPts + uniquenessPts,
	}
}

// ErrNoNameData means neither name part had any usable STT recognition
// data across all five representation types.
var ErrNoNameData = errors.New("identity: no name recognition data supplied")

// MsgraphPersonPrefixFinder looks up MSGraph-sourced Person nodes whose
// first and last name start with the given prefixes.
type MsgraphPersonPrefixFinder interface {
	FindByNamePrefix(ctx context.Context, firstNamePrefix, lastNamePrefix string) ([]sgo.Person, error)
	// FindByLastNameSuffix finds persons whose first name starts with
	// firstNamePrefix and whose last name matches any single leading character
	// followed by lastNameSuffix. Used to catch first-letter STT errors.
	FindByLastNameSuffix(ctx context.Context, firstNamePrefix, lastNameSuffix string) ([]sgo.Person, error)
}

// ValuemationPersonPrefixFinder is the Valuemation-side equivalent.
type ValuemationPersonPrefixFinder interface {
	FindByNamePrefix(ctx context.Context, firstNamePrefix, lastNamePrefix string) ([]bardioc.ValuemationPerson, error)
	// FindByLastNameSuffix finds persons whose first name starts with
	// firstNamePrefix and whose last name matches any single leading character
	// followed by lastNameSuffix. Used to catch first-letter STT errors.
	FindByLastNameSuffix(ctx context.Context, firstNamePrefix, lastNameSuffix string) ([]bardioc.ValuemationPerson, error)
}

// Matcher scores candidate persons for a possibly-incomplete STT name
// recognition result. Unlike Resolver, it never errors on ambiguity — it
// always returns its best guesses, each with a confidence score, so the
// caller (the voicebot) can decide what to do next. Either field may be
// nil in tests that only need one source.
type Matcher struct {
	Msgraph     MsgraphPersonPrefixFinder
	Valuemation ValuemationPersonPrefixFinder
}

// queryRecords runs one prefix query per configured source and wraps the
// (domain-filtered) results as personRecords.
func (m *Matcher) queryRecords(ctx context.Context, firstPrefix, lastPrefix string, domains []string) ([]personRecord, error) {
	var records []personRecord

	if m.Msgraph != nil {
		msPersons, err := m.Msgraph.FindByNamePrefix(ctx, firstPrefix, lastPrefix)
		if err != nil {
			return nil, fmt.Errorf("msgraph name-prefix lookup: %w", err)
		}
		for _, p := range filterMsgraphPersonsByDomain(msPersons, domains) {
			records = append(records, personRecord{ms: &p})
		}
	}

	if m.Valuemation != nil {
		vmPersons, err := m.Valuemation.FindByNamePrefix(ctx, firstPrefix, lastPrefix)
		if err != nil {
			return nil, fmt.Errorf("valuemation name-prefix lookup: %w", err)
		}
		for _, p := range filterValuemationPersonsByDomain(vmPersons, domains) {
			records = append(records, personRecord{vm: &p})
		}
	}

	return records, nil
}

// topCandidatesPerRepresentation caps how many scored candidates from a
// single representation type's prefix query survive into the
// cross-representation merge, keeping the merge step's cost bounded even
// when a common 3-rune prefix matches many people.
const topCandidatesPerRepresentation = 15

// querySuffixRecords runs one suffix query per configured source: finds
// persons whose first name starts with firstPrefix and whose last name
// matches any single leading character followed by lastSuffix. This catches
// STT errors where only the first letter of the last name was mis-recognised
// (e.g. "Fellner" → suffix "ellner" also finds "Sellner").
func (m *Matcher) querySuffixRecords(ctx context.Context, firstPrefix, lastSuffix string, domains []string) ([]personRecord, error) {
	var records []personRecord

	if m.Msgraph != nil {
		msPersons, err := m.Msgraph.FindByLastNameSuffix(ctx, firstPrefix, lastSuffix)
		if err != nil {
			return nil, fmt.Errorf("msgraph last-name suffix lookup: %w", err)
		}
		for _, p := range filterMsgraphPersonsByDomain(msPersons, domains) {
			records = append(records, personRecord{ms: &p})
		}
	}

	if m.Valuemation != nil {
		vmPersons, err := m.Valuemation.FindByLastNameSuffix(ctx, firstPrefix, lastSuffix)
		if err != nil {
			return nil, fmt.Errorf("valuemation last-name suffix lookup: %w", err)
		}
		for _, p := range filterValuemationPersonsByDomain(vmPersons, domains) {
			records = append(records, personRecord{vm: &p})
		}
	}

	return records, nil
}

// queryRecordsByPrefix queries the graph using both of a name part's
// normalized variants' 3-rune prefixes, and unions the results by record
// ID. Most names have no umlaut/ß, so variant A and B are identical and
// only one query runs; when they differ (e.g. STT "Mueller" vs a stored
// "Müller"), a second query with the other variant's prefix catches
// records the first would have missed.
//
// Additionally, when the last name has 4+ runes, a suffix query runs:
// the last name minus its first rune (e.g. "Fellner" → "ellner") is sent
// as a wildcard-first-letter pattern, so "Sellner" is found even though F
// and S are not phonetically equivalent.
func (m *Matcher) queryRecordsByPrefix(ctx context.Context, firstName, lastName string, domains []string) ([]personRecord, error) {
	firstA, firstB := normalize(firstName)
	lastA, lastB := normalize(lastName)

	records, err := m.queryRecords(ctx, shortPrefix(firstA, 3), shortPrefix(lastA, 3), domains)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, r := range records {
		seen[r.recordID()] = true
	}

	// Second query for umlaut/ß normalization variants (e.g. STT "Mueller" vs stored "Müller")
	if firstA != firstB || lastA != lastB {
		moreRecords, err := m.queryRecords(ctx, shortPrefix(firstB, 3), shortPrefix(lastB, 3), domains)
		if err != nil {
			return nil, err
		}
		for _, r := range moreRecords {
			if !seen[r.recordID()] {
				seen[r.recordID()] = true
				records = append(records, r)
			}
		}
	}

	// Suffix query: catches STT errors where the first letter of the last name
	// is wrong. Only runs when the suffix is long enough (>= 3 runes) to avoid
	// returning overly broad results.
	if runes := []rune(lastA); len(runes) >= 4 {
		suffixRecords, err := m.querySuffixRecords(ctx, shortPrefix(firstA, 3), string(runes[1:]), domains)
		if err != nil {
			return nil, err
		}
		for _, r := range suffixRecords {
			if !seen[r.recordID()] {
				seen[r.recordID()] = true
				records = append(records, r)
			}
		}
	}

	// First-name alias queries: catches common spelling variants where the
	// STT produced a different but equivalent spelling (e.g. "Mike"→"Maik").
	// firstNameAliasPrefixes returns only prefixes that differ from firstA's
	// own prefix, so this never duplicates the main query.
	for _, aliasPrefix := range firstNameAliasPrefixes(firstA) {
		aliasRecords, err := m.queryRecords(ctx, aliasPrefix, shortPrefix(lastA, 3), domains)
		if err != nil {
			return nil, err
		}
		for _, r := range aliasRecords {
			if !seen[r.recordID()] {
				seen[r.recordID()] = true
				records = append(records, r)
			}
		}
	}

	// Last-name alias queries: catches common spelling variants of the last name
	// (e.g. "Meyer"→"Maier"/"Mayer"/"Meier").
	for _, aliasPrefix := range lastNameAliasPrefixes(lastA) {
		aliasRecords, err := m.queryRecords(ctx, shortPrefix(firstA, 3), aliasPrefix, domains)
		if err != nil {
			return nil, err
		}
		for _, r := range aliasRecords {
			if !seen[r.recordID()] {
				seen[r.recordID()] = true
				records = append(records, r)
			}
		}
	}

	return records, nil
}

// Match scores every distinct person found across all usable STT
// representation types, returning up to 5 candidates sorted by descending
// confidence. It returns ErrNoNameData if q has no usable data at all.
//
// For each usable representation, a prefix query (first 3 runes of each
// name part) always runs — there is no separate exact-vs-fuzzy fallback
// stage. Every returned candidate is scored by matchQuality (normalized
// fuzzy edit distance plus a Kölner Phonetik bonus), optionally boosted
// by q.LastNameHint, and the best topCandidatesPerRepresentation are kept
// per representation before merging across representation types. When a
// name part contains an umlaut or ß, both normalized variants' prefixes
// are queried and unioned (see queryRecordsByPrefix), so a spelling
// mismatch between STT output and the stored name doesn't prevent
// retrieval.
func (m *Matcher) Match(ctx context.Context, q NameMatchQuery) ([]NameMatchCandidate, error) {
	candidates := extractCandidates(q)
	if len(candidates) == 0 {
		return nil, ErrNoNameData
	}

	var allGroups [][]personGroup
	for _, c := range candidates {
		records, err := m.queryRecordsByPrefix(ctx, c.firstName, c.lastName, q.Domains)
		if err != nil {
			return nil, err
		}
		groups := groupRecords(records, c.rtype, c.firstName, c.lastName, q.LastNameHint)
		allGroups = append(allGroups, topByStagePoints(groups, topCandidatesPerRepresentation))
	}

	finals := mergeGroups(allGroups)
	total := len(candidates)

	// Uniqueness counts only candidates within 50% of the winner's stagePoints.
	// Weak matches (e.g. a fuzzy hit with only partial phonetic similarity) don't
	// reduce the winner's uniqueness score when the winner is clearly ahead.
	maxSP := 0
	for _, fc := range finals {
		if fc.stagePoints > maxSP {
			maxSP = fc.stagePoints
		}
	}
	competitive := 0
	for _, fc := range finals {
		if fc.stagePoints*2 >= maxSP {
			competitive++
		}
	}
	uPoints := uniquenessPoints(competitive)

	result := make([]NameMatchCandidate, 0, len(finals))
	for _, fc := range finals {
		rPoints := representationPoints(len(fc.repTypes), total)
		result = append(result, candidateFromCluster(fc, rPoints, uPoints))
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Confidence > result[j].Confidence })
	if len(result) > 5 {
		result = result[:5]
	}
	return result, nil
}
