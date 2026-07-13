// Package bardioc provides read-only Bardioc/HIRO graph repositories for the
// Aristech voicebot API.
package bardioc

import (
	"context"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/gremlin"
)

// EdgeClient extends graph.Client with Gremlin traversal support, matching
// the pattern used by hiro-conn-msgraph and hiro-conn-valuemation.
type EdgeClient interface {
	graph.Client
	QueryGremlin(ctx context.Context, builder *gremlin.Builder, rootID graph.MetadataID, p ...graph.Param) (graph.Rows, error)
}
