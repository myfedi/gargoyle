package clientapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type CreateDomainBlockInput struct {
	Domain         string
	PublicComment  string
	PrivateComment string
	RejectMedia    bool
}

type EnqueuePurgeDomainResult struct {
	Job models.ModerationJob
}

type PurgeDomainPayload struct {
	Domain string `json:"domain"`
}

func (u Moderation) ListDomainBlocks(ctx context.Context) ([]models.DomainBlock, *domainerrors.DomainError) {
	blocks, err := u.deps.DomainBlocksRepo.ListDomainBlocks(ctx, nil)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return blocks, nil
}

func (u Moderation) CreateDomainBlock(ctx context.Context, admin *models.User, input CreateDomainBlockInput) (*models.DomainBlock, *domainerrors.DomainError) {
	if derr := requireAdmin(admin); derr != nil {
		return nil, derr
	}
	domain, derr := normalizeModerationDomain(input.Domain)
	if derr != nil {
		return nil, derr
	}
	publicComment := stringPtrOrNil(strings.TrimSpace(input.PublicComment))
	privateComment := stringPtrOrNil(strings.TrimSpace(input.PrivateComment))
	block, err := u.deps.DomainBlocksRepo.CreateDomainBlock(ctx, nil, repos.CreateDomainBlockInput{Domain: domain, Severity: models.DomainBlockSeveritySuspend, RejectMedia: input.RejectMedia, PublicComment: publicComment, PrivateComment: privateComment, CreatedByUserID: admin.ID})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return block, nil
}

func (u Moderation) DeleteDomainBlock(ctx context.Context, admin *models.User, domain string) *domainerrors.DomainError {
	if derr := requireAdmin(admin); derr != nil {
		return derr
	}
	normalized, derr := normalizeModerationDomain(domain)
	if derr != nil {
		return derr
	}
	if err := u.deps.DomainBlocksRepo.DeleteDomainBlock(ctx, nil, normalized); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u Moderation) EnqueuePurgeDomain(ctx context.Context, admin *models.User, domain string) (*EnqueuePurgeDomainResult, *domainerrors.DomainError) {
	if derr := requireAdmin(admin); derr != nil {
		return nil, derr
	}
	normalized, derr := normalizeModerationDomain(domain)
	if derr != nil {
		return nil, derr
	}
	if _, err := u.deps.DomainBlocksRepo.GetDomainBlock(ctx, nil, normalized); err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "domain block not found")
	}
	payload, err := json.Marshal(PurgeDomainPayload{Domain: normalized})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	job, err := u.deps.ModerationJobsRepo.CreateModerationJob(ctx, nil, repos.CreateModerationJobInput{Kind: models.ModerationJobKindPurgeDomain, Payload: string(payload), NextAttemptAt: time.Now().UTC()})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &EnqueuePurgeDomainResult{Job: *job}, nil
}

func (u Moderation) PurgeDomain(ctx context.Context, domain string) (*models.PurgeDomainResult, []string, error) {
	normalized, derr := normalizeModerationDomain(domain)
	if derr != nil {
		return nil, nil, derr
	}
	var result *models.PurgeDomainResult
	var storagePaths []string
	err := u.deps.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		result, storagePaths, err = u.deps.DomainPurgeRepo.PurgeDomain(ctx, &tx, normalized)
		return err
	})
	return result, storagePaths, err
}

func requireAdmin(user *models.User) *domainerrors.DomainError {
	if user == nil {
		return domainerrors.New(domainerrors.ErrUnauthorized, "missing user")
	}
	if !user.Admin {
		return domainerrors.New(domainerrors.ErrUnauthorized, "admin access required")
	}
	return nil
}

func normalizeModerationDomain(raw string) (string, *domainerrors.DomainError) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "domain is required")
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" {
			return "", domainerrors.New(domainerrors.ErrBadRequest, "invalid domain")
		}
		value = parsed.Host
	}
	value = strings.Trim(value, ".")
	if value == "" || strings.ContainsAny(value, "/?#@") {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "invalid domain")
	}
	return value, nil
}
