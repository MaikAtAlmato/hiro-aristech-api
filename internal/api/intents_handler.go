// internal/api/intents_handler.go
package api

import (
	"context"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog/log"
)

func (s *Server) handleListIntents(ctx context.Context, input *IntentsInput) (*IntentsOutput, error) {
	if _, err := s.verifyBearer(input.Authorization); err != nil {
		return nil, err
	}

	intents, err := s.intentRepo.List(ctx, parseIntentTypes(input.IntentType))
	if err != nil {
		log.Error().Err(err).Msg("intent list failed")
		return nil, huma.Error503ServiceUnavailable("intent list failed")
	}

	resp := &IntentsOutput{}
	resp.Body.Intents = []map[string]any{}
	resp.Body.Intents = append(resp.Body.Intents, intents...)
	return resp, nil
}

// parseIntentTypes splits a comma-separated /IntentType list into a
// normalized slice: whitespace-trimmed, empty entries dropped. Unlike
// parseDomains, values are NOT lowercased — /IntentType is matched
// case-sensitively against the graph. Returns nil (no filter) when raw has
// no non-empty entries.
func parseIntentTypes(raw string) []string {
	var types []string
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			types = append(types, t)
		}
	}
	return types
}
