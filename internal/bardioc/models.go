package bardioc

import (
	servicemanagement "bitbucket.org/almatoag/graph-go/NTO/ServiceManagement"
	"bitbucket.org/almatoag/graph-go/SGO/sgo"
)

// Ticket extends servicemanagement.Ticket with Valuemation-specific custom
// fields that are not part of the OGIT ontology (hence the "/" prefix).
type Ticket struct {
	servicemanagement.Ticket
	HLQ1Value    string `json:"/hlq1Value,omitempty"`
	DateFinished string `json:"/dateFinished,omitempty"`
	KnownError   string `json:"/knownerror,omitempty"`
}

// OgitType delegates to the embedded Ticket so graph scanning works correctly.
func (t Ticket) OgitType() string {
	return t.Ticket.OgitType()
}

// ValuemationPerson mirrors the subset of hiro-conn-valuemation's Person
// node this service needs to read. It cannot import that connector's
// internal package, so the shape is redefined here.
type ValuemationPerson struct {
	sgo.Person
	PhoneNo string `json:"/phoneNo,omitempty"`
}

// OgitType matches the graph vertex type shared by every Person node
// (both MSGraph- and Valuemation-sourced).
func (p ValuemationPerson) OgitType() string {
	return p.Person.OgitType()
}

// MsgraphAccount mirrors the subset of hiro-conn-msgraph's Account node this
// service needs to read, extending sgo.Account with the status attribute
// (a Bardioc-internal property, not part of the OGIT ontology, hence the
// "/status" key rather than "ogit/status" — same convention as /pFlag).
type MsgraphAccount struct {
	sgo.Account
	Status string `json:"/status,omitempty"`
}

// OgitType matches the graph vertex type of every Account node.
func (a MsgraphAccount) OgitType() string {
	return a.Account.OgitType()
}

const (
	// PFlagMsgraph identifies Person nodes written by hiro-conn-msgraph.
	PFlagMsgraph = "msGraphConnector"
	// PFlagValuemation identifies Person nodes written by hiro-conn-valuemation.
	PFlagValuemation = "valuemation-connector-2025"
)
