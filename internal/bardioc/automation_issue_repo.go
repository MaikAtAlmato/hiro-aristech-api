// internal/bardioc/automation_issue_repo.go
package bardioc

import (
	"context"
	"encoding/json"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	automation "bitbucket.org/almatoag/graph-go/NTO/Automation"
)

// AutomationIssuePayload is the ogit/Automation/AutomationIssue node the
// voicebot writes to trigger the Reasoning Engine. Its variables are
// dynamic /-prefixed properties determined by whichever Intent matched the
// caller's request, so it marshals Attributes flattened at the top level
// instead of modeling every possible variable as a Go struct field.
type AutomationIssuePayload struct {
	graph.Entity
	Attributes map[string]string
}

// OgitType matches the graph vertex type of every AutomationIssue node.
func (p AutomationIssuePayload) OgitType() string {
	return "ogit/Automation/AutomationIssue"
}

// MarshalJSON flattens Attributes alongside the entity's own metadata
// fields (e.g. ogit/_scope, set by the client before creation) so the
// server sees one flat property set, matching how every other OGIT node is
// represented on the wire.
func (p AutomationIssuePayload) MarshalJSON() ([]byte, error) {
	base, err := json.Marshal(p.Entity)
	if err != nil {
		return nil, fmt.Errorf("marshal entity metadata: %w", err)
	}

	out := map[string]any{}
	if err := json.Unmarshal(base, &out); err != nil {
		return nil, fmt.Errorf("unmarshal entity metadata: %w", err)
	}
	for k, v := range p.Attributes {
		out[k] = v
	}
	return json.Marshal(out)
}

// AutomationIssueRepository creates and monitors ogit/Automation/AutomationIssue nodes.
type AutomationIssueRepository struct {
	client EdgeClient
}

// NewAutomationIssueRepository creates a new AutomationIssueRepository.
func NewAutomationIssueRepository(client EdgeClient) *AutomationIssueRepository {
	return &AutomationIssueRepository{client: client}
}

// Create writes a new AutomationIssue with the given attributes (expected
// to already include "ogit/subject") and returns its graph ID. As soon as
// the node exists, the Reasoning Engine picks it up.
func (r *AutomationIssueRepository) Create(ctx context.Context, attributes map[string]string) (graph.MetadataID, error) {
	payload := AutomationIssuePayload{Attributes: attributes}

	var issue automation.AutomationIssue
	if err := r.client.CreateEntity(ctx, &payload).Scan(&issue); err != nil {
		return "", fmt.Errorf("create automation issue: %w", err)
	}
	return issue.Metadata.ID, nil
}

// Status returns the current ogit/status of the AutomationIssue with the
// given ID (e.g. UNPROCESSED, PROCESSING, WAITING, RESOLVED, STOPPED).
func (r *AutomationIssueRepository) Status(ctx context.Context, id graph.MetadataID) (string, error) {
	row := r.client.GetEntity(ctx, id, graph.WithIncludeDeleted(false))
	defer row.Close()

	var issue automation.AutomationIssue
	if err := row.Scan(&issue); err != nil {
		return "", fmt.Errorf("get automation issue %s: %w", id, err)
	}
	return issue.Status, nil
}
