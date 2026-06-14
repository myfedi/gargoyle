package clientapi

import (
	"context"
	"sync"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

const remoteOutboxCacheTimeout = 20 * time.Second

var remoteOutboxCacheJobs sync.Map

func (u Accounts) cacheRemoteOutboxUntilAsync(localAccount *models.Account, remote models.Account, limit int, maxID string) {
	if localAccount == nil || remote.OutboxURI == nil || *remote.OutboxURI == "" {
		return
	}
	key := localAccount.ID + "\x00" + remote.URI + "\x00" + maxID
	if _, loaded := remoteOutboxCacheJobs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	local := *localAccount
	go func() {
		defer remoteOutboxCacheJobs.Delete(key)
		ctx, cancel := context.WithTimeout(context.Background(), remoteOutboxCacheTimeout)
		defer cancel()
		enough := func() (bool, error) {
			notes, err := u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, local.ID, remote.URI, limit, maxID)
			if err != nil {
				return false, err
			}
			boosts, err := u.deps.BoostsRepo.ListActorBoosts(ctx, nil, local.ID, remote.URI, limit, maxID)
			return err == nil && len(notes)+len(boosts) >= limit, err
		}
		_ = u.cacheRemoteOutboxUntil(ctx, &local, remote, enough)
	}()
}

func (u Accounts) cacheRemoteOutboxUntil(ctx context.Context, localAccount *models.Account, remote models.Account, enough func() (bool, error)) *domainerrors.DomainError {
	if remote.OutboxURI == nil || *remote.OutboxURI == "" || localAccount == nil {
		return nil
	}
	page := *remote.OutboxURI
	seen := map[string]bool{}
	for page != "" && !seen[page] {
		ok, err := enough()
		if err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if ok {
			return nil
		}
		seen[page] = true
		next, err := u.deps.HydrateRemoteObjectUC.CacheRemoteOutboxPage(ctx, *localAccount, page, remote.URI, enough)
		if err != nil {
			return nil
		}
		page = next
	}
	return nil
}
