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
}

// NameMatchCandidate is one resolved person with its confidence breakdown,
// returned by Matcher.Match.
type NameMatchCandidate struct {
	Name                 string
	FirstName            string
	LastName             string
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

// specificity is how much of matchedName the candidate string specifies,
// as a length ratio clamped to 1.0 (e.g. "M" against "Maik" is 0.25;
// "Maik" against "Maik" is 1.0). Returns 0 if matchedName is empty.
func specificity(candidate, matchedName string) float64 {
	matchedLen := len([]rune(matchedName))
	if matchedLen == 0 {
		return 0
	}
	ratio := float64(len([]rune(candidate))) / float64(matchedLen)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// stagePoints scores how specific/complete a matched name was: 20 (both
// initials) to 50 (both full names).
func stagePoints(firstSpecificity, lastSpecificity float64) int {
	stageScore := (firstSpecificity + lastSpecificity) / 2
	return int(20 + 30*stageScore + 0.5) // round half up; inputs are always >= 0
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
// specificity scoring.
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
// (one representation type), with the specificity of the match that found
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
// representative's actual name.
func groupRecords(records []personRecord, rtype representationType, firstCandidate, lastCandidate string) []personGroup {
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

		groups = append(groups, personGroup{
			records:     members,
			recordIDs:   ids,
			stagePoints: stagePoints(specificity(firstCandidate, firstName), specificity(lastCandidate, lastName)),
			repType:     rtype,
		})
	}
	return groups
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

// fuzzyStagePointsMalus is subtracted from a fuzzy-matched group's
// stagePoints, so a candidate found only via edit-distance correction
// always ranks below an exact prefix match of equivalent specificity.
const fuzzyStagePointsMalus = 15

// fuzzyStagePointsFloor mirrors stagePoints' documented minimum (see
// stagePoints) — the malus never pushes a fuzzy match below it.
const fuzzyStagePointsFloor = 20

// applyFuzzyMalus returns groups with stagePoints reduced by
// fuzzyStagePointsMalus, floored at fuzzyStagePointsFloor. Used only for
// groups found via Match's fuzzy fallback, never for exact-prefix groups.
func applyFuzzyMalus(groups []personGroup) []personGroup {
	out := make([]personGroup, len(groups))
	for i, g := range groups {
		g.stagePoints -= fuzzyStagePointsMalus
		if g.stagePoints < fuzzyStagePointsFloor {
			g.stagePoints = fuzzyStagePointsFloor
		}
		out[i] = g
	}
	return out
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

	return NameMatchCandidate{
		Name:                 strings.TrimSpace(firstName + " " + lastName),
		FirstName:            firstName,
		LastName:             lastName,
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
}

// ValuemationPersonPrefixFinder is the Valuemation-side equivalent.
type ValuemationPersonPrefixFinder interface {
	FindByNamePrefix(ctx context.Context, firstNamePrefix, lastNamePrefix string) ([]bardioc.ValuemationPerson, error)
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

// fuzzyQueryRecords widens firstName/lastName to their first 3 runes and
// re-runs queryRecords, then keeps only records whose actual name is
// within fuzzyThreshold edit distance of the original (un-widened)
// candidate strings on both name parts. Returns (nil, nil) without
// querying anything if either name part is shorter than
// minFuzzyNameLength.
func (m *Matcher) fuzzyQueryRecords(ctx context.Context, firstName, lastName string, domains []string) ([]personRecord, error) {
	if len([]rune(firstName)) < minFuzzyNameLength || len([]rune(lastName)) < minFuzzyNameLength {
		return nil, nil
	}

	widened, err := m.queryRecords(ctx, shortPrefix(firstName, 3), shortPrefix(lastName, 3), domains)
	if err != nil {
		return nil, err
	}

	firstThreshold := fuzzyThreshold(firstName)
	lastThreshold := fuzzyThreshold(lastName)

	var filtered []personRecord
	for _, r := range widened {
		var recFirst, recLast string
		switch {
		case r.ms != nil:
			recFirst, recLast = r.ms.FirstName, r.ms.LastName
		case r.vm != nil:
			recFirst, recLast = r.vm.FirstName, r.vm.LastName
		}
		if levenshtein(firstName, recFirst) <= firstThreshold && levenshtein(lastName, recLast) <= lastThreshold {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// Match scores every distinct person found across all usable STT
// representation types, returning up to 5 candidates sorted by descending
// confidence. It returns ErrNoNameData if q has no usable data at all.
//
// If the exact-prefix search finds nothing at all, a fuzzy (edit-distance)
// fallback retries with a widened prefix per candidate and applies a
// malus to the resulting stagePoints — see fuzzyQueryRecords and
// applyFuzzyMalus. The fallback never runs when the exact search already
// found something.
func (m *Matcher) Match(ctx context.Context, q NameMatchQuery) ([]NameMatchCandidate, error) {
	candidates := extractCandidates(q)
	if len(candidates) == 0 {
		return nil, ErrNoNameData
	}

	var allGroups [][]personGroup
	for _, c := range candidates {
		records, err := m.queryRecords(ctx, c.firstName, c.lastName, q.Domains)
		if err != nil {
			return nil, err
		}
		allGroups = append(allGroups, groupRecords(records, c.rtype, c.firstName, c.lastName))
	}

	finals := mergeGroups(allGroups)

	if len(finals) == 0 {
		var fuzzyGroups [][]personGroup
		for _, c := range candidates {
			records, err := m.fuzzyQueryRecords(ctx, c.firstName, c.lastName, q.Domains)
			if err != nil {
				return nil, err
			}
			fuzzyGroups = append(fuzzyGroups, applyFuzzyMalus(groupRecords(records, c.rtype, c.firstName, c.lastName)))
		}
		finals = mergeGroups(fuzzyGroups)
	}

	total := len(candidates)
	uPoints := uniquenessPoints(len(finals))

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
