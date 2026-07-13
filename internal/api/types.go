package api

// AuthInput is the query-parameter input for GET /api/v1/auth.
type AuthInput struct {
	Phone  string `query:"phone" doc:"Caller phone number, e.g. +491234567890"`
	Name   string `query:"name" doc:"Caller full name, e.g. 'Max Mustermann'"`
	Domain string `query:"domain" doc:"Comma-separated list of email domains to restrict matching to, e.g. 'almato.com' or 'almato.com,datagroup.com'"`
}

// AuthOutputBody is the JSON body returned by a successful authentication.
type AuthOutputBody struct {
	Token                    string `json:"token" doc:"Bearer token to use on GET /api/v1/tickets"`
	ExpiresIn                int    `json:"expiresIn" doc:"Token lifetime in seconds"`
	Name                     string `json:"name" doc:"Resolved caller display name"`
	ValuemationExternalID    string `json:"valuemationExternalId,omitempty" doc:"External ID of the resolved Valuemation person, present only if that source resolved"`
	MsgraphExternalID        string `json:"msgraphExternalId,omitempty" doc:"External ID of the resolved MSGraph person, present only if that source resolved"`
	MsgraphAccountExternalID string `json:"msgraphAccountExternalId,omitempty" doc:"External ID of the Account connected to the resolved MSGraph person, present only if such an account exists"`
	MsgraphAccountStatus     string `json:"msgraphAccountStatus,omitempty" doc:"Status of the Account connected to the resolved MSGraph person, present only if such an account exists"`
}

// AuthOutput wraps AuthOutputBody for huma.
type AuthOutput struct {
	Body AuthOutputBody
}

// TicketDTO is the subset of ticket data exposed to the voicebot.
type TicketDTO struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	CreatedAt   string `json:"createdAt"`
	ClosedAt    string `json:"closedAt"`
}

// TicketsInput is the header input for GET /api/v1/tickets.
type TicketsInput struct {
	Authorization string `header:"Authorization" doc:"Bearer <JWT>"`
}

// TicketsOutputBody is the JSON body returned by GET /api/v1/tickets.
type TicketsOutputBody struct {
	Tickets []TicketDTO `json:"tickets"`
}

// TicketsOutput wraps TicketsOutputBody for huma.
type TicketsOutput struct {
	Body TicketsOutputBody
}

// CreateIssueBody is the JSON body for POST /api/v1/issues.
type CreateIssueBody struct {
	IntentID   string            `json:"intentId,omitempty" doc:"Internal graph ID (ogit/_id) of the matched Intent; if set, its fixed system variables are merged in before variables"`
	Subject    string            `json:"subject,omitempty" doc:"ogit/subject for the created issue; defaults to variables[\"/subject\"], then to \"Voicebot-Anliegen\""`
	Variables  map[string]string `json:"variables,omitempty" doc:"Free-form /-prefixed attributes for the issue: user variables if intentId is set, otherwise the complete attribute set"`
	OriginNode string            `json:"originNode,omitempty" doc:"ogit/Automation/originNode: graph ID of the node this AutomationIssue originates from"`
	Scope      string            `json:"scope,omitempty" doc:"ogit/_scope override for this issue; if empty, the connection's default scope (BARDIOC_SCOPE) is used"`
}

// CreateIssueInput is the header+body input for POST /api/v1/issues.
type CreateIssueInput struct {
	Authorization string `header:"Authorization" doc:"Bearer <JWT>"`
	Body          CreateIssueBody
}

// CreateIssueOutputBody is the JSON body returned after creating an AutomationIssue.
type CreateIssueOutputBody struct {
	IssueID string `json:"issueId" doc:"Internal graph ID of the created AutomationIssue; poll GET /api/v1/issues/{issueId}/status with this"`
}

// CreateIssueOutput wraps CreateIssueOutputBody for huma.
type CreateIssueOutput struct {
	Body CreateIssueOutputBody
}

// IssueStatusInput is the header+path input for GET /api/v1/issues/{issueId}/status.
type IssueStatusInput struct {
	Authorization string `header:"Authorization" doc:"Bearer <JWT>"`
	IssueID       string `path:"issueId" doc:"Internal graph ID of the AutomationIssue, as returned by POST /api/v1/issues"`
}

// IssueStatusOutputBody is the JSON body returned by the status endpoint.
type IssueStatusOutputBody struct {
	Status string `json:"status" doc:"Current ogit/status: UNPROCESSED, PROCESSING, WAITING, RESOLVED, or STOPPED"`
}

// IssueStatusOutput wraps IssueStatusOutputBody for huma.
type IssueStatusOutput struct {
	Body IssueStatusOutputBody
}

// NameCandidate is one Aristech STT recognition result for a single name
// part (first or last name), as documented in
// docs/AST-Spracherkennung-STT.odt.
type NameCandidate struct {
	ResultType       string  `json:"resultType,omitempty" doc:"STT result type/mode metadata"`
	ResultRaw        string  `json:"resultRaw,omitempty" doc:"Unprocessed STT transcription"`
	ResultTagged     string  `json:"resultTagged,omitempty" doc:"Result after semantic-tag processing (grammar mode)"`
	ResultSlotted    string  `json:"resultSlotted,omitempty" doc:"Result including XML slots (slot graphs/grammars)"`
	ResultNlp        string  `json:"resultNlp,omitempty" doc:"NLP-server-processed result string"`
	ResultStructured string  `json:"resultStructured,omitempty" doc:"JSON array of {word,start,end,confidence,phones}"`
	ResultLanguage   string  `json:"resultLanguage,omitempty" doc:"Detected/configured recognition language"`
	Confidence       float64 `json:"confidence,omitempty" doc:"STT's own recognition confidence; not used in match scoring"`
}

// NameMatchBody is the JSON body for POST /api/v1/auth/match.
type NameMatchBody struct {
	FirstName       NameCandidate `json:"firstName"`
	LastName        NameCandidate `json:"lastName"`
	Domain          string        `json:"domain,omitempty" doc:"Comma-separated list of email domains to restrict matching to, same semantics as /auth's domain param"`
	TickerMessageID string        `json:"tickerMessageId,omitempty" doc:"Aristech ticker message ID, carried through for log correlation only"`
}

// NameMatchInput is the body input for POST /api/v1/auth/match.
type NameMatchInput struct {
	Body NameMatchBody
}

// NameMatchCandidateDTO is one resolved person with its confidence breakdown.
type NameMatchCandidateDTO struct {
	Name                  string `json:"name"`
	FirstName             string `json:"firstName"`
	LastName              string `json:"lastName"`
	ValuemationExternalID string `json:"valuemationExternalId,omitempty"`
	MsgraphExternalID     string `json:"msgraphExternalId,omitempty"`
	Confidence            int    `json:"confidence" doc:"0-100 overall score"`
	StagePoints           int    `json:"stagePoints" doc:"0-50, how specific/complete the matched name was"`
	RepresentationPoints  int    `json:"representationPoints" doc:"0-30, how many STT representations agreed on this candidate"`
	UniquenessPoints      int    `json:"uniquenessPoints" doc:"0-20, how many distinct people this match run found overall"`
}

// NameMatchOutputBody is the JSON body returned by POST /api/v1/auth/match.
type NameMatchOutputBody struct {
	Candidates []NameMatchCandidateDTO `json:"candidates" doc:"Descending by confidence, max 5"`
}

// NameMatchOutput wraps NameMatchOutputBody for huma.
type NameMatchOutput struct {
	Body NameMatchOutputBody
}

// IntentsInput is the header+query input for GET /api/v1/intents.
type IntentsInput struct {
	Authorization string `header:"Authorization" doc:"Bearer <JWT>"`
	IntentType    string `query:"intentType" doc:"Comma-separated list of /IntentType values to filter by, e.g. 'Mainintents' or 'Mainintents,Subintents'; empty returns every Intent"`
}

// IntentsOutputBody is the JSON body returned by GET /api/v1/intents.
type IntentsOutputBody struct {
	Intents []map[string]any `json:"intents" doc:"Raw Intent vertices (ogit/_id, ogit/description, /IntentType, and every other dynamic field), passed through unfiltered from the graph"`
}

// IntentsOutput wraps IntentsOutputBody for huma.
type IntentsOutput struct {
	Body IntentsOutputBody
}
