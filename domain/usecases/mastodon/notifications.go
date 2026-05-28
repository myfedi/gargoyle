package mastodon

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

func (u UseCase) Notifications(ctx context.Context, account *models.Account, limit int) ([]NotificationItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	notifications, err := u.cfg.SocialRepo.ListNotifications(ctx, nil, account.ID, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]NotificationItem, 0, len(notifications))
	for _, notification := range notifications {
		actor := u.notificationActor(ctx, account, notification.ActorAccountID)
		var status *TimelineItem
		if notification.StatusID != nil {
			status, _ = u.GetStatus(ctx, account, *notification.StatusID)
		}
		items = append(items, NotificationItem{Notification: notification, Account: *actor, Status: status})
	}
	return items, nil
}
func (u UseCase) notificationActor(ctx context.Context, localAccount *models.Account, actorURI string) *models.Account {
	if actorURI == localAccount.URI || actorURI == localAccount.ID {
		return localAccount
	}
	localPrefix := strings.TrimRight(u.cfg.Host, "/") + "/users/"
	if strings.HasPrefix(actorURI, localPrefix) {
		username := strings.TrimPrefix(actorURI, localPrefix)
		if account, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(ctx, nil, username); err == nil {
			return account
		}
	}
	if actor, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actorURI); err == nil {
		return actor
	}
	if actor, err := u.resolveAndCacheRemoteAccount(ctx, actorURI, localAccount); err == nil {
		return actor
	}
	return &models.Account{ID: AccountIDForRemoteActor(actorURI), Username: actorURI, URI: actorURI, InboxURI: "", PublicKey: "", ActorType: models.ActorTypePerson}
}

func (u UseCase) DismissNotification(ctx context.Context, account *models.Account, notificationID string) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if notificationID == "" {
		return domainerrors.New(domainerrors.ErrBadRequest, "notification id is required")
	}
	if err := u.cfg.SocialRepo.DeleteNotification(ctx, nil, account.ID, notificationID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u UseCase) ClearNotifications(ctx context.Context, account *models.Account) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if err := u.cfg.SocialRepo.ClearNotifications(ctx, nil, account.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}
