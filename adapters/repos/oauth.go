package repos

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type OAuthRepo struct{ db bun.IDB }

func NewOAuthRepo(db *bun.DB) *OAuthRepo { return &OAuthRepo{db: db} }

var _ repos.OAuthRepository = &OAuthRepo{}

func (r *OAuthRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *OAuthRepo) CreateApplication(ctx context.Context, tx *dbPorts.Tx, input repos.CreateOAuthApplicationInput) (*models.OAuthApplication, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	app := &dbModels.OAuthApplication{ID: id, Name: input.Name, RedirectURI: input.RedirectURI, Scopes: input.Scopes, Website: input.Website, ClientID: input.ClientID, ClientSecret: input.ClientSecret}
	if _, err := db.NewInsert().Model(app).Exec(ctx); err != nil {
		return nil, err
	}
	model := app.ToModel()
	return &model, nil
}

func (r *OAuthRepo) GetApplicationByClientID(ctx context.Context, tx *dbPorts.Tx, clientID string) (*models.OAuthApplication, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var app dbModels.OAuthApplication
	if err := db.NewSelect().Model(&app).Where("client_id = ?", clientID).Scan(ctx); err != nil {
		return nil, err
	}
	model := app.ToModel()
	return &model, nil
}

func (r *OAuthRepo) CreateAccessToken(ctx context.Context, tx *dbPorts.Tx, input repos.CreateOAuthAccessTokenInput) (*models.OAuthAccessToken, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	token := &dbModels.OAuthAccessToken{ID: id, ApplicationID: input.ApplicationID, UserID: input.UserID, TokenHash: input.TokenHash, Scopes: input.Scopes, ExpiresAt: input.ExpiresAt}
	if _, err := db.NewInsert().Model(token).Exec(ctx); err != nil {
		return nil, err
	}
	model := token.ToModel()
	return &model, nil
}

func (r *OAuthRepo) GetAccessTokenByHash(ctx context.Context, tx *dbPorts.Tx, tokenHash string) (*models.OAuthAccessToken, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var token dbModels.OAuthAccessToken
	if err := db.NewSelect().Model(&token).Where("token_hash = ?", tokenHash).Scan(ctx); err != nil {
		return nil, err
	}
	model := token.ToModel()
	return &model, nil
}

func (r *OAuthRepo) CreateAuthorizationCode(ctx context.Context, tx *dbPorts.Tx, input repos.CreateOAuthAuthorizationCodeInput) (*models.OAuthAuthorizationCode, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	code := &dbModels.OAuthAuthorizationCode{ID: id, ApplicationID: input.ApplicationID, UserID: input.UserID, CodeHash: input.CodeHash, RedirectURI: input.RedirectURI, Scopes: input.Scopes, CodeChallenge: input.CodeChallenge, CodeChallengeMethod: input.CodeChallengeMethod, ExpiresAt: input.ExpiresAt}
	if _, err := db.NewInsert().Model(code).Exec(ctx); err != nil {
		return nil, err
	}
	model := code.ToModel()
	return &model, nil
}

func (r *OAuthRepo) GetAuthorizationCodeByHash(ctx context.Context, tx *dbPorts.Tx, codeHash string) (*models.OAuthAuthorizationCode, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var code dbModels.OAuthAuthorizationCode
	if err := db.NewSelect().Model(&code).Where("code_hash = ?", codeHash).Scan(ctx); err != nil {
		return nil, err
	}
	model := code.ToModel()
	return &model, nil
}

func (r *OAuthRepo) MarkAuthorizationCodeUsed(ctx context.Context, tx *dbPorts.Tx, id string, usedAt time.Time) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().Model((*dbModels.OAuthAuthorizationCode)(nil)).Set("used_at = ?", usedAt).Set("updated_at = ?", usedAt).Where("id = ?", id).Where("used_at IS NULL").Exec(ctx)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("authorization code is already used or missing")
	}
	return nil
}
