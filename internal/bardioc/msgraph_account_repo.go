// internal/bardioc/msgraph_account_repo.go
package bardioc

import (
	"context"
	"fmt"

	"bitbucket.org/almatoag/bardioc-go/graph"
	"bitbucket.org/almatoag/bardioc-go/graph/gremlin"
	"bitbucket.org/almatoag/graph-go/connx"
)

// MsgraphAccountRepository finds the Account node hiro-conn-msgraph connects
// to an MSGraph Person node.
type MsgraphAccountRepository struct {
	client EdgeClient
}

// NewMsgraphAccountRepository creates a new MsgraphAccountRepository.
func NewMsgraphAccountRepository(client EdgeClient) *MsgraphAccountRepository {
	return &MsgraphAccountRepository{client: client}
}

// FindForPerson returns the Account connected to personID via an incoming
// "connects" edge (Account --connects--> Person, per sgo.Account's allowed
// connections), scoped to accounts written by hiro-conn-msgraph. Returns nil
// if the person has no such connected account.
func (r *MsgraphAccountRepository) FindForPerson(ctx context.Context, personID graph.MetadataID) (*MsgraphAccount, error) {
	builder := gremlin.NewBuilder().InE(string(connx.Connects))
	rows, err := r.client.QueryGremlin(ctx, builder, personID, graph.WithListMeta(true), graph.WithIncludeDeleted(false))
	if err != nil {
		return nil, fmt.Errorf("query incoming connects edges: %w", err)
	}
	defer rows.Close()

	verbs, err := graph.ScanRows[graph.Verb](rows)
	if err != nil {
		return nil, fmt.Errorf("scan incoming connects edges: %w", err)
	}

	for _, v := range verbs {
		account, err := r.getAccount(ctx, v.OutID)
		if err != nil {
			return nil, err
		}
		if account.PFlag == PFlagMsgraph {
			return account, nil
		}
	}
	return nil, nil
}

func (r *MsgraphAccountRepository) getAccount(ctx context.Context, id graph.MetadataID) (*MsgraphAccount, error) {
	row := r.client.GetEntity(ctx, id, graph.WithIncludeDeleted(false))
	defer row.Close()

	var account MsgraphAccount
	if err := row.Scan(&account); err != nil {
		return nil, fmt.Errorf("get account %s: %w", id, err)
	}
	return &account, nil
}
