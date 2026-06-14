package clientapi

import (
	"sync"

	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
)

type AccountUpdateResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

// ProfileCacheNotifier adapts domain cache-completion notifications to the SSE
// realtime hub without making use cases depend on web infrastructure.
type ProfileCacheNotifier struct {
	mu  sync.RWMutex
	hub *RealtimeHub
}

func NewProfileCacheNotifier() *ProfileCacheNotifier { return &ProfileCacheNotifier{} }

func (n *ProfileCacheNotifier) SetHub(hub *RealtimeHub) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.hub = hub
}

func (n *ProfileCacheNotifier) RemoteProfileImagesCached(localAccountID, actorURI string) {
	if n == nil || localAccountID == "" || actorURI == "" {
		return
	}
	n.mu.RLock()
	hub := n.hub
	n.mu.RUnlock()
	if hub == nil {
		return
	}
	hub.Publish(RealtimeEvent{LocalAccountID: localAccountID, AccountUpdate: &AccountUpdateResponse{ID: clientapiUC.AccountIDForRemoteActor(actorURI), URI: actorURI}})
}
