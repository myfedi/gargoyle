package clientapi

import (
	"context"
	"sync"

	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
)

type RealtimeHub struct {
	mu            sync.Mutex
	clients       map[string]map[*realtimeClient]bool
	accounts      clientapiUC.Accounts
	notifications clientapiUC.Notifications
}

type RealtimeEvent struct {
	LocalAccountID string
	RemoteActor    string
	Notifications  bool
}

type realtimeClient struct {
	account *oauth.AuthenticatedUser
	watched map[string]bool
	seen    map[string]bool
	send    chan streamEvent
}

type streamEvent struct {
	Event string
	Data  any
}

func NewRealtimeHub(accounts clientapiUC.Accounts, notifications clientapiUC.Notifications) *RealtimeHub {
	return &RealtimeHub{clients: map[string]map[*realtimeClient]bool{}, accounts: accounts, notifications: notifications}
}

func (h *RealtimeHub) Add(account *oauth.AuthenticatedUser) *realtimeClient {
	client := &realtimeClient{account: account, watched: map[string]bool{}, seen: map[string]bool{}, send: make(chan streamEvent, 32)}
	if h == nil || account == nil || account.Account == nil {
		return client
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	id := account.Account.ID
	if h.clients[id] == nil {
		h.clients[id] = map[*realtimeClient]bool{}
	}
	h.clients[id][client] = true
	return client
}

func (h *RealtimeHub) Remove(client *realtimeClient) {
	if h == nil || client == nil || client.account == nil || client.account.Account == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	id := client.account.Account.ID
	delete(h.clients[id], client)
	if len(h.clients[id]) == 0 {
		delete(h.clients, id)
	}
}

func (h *RealtimeHub) WatchRelationship(client *realtimeClient, id string) {
	if h == nil || client == nil || id == "" {
		return
	}
	client.watched[id] = true
	h.deliverRelationship(client, id)
}

func (h *RealtimeHub) MarkNotificationsSeen(client *realtimeClient, notifications []notificationResponse) {
	if client == nil {
		return
	}
	for _, notification := range notifications {
		client.seen[notification.ID] = true
	}
}

func (h *RealtimeHub) Publish(event RealtimeEvent) {
	if h == nil || event.LocalAccountID == "" {
		return
	}
	h.mu.Lock()
	clients := make([]*realtimeClient, 0, len(h.clients[event.LocalAccountID]))
	for client := range h.clients[event.LocalAccountID] {
		clients = append(clients, client)
	}
	h.mu.Unlock()
	for _, client := range clients {
		h.deliver(client, event)
	}
}

func (h *RealtimeHub) deliver(client *realtimeClient, event RealtimeEvent) {
	if event.Notifications {
		h.deliverNotifications(client)
	}
	if event.RemoteActor != "" {
		id := clientapiUC.AccountIDForRemoteActor(event.RemoteActor)
		if client.watched[id] {
			h.deliverRelationship(client, id)
		}
	}
}

func (h *RealtimeHub) deliverNotifications(client *realtimeClient) {
	items, derr := h.notifications.Notifications(context.Background(), client.account.Account, 20)
	if derr != nil {
		return
	}
	responses := notificationItemsToResponses(items)
	for i := len(responses) - 1; i >= 0; i-- {
		item := responses[i]
		if client.seen[item.ID] {
			continue
		}
		client.seen[item.ID] = true
		h.send(client, streamEvent{Event: "notification", Data: item})
	}
}

func (h *RealtimeHub) deliverRelationship(client *realtimeClient, id string) {
	rels, derr := h.accounts.Relationships(context.Background(), client.account.Account, []string{id})
	if derr != nil {
		return
	}
	rel := rels[id]
	resp := relationshipResponse{ID: id, Following: rel.Following, Requested: rel.Requested, ShowingReblogs: true}
	h.send(client, streamEvent{Event: "relationship_update", Data: resp})
}

func (h *RealtimeHub) send(client *realtimeClient, event streamEvent) {
	select {
	case client.send <- event:
	default:
	}
}
