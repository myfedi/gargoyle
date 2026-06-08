package mastodon_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/myfedi/gargoyle/integration/shared"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func TestMain(m *testing.M) {
	if os.Getenv("GARGOYLE_RUN_INTEGRATION") != "1" {
		os.Exit(m.Run())
	}
	if err := resetIntegrationStackOnce(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start integration stack: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = dockerComposeDir("down", "-v", "--remove-orphans", "--timeout", "0")
	os.Exit(code)
}

func requireIntegration(t testing.TB) {
	t.Helper()
	if os.Getenv("GARGOYLE_RUN_INTEGRATION") != "1" {
		t.Skip("set GARGOYLE_RUN_INTEGRATION=1 to run Docker-backed integration tests")
	}
}

func mustNoResponseError(t testing.TB, err *shared.ResponseError) {
	t.Helper()
	if err != nil {
		status := 0
		if err.Response != nil {
			status = err.Response.StatusCode
		}
		t.Fatalf("request failed status=%d body=%q err=%v", status, err.Body, err.Err)
	}
}

type suite struct {
	ctx             context.Context
	gargoyle        shared.Client
	mastodon        shared.Client
	gargoyleToken   string
	mastodonToken   string
	alice           shared.Account
	bob             shared.Account
	aliceOnMastodon shared.Account
	bobOnGargoyle   shared.Account
}

var (
	baseSuiteMu sync.Mutex
	baseSuite   *suite
)

func setupSuite(t testing.TB) suite {
	t.Helper()
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	t.Cleanup(cancel)

	baseSuiteMu.Lock()
	defer baseSuiteMu.Unlock()
	if baseSuite == nil {
		baseSuite = initSuite(t, ctx)
	}
	s := *baseSuite
	s.ctx = ctx
	return s
}

func initSuite(t testing.TB, ctx context.Context) *suite {
	t.Helper()
	baseURL := env("INTEGRATION_PROXY_URL", "http://127.0.0.1:18081")
	gargoyleUser := env("GARGOYLE_USERNAME", "alice")
	gargoylePass := env("GARGOYLE_PASSWORD", "Str0ngP@ssword!")
	mastodonUser := env("MASTODON_USERNAME", "bob")
	mastodonPass := env("MASTODON_PASSWORD", "Str0ngP@ssword!")

	s := suite{ctx: ctx, gargoyle: shared.NewHostClient(baseURL, "gargoyle.test"), mastodon: shared.NewHostClient(baseURL, "mastodon.test")}
	waitForHTTP(t, ctx, s.gargoyle, "/api/v1/instance")
	waitForHTTP(t, ctx, s.mastodon, "/health")
	setupMastodonAccount(t, mastodonUser, mastodonPass)

	gargoyleApp, rerr := shared.RegisterApp(ctx, s.gargoyle)
	mustNoResponseError(t, rerr)
	mastodonApp, rerr := shared.RegisterApp(ctx, s.mastodon)
	mustNoResponseError(t, rerr)
	s.gargoyleToken, rerr = shared.PasswordToken(ctx, s.gargoyle, gargoyleApp, gargoyleUser, gargoylePass)
	mustNoResponseError(t, rerr)
	s.mastodonToken = mastodonAccessToken(t, mastodonUser, mastodonApp)
	s.alice, rerr = shared.VerifyCredentials(ctx, s.gargoyle, s.gargoyleToken)
	mustNoResponseError(t, rerr)
	s.bob, rerr = shared.VerifyCredentials(ctx, s.mastodon, s.mastodonToken)
	mustNoResponseError(t, rerr)
	s.aliceOnMastodon = searchAccount(t, ctx, s.mastodon, s.mastodonToken, gargoyleUser+"@gargoyle.test")
	s.bobOnGargoyle = searchAccount(t, ctx, s.gargoyle, s.gargoyleToken, mastodonUser+"@mastodon.test")
	return &s
}

func TestMastodonFederation(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)

	marker := fmt.Sprintf("gargoyle-to-mastodon-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "hello from gargoyle "+marker, "public")
	waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=40", marker)

	mentionMarker := fmt.Sprintf("mastodon-to-gargoyle-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.mastodon, s.mastodonToken, "@alice@gargoyle.test hello from mastodon "+mentionMarker, "public")
	waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=40", mentionMarker)
}

func TestMastodonFollowUnfollow(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)
	resp, body, err := s.mastodon.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID)+"/unfollow", s.mastodonToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "Mastodon unfollow removed from Gargoyle", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var followers []shared.Account
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", s.gargoyleToken, &followers)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		for _, follower := range followers {
			if follower.Acct == "bob@mastodon.test" || follower.Username == "bob" {
				return struct{}{}, false, nil
			}
		}
		return struct{}{}, true, nil
	})
}

func TestMastodonVisibilityDeliveryAndInteractions(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)
	ensureGargoyleFollowsMastodon(t, s)

	for _, visibility := range []string{"public", "unlisted", "private", "direct"} {
		t.Run(visibility, func(t *testing.T) {
			marker := fmt.Sprintf("mastodon-visibility-%s-%d", visibility, time.Now().UnixNano())
			status := "visibility " + visibility + " " + marker
			if visibility == "direct" {
				status = "@bob@mastodon.test " + status
			}
			postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, status, visibility)
			waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", marker)
		})
	}

	marker := fmt.Sprintf("mastodon-interactions-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "interaction target "+marker, "public")
	remote := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", marker)
	resp, body, err := s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/favourite", s.mastodonToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.mastodonToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=80", marker)
}

func TestMastodonFollowRequests(t *testing.T) {
	s := setupSuite(t)

	// Mastodon -> locked Gargoyle: request is pending until Gargoyle explicitly accepts.
	_, _, _ = s.mastodon.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID)+"/unfollow", s.mastodonToken, url.Values{}, nil)
	waitForAccountAbsent(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "bob@mastodon.test")
	var updated shared.Account
	resp, body, err := s.gargoyle.PatchForm(s.ctx, "/api/v1/accounts/update_credentials", s.gargoyleToken, url.Values{"locked": {"true"}}, &updated)
	shared.Require2xx(t, resp, body, err)
	defer func() {
		_, _, _ = s.gargoyle.PatchForm(context.Background(), "/api/v1/accounts/update_credentials", s.gargoyleToken, url.Values{"locked": {"false"}}, nil)
	}()

	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID)+"/follow", s.mastodonToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	requester := waitForAccountInList(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/follow_requests", "bob@mastodon.test")
	assertAccountAbsent(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "bob@mastodon.test", 3*time.Second)
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/follow_requests/"+url.PathEscape(requester.ID)+"/authorize", s.gargoyleToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForAccountInList(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "bob@mastodon.test")

	// Reject path: a second Mastodon account requests locked Gargoyle and is explicitly rejected.
	setupMastodonAccount(t, "charlie", env("MASTODON_PASSWORD", "Str0ngP@ssword!"))
	mastodonApp, rerr := shared.RegisterApp(s.ctx, s.mastodon)
	mustNoResponseError(t, rerr)
	charlieToken := mastodonAccessToken(t, "charlie", mastodonApp)
	charlieAlice := searchAccount(t, s.ctx, s.mastodon, charlieToken, "alice@gargoyle.test")
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(charlieAlice.ID)+"/follow", charlieToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	charlieReq := waitForAccountInList(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/follow_requests", "charlie@mastodon.test")
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/follow_requests/"+url.PathEscape(charlieReq.ID)+"/reject", s.gargoyleToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	assertAccountAbsent(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "charlie@mastodon.test", 3*time.Second)

	// Gargoyle -> locked Mastodon: Gargoyle relationship is requested until Mastodon accepts.
	lockedUser := fmt.Sprintf("locked%d", time.Now().UnixNano())
	setupMastodonAccount(t, lockedUser, env("MASTODON_PASSWORD", "Str0ngP@ssword!"))
	setMastodonLocked(t, lockedUser, true)
	lockedApp, rerr := shared.RegisterApp(s.ctx, s.mastodon)
	mustNoResponseError(t, rerr)
	lockedToken := mastodonAccessToken(t, lockedUser, lockedApp)
	lockedOnGargoyle := searchAccount(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedUser+"@mastodon.test")
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(lockedOnGargoyle.ID)+"/follow", s.gargoyleToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForRelationship(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedOnGargoyle.ID, func(rel relationship) bool { return rel.Requested })
	aliceReq := waitForAccountInList(t, s.ctx, s.mastodon, lockedToken, "/api/v1/follow_requests", "alice@gargoyle.test")
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/follow_requests/"+url.PathEscape(aliceReq.ID)+"/authorize", lockedToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForRelationship(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedOnGargoyle.ID, func(rel relationship) bool { return rel.Following })
}

func TestMastodonDirectMessages(t *testing.T) {
	s := setupSuite(t)
	setupMastodonAccount(t, "charlie", env("MASTODON_PASSWORD", "Str0ngP@ssword!"))
	mastodonApp, rerr := shared.RegisterApp(s.ctx, s.mastodon)
	mustNoResponseError(t, rerr)
	charlieToken := mastodonAccessToken(t, "charlie", mastodonApp)

	gargoyleMarker := fmt.Sprintf("dm-gargoyle-to-mastodon-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "@bob@mastodon.test secret from gargoyle "+gargoyleMarker, "direct")
	mastodonStatusID := waitForMastodonStatusID(t, s.ctx, gargoyleMarker)
	requireStatusAccessible(t, s.ctx, s.mastodon, s.mastodonToken, mastodonStatusID, gargoyleMarker)
	requireStatusInaccessible(t, s.ctx, s.mastodon, charlieToken, mastodonStatusID)
	assertStatusAbsent(t, s.ctx, s.mastodon, charlieToken, "/api/v1/timelines/home?limit=80", gargoyleMarker, 4*time.Second)
	assertStatusAbsent(t, s.ctx, s.mastodon, charlieToken, "/api/v1/timelines/public?limit=80", gargoyleMarker, 4*time.Second)

	mastodonMarker := fmt.Sprintf("dm-mastodon-to-gargoyle-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.mastodon, s.mastodonToken, "@alice@gargoyle.test secret from mastodon "+mastodonMarker, "direct")
	waitForConversation(t, s.ctx, s.gargoyle, s.gargoyleToken, mastodonMarker)
	assertStatusAbsent(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=80", mastodonMarker, 4*time.Second)
}

func TestMastodonDeleteProfileUpdateAndUnsignedInbox(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)

	payload := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "type": "Follow", "actor": "http://mastodon.test/users/bob", "object": "http://gargoyle.test/users/alice"}
	resp, body, err := s.gargoyle.PostJSON(s.ctx, "/users/alice/inbox", "", payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 400 {
		t.Fatalf("unsigned inbox POST should be rejected, got %d: %s", resp.StatusCode, body)
	}

	deleteMarker := fmt.Sprintf("mastodon-delete-%d", time.Now().UnixNano())
	toDelete := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "temporary "+deleteMarker, "public")
	remoteDelete := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", deleteMarker)
	resp, body, err = s.gargoyle.Delete(s.ctx, "/api/v1/statuses/"+url.PathEscape(toDelete.ID), s.gargoyleToken, nil)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "delete propagated to Mastodon", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var status shared.Status
		resp, _, err := s.mastodon.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(remoteDelete.ID), s.mastodonToken, &status)
		if err != nil {
			return struct{}{}, false, nil
		}
		return struct{}{}, resp.StatusCode == http.StatusNotFound || !strings.Contains(status.Content, deleteMarker), nil
	})

	marker := fmt.Sprintf("mastodon-profile-%d", time.Now().UnixNano())
	var account shared.Account
	resp, body, err = s.gargoyle.PatchForm(s.ctx, "/api/v1/accounts/update_credentials", s.gargoyleToken, url.Values{"display_name": {"Alice Mastodon " + marker}, "note": {"bio " + marker}}, &account)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "profile update federates to Mastodon", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var remote shared.Account
		resp, _, err := s.mastodon.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID), s.mastodonToken, &remote)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		return struct{}{}, strings.Contains(remote.DisplayName, marker) || strings.Contains(remote.Note, marker), nil
	})
}

func searchAccount(t testing.TB, ctx context.Context, client shared.Client, token, acct string) shared.Account {
	t.Helper()
	var search shared.SearchResponse
	resp, body, err := client.GetJSON(ctx, "/api/v2/search?q="+url.QueryEscape(acct)+"&type=accounts&resolve=true", token, &search)
	shared.Require2xx(t, resp, body, err)
	for _, account := range search.Accounts {
		if account.Acct == acct || account.Acct == strings.TrimPrefix(acct, "@") {
			return account
		}
	}
	t.Fatalf("account %q not found in search result: %+v", acct, search.Accounts)
	return shared.Account{}
}

func waitForHTTP(t testing.TB, ctx context.Context, client shared.Client, path string) {
	t.Helper()
	shared.WaitFor(ctx, path, 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		resp, _, err := client.GetJSON(ctx, path, "", nil)
		if err != nil {
			return struct{}{}, false, err
		}
		return struct{}{}, resp.StatusCode >= 200 && resp.StatusCode < 300, nil
	})
}

func setupMastodonAccount(t testing.TB, username, password string) {
	t.Helper()
	runner := fmt.Sprintf(`u = User.find_by(email: '%[1]s@mastodon.test') || User.create!(email: '%[1]s@mastodon.test', password: '%[2]s', password_confirmation: '%[2]s', confirmed_at: Time.now.utc, approved: true, agreement: true, account: Account.new(username: '%[1]s')); u.update!(password: '%[2]s', password_confirmation: '%[2]s', confirmed_at: Time.now.utc, approved: true); u.account.update!(suspended_at: nil); u.save!`, username, password)
	mastodonRails(t, runner)
}

func mastodonAccessToken(t testing.TB, username string, app shared.AppCredentials) string {
	t.Helper()
	runner := fmt.Sprintf(`user = User.find_by!(email: '%s@mastodon.test'); app = Doorkeeper::Application.find_by!(uid: '%s'); token = Doorkeeper::AccessToken.create!(application: app, resource_owner_id: user.id, scopes: 'read write follow'); puts token.token`, username, app.ClientID)
	return strings.TrimSpace(mastodonRails(t, runner))
}

func setMastodonLocked(t testing.TB, username string, locked bool) {
	t.Helper()
	runner := fmt.Sprintf(`Account.find_by!(username: '%s').update!(locked: %t)`, username, locked)
	mastodonRails(t, runner)
}

func mastodonRails(t testing.TB, runner string) string {
	t.Helper()
	cmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "exec", "-T", "mastodon-web", "su", "mastodon", "-s", "/bin/bash", "-c", "bundle exec rails runner "+shellQuote(runner))
	cmd.Dir = integrationDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose mastodon rails runner failed: %v\n%s", err, out)
	}
	return string(out)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ensureMastodonFollowsGargoyle(t testing.TB, s suite) {
	t.Helper()
	resp, body, err := s.mastodon.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID)+"/follow", s.mastodonToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForAccountInList(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "bob@mastodon.test")
	waitForRelationship(t, s.ctx, s.mastodon, s.mastodonToken, s.aliceOnMastodon.ID, func(rel relationship) bool { return rel.Following })
}

func ensureGargoyleFollowsMastodon(t testing.TB, s suite) {
	t.Helper()
	_, _, _ = s.gargoyle.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.bobOnGargoyle.ID)+"/follow", s.gargoyleToken, url.Values{}, nil)
	shared.WaitFor(s.ctx, "Gargoyle outbound follow to Mastodon is tracked", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var relationships []struct {
			ID        string `json:"id"`
			Following bool   `json:"following"`
			Requested bool   `json:"requested"`
		}
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/accounts/relationships?id="+url.QueryEscape(s.bobOnGargoyle.ID), s.gargoyleToken, &relationships)
		return struct{}{}, err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && len(relationships) > 0 && (relationships[0].Following || relationships[0].Requested), nil
	})
}

func waitForAccountInList(t testing.TB, ctx context.Context, client shared.Client, token, path, acct string) shared.Account {
	t.Helper()
	return shared.WaitFor(ctx, acct+" in "+path, 2*time.Second, func(ctx context.Context) (shared.Account, bool, error) {
		var accounts []shared.Account
		resp, _, err := client.GetJSON(ctx, path, token, &accounts)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Account{}, false, err
		}
		for _, account := range accounts {
			if account.Acct == acct || account.Username == strings.Split(acct, "@")[0] {
				return account, true, nil
			}
		}
		return shared.Account{}, false, nil
	})
}

func waitForAccountAbsent(t testing.TB, ctx context.Context, client shared.Client, token, path, acct string) {
	t.Helper()
	shared.WaitFor(ctx, acct+" absent from "+path, 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		return struct{}{}, !accountPresent(ctx, client, token, path, acct), nil
	})
}

func assertAccountAbsent(t testing.TB, ctx context.Context, client shared.Client, token, path, acct string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if accountPresent(ctx, client, token, path, acct) {
			t.Fatalf("account %q unexpectedly present in %s", acct, path)
		}
		time.Sleep(1 * time.Second)
	}
}

func accountPresent(ctx context.Context, client shared.Client, token, path, acct string) bool {
	var accounts []shared.Account
	resp, _, err := client.GetJSON(ctx, path, token, &accounts)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	for _, account := range accounts {
		if account.Acct == acct || account.Username == strings.Split(acct, "@")[0] {
			return true
		}
	}
	return false
}

type relationship struct {
	ID         string `json:"id"`
	Following  bool   `json:"following"`
	Requested  bool   `json:"requested"`
	FollowedBy bool   `json:"followed_by"`
}

func waitForRelationship(t testing.TB, ctx context.Context, client shared.Client, token, accountID string, predicate func(relationship) bool) relationship {
	t.Helper()
	return shared.WaitFor(ctx, "relationship for "+accountID, 2*time.Second, func(ctx context.Context) (relationship, bool, error) {
		var relationships []relationship
		resp, _, err := client.GetJSON(ctx, "/api/v1/accounts/relationships?id="+url.QueryEscape(accountID), token, &relationships)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || len(relationships) == 0 {
			return relationship{}, false, err
		}
		return relationships[0], predicate(relationships[0]), nil
	})
}

func waitForStatus(t testing.TB, ctx context.Context, client shared.Client, token, path, marker string) shared.Status {
	t.Helper()
	return shared.WaitFor(ctx, marker+" in "+path, 2*time.Second, func(ctx context.Context) (shared.Status, bool, error) {
		var statuses []shared.Status
		resp, _, err := client.GetJSON(ctx, path, token, &statuses)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Status{}, false, err
		}
		for _, status := range statuses {
			if strings.Contains(status.Content, marker) || (status.Reblog != nil && strings.Contains(status.Reblog.Content, marker)) {
				return status, true, nil
			}
		}
		return shared.Status{}, false, nil
	})
}

func postStatus(t testing.TB, ctx context.Context, client shared.Client, token, content, visibility string) shared.Status {
	t.Helper()
	var status shared.Status
	resp, body, err := client.PostForm(ctx, "/api/v1/statuses", token, url.Values{"status": {content}, "visibility": {visibility}}, &status)
	shared.Require2xx(t, resp, body, err)
	return status
}

func assertStatusAbsent(t testing.TB, ctx context.Context, client shared.Client, token, path, marker string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var statuses []shared.Status
		resp, _, err := client.GetJSON(ctx, path, token, &statuses)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			for _, status := range statuses {
				if statusContains(status, marker) {
					t.Fatalf("status %q unexpectedly present in %s: %+v", marker, path, status)
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

type conversation struct {
	ID         string           `json:"id"`
	Unread     bool             `json:"unread"`
	Accounts   []shared.Account `json:"accounts"`
	LastStatus shared.Status    `json:"last_status"`
}

func waitForConversation(t testing.TB, ctx context.Context, client shared.Client, token, marker string) conversation {
	t.Helper()
	return shared.WaitFor(ctx, marker+" in conversations", 2*time.Second, func(ctx context.Context) (conversation, bool, error) {
		items, err := listConversations(ctx, client, token)
		if err != nil {
			return conversation{}, false, err
		}
		for _, item := range items {
			if statusContains(item.LastStatus, marker) {
				return item, true, nil
			}
		}
		return conversation{}, false, nil
	})
}

func waitForMastodonStatusID(t testing.TB, ctx context.Context, marker string) string {
	t.Helper()
	return shared.WaitFor(ctx, marker+" stored by Mastodon", 2*time.Second, func(ctx context.Context) (string, bool, error) {
		out := mastodonRails(t, fmt.Sprintf(`status = Status.where("text LIKE ?", '%%%s%%').order(id: :desc).first; puts status&.id`, marker))
		id := strings.TrimSpace(out)
		return id, id != "", nil
	})
}

func requireStatusAccessible(t testing.TB, ctx context.Context, client shared.Client, token, id, marker string) {
	t.Helper()
	var status shared.Status
	resp, body, err := client.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(id), token, &status)
	shared.Require2xx(t, resp, body, err)
	if !strings.Contains(status.Content, marker) {
		t.Fatalf("status %s did not contain marker %q: %+v", id, marker, status)
	}
}

func requireStatusInaccessible(t testing.TB, ctx context.Context, client shared.Client, token, id string) {
	t.Helper()
	var status shared.Status
	resp, body, err := client.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(id), token, &status)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 400 {
		t.Fatalf("status %s should not be accessible, got %d: %s", id, resp.StatusCode, body)
	}
}

func assertConversationAbsent(t testing.TB, ctx context.Context, client shared.Client, token, marker string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		items, err := listConversations(ctx, client, token)
		if err == nil {
			for _, item := range items {
				if statusContains(item.LastStatus, marker) {
					t.Fatalf("conversation %q unexpectedly present: %+v", marker, item)
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func listConversations(ctx context.Context, client shared.Client, token string) ([]conversation, error) {
	var items []conversation
	resp, _, err := client.GetJSON(ctx, "/api/v1/conversations?limit=20", token, &items)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("conversations returned status %d", resp.StatusCode)
	}
	return items, nil
}

func statusContains(status shared.Status, marker string) bool {
	return strings.Contains(status.Content, marker) || (status.Reblog != nil && strings.Contains(status.Reblog.Content, marker))
}

func dockerCompose(t testing.TB, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"compose", "-f", "docker-compose.yml"}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = integrationDir(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s failed: %v\n%s", strings.Join(cmdArgs, " "), err, out)
	}
	return string(out)
}

func resetIntegrationStackOnce() error {
	if err := dockerComposeDir("down", "-v", "--remove-orphans", "--timeout", "0"); err != nil {
		// Continue with hard cleanup below.
	}
	cleanup := exec.Command("bash", "--noprofile", "--norc", "-lc", strings.Join([]string{
		"docker rm -f gargoyle-integration-mastodon-proxy-1 gargoyle-integration-mastodon-gargoyle-1 gargoyle-integration-mastodon-postgres-1 gargoyle-integration-mastodon-redis-1 gargoyle-integration-mastodon-mastodon-web-1 gargoyle-integration-mastodon-mastodon-sidekiq-1 2>/dev/null || true",
		"docker network rm gargoyle-integration-mastodon_default 2>/dev/null || true",
		"docker volume rm gargoyle-integration-mastodon_gargoyle-data gargoyle-integration-mastodon_gargoyle-media gargoyle-integration-mastodon_mastodon-postgres gargoyle-integration-mastodon_mastodon-redis gargoyle-integration-mastodon_mastodon-public 2>/dev/null || true",
		"sleep 1",
	}, "; "))
	cleanup.Dir = integrationDirFromWD()
	_, _ = cleanup.CombinedOutput()
	return dockerComposeDir("up", "-d", "--build")
}

func dockerComposeDir(args ...string) error {
	cmdArgs := append([]string{"compose", "-f", "docker-compose.yml"}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Dir = integrationDirFromWD()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s failed: %w\n%s", strings.Join(cmdArgs, " "), err, out)
	}
	return nil
}

func integrationDir(t testing.TB) string {
	t.Helper()
	return integrationDirFromWD()
}

func integrationDirFromWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(wd) == "mastodon" {
		return wd
	}
	return filepath.Join(wd, "integration", "mastodon")
}
