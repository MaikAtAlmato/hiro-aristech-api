// internal/api/issues_handler.go
package api

import (
	"context"
	"errors"
	"strings"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/hiro-aristech-api/internal/bardioc"
	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

// defaultIssueSubject is used when neither Body.Subject nor a "/subject"
// variable is supplied, mirroring the fallback in the reference Python
// integration (baue_issue_payload's setdefault).
const defaultIssueSubject = "Voicebot-Anliegen"

func (s *Server) handleCreateIssue(ctx context.Context, input *CreateIssueInput) (*CreateIssueOutput, error) {
	if _, err := s.verifyBearer(input.Authorization); err != nil {
		return nil, err
	}

	attributes := map[string]string{}
	if input.Body.IntentID != "" {
		intent, err := s.intentRepo.Get(ctx, graph.MetadataID(input.Body.IntentID))
		switch {
		case errors.Is(err, graph.ErrEntityNotFound):
			return nil, huma.Error404NotFound("intent not found")
		case err != nil:
			log.Error().Err(err).Msg("intent lookup failed")
			return nil, huma.Error503ServiceUnavailable("intent lookup failed")
		}
		for k, v := range bardioc.SystemVariables(*intent) {
			attributes[k] = v
		}
	}
	// User-supplied variables win over the Intent's fixed system variables,
	// matching baue_issue_payload's update order in the reference doc.
	for k, v := range input.Body.Variables {
		attributes[k] = v
	}

	subject := strings.TrimSpace(input.Body.Subject)
	if subject == "" {
		subject = attributes["/subject"]
	}
	if subject == "" {
		subject = defaultIssueSubject
	}
	attributes["ogit/subject"] = subject

	if input.Body.OriginNode != "" {
		attributes["ogit/Automation/originNode"] = input.Body.OriginNode
	}
	if input.Body.Scope != "" {
		attributes["ogit/_scope"] = input.Body.Scope
	}

	issueID, err := s.issueRepo.Create(ctx, attributes)
	if err != nil {
		log.Error().Err(err).Msg("create automation issue failed")
		return nil, huma.Error503ServiceUnavailable("create automation issue failed")
	}

	resp := &CreateIssueOutput{}
	resp.Body.IssueID = issueID.String()
	return resp, nil
}

func (s *Server) handleIssueStatus(ctx context.Context, input *IssueStatusInput) (*IssueStatusOutput, error) {
	if _, err := s.verifyBearer(input.Authorization); err != nil {
		return nil, err
	}

	variables, err := s.issueRepo.Variables(ctx, graph.MetadataID(input.IssueID))
	switch {
	case errors.Is(err, graph.ErrEntityNotFound):
		return nil, huma.Error404NotFound("automation issue not found")
	case err != nil:
		log.Error().Err(err).Msg("automation issue status lookup failed")
		return nil, huma.Error503ServiceUnavailable("automation issue status lookup failed")
	}

	resp := &IssueStatusOutput{}
	resp.Body.Variables = variables
	return resp, nil
}
