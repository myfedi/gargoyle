package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateBoostInput struct {
	LocalAccountID string
	Actor          string
	NoteID         string
	URI            string
	PublishedAt    time.Time
}

type BoostsRepository interface {
	CreateBoost(ctx context.Context, tx *db.Tx, input CreateBoostInput) (*models.Boost, error)
	DeleteBoost(ctx context.Context, tx *db.Tx, localAccountID, actor, noteID string) error
	ListTimelineBoosts(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Boost, error)
	ListActorBoosts(ctx context.Context, tx *db.Tx, localAccountID, actor string, limit int, maxID string) ([]models.Boost, error)
	CountBoostsForNote(ctx context.Context, tx *db.Tx, noteID string) (int, error)
	BoostExists(ctx context.Context, tx *db.Tx, localAccountID, actor, noteID string) (bool, error)
}
