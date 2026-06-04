package mastodon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type ConversationItem struct {
	ID         string
	Unread     bool
	Accounts   []models.Account
	LastStatus TimelineItem
}

func (u UseCase) Conversations(ctx context.Context, account *models.Account, limit int, maxID string) ([]ConversationItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	notes, err := u.cfg.NotesRepo.ListDirectNotesPaged(ctx, nil, account.ID, limit*3, maxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]ConversationItem, 0, limit)
	seen := map[string]bool{}
	for _, note := range notes {
		item, id, ok, derr := u.conversationItem(ctx, account, note, seen)
		if derr != nil {
			return nil, derr
		}
		if !ok {
			continue
		}
		items = append(items, *item)
		seen[id] = true
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (u UseCase) conversationItem(ctx context.Context, account *models.Account, note models.Note, seen map[string]bool) (*ConversationItem, string, bool, *domainerrors.DomainError) {
	accounts, derr := u.conversationAccounts(ctx, account, note)
	if derr != nil {
		return nil, "", false, derr
	}
	id := conversationID(account.URI, accounts)
	if seen[id] {
		return nil, id, false, nil
	}
	dismissed, err := u.cfg.ConversationsRepo.ConversationDismissed(ctx, nil, account.ID, id)
	if err != nil {
		return nil, id, false, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if dismissed {
		return nil, id, false, nil
	}
	status, derr := u.GetStatus(ctx, account, note.ID)
	if derr != nil {
		return nil, id, false, nil
	}
	item := &ConversationItem{ID: id, Unread: false, Accounts: accounts, LastStatus: *status}
	return item, id, true, nil
}

func (u UseCase) DismissConversation(ctx context.Context, account *models.Account, id string) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if id == "" {
		return domainerrors.New(domainerrors.ErrBadRequest, "conversation id is required")
	}
	if err := u.cfg.ConversationsRepo.DismissConversation(ctx, nil, account.ID, id); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u UseCase) ReadConversation(ctx context.Context, account *models.Account, id string) *domainerrors.DomainError {
	if derr := requireAccount(account); derr != nil {
		return derr
	}
	if id == "" {
		return domainerrors.New(domainerrors.ErrBadRequest, "conversation id is required")
	}
	return nil
}

func (u UseCase) conversationAccounts(ctx context.Context, local *models.Account, note models.Note) ([]models.Account, *domainerrors.DomainError) {
	byURI := map[string]models.Account{}
	if note.AttributedTo != local.URI {
		actor, derr := u.accountForActor(ctx, local, note.AttributedTo)
		if derr == nil {
			byURI[actor.URI] = *actor
		}
	}
	mentions := remoteMentionPattern.FindAllStringSubmatch(note.Content, -1)
	for _, match := range mentions {
		username := match[1]
		domain := strings.ToLower(match[2])
		if domain == strings.ToLower(u.cfg.Domain) {
			acct, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(ctx, nil, username)
			if err == nil && acct.URI != local.URI {
				byURI[acct.URI] = *acct
			}
			continue
		}
		cached, err := u.cfg.RemoteAccountsRepo.SearchRemoteAccounts(ctx, nil, username+"@"+domain, 1)
		if err == nil && len(cached) > 0 && cached[0].URI != local.URI {
			byURI[cached[0].URI] = cached[0]
		}
	}
	accounts := make([]models.Account, 0, len(byURI))
	for _, account := range byURI {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].URI < accounts[j].URI })
	return accounts, nil
}

func conversationID(localActor string, accounts []models.Account) string {
	parts := make([]string, 0, len(accounts)+1)
	parts = append(parts, localActor)
	for _, account := range accounts {
		parts = append(parts, account.URI)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
