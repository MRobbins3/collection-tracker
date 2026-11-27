package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
)

// Relationships answers "how does the viewer relate to the owner?" for the
// privacy rule (package privacy, ADR 0009).
type Relationships struct {
	pool *pgxpool.Pool
}

func NewRelationships(pool *pgxpool.Pool) *Relationships {
	return &Relationships{pool: pool}
}

// Between returns the viewer's relationship to the owner. viewerID is ""
// for anonymous viewers.
//
// Friends and followers don't exist yet, so today this is Self or None.
// When they land, this is the only function that changes: look up an
// accepted friendship (→ Friend) or an approved follow (→ Follower), and
// every privacy check starts honoring the follower and friend levels.
func (r *Relationships) Between(_ context.Context, viewerID, ownerID string) (privacy.Relationship, error) {
	return privacy.RelationshipFor(viewerID, ownerID, privacy.None), nil
}
