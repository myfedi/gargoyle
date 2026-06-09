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
	mastodonApp     shared.AppCredentials
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
	s.mastodonApp = mastodonApp
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
	charlieToken := mastodonAccessToken(t, "charlie", s.mastodonApp)
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
	lockedToken := mastodonAccessToken(t, lockedUser, s.mastodonApp)
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
	charlieToken := mastodonAccessToken(t, "charlie", s.mastodonApp)

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

func TestMastodonReplyContextMediaEditPolls(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)
	ensureGargoyleFollowsMastodon(t, s)

	rootMarker := fmt.Sprintf("mastodon-context-%d", time.Now().UnixNano())
	root := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "context root "+rootMarker, "public")
	remoteRoot := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", rootMarker)
	replyMarker := fmt.Sprintf("mastodon-reply-%d", time.Now().UnixNano())
	postStatusForm(t, s.ctx, s.mastodon, s.mastodonToken, url.Values{"status": {"@alice@gargoyle.test reply " + replyMarker}, "visibility": {"public"}, "in_reply_to_id": {remoteRoot.ID}})
	shared.WaitFor(s.ctx, "Mastodon reply appears in Gargoyle context", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var contextResp shared.StatusContext
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(root.ID)+"/context", s.gargoyleToken, &contextResp)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		for _, descendant := range contextResp.Descendants {
			if strings.Contains(descendant.Content, replyMarker) {
				return struct{}{}, true, nil
			}
		}
		return struct{}{}, false, nil
	})

	media := uploadOnePixel(t, s.ctx, s.gargoyle, s.gargoyleToken)
	mediaMarker := fmt.Sprintf("mastodon-media-%d", time.Now().UnixNano())
	postStatusForm(t, s.ctx, s.gargoyle, s.gargoyleToken, url.Values{"status": {"media status " + mediaMarker}, "visibility": {"public"}, "media_ids[]": {media.ID}, "media_ids": {media.ID}})
	remoteMedia := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", mediaMarker)
	if len(remoteMedia.MediaAttachments) == 0 && !strings.Contains(remoteMedia.Content, media.URL) {
		t.Fatalf("Mastodon media status had no attachment and did not preserve media URL %q: %+v", media.URL, remoteMedia)
	}

	editMarker := fmt.Sprintf("mastodon-edit-%d", time.Now().UnixNano())
	created := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "original "+editMarker, "public")
	remoteEdit := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", editMarker)
	edited := "edited " + editMarker
	var updated shared.Status
	resp, body, err := s.gargoyle.PatchForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(created.ID), s.gargoyleToken, url.Values{"status": {edited}, "visibility": {"public"}}, &updated)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "status edit federates to Mastodon", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var status shared.Status
		resp, _, err := s.mastodon.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(remoteEdit.ID), s.mastodonToken, &status)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		return struct{}{}, strings.Contains(status.Content, edited), nil
	})

	pollMarker := fmt.Sprintf("mastodon-poll-%d", time.Now().UnixNano())
	localPoll := postStatusForm(t, s.ctx, s.gargoyle, s.gargoyleToken, url.Values{"status": {"poll from gargoyle " + pollMarker}, "visibility": {"public"}, "activitypub_type": {"Question"}, "poll[options][]": {"red " + pollMarker, "blue " + pollMarker}, "poll[expires_in]": {"3600"}, "poll[multiple]": {"false"}})
	if localPoll.Poll == nil || len(localPoll.Poll.Options) != 2 {
		t.Fatalf("created Gargoyle status did not include poll: %+v", localPoll)
	}
	remotePoll := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", pollMarker)
	if remotePoll.Poll == nil || len(remotePoll.Poll.Options) != 2 {
		t.Fatalf("Mastodon did not receive Gargoyle poll as poll: %+v", remotePoll)
	}
	var voted shared.Poll
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/polls/"+url.PathEscape(remotePoll.Poll.ID)+"/votes", s.mastodonToken, url.Values{"choices[]": {"0"}}, &voted)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "Mastodon poll vote reaches Gargoyle", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var status shared.Status
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(localPoll.ID), s.gargoyleToken, &status)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 || status.Poll == nil || len(status.Poll.Options) == 0 {
			return struct{}{}, false, err
		}
		return struct{}{}, status.Poll.Options[0].VotesCount > 0, nil
	})

	mastodonPollMarker := fmt.Sprintf("mastodon-origin-poll-%d", time.Now().UnixNano())
	postStatusForm(t, s.ctx, s.mastodon, s.mastodonToken, url.Values{"status": {"@alice@gargoyle.test poll from mastodon " + mastodonPollMarker}, "visibility": {"public"}, "poll[options][]": {"cat " + mastodonPollMarker, "dog " + mastodonPollMarker}, "poll[expires_in]": {"3600"}, "poll[multiple]": {"false"}})
	localRemotePoll := waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=100", mastodonPollMarker)
	if localRemotePoll.Poll == nil || len(localRemotePoll.Poll.Options) != 2 {
		t.Fatalf("Gargoyle did not receive Mastodon poll as poll: %+v", localRemotePoll)
	}
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/polls/"+url.PathEscape(localRemotePoll.Poll.ID)+"/votes", s.gargoyleToken, url.Values{"choices[]": {"1"}}, &voted)
	shared.Require2xx(t, resp, body, err)
}

func TestMastodonUnfavouriteUnboostAndBoostVisibility(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)

	marker := fmt.Sprintf("mastodon-undo-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "undo target "+marker, "public")
	remote := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=80", marker)
	var interacted shared.Status
	resp, body, err := s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/favourite", s.mastodonToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if !interacted.Favourited {
		t.Fatalf("expected favourite response to set favourited")
	}
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/unfavourite", s.mastodonToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if interacted.Favourited {
		t.Fatalf("expected unfavourite response to clear favourited")
	}
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.mastodonToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if !interacted.Reblogged {
		t.Fatalf("expected reblog response to set reblogged")
	}
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/unreblog", s.mastodonToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if interacted.Reblogged {
		t.Fatalf("expected unreblog response to clear reblogged")
	}

	for _, tc := range []struct {
		name, visibility, content string
		wantReblog                bool
	}{{"public", "public", "boost public", true}, {"unlisted", "unlisted", "boost unlisted", true}, {"private", "private", "boost private", false}, {"direct", "direct", "@bob@mastodon.test boost direct", false}} {
		t.Run(tc.name, func(t *testing.T) {
			boostMarker := fmt.Sprintf("mastodon-boost-%s-%d", tc.name, time.Now().UnixNano())
			postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, tc.content+" "+boostMarker, tc.visibility)
			remote := waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=100", boostMarker)
			var result shared.Status
			resp, body, err := s.mastodon.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.mastodonToken, url.Values{}, &result)
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantReblog {
				shared.Require2xx(t, resp, body, err)
				if !result.Reblogged {
					t.Fatalf("expected reblogged=true")
				}
				return
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 && result.Reblogged {
				t.Fatalf("expected %s boost rejected or not reblogged; got %d %+v", tc.visibility, resp.StatusCode, result)
			}
		})
	}
}

func TestMastodonNonLeakRetryHardeningAndFollowerDelivery(t *testing.T) {
	s := setupSuite(t)
	ensureMastodonFollowsGargoyle(t, s)
	ensureGargoyleFollowsMastodon(t, s)
	charlieToken := mastodonUserToken(t, "charlie", s.mastodonApp)

	privateMarker := fmt.Sprintf("mastodon-private-nonleak-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "private nonleak "+privateMarker, "private")
	waitForStatus(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/timelines/home?limit=100", privateMarker)
	assertStatusAbsent(t, s.ctx, s.mastodon, charlieToken, "/api/v1/timelines/home?limit=100", privateMarker, 5*time.Second)
	assertStatusAbsent(t, s.ctx, s.mastodon, charlieToken, "/api/v1/timelines/public?limit=100", privateMarker, 5*time.Second)

	directMarker := fmt.Sprintf("mastodon-direct-nonleak-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "@bob@mastodon.test direct nonleak "+directMarker, "direct")
	mastodonDirectID := waitForMastodonStatusID(t, s.ctx, directMarker)
	requireStatusAccessible(t, s.ctx, s.mastodon, s.mastodonToken, mastodonDirectID, directMarker)
	requireStatusInaccessible(t, s.ctx, s.mastodon, charlieToken, mastodonDirectID)

	normalMarker := fmt.Sprintf("mastodon-follower-normal-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.mastodon, s.mastodonToken, "normal follower delivery "+normalMarker, "public")
	waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=100", normalMarker)

	var search shared.SearchResponse
	resp, _, err := s.gargoyle.GetJSON(s.ctx, "/api/v2/search?q="+url.QueryEscape("mallory@127.0.0.1")+"&type=accounts&resolve=true", s.gargoyleToken, &search)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && len(search.Accounts) > 0 {
		t.Fatalf("unexpectedly resolved unconfigured private host: %+v", search.Accounts)
	}

	dockerCompose(t, "stop", "mastodon-sidekiq", "mastodon-web")
	defer dockerCompose(t, "up", "-d", "mastodon-web", "mastodon-sidekiq")
	retryMarker := fmt.Sprintf("mastodon-retry-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "retry while mastodon down "+retryMarker, "public")
	time.Sleep(7 * time.Second)
	dockerCompose(t, "exec", "-T", "gargoyle", "sqlite3", "/data/gargoyle.db", "update delivery_jobs set next_attempt_at=datetime('now','-1 minute'), status='pending' where status='pending'")
	dockerCompose(t, "up", "-d", "mastodon-web", "mastodon-sidekiq")
	waitForHTTP(t, s.ctx, s.mastodon, "/health")
	shared.WaitFor(s.ctx, "retry delivery reaches Mastodon", 6*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		dockerCompose(t, "exec", "-T", "gargoyle", "sqlite3", "/data/gargoyle.db", "update delivery_jobs set next_attempt_at=datetime('now','-1 minute'), status='pending' where status='pending'")
		var statuses []shared.Status
		resp, _, err := s.mastodon.GetJSON(ctx, "/api/v1/timelines/home?limit=100", s.mastodonToken, &statuses)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		for _, status := range statuses {
			if statusContains(status, retryMarker) {
				return struct{}{}, true, nil
			}
		}
		return struct{}{}, false, nil
	})
}

func TestMastodonAdditionalDirectAndLockedFlows(t *testing.T) {
	s := setupSuite(t)
	charlieToken := mastodonUserToken(t, "charlie", s.mastodonApp)

	lockedRejectUser := fmt.Sprintf("lr%d", time.Now().UnixNano()%1_000_000_000)
	setupMastodonAccount(t, lockedRejectUser, env("MASTODON_PASSWORD", "Str0ngP@ssword!"))
	setMastodonLocked(t, lockedRejectUser, true)
	lockedRejectToken := mastodonUserToken(t, lockedRejectUser, s.mastodonApp)
	lockedRejectOnGargoyle := searchAccount(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedRejectUser+"@mastodon.test")
	resp, body, err := s.gargoyle.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(lockedRejectOnGargoyle.ID)+"/follow", s.gargoyleToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForRelationship(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedRejectOnGargoyle.ID, func(rel relationship) bool { return rel.Requested })
	aliceReq := waitForAccountInList(t, s.ctx, s.mastodon, lockedRejectToken, "/api/v1/follow_requests", "alice@gargoyle.test")
	resp, body, err = s.mastodon.PostForm(s.ctx, "/api/v1/follow_requests/"+url.PathEscape(aliceReq.ID)+"/reject", lockedRejectToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForRelationship(t, s.ctx, s.gargoyle, s.gargoyleToken, lockedRejectOnGargoyle.ID, func(rel relationship) bool { return !rel.Following && !rel.Requested })

	multiMarker := fmt.Sprintf("mastodon-multi-dm-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "@bob@mastodon.test @charlie@mastodon.test multi secret "+multiMarker, "direct")
	multiID := waitForMastodonStatusID(t, s.ctx, multiMarker)
	requireStatusAccessible(t, s.ctx, s.mastodon, s.mastodonToken, multiID, multiMarker)
	requireStatusAccessible(t, s.ctx, s.mastodon, charlieToken, multiID, multiMarker)

	noMentionMarker := fmt.Sprintf("mastodon-invalid-dm-%d", time.Now().UnixNano())
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/statuses", s.gargoyleToken, url.Values{"status": {"no valid local recipient " + noMentionMarker}, "visibility": {"direct"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 400 {
		t.Fatalf("direct status without recipient should fail, got %d: %s", resp.StatusCode, body)
	}

	profileMarker := fmt.Sprintf("mastodon-profile-inbound-%d", time.Now().UnixNano())
	var account shared.Account
	resp, body, err = s.mastodon.PatchForm(s.ctx, "/api/v1/accounts/update_credentials", s.mastodonToken, url.Values{"display_name": {"Bob " + profileMarker[:20]}, "note": {"bio " + profileMarker}, "locked": {"true"}, "fields_attributes[0][name]": {"Website"}, "fields_attributes[0][value]": {"http://bob.example/" + profileMarker}}, &account)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "Mastodon profile update reaches Gargoyle", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var remote shared.Account
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.bobOnGargoyle.ID), s.gargoyleToken, &remote)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		return struct{}{}, (strings.Contains(remote.DisplayName, profileMarker) || strings.Contains(remote.Note, profileMarker)) && accountHasField(remote, "Website", profileMarker), nil
	})
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
	resp, body, err = s.gargoyle.PatchForm(s.ctx, "/api/v1/accounts/update_credentials", s.gargoyleToken, url.Values{"display_name": {"Alice Mastodon " + marker}, "note": {"bio " + marker}, "fields_attributes[0][name]": {"Website"}, "fields_attributes[0][value]": {"http://alice.example/" + marker}}, &account)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "profile update federates to Mastodon", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var remote shared.Account
		resp, _, err := s.mastodon.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnMastodon.ID), s.mastodonToken, &remote)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		return struct{}{}, (strings.Contains(remote.DisplayName, marker) || strings.Contains(remote.Note, marker)) && accountHasField(remote, "Website", marker), nil
	})
}

func accountHasField(account shared.Account, name, valueSubstring string) bool {
	for _, field := range account.Fields {
		if field.Name == name && strings.Contains(field.Value, valueSubstring) {
			return true
		}
	}
	return false
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

func mastodonUserToken(t testing.TB, username string, app shared.AppCredentials) string {
	t.Helper()
	setupMastodonAccount(t, username, env("MASTODON_PASSWORD", "Str0ngP@ssword!"))
	return mastodonAccessToken(t, username, app)
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
	waitForRelationship(t, s.ctx, s.gargoyle, s.gargoyleToken, s.bobOnGargoyle.ID, func(rel relationship) bool { return rel.Following })
	waitForAccountInList(t, s.ctx, s.mastodon, s.mastodonToken, "/api/v1/accounts/"+url.PathEscape(s.bob.ID)+"/followers", "alice@gargoyle.test")
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
	return postStatusForm(t, ctx, client, token, url.Values{"status": {content}, "visibility": {visibility}})
}

func postStatusForm(t testing.TB, ctx context.Context, client shared.Client, token string, form url.Values) shared.Status {
	t.Helper()
	var status shared.Status
	resp, body, err := client.PostForm(ctx, "/api/v1/statuses", token, form, &status)
	shared.Require2xx(t, resp, body, err)
	return status
}

func uploadOnePixel(t testing.TB, ctx context.Context, client shared.Client, token string) shared.MediaAttachment {
	t.Helper()
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 153, 99, 248, 15, 4, 0, 9, 251, 3, 253, 167, 89, 85, 191, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
	var media shared.MediaAttachment
	resp, body, err := client.PostMultipart(ctx, "/api/v2/media", token, map[string]string{"description": "one red pixel"}, "file", "pixel.png", "image/png", png, &media)
	shared.Require2xx(t, resp, body, err)
	return media
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

func accountEventuallyContains(ctx context.Context, client shared.Client, token, path, marker string, duration time.Duration) bool {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var account shared.Account
		resp, _, err := client.GetJSON(ctx, path, token, &account)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && (strings.Contains(account.DisplayName, marker) || strings.Contains(account.Note, marker)) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func statusEventuallyContains(ctx context.Context, client shared.Client, token, path, marker string, duration time.Duration) bool {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var status shared.Status
		resp, _, err := client.GetJSON(ctx, path, token, &status)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && statusContains(status, marker) {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func statusEventuallyInTimeline(ctx context.Context, client shared.Client, token, path, marker string, duration time.Duration) bool {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var statuses []shared.Status
		resp, _, err := client.GetJSON(ctx, path, token, &statuses)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			for _, status := range statuses {
				if statusContains(status, marker) {
					return true
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return false
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
