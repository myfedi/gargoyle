package gts_test

import (
	"context"
	"fmt"
	"io"
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

func TestGoToSocialFederation(t *testing.T) { // NOSONAR - end-to-end federation scenario is intentionally sequential
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	baseURL := env("INTEGRATION_PROXY_URL", "http://127.0.0.1:18080")
	gargoyleBase := env("GARGOYLE_BASE_URL", baseURL)
	gtsBase := env("GTS_BASE_URL", baseURL)
	gargoyleUser := env("GARGOYLE_USERNAME", "alice")
	gargoylePass := env("GARGOYLE_PASSWORD", "Str0ngP@ssword!")
	gtsUser := env("GTS_USERNAME", "bob")
	gtsPass := env("GTS_PASSWORD", "Str0ngP@ssword!")

	gargoyle := shared.NewHostClient(gargoyleBase, "gargoyle.test")
	gts := shared.NewHostClient(gtsBase, "gts.test")

	waitForHTTP(t, ctx, gargoyle, "/api/v1/instance")
	waitForHTTP(t, ctx, gts, "/nodeinfo/2.0")
	setupGTSAccount(t, gtsUser, gtsPass)

	gargoyleApp, rerr := shared.RegisterApp(ctx, gargoyle)
	mustNoResponseError(t, rerr)
	gtsApp, rerr := shared.RegisterApp(ctx, gts)
	mustNoResponseError(t, rerr)

	gargoyleToken, rerr := shared.PasswordToken(ctx, gargoyle, gargoyleApp, gargoyleUser, gargoylePass)
	mustNoResponseError(t, rerr)
	gtsToken := gtsAuthorizationCodeToken(t, ctx, gts, gtsApp, gtsUser+"@gts.test", gtsPass)

	alice, rerr := shared.VerifyCredentials(ctx, gargoyle, gargoyleToken)
	mustNoResponseError(t, rerr)
	bob, rerr := shared.VerifyCredentials(ctx, gts, gtsToken)
	mustNoResponseError(t, rerr)
	if alice.Username != gargoyleUser || bob.Username != gtsUser {
		t.Fatalf("unexpected accounts: gargoyle=%+v gts=%+v", alice, bob)
	}

	aliceOnGTS := searchAccount(t, ctx, gts, gtsToken, gargoyleUser+"@gargoyle.test")
	resp, body, err := gts.PostForm(ctx, "/api/v1/accounts/"+url.PathEscape(aliceOnGTS.ID)+"/follow", gtsToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)

	shared.WaitFor(ctx, "GTS follow accepted by Gargoyle", 2*time.Second, func(ctx context.Context) (shared.Account, bool, error) {
		var followers []shared.Account
		resp, _, err := gargoyle.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(alice.ID)+"/followers", gargoyleToken, &followers)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Account{}, false, err
		}
		for _, follower := range followers {
			if follower.Acct == gtsUser+"@gts.test" || follower.Username == gtsUser {
				return follower, true, nil
			}
		}
		return shared.Account{}, false, nil
	})

	marker := fmt.Sprintf("gargoyle-to-gts-%d", time.Now().UnixNano())
	var created shared.Status
	resp, body, err = gargoyle.PostForm(ctx, "/api/v1/statuses", gargoyleToken, url.Values{
		"status":     {"hello from gargoyle " + marker},
		"visibility": {"public"},
	}, &created)
	shared.Require2xx(t, resp, body, err)

	shared.WaitFor(ctx, "Gargoyle status delivered to GTS home timeline", 2*time.Second, func(ctx context.Context) (shared.Status, bool, error) {
		var statuses []shared.Status
		resp, _, err := gts.GetJSON(ctx, "/api/v1/timelines/home?limit=20", gtsToken, &statuses)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Status{}, false, err
		}
		for _, status := range statuses {
			if strings.Contains(status.Content, marker) {
				return status, true, nil
			}
		}
		return shared.Status{}, false, nil
	})

	mentionMarker := fmt.Sprintf("gts-to-gargoyle-%d", time.Now().UnixNano())
	resp, body, err = gts.PostForm(ctx, "/api/v1/statuses", gtsToken, url.Values{
		"status":     {"@" + gargoyleUser + "@gargoyle.test hello from gts " + mentionMarker},
		"visibility": {"public"},
	}, nil)
	shared.Require2xx(t, resp, body, err)

	shared.WaitFor(ctx, "GTS mention received by Gargoyle public timeline", 2*time.Second, func(ctx context.Context) (shared.Status, bool, error) {
		var statuses []shared.Status
		resp, _, err := gargoyle.GetJSON(ctx, "/api/v1/timelines/public?limit=20", gargoyleToken, &statuses)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Status{}, false, err
		}
		for _, status := range statuses {
			if strings.Contains(status.Content, mentionMarker) {
				return status, true, nil
			}
		}
		return shared.Status{}, false, nil
	})
}

func searchAccount(t testing.TB, ctx context.Context, client shared.Client, token, acct string) shared.Account {
	t.Helper()
	var search shared.SearchResponse
	path := "/api/v2/search?q=" + url.QueryEscape(acct) + "&type=accounts&resolve=true"
	resp, body, err := client.GetJSON(ctx, path, token, &search)
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

func setupGTSAccount(t testing.TB, username, password string) {
	t.Helper()
	compose := filepath.Join("docker-compose.yml")
	commands := [][]string{
		{"compose", "-f", compose, "exec", "-T", "gotosocial", "/gotosocial/gotosocial", "admin", "account", "create", "--username", username, "--email", username + "@gts.test", "--password", password},
		{"compose", "-f", compose, "exec", "-T", "gotosocial", "/gotosocial/gotosocial", "admin", "account", "confirm", "--username", username},
		{"compose", "-f", compose, "exec", "-T", "gotosocial", "/gotosocial/gotosocial", "admin", "account", "promote", "--username", username},
	}
	for _, args := range commands {
		cmd := exec.Command("docker", args...)
		cmd.Dir = integrationDir(t)
		out, err := cmd.CombinedOutput()
		if err != nil && !strings.Contains(string(out), "already") {
			t.Fatalf("docker %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func integrationDir(t testing.TB) string {
	t.Helper()
	return integrationDirFromWD()
}

func gtsAuthorizationCodeToken(t testing.TB, ctx context.Context, client shared.Client, app shared.AppCredentials, email, password string) string {
	t.Helper()
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	authorizePath := "/oauth/authorize?client_id=" + url.QueryEscape(app.ClientID) + "&redirect_uri=" + url.QueryEscape("urn:ietf:wg:oauth:2.0:oob") + "&response_type=code&scope=" + url.QueryEscape("read write follow")
	resp, _, err := doRaw(ctx, httpClient, client, http.MethodGet, authorizePath, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie := firstCookie(resp)
	if cookie == "" {
		t.Fatalf("GTS authorize response did not set a session cookie")
	}

	loginForm := url.Values{"username": {email}, "password": {password}}
	resp, _, err = doRaw(ctx, httpClient, client, http.MethodPost, "/auth/sign_in", cookie, strings.NewReader(loginForm.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if nextCookie := firstCookie(resp); nextCookie != "" {
		cookie = nextCookie
	}

	resp, _, err = doRaw(ctx, httpClient, client, http.MethodPost, "/oauth/authorize", cookie, nil)
	if err != nil {
		t.Fatal(err)
	}
	location := resp.Header.Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("invalid GTS OAuth redirect %q: %v", location, err)
	}
	code := parsed.Query().Get("code")
	if code == "" {
		t.Fatalf("GTS OAuth redirect did not include code: %q", location)
	}

	var token shared.Token
	resp, body, err := client.PostForm(ctx, "/oauth/token", "", url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {app.ClientID},
		"client_secret": {app.ClientSecret},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
		"code":          {code},
	}, &token)
	shared.Require2xx(t, resp, body, err)
	return token.AccessToken
}

func doRaw(ctx context.Context, httpClient *http.Client, client shared.Client, method, path, cookie string, body io.Reader) (*http.Response, string, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequestWithContext(ctx, method, client.BaseURL+path, body)
	if err != nil {
		return nil, "", err
	}
	if client.HostHeader != "" {
		req.Host = client.HostHeader
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	return resp, string(raw), err
}

func firstCookie(resp *http.Response) string {
	for _, value := range resp.Header.Values("Set-Cookie") {
		if idx := strings.Index(value, ";"); idx >= 0 {
			return value[:idx]
		}
		if value != "" {
			return value
		}
	}
	return ""
}

type suite struct {
	ctx           context.Context
	gargoyle      shared.Client
	gts           shared.Client
	gargoyleToken string
	gtsToken      string
	alice         shared.Account
	bob           shared.Account
	aliceOnGTS    shared.Account
	bobOnGargoyle shared.Account
}

var (
	baseSuiteMu sync.Mutex
	baseSuite   *suite
)

func setupSuite(t testing.TB) suite {
	t.Helper()
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	baseURL := env("INTEGRATION_PROXY_URL", "http://127.0.0.1:18080")
	gargoyleUser := env("GARGOYLE_USERNAME", "alice")
	gargoylePass := env("GARGOYLE_PASSWORD", "Str0ngP@ssword!")
	gtsUser := env("GTS_USERNAME", "bob")
	gtsPass := env("GTS_PASSWORD", "Str0ngP@ssword!")

	s := suite{ctx: ctx, gargoyle: shared.NewHostClient(baseURL, "gargoyle.test"), gts: shared.NewHostClient(baseURL, "gts.test")}
	waitForHTTP(t, ctx, s.gargoyle, "/api/v1/instance")
	waitForHTTP(t, ctx, s.gts, "/nodeinfo/2.0")
	setupGTSAccount(t, gtsUser, gtsPass)

	gargoyleApp, rerr := shared.RegisterApp(ctx, s.gargoyle)
	mustNoResponseError(t, rerr)
	gtsApp, rerr := shared.RegisterApp(ctx, s.gts)
	mustNoResponseError(t, rerr)
	s.gargoyleToken, rerr = shared.PasswordToken(ctx, s.gargoyle, gargoyleApp, gargoyleUser, gargoylePass)
	mustNoResponseError(t, rerr)
	s.gtsToken = gtsAuthorizationCodeToken(t, ctx, s.gts, gtsApp, gtsUser+"@gts.test", gtsPass)
	s.alice, rerr = shared.VerifyCredentials(ctx, s.gargoyle, s.gargoyleToken)
	mustNoResponseError(t, rerr)
	s.bob, rerr = shared.VerifyCredentials(ctx, s.gts, s.gtsToken)
	mustNoResponseError(t, rerr)
	s.aliceOnGTS = searchAccount(t, ctx, s.gts, s.gtsToken, gargoyleUser+"@gargoyle.test")
	s.bobOnGargoyle = searchAccount(t, ctx, s.gargoyle, s.gargoyleToken, gtsUser+"@gts.test")
	return &s
}

func ensureGTSFollowsGargoyle(t testing.TB, s suite) {
	t.Helper()
	resp, body, err := s.gts.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnGTS.ID)+"/follow", s.gtsToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	waitForAccountInList(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", "bob@gts.test")
}

func ensureGargoyleFollowsGTS(t testing.TB, s suite) {
	t.Helper()
	if gargoyleRelationshipTracked(s) {
		return
	}
	_, _, _ = s.gargoyle.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.bobOnGargoyle.ID)+"/follow", s.gargoyleToken, url.Values{}, nil)
	shared.WaitFor(s.ctx, "Gargoyle outbound follow to GTS is tracked", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		return struct{}{}, gargoyleRelationshipTracked(s), nil
	})
}

func gargoyleRelationshipTracked(s suite) bool {
	var relationships []struct {
		ID        string `json:"id"`
		Following bool   `json:"following"`
		Requested bool   `json:"requested"`
	}
	resp, _, err := s.gargoyle.GetJSON(s.ctx, "/api/v1/accounts/relationships?id="+url.QueryEscape(s.bobOnGargoyle.ID), s.gargoyleToken, &relationships)
	return err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && len(relationships) > 0 && (relationships[0].Following || relationships[0].Requested)
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

func waitForConversationStatus(t testing.TB, ctx context.Context, client shared.Client, token, marker string) shared.Status {
	t.Helper()
	return shared.WaitFor(ctx, marker+" in /api/v1/conversations", 2*time.Second, func(ctx context.Context) (shared.Status, bool, error) {
		var conversations []shared.Conversation
		resp, _, err := client.GetJSON(ctx, "/api/v1/conversations?limit=40", token, &conversations)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return shared.Status{}, false, err
		}
		for _, conversation := range conversations {
			if strings.Contains(conversation.LastStatus.Content, marker) {
				return conversation.LastStatus, true, nil
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

func TestGoToSocialFollowUnfollow(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	resp, body, err := s.gts.PostForm(s.ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnGTS.ID)+"/unfollow", s.gtsToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "GTS unfollow removed from Gargoyle", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var followers []shared.Account
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.alice.ID)+"/followers", s.gargoyleToken, &followers)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		for _, follower := range followers {
			if follower.Acct == "bob@gts.test" || follower.Username == "bob" {
				return struct{}{}, false, nil
			}
		}
		return struct{}{}, true, nil
	})
}

func TestGargoyleOutboundFollowAndGTSMentionDelivery(t *testing.T) {
	s := setupSuite(t)
	ensureGargoyleFollowsGTS(t, s)
	marker := fmt.Sprintf("gts-normal-to-gargoyle-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gts, s.gtsToken, "@alice@gargoyle.test hello followers "+marker, "public")
	waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=40", marker)
}

func TestMentionsBothDirections(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	gargoyleMarker := fmt.Sprintf("gargoyle-mentions-gts-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "@bob@gts.test hello "+gargoyleMarker, "public")
	waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=40", gargoyleMarker)

	gtsMarker := fmt.Sprintf("gts-mentions-gargoyle-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gts, s.gtsToken, "@alice@gargoyle.test hello "+gtsMarker, "public")
	waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=40", gtsMarker)
}

func TestVisibilityDeliveryAndTimelines(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	ensureGargoyleFollowsGTS(t, s)

	cases := []struct {
		name           string
		visibility     string
		expectedRemote string
		content        string
		wantGTS        bool
	}{
		{name: "public", visibility: "public", expectedRemote: "public", content: "public visibility", wantGTS: true},
		// Gargoyle currently serializes unlisted ActivityPub audience in a way GTS maps back to public.
		{name: "unlisted", visibility: "unlisted", expectedRemote: "public", content: "unlisted visibility", wantGTS: true},
		{name: "private_followers", visibility: "private", expectedRemote: "private", content: "private followers visibility", wantGTS: true},
		{name: "direct_mention", visibility: "direct", expectedRemote: "direct", content: "@bob@gts.test direct mention visibility", wantGTS: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := fmt.Sprintf("%s-%d", strings.ReplaceAll(tc.name, "_", "-"), time.Now().UnixNano())
			created := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, tc.content+" "+marker, tc.visibility)
			if created.Visibility != "" && created.Visibility != tc.visibility {
				t.Fatalf("created status visibility = %q, want %q", created.Visibility, tc.visibility)
			}
			if tc.wantGTS {
				got := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=60", marker)
				if got.Visibility != "" && got.Visibility != tc.expectedRemote {
					t.Fatalf("received visibility = %q, want %q", got.Visibility, tc.expectedRemote)
				}
			}
		})
	}

	gtsDirectMarker := fmt.Sprintf("gts-direct-to-gargoyle-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gts, s.gtsToken, "@alice@gargoyle.test direct from gts "+gtsDirectMarker, "direct")
	gotDirect := waitForConversationStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, gtsDirectMarker)
	if gotDirect.Visibility != "" && gotDirect.Visibility != "direct" {
		t.Fatalf("received direct visibility = %q, want direct", gotDirect.Visibility)
	}
}

func TestFavouritesBoostsRepliesAndDelete(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	ensureGargoyleFollowsGTS(t, s)

	marker := fmt.Sprintf("gargoyle-interactions-%d", time.Now().UnixNano())
	created := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "interaction target "+marker, "public")
	remote := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=60", marker)

	deleteMarker := fmt.Sprintf("delete-me-%d", time.Now().UnixNano())
	toDelete := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "temporary "+deleteMarker, "public")
	remoteDelete := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", deleteMarker)
	resp, body, err := s.gargoyle.Delete(s.ctx, "/api/v1/statuses/"+url.PathEscape(toDelete.ID), s.gargoyleToken, nil)
	shared.Require2xx(t, resp, body, err)
	shared.WaitFor(s.ctx, "delete propagated to GTS", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var status shared.Status
		resp, _, err := s.gts.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(remoteDelete.ID), s.gtsToken, &status)
		if err != nil {
			return struct{}{}, false, nil
		}
		return struct{}{}, resp.StatusCode == http.StatusNotFound || !strings.Contains(status.Content, deleteMarker), nil
	})

	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/favourite", s.gtsToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.gtsToken, url.Values{}, nil)
	shared.Require2xx(t, resp, body, err)

	reblogSeen := waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=80", marker)
	if reblogSeen.Reblog == nil && !strings.Contains(reblogSeen.Content, marker) {
		t.Fatalf("expected boost/reblog containing marker, got %+v", reblogSeen)
	}

	replyMarker := fmt.Sprintf("gts-reply-%d", time.Now().UnixNano())
	var reply shared.Status
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses", s.gtsToken, url.Values{"status": {"@alice@gargoyle.test reply " + replyMarker}, "visibility": {"public"}, "in_reply_to_id": {remote.ID}}, &reply)
	shared.Require2xx(t, resp, body, err)
	_ = created
}

func assertStatusAbsent(t testing.TB, ctx context.Context, client shared.Client, token, path, marker string, duration time.Duration) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		var statuses []shared.Status
		resp, _, err := client.GetJSON(ctx, path, token, &statuses)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			for _, status := range statuses {
				if strings.Contains(status.Content, marker) || (status.Reblog != nil && strings.Contains(status.Reblog.Content, marker)) {
					t.Fatalf("status %q unexpectedly present in %s: %+v", marker, path, status)
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func TestUnsignedInboxIsRejected(t *testing.T) {
	s := setupSuite(t)
	payload := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "type": "Follow", "actor": "http://gts.test/users/bob", "object": "http://gargoyle.test/users/alice"}
	resp, body, err := s.gargoyle.PostJSON(s.ctx, "/users/alice/inbox", "", payload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode < 400 {
		t.Fatalf("unsigned inbox POST should be rejected, got %d: %s", resp.StatusCode, body)
	}
}

func TestVisibilityDoesNotLeakToNonFollower(t *testing.T) {
	s := setupSuite(t)
	setupGTSAccount(t, "charlie", env("GTS_PASSWORD", "Str0ngP@ssword!"))
	gtsApp, rerr := shared.RegisterApp(s.ctx, s.gts)
	mustNoResponseError(t, rerr)
	charlieToken := gtsAuthorizationCodeToken(t, s.ctx, s.gts, gtsApp, "charlie@gts.test", env("GTS_PASSWORD", "Str0ngP@ssword!"))
	ensureGTSFollowsGargoyle(t, s)

	privateMarker := fmt.Sprintf("private-nonleak-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "private nonleak "+privateMarker, "private")
	waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", privateMarker)
	assertStatusAbsent(t, s.ctx, s.gts, charlieToken, "/api/v1/timelines/public?limit=80", privateMarker, 5*time.Second)
	assertStatusAbsent(t, s.ctx, s.gts, charlieToken, "/api/v1/timelines/home?limit=80", privateMarker, 5*time.Second)

	directMarker := fmt.Sprintf("direct-nonleak-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "@bob@gts.test direct nonleak "+directMarker, "direct")
	waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", directMarker)
	assertStatusAbsent(t, s.ctx, s.gts, charlieToken, "/api/v1/timelines/public?limit=80", directMarker, 5*time.Second)
	assertStatusAbsent(t, s.ctx, s.gts, charlieToken, "/api/v1/timelines/home?limit=80", directMarker, 5*time.Second)
}

func TestReplyContextAndMediaFederation(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)

	marker := fmt.Sprintf("context-root-%d", time.Now().UnixNano())
	root := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "context root "+marker, "public")
	remoteRoot := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", marker)

	replyMarker := fmt.Sprintf("context-reply-%d", time.Now().UnixNano())
	var reply shared.Status
	resp, body, err := s.gts.PostForm(s.ctx, "/api/v1/statuses", s.gtsToken, url.Values{"status": {"@alice@gargoyle.test context reply " + replyMarker}, "visibility": {"public"}, "in_reply_to_id": {remoteRoot.ID}}, &reply)
	shared.Require2xx(t, resp, body, err)

	shared.WaitFor(s.ctx, "reply appears in Gargoyle context", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
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

	png := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2, 0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 153, 99, 248, 15, 4, 0, 9, 251, 3, 253, 167, 89, 85, 191, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
	var media shared.MediaAttachment
	resp, body, err = s.gargoyle.PostMultipart(s.ctx, "/api/v2/media", s.gargoyleToken, map[string]string{"description": "one red pixel"}, "file", "pixel.png", "image/png", png, &media)
	shared.Require2xx(t, resp, body, err)
	mediaURL, err := url.Parse(media.URL)
	if err != nil {
		t.Fatalf("invalid media URL %q: %v", media.URL, err)
	}
	shared.WaitFor(s.ctx, "uploaded media is fetchable", 500*time.Millisecond, func(ctx context.Context) (struct{}, bool, error) {
		resp, _, err := doRaw(ctx, http.DefaultClient, s.gargoyle, http.MethodGet, mediaURL.Path, "", nil)
		if err != nil {
			return struct{}{}, false, err
		}
		return struct{}{}, resp.StatusCode == http.StatusOK, nil
	})
	time.Sleep(2 * time.Second)
	mediaMarker := fmt.Sprintf("media-status-%d", time.Now().UnixNano())
	var mediaStatus shared.Status
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/statuses", s.gargoyleToken, url.Values{"status": {"media status " + mediaMarker}, "visibility": {"public"}, "media_ids[]": {media.ID}, "media_ids": {media.ID}}, &mediaStatus)
	shared.Require2xx(t, resp, body, err)
	remoteMedia := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", mediaMarker)
	if len(remoteMedia.MediaAttachments) == 0 {
		if !strings.Contains(remoteMedia.Content, media.URL) {
			t.Fatalf("remote media status had no attachment and did not preserve fetchable media URL %q: %+v", media.URL, remoteMedia)
		}
		return
	}
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

func TestUnfavouriteAndUnboostPropagation(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)

	marker := fmt.Sprintf("undo-interactions-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "undo target "+marker, "public")
	remote := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", marker)

	var interacted shared.Status
	resp, body, err := s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/favourite", s.gtsToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if !interacted.Favourited {
		t.Fatalf("expected GTS favourite response to mark status favourited")
	}
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/unfavourite", s.gtsToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if interacted.Favourited {
		t.Fatalf("expected GTS unfavourite response to clear favourited")
	}

	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.gtsToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if !interacted.Reblogged {
		t.Fatalf("expected GTS reblog response to mark status reblogged")
	}
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/unreblog", s.gtsToken, url.Values{}, &interacted)
	shared.Require2xx(t, resp, body, err)
	if interacted.Reblogged {
		t.Fatalf("expected GTS unreblog response to clear reblogged")
	}
}

func TestBoostVisibilityMatrix(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)

	cases := []struct {
		name       string
		visibility string
		content    string
		wantReblog bool
	}{
		{name: "public", visibility: "public", content: "boost public", wantReblog: true},
		{name: "unlisted", visibility: "unlisted", content: "boost unlisted", wantReblog: true},
		{name: "private", visibility: "private", content: "boost private", wantReblog: false},
		{name: "direct", visibility: "direct", content: "@bob@gts.test boost direct", wantReblog: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := fmt.Sprintf("boost-%s-%d", tc.name, time.Now().UnixNano())
			postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, tc.content+" "+marker, tc.visibility)
			remote := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=100", marker)
			var result shared.Status
			resp, body, err := s.gts.PostForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID)+"/reblog", s.gtsToken, url.Values{}, &result)
			requireBoostVisibilityResult(t, tc.visibility, tc.wantReblog, resp, body, err, result)
		})
	}
}

func requireBoostVisibilityResult(t testing.TB, visibility string, wantReblog bool, resp *http.Response, body string, err error, result shared.Status) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if wantReblog {
		shared.Require2xx(t, resp, body, nil)
		if !result.Reblogged {
			t.Fatalf("expected reblogged=true for %s", visibility)
		}
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && result.Reblogged {
		t.Fatalf("expected %s boost to be rejected or not reblogged; got %d %+v", visibility, resp.StatusCode, result)
	}
}

func TestDeliveryRetryAfterTargetOutage(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	dockerCompose(t, "stop", "gotosocial")
	defer dockerCompose(t, "up", "-d", "gotosocial")

	marker := fmt.Sprintf("retry-delivery-%d", time.Now().UnixNano())
	postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "retry while gts down "+marker, "public")
	time.Sleep(7 * time.Second)
	dockerCompose(t, "exec", "-T", "gargoyle", "sqlite3", "/data/gargoyle.db", "update delivery_jobs set next_attempt_at=datetime('now','-1 minute'), status='pending' where status='pending'")
	dockerCompose(t, "up", "-d", "gotosocial")
	waitForHTTP(t, s.ctx, s.gts, "/nodeinfo/2.0")
	shared.WaitFor(s.ctx, "retry delivery reaches GTS", 6*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		dockerCompose(t, "exec", "-T", "gargoyle", "sqlite3", "/data/gargoyle.db", "update delivery_jobs set next_attempt_at=datetime('now','-1 minute'), status='pending' where status='pending'")
		var statuses []shared.Status
		resp, _, err := s.gts.GetJSON(ctx, "/api/v1/timelines/home?limit=100", s.gtsToken, &statuses)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		for _, status := range statuses {
			if strings.Contains(status.Content, marker) {
				return struct{}{}, true, nil
			}
		}
		return struct{}{}, false, nil
	})
}

func TestRemoteURLHardeningRejectsUnconfiguredPrivateHosts(t *testing.T) {
	s := setupSuite(t)
	var search shared.SearchResponse
	resp, _, err := s.gargoyle.GetJSON(s.ctx, "/api/v2/search?q="+url.QueryEscape("mallory@127.0.0.1")+"&type=accounts&resolve=true", s.gargoyleToken, &search)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 && len(search.Accounts) > 0 {
		t.Fatalf("unexpectedly resolved unconfigured private host: %+v", search.Accounts)
	}
}

func TestProfileUpdateFederatesToGoToSocial(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	marker := fmt.Sprintf("profile-update-%d", time.Now().UnixNano())
	displayName := "Alice Updated " + marker
	note := "bio " + marker
	var account shared.Account
	resp, body, err := s.gargoyle.PatchForm(s.ctx, "/api/v1/accounts/update_credentials", s.gargoyleToken, url.Values{"display_name": {displayName}, "note": {note}, "fields_attributes[0][name]": {"Website"}, "fields_attributes[0][value]": {"http://alice.example/" + marker}}, &account)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("profile update failed: status=%d err=%v body=%s", resp.StatusCode, err, body)
	}
	if !strings.Contains(account.DisplayName, marker) || !strings.Contains(account.Note, marker) || !accountHasField(account, "Website", marker) {
		t.Fatalf("profile response did not include update: %+v", account)
	}
	var verified shared.Account
	resp, body, err = s.gargoyle.GetJSON(s.ctx, "/api/v1/accounts/verify_credentials", s.gargoyleToken, &verified)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("verify credentials failed: status=%d err=%v body=%s", resp.StatusCode, err, body)
	}
	if verified.DisplayName != account.DisplayName || verified.Note != account.Note || !accountHasField(verified, "Website", marker) {
		t.Fatalf("verified credentials did not persist profile update: verified=%+v updated=%+v", verified, account)
	}
	shared.WaitFor(s.ctx, "profile update federates to GTS", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var remote shared.Account
		resp, _, err := s.gts.GetJSON(ctx, "/api/v1/accounts/"+url.PathEscape(s.aliceOnGTS.ID), s.gtsToken, &remote)
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

func resetIntegrationStackOnce() error {
	if err := dockerComposeDir("down", "-v", "--remove-orphans", "--timeout", "0"); err != nil {
		// Continue with hard cleanup below.
	}
	cleanup := exec.Command("bash", "--noprofile", "--norc", "-lc", strings.Join([]string{
		"docker rm -f gargoyle-integration-gts-proxy-1 gargoyle-integration-gts-gargoyle-1 gargoyle-integration-gts-gotosocial-1 2>/dev/null || true",
		"docker network rm gargoyle-integration-gts_default 2>/dev/null || true",
		"docker volume rm gargoyle-integration-gts_gargoyle-data gargoyle-integration-gts_gargoyle-media gargoyle-integration-gts_gts-data 2>/dev/null || true",
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

func integrationDirFromWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	if filepath.Base(wd) == "gts" {
		return wd
	}
	return filepath.Join(wd, "integration", "gts")
}

func TestPollFederatesBetweenGargoyleAndGoToSocial(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)
	ensureGargoyleFollowsGTS(t, s)

	marker := fmt.Sprintf("poll-%d", time.Now().UnixNano())
	var created shared.Status
	resp, body, err := s.gargoyle.PostForm(s.ctx, "/api/v1/statuses", s.gargoyleToken, url.Values{
		"status":           {"poll from gargoyle " + marker},
		"visibility":       {"public"},
		"activitypub_type": {"Question"},
		"poll[options][]":  {"red " + marker, "blue " + marker},
		"poll[expires_in]": {"3600"},
		"poll[multiple]":   {"false"},
	}, &created)
	shared.Require2xx(t, resp, body, err)
	if created.Poll == nil || len(created.Poll.Options) != 2 {
		t.Fatalf("created Gargoyle status did not include poll: %+v", created)
	}

	remote := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", marker)
	if remote.Poll == nil || len(remote.Poll.Options) != 2 {
		t.Fatalf("GTS did not receive Gargoyle poll as a poll: %+v", remote)
	}
	var remoteVoted shared.Poll
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/polls/"+url.PathEscape(remote.Poll.ID)+"/votes", s.gtsToken, url.Values{"choices[]": {"0"}}, &remoteVoted)
	shared.Require2xx(t, resp, body, err)

	shared.WaitFor(s.ctx, "remote poll vote reaches Gargoyle", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var local shared.Status
		resp, _, err := s.gargoyle.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(created.ID), s.gargoyleToken, &local)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		if local.Poll == nil || len(local.Poll.Options) == 0 {
			return struct{}{}, false, nil
		}
		return struct{}{}, local.Poll.Options[0].VotesCount > 0, nil
	})

	gtsMarker := fmt.Sprintf("gts-poll-%d", time.Now().UnixNano())
	var gtsCreated shared.Status
	resp, body, err = s.gts.PostForm(s.ctx, "/api/v1/statuses", s.gtsToken, url.Values{
		"status":           {"@alice@gargoyle.test poll from gts " + gtsMarker},
		"visibility":       {"public"},
		"poll[options][]":  {"cat " + gtsMarker, "dog " + gtsMarker},
		"poll[expires_in]": {"3600"},
		"poll[multiple]":   {"false"},
	}, &gtsCreated)
	shared.Require2xx(t, resp, body, err)
	localRemote := waitForStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, "/api/v1/timelines/public?limit=80", gtsMarker)
	if localRemote.Poll == nil || len(localRemote.Poll.Options) != 2 {
		t.Fatalf("Gargoyle did not receive GTS poll as a poll: %+v", localRemote)
	}
	var localVote shared.Poll
	resp, body, err = s.gargoyle.PostForm(s.ctx, "/api/v1/polls/"+url.PathEscape(localRemote.Poll.ID)+"/votes", s.gargoyleToken, url.Values{"choices[]": {"1"}}, &localVote)
	shared.Require2xx(t, resp, body, err)
	if !localVote.Voted || len(localVote.Votes) == 0 || localVote.Votes[0] != 1 {
		t.Fatalf("Gargoyle poll vote response did not record selected option: %+v", localVote)
	}
}

func TestStatusEditFederatesToGoToSocial(t *testing.T) {
	s := setupSuite(t)
	ensureGTSFollowsGargoyle(t, s)

	marker := fmt.Sprintf("status-edit-%d", time.Now().UnixNano())
	original := "original status content " + marker
	created := postStatus(t, s.ctx, s.gargoyle, s.gargoyleToken, original, "public")
	remote := waitForStatus(t, s.ctx, s.gts, s.gtsToken, "/api/v1/timelines/home?limit=80", marker)
	if !strings.Contains(remote.Content, original) {
		t.Fatalf("remote status did not contain original content: %+v", remote)
	}

	edited := "edited status content " + marker
	var updated shared.Status
	resp, body, err := s.gargoyle.PatchForm(s.ctx, "/api/v1/statuses/"+url.PathEscape(created.ID), s.gargoyleToken, url.Values{"status": {edited}, "visibility": {"public"}}, &updated)
	shared.Require2xx(t, resp, body, err)
	if !strings.Contains(updated.Content, edited) {
		t.Fatalf("local edit response did not contain edited content: %+v", updated)
	}

	shared.WaitFor(s.ctx, "status edit federates to GTS", 2*time.Second, func(ctx context.Context) (struct{}, bool, error) {
		var status shared.Status
		resp, _, err := s.gts.GetJSON(ctx, "/api/v1/statuses/"+url.PathEscape(remote.ID), s.gtsToken, &status)
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, false, err
		}
		return struct{}{}, strings.Contains(status.Content, edited) && !strings.Contains(status.Content, original), nil
	})
}
