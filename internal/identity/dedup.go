// internal/identity/dedup.go
package identity

import (
	"strings"

	"bitbucket.org/almatoag/graph-go/SGO/sgo"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
)

// isUsablePhone reports whether phone carries real matching signal. Empty
// string and "0" (an observed Valuemation placeholder for "no phone on
// file") do not count.
func isUsablePhone(phone string) bool {
	return phone != "" && phone != "0"
}

// sameEmail compares two emails case-insensitively; an empty value on
// either side never counts as a match.
func sameEmail(a, b string) bool {
	return a != "" && b != "" && strings.EqualFold(a, b)
}

// emailDomain returns the lowercase domain part of email (the substring
// after the last '@'), or "" if email is empty or has no '@'.
func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

// domainMatches reports whether domain (already lowercase, as returned by
// emailDomain) is one of domains (expected already lowercase). An empty
// domain never matches — it means the candidate's email couldn't be
// verified against any domain.
func domainMatches(domain string, domains []string) bool {
	if domain == "" {
		return false
	}
	for _, d := range domains {
		if domain == d {
			return true
		}
	}
	return false
}

// filterValuemationPersonsByDomain keeps only persons whose Email domain is
// in domains. An empty domains list means no filter — persons is returned
// unchanged.
func filterValuemationPersonsByDomain(persons []bardioc.ValuemationPerson, domains []string) []bardioc.ValuemationPerson {
	if len(domains) == 0 {
		return persons
	}
	var kept []bardioc.ValuemationPerson
	for _, p := range persons {
		if domainMatches(emailDomain(p.Email), domains) {
			kept = append(kept, p)
		}
	}
	return kept
}

// filterMsgraphPersonsByDomain is the MSGraph-side equivalent of
// filterValuemationPersonsByDomain.
func filterMsgraphPersonsByDomain(persons []sgo.Person, domains []string) []sgo.Person {
	if len(domains) == 0 {
		return persons
	}
	var kept []sgo.Person
	for _, p := range persons {
		if domainMatches(emailDomain(p.Email), domains) {
			kept = append(kept, p)
		}
	}
	return kept
}

func msgraphPhones(p sgo.Person) []string {
	var phones []string
	for _, ph := range []string{p.OfficePhone, p.MobilePhone, p.OtherPhone} {
		if isUsablePhone(ph) {
			phones = append(phones, ph)
		}
	}
	return phones
}

func sharesAny(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// clusterMsgraphPersons groups persons that represent the same real person.
// excludePhone must be true when the query itself searched by phone number:
// every candidate already shares that phone by construction in that case,
// so treating phone as a clustering signal would be tautological and would
// wrongly merge different people who happen to share e.g. an office line.
func clusterMsgraphPersons(persons []sgo.Person, excludePhone bool) [][]sgo.Person {
	linked := func(i, j int) bool {
		a, b := persons[i], persons[j]
		if sameEmail(a.Email, b.Email) {
			return true
		}
		if !excludePhone && sharesAny(msgraphPhones(a), msgraphPhones(b)) {
			return true
		}
		return false
	}

	groups := clusterIndices(len(persons), linked)
	clusters := make([][]sgo.Person, len(groups))
	for gi, idxs := range groups {
		for _, idx := range idxs {
			clusters[gi] = append(clusters[gi], persons[idx])
		}
	}
	return clusters
}

// clusterValuemationPersons is the Valuemation-side equivalent of
// clusterMsgraphPersons.
func clusterValuemationPersons(persons []bardioc.ValuemationPerson, excludePhone bool) [][]bardioc.ValuemationPerson {
	linked := func(i, j int) bool {
		a, b := persons[i], persons[j]
		if sameEmail(a.Email, b.Email) {
			return true
		}
		if !excludePhone && isUsablePhone(a.PhoneNo) && isUsablePhone(b.PhoneNo) && a.PhoneNo == b.PhoneNo {
			return true
		}
		return false
	}

	groups := clusterIndices(len(persons), linked)
	clusters := make([][]bardioc.ValuemationPerson, len(groups))
	for gi, idxs := range groups {
		for _, idx := range idxs {
			clusters[gi] = append(clusters[gi], persons[idx])
		}
	}
	return clusters
}

// msgraphFieldScore counts populated identifying fields, used to break
// representative-selection ties deterministically.
func msgraphFieldScore(p sgo.Person) int {
	score := 0
	for _, v := range []string{p.Email, p.OfficePhone, p.MobilePhone, p.OtherPhone, p.FirstName, p.LastName} {
		if v != "" {
			score++
		}
	}
	return score
}

func valuemationFieldScore(p bardioc.ValuemationPerson) int {
	score := 0
	for _, v := range []string{p.Email, p.PhoneNo, p.FirstName, p.LastName} {
		if v != "" {
			score++
		}
	}
	return score
}

// msgraphRepresentative picks one record from a cluster of duplicates:
// prefer a populated XID, then the most populated record, then the lowest
// node ID, so the choice is deterministic regardless of input order.
// cluster must be non-empty — guaranteed by clusterIndices, which never
// returns an empty group.
func msgraphRepresentative(cluster []sgo.Person) sgo.Person {
	best := cluster[0]
	for _, p := range cluster[1:] {
		if betterMsgraphCandidate(p, best) {
			best = p
		}
	}
	return best
}

func betterMsgraphCandidate(a, b sgo.Person) bool {
	aXID, bXID := a.XID != "", b.XID != ""
	if aXID != bXID {
		return aXID
	}
	aScore, bScore := msgraphFieldScore(a), msgraphFieldScore(b)
	if aScore != bScore {
		return aScore > bScore
	}
	return a.Metadata.ID.String() < b.Metadata.ID.String()
}

// valuemationRepresentative is the Valuemation-side equivalent of
// msgraphRepresentative. cluster must be non-empty — guaranteed by
// clusterIndices, which never returns an empty group.
func valuemationRepresentative(cluster []bardioc.ValuemationPerson) bardioc.ValuemationPerson {
	best := cluster[0]
	for _, p := range cluster[1:] {
		if betterValuemationCandidate(p, best) {
			best = p
		}
	}
	return best
}

func betterValuemationCandidate(a, b bardioc.ValuemationPerson) bool {
	aXID, bXID := a.XID != "", b.XID != ""
	if aXID != bXID {
		return aXID
	}
	aScore, bScore := valuemationFieldScore(a), valuemationFieldScore(b)
	if aScore != bScore {
		return aScore > bScore
	}
	return a.Metadata.ID.String() < b.Metadata.ID.String()
}
