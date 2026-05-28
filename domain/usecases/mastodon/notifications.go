package mastodon

import (
	"context"

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
		actor, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, notification.ActorAccountID)
		if err != nil {
			actor = account
		}
		var status *TimelineItem
		if notification.StatusID != nil {
			status, _ = u.GetStatus(ctx, account, *notification.StatusID)
		}
		items = append(items, NotificationItem{Notification: notification, Account: *actor, Status: status})
	}
	return items, nil
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
