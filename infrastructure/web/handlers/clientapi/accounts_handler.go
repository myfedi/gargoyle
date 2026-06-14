package clientapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type searchResponse struct {
	Accounts []accountResponse `json:"accounts"`
	Statuses []statusResponse  `json:"statuses"`
	Hashtags []any             `json:"hashtags"`
}

func (h APIHandler) accountsSearch(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var accounts []models.Account
	if c.QueryBool("resolve") {
		accounts, derr = h.accountsWorkflow.ResolveAccountSearch(c.UserContext(), principal.Account, c.Query("q"))
	} else {
		accounts, derr = h.accountsWorkflow.SearchAccounts(c.UserContext(), principal.Account, c.Query("q"), c.QueryInt("limit"))
	}
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(accountsToResponses(accounts))
}

func (h APIHandler) search(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var accounts []models.Account
	if c.Query("type") == "" || c.Query("type") == "accounts" {
		if c.QueryBool("resolve") {
			accounts, derr = h.accountsWorkflow.ResolveAccountSearch(c.UserContext(), principal.Account, c.Query("q"))
		} else {
			accounts, derr = h.accountsWorkflow.SearchAccounts(c.UserContext(), principal.Account, c.Query("q"), c.QueryInt("limit"))
		}
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
	}
	return c.JSON(searchResponse{Accounts: accountsToResponses(accounts), Statuses: []statusResponse{}, Hashtags: []any{}})
}

func accountsToResponses(accounts []models.Account) []accountResponse {
	resp := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		acct := accountToResponse(&account)
		if account.Domain != nil && *account.Domain != "" {
			acct.Acct = account.Username + "@" + *account.Domain
		}
		resp = append(resp, acct)
	}
	return resp
}

type relationshipResponse struct {
	ID                  string `json:"id"`
	Following           bool   `json:"following"`
	ShowingReblogs      bool   `json:"showing_reblogs"`
	Notifying           bool   `json:"notifying"`
	FollowedBy          bool   `json:"followed_by"`
	Blocking            bool   `json:"blocking"`
	BlockedBy           bool   `json:"blocked_by"`
	Muting              bool   `json:"muting"`
	MutingNotifications bool   `json:"muting_notifications"`
	Requested           bool   `json:"requested"`
	DomainBlocking      bool   `json:"domain_blocking"`
	Endorsed            bool   `json:"endorsed"`
}

func (h APIHandler) relationships(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	ids := c.Queries()["id[]"]
	if ids == "" {
		ids = c.Query("id")
	}
	idList := strings.Split(ids, ",")
	relationships, derr := h.accountsWorkflow.Relationships(c.UserContext(), principal.Account, idList)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]relationshipResponse, 0, len(idList))
	for _, id := range idList {
		if id == "" {
			continue
		}
		rel := relationships[id]
		resp = append(resp, relationshipResponse{ID: id, Following: rel.Following, Requested: rel.Requested, ShowingReblogs: true})
	}
	return c.JSON(resp)
}

func (h APIHandler) account(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	account, derr := h.accountsWorkflow.GetAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(accountToResponse(account))
}

func (h APIHandler) accountList(c *fiber.Ctx, accounts []models.Account) error {
	resp := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		acct := accountToResponse(&account)
		if account.Domain != nil && *account.Domain != "" {
			acct.Acct = account.Username + "@" + *account.Domain
		}
		resp = append(resp, acct)
	}
	return c.JSON(resp)
}

func (h APIHandler) accountStatuses(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if c.QueryBool("pinned") {
		items, derr := h.interactionsWorkflow.PinnedAccountStatuses(c.UserContext(), principal.Account, c.Params("id"), c.QueryInt("limit"))
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		return c.JSON(timelineItemsToStatuses(items))
	}
	items, derr := h.accountsWorkflow.AccountStatuses(c.UserContext(), principal.Account, c.Params("id"), c.QueryInt("limit"), c.Query("max_id"), c.QueryBool("exclude_reblogs"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}

func (h APIHandler) followers(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.accountsWorkflow.FollowerAccounts(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return h.accountList(c, accounts)
}

func (h APIHandler) following(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.accountsWorkflow.FollowingAccounts(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return h.accountList(c, accounts)
}

func (h APIHandler) followRequests(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "follow")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.accountsWorkflow.FollowRequests(c.UserContext(), principal.Account)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return h.accountList(c, accounts)
}

func (h APIHandler) authorizeFollowRequest(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "follow")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.accountsWorkflow.AuthorizeFollowRequest(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id"), FollowedBy: true})
}

func (h APIHandler) rejectFollowRequest(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "follow")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.accountsWorkflow.RejectFollowRequest(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id")})
}

func (h APIHandler) followAccount(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "follow")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.accountsWorkflow.FollowAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id"), Following: res.Following, Requested: res.Requested, ShowingReblogs: true})
}

func (h APIHandler) unfollowAccount(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "follow")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.accountsWorkflow.UnfollowAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id"), Following: res.Following, Requested: res.Requested, ShowingReblogs: true})
}
