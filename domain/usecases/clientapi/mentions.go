package clientapi

import (
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
)

func mentionAcct(account models.Account) string {
	if account.Domain != nil && *account.Domain != "" {
		return account.Username + "@" + *account.Domain
	}
	return account.Username
}

func mentionURL(account models.Account, host string) string {
	if account.URL != nil && *account.URL != "" {
		return *account.URL
	}
	if account.Domain == nil {
		return strings.TrimRight(host, "/") + "/@" + account.Username
	}
	return account.URI
}

func mentionInboxes(mentions []models.Account) []string {
	inboxes := make([]string, 0, len(mentions))
	seen := map[string]bool{}
	for _, mention := range mentions {
		if mention.Domain == nil || mention.InboxURI == "" || seen[mention.InboxURI] {
			continue
		}
		seen[mention.InboxURI] = true
		inboxes = append(inboxes, mention.InboxURI)
	}
	return inboxes
}
