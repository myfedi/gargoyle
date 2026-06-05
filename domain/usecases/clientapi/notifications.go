package clientapi

import (
	"context"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type NotificationItem struct {
	Notification models.Notification
	Account      models.Account
	Status       *TimelineItem
}

func (u Notifications) Notifications(ctx context.Context, account *models.Account, limit int) ([]NotificationItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	notifications, err := u.deps.SocialRepo.ListNotifications(ctx, nil, account.ID, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]NotificationItem, 0, len(notifications))
	for _, notification := range notifications {
		actor := u.notificationActor(ctx, account, notification.ActorAccountID)
		if actor.Domain != nil && *actor.Domain != "" {
			blocked, err := u.deps.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *actor.Domain)
			if err == nil && blocked {
				continue
			}
		}
		var status *TimelineItem
		if notification.StatusID != nil {
			status, _ = u.getStatus(ctx, account, *notification.StatusID)
		}
		items = append(items, NotificationItem{Notification: notification, Account: *actor, Status: status})
	}
	return items, nil
}
func (u Notifications) notificationActor(ctx context.Context, localAccount *models.Account, actorURI string) *models.Account {
	if actorURI == localAccount.URI || actorURI == localAccount.ID {
		return localAccount
	}
	localPrefix := strings.TrimRight(u.deps.Host, "/") + "/users/"
	if strings.HasPrefix(actorURI, localPrefix) {
		username := strings.TrimPrefix(actorURI, localPrefix)
		if account, err := u.deps.AccountsRepo.GetLocalAccountByUsername(ctx, nil, username); err == nil {
			return account
		}
	}
	if actor, err := u.deps.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actorURI); err == nil {
		return actor
	}
	if actor, err := u.resolveAndCacheRemoteAccount(ctx, actorURI, localAccount); err == nil {
		return actor
	}
	return &models.Account{ID: AccountIDForRemoteActor(actorURI), Username: actorURI, URI: actorURI, InboxURI: "", PublicKey: "", ActorType: models.ActorTypePerson}
}

func (u Notifications) DismissNotification(ctx context.Context, account *models.Account, notificationID string) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if notificationID == "" {
		return domainerrors.New(domainerrors.ErrBadRequest, "notification id is required")
	}
	if err := u.deps.SocialRepo.DeleteNotification(ctx, nil, account.ID, notificationID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u Notifications) ClearNotifications(ctx context.Context, account *models.Account) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if err := u.deps.SocialRepo.ClearNotifications(ctx, nil, account.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}
