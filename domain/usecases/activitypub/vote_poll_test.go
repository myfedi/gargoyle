package activitypub

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	apPorts "github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type pollRemoteAccountsRepo struct{ account *models.Account }

func (r pollRemoteAccountsRepo) UpsertRemoteAccount(context.Context, *db.Tx, models.Account) (*models.Account, error) {
	panic("not used")
}
func (r pollRemoteAccountsRepo) GetRemoteAccountByURI(_ context.Context, _ *db.Tx, uri string) (*models.Account, error) {
	if r.account != nil && r.account.URI == uri {
		return r.account, nil
	}
	return nil, sql.ErrNoRows
}
func (r pollRemoteAccountsRepo) SearchRemoteAccounts(context.Context, *db.Tx, string, int) ([]models.Account, error) {
	panic("not used")
}

type pollActorFetcher struct {
	inbox string
	calls int
}

func (f *pollActorFetcher) FetchActor(ctx context.Context, actor string, signer *models.Account) (*apPorts.RemoteActorDocument, error) {
	f.calls++
	return &apPorts.RemoteActorDocument{Inbox: f.inbox}, nil
}

func TestVotePollRemoteAuthorInboxRefreshesStaleCachedActor(t *testing.T) {
	fetcher := &pollActorFetcher{inbox: "https://remote.example/new-inbox"}
	u := &VotePollUseCase{cfg: ActivityPubFlowConfig{
		Host: "https://local.example",
		RemoteAccountsRepo: pollRemoteAccountsRepo{account: &models.Account{
			URI:       "https://remote.example/users/alice",
			InboxURI:  "https://remote.example/old-inbox",
			FetchedAt: time.Now().Add(-48 * time.Hour),
		}},
		ActorFetcher: fetcher,
	}}

	inbox := u.remotePollAuthorInbox(context.Background(), models.Account{URI: "https://local.example/users/bob"}, "https://remote.example/users/alice")
	if inbox != "https://remote.example/new-inbox" {
		t.Fatalf("inbox = %q, want refreshed inbox", inbox)
	}
	if fetcher.calls != 1 {
		t.Fatalf("fetcher calls = %d, want 1", fetcher.calls)
	}
}

func TestVotePollRemoteAuthorInboxUsesFreshCachedActor(t *testing.T) {
	fetcher := &pollActorFetcher{inbox: "https://remote.example/new-inbox"}
	u := &VotePollUseCase{cfg: ActivityPubFlowConfig{
		Host: "https://local.example",
		RemoteAccountsRepo: pollRemoteAccountsRepo{account: &models.Account{
			URI:       "https://remote.example/users/alice",
			InboxURI:  "https://remote.example/cached-inbox",
			FetchedAt: time.Now(),
		}},
		ActorFetcher: fetcher,
	}}

	inbox := u.remotePollAuthorInbox(context.Background(), models.Account{URI: "https://local.example/users/bob"}, "https://remote.example/users/alice")
	if inbox != "https://remote.example/cached-inbox" {
		t.Fatalf("inbox = %q, want cached inbox", inbox)
	}
	if fetcher.calls != 0 {
		t.Fatalf("fetcher calls = %d, want 0", fetcher.calls)
	}
}
