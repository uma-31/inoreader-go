// Package e2e_test runs the client against a mock server that serves the
// API from its description, api/openapi.yaml. Each test starts its own.
//
// The mock server checks every request against the description and every
// response example against its schema, and fails the test on a mismatch.
// So these tests check the requests the client sends as well as how it
// decodes the documented responses.
package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/uma-31/inoreader-go"
	"github.com/uma-31/inoreader-go/e2e/mock"
)

const (
	appID  = "1000001414"
	appKey = "mock-app-key"
)

var spec []byte

func TestMain(m *testing.M) {
	var err error
	spec, err = os.ReadFile("../api/openapi.yaml")
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// startMock starts a mock server for the test and returns its URL. A request
// or response that violates the API description fails the test.
func startMock(t *testing.T) string {
	t.Helper()

	handler, err := mock.New(spec, func(v string) { t.Error(v) })
	if err != nil {
		t.Fatalf("mock.New: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv.URL
}

// headerTransport adds extra headers, such as Prefer, to each request.
type headerTransport struct {
	base   http.RoundTripper
	header http.Header
}

func (ht *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, vs := range ht.header {
		req.Header[k] = vs
	}
	return ht.base.RoundTrip(req)
}

func httpClient(header http.Header) *http.Client {
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &headerTransport{base: http.DefaultTransport, header: header},
	}
}

// newClient returns a client signed in with OAuth 2.0 through the mock
// server's token endpoint. Extra headers, such as Prefer, go on every API
// request.
func newClient(t *testing.T, header http.Header) *inoreader.Client {
	t.Helper()
	base := startMock(t)

	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, httpClient(nil))
	conf := &oauth2.Config{
		ClientID:     appID,
		ClientSecret: appKey,
		Endpoint:     inoreader.NewEndpoint(base),
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{inoreader.ScopeRead, inoreader.ScopeWrite},
	}
	token, err := conf.Exchange(ctx, "mock-authorization-code")
	if err != nil {
		t.Fatalf("exchanging the authorization code: %v", err)
	}

	ctx = context.WithValue(t.Context(), oauth2.HTTPClient, httpClient(header))
	c, err := inoreader.NewClient(appID, appKey,
		inoreader.WithBaseURL(base),
		inoreader.WithHTTPClient(conf.Client(ctx, token)),
		inoreader.WithUserAgent("inoreader-go-e2e"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestOAuth2(t *testing.T) {
	base := startMock(t)
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, httpClient(nil))
	conf := &oauth2.Config{
		ClientID:     appID,
		ClientSecret: appKey,
		Endpoint:     inoreader.NewEndpoint(base),
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{inoreader.ScopeRead},
	}

	token, err := conf.Exchange(ctx, "mock-authorization-code")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if token.AccessToken != "mock-access-token" || token.RefreshToken != "mock-refresh-token" {
		t.Errorf("token = %+v", token)
	}

	// Force a refresh through the refresh token grant.
	token.Expiry = time.Now().Add(-time.Hour)
	refreshed, err := conf.TokenSource(ctx, token).Token()
	if err != nil {
		t.Fatalf("refreshing the token: %v", err)
	}
	if refreshed.AccessToken != "mock-access-token" {
		t.Errorf("refreshed token = %+v", refreshed)
	}
}

func TestClientLogin(t *testing.T) {
	base := startMock(t)
	c, err := inoreader.NewClient(appID, appKey,
		inoreader.WithBaseURL(base),
		inoreader.WithHTTPClient(httpClient(nil)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	token, err := c.ClientLogin(t.Context(), "user@example.com", "password")
	if err != nil {
		t.Fatalf("ClientLogin: %v", err)
	}
	if token != "mock-client-login-token" {
		t.Errorf("token = %q", token)
	}

	c, err = inoreader.NewClient(appID, appKey,
		inoreader.WithBaseURL(base),
		inoreader.WithHTTPClient(httpClient(nil)),
		inoreader.WithClientLoginToken(token),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.UserInfo(t.Context()); err != nil {
		t.Errorf("UserInfo with the ClientLogin token: %v", err)
	}
}

func TestUserInfo(t *testing.T) {
	c := newClient(t, nil)

	u, err := c.UserInfo(t.Context())
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	if u.ID != "1005921515" || u.Name != "mock-user" || u.SignedUpAt.Unix() != 1163850013 {
		t.Errorf("UserInfo() = %+v", u)
	}

	rl := c.RateLimit()
	if rl == nil {
		t.Fatal("RateLimit() is nil")
	}
	if rl.Zone1.Limit != 100 || rl.Zone2.Limit != 100 || rl.ResetAfter != time.Hour {
		t.Errorf("RateLimit() = %+v", rl)
	}
}

func TestSubscriptions(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()
	feed := inoreader.FeedStream("http://feeds.arstechnica.com/arstechnica/science")

	subs, err := c.ListSubscriptions(ctx, &inoreader.ListSubscriptionsOptions{TeamAssets: true})
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0].ID != feed || subs[0].SortID != "00DA6134" || subs[0].FirstItemAt.IsZero() {
		t.Errorf("ListSubscriptions() = %+v", subs)
	}

	added, err := c.Subscribe(ctx, feed)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if added.StreamID != feed {
		t.Errorf("Subscribe() = %+v", added)
	}

	if err := c.EditSubscription(ctx, inoreader.EditSubscriptionRequest{
		Action: inoreader.SubscriptionEdit,
		Feed:   feed,
		Title:  "Science",
		AddTo:  inoreader.LabelStream("Tech"),
	}); err != nil {
		t.Errorf("EditSubscription: %v", err)
	}
	if err := c.Unsubscribe(ctx, feed); err != nil {
		t.Errorf("Unsubscribe: %v", err)
	}
}

func TestTags(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()

	tags, err := c.ListTags(ctx, &inoreader.ListTagsOptions{Counts: true, TeamAssets: true})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 2 || tags[1].Type != inoreader.TagTypeFolder || tags[1].UnreadCount != 4 {
		t.Errorf("ListTags() = %+v", tags)
	}

	if err := c.RenameTag(ctx, inoreader.LabelStream("Tech"), "Technology"); err != nil {
		t.Errorf("RenameTag: %v", err)
	}
	if err := c.DeleteTag(ctx, inoreader.LabelStream("Technology")); err != nil {
		t.Errorf("DeleteTag: %v", err)
	}
}

func TestEditTag(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()
	ids := []inoreader.ItemID{344691561, 344668222}

	for name, call := range map[string]func() error{
		"MarkAsRead":   func() error { return c.MarkAsRead(ctx, ids...) },
		"MarkAsUnread": func() error { return c.MarkAsUnread(ctx, ids...) },
		"Star":         func() error { return c.Star(ctx, ids[0]) },
		"Unstar":       func() error { return c.Unstar(ctx, ids[0]) },
		"EditTag": func() error {
			return c.EditTag(ctx, inoreader.EditTagRequest{Add: inoreader.LabelStream("later"), Items: ids})
		},
	} {
		if err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestStreams(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()
	folder := inoreader.LabelStream("Tech")

	counts, err := c.UnreadCounts(ctx)
	if err != nil {
		t.Fatalf("UnreadCounts: %v", err)
	}
	if counts.Max != 1000 || len(counts.Counts) != 2 || counts.Counts[0].ID.Normalize() != inoreader.StreamReadingList {
		t.Errorf("UnreadCounts() = %+v", counts)
	}

	contents, err := c.StreamContents(ctx, inoreader.FeedStream("http://feeds.arstechnica.com/arstechnica/science"),
		&inoreader.StreamContentsOptions{
			StreamOptions: inoreader.StreamOptions{
				Count:       100,
				OldestFirst: true,
				Since:       time.Now().Add(-24 * time.Hour),
				Exclude:     inoreader.StreamRead,
				Include:     inoreader.StreamStarred,
			},
			Annotations: true,
			Summaries:   true,
		})
	if err != nil {
		t.Fatalf("StreamContents: %v", err)
	}
	if len(contents.Items) != 1 || contents.Continuation == "" {
		t.Fatalf("StreamContents() = %+v", contents)
	}
	item := contents.Items[0]
	if item.ID.Long() != "tag:google.com,2005:reader/item/0000000693c3bc0c" || item.URL() != "https://example.com/articles/1" {
		t.Errorf("item = %+v", item)
	}

	refs, err := c.ItemIDs(ctx, folder, &inoreader.StreamOptions{Count: 1000, ExcludeFolderTags: true})
	if err != nil {
		t.Fatalf("ItemIDs: %v", err)
	}
	if len(refs.Refs) != 2 || refs.Refs[0].ID != 3614359203 {
		t.Errorf("ItemIDs() = %+v", refs)
	}

	// The mock returns the same continuation on every page, so take the
	// first page only.
	for it, err := range c.AllItems(ctx, folder, nil) {
		if err != nil {
			t.Fatalf("AllItems: %v", err)
		}
		if it.Title != "Mock article" {
			t.Errorf("AllItems() item = %+v", it)
		}
		break
	}

	if err := c.MarkAllAsRead(ctx, folder, time.Now()); err != nil {
		t.Errorf("MarkAllAsRead: %v", err)
	}
}

func TestPreferences(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()

	prefs, err := c.StreamPreferences(ctx)
	if err != nil {
		t.Fatalf("StreamPreferences: %v", err)
	}
	order := prefs.SubscriptionOrdering(inoreader.LabelStream("Tech"))
	if len(order) != 1 || order[0] != "00DA6134" {
		t.Errorf("SubscriptionOrdering() = %v", order)
	}

	if err := c.SetSubscriptionOrdering(ctx, inoreader.StreamRoot, []string{"BECC0BE1", "00DA6134"}); err != nil {
		t.Errorf("SetSubscriptionOrdering: %v", err)
	}
}

func TestActiveSearch(t *testing.T) {
	c := newClient(t, nil)
	ctx := t.Context()

	search, err := c.CreateActiveSearch(ctx, inoreader.ActiveSearchRequest{
		Term:          "amazing",
		FilterType:    inoreader.ActiveSearchInSubscription,
		FilterID:      inoreader.FeedStream("http://feeds.arstechnica.com/arstechnica/science"),
		ArticleIDs:    []inoreader.ItemID{12363668564},
		SearchFilters: "titles",
		SearchMatch:   "full_phrase",
	})
	if err != nil {
		t.Fatalf("CreateActiveSearch: %v", err)
	}
	if search.Title != "amazing" || search.ID == "" {
		t.Errorf("CreateActiveSearch() = %+v", search)
	}

	if err := c.DeleteActiveSearch(ctx, search.ID); err != nil {
		t.Errorf("DeleteActiveSearch: %v", err)
	}
}

func TestActiveSearchRefused(t *testing.T) {
	c := newClient(t, http.Header{"Prefer": {"code=202"}})

	_, err := c.CreateActiveSearch(t.Context(), inoreader.ActiveSearchRequest{
		Term:       "amazing",
		FilterType: inoreader.ActiveSearchInPublic,
	})
	var asErr *inoreader.ActiveSearchError
	if !errors.As(err, &asErr) {
		t.Fatalf("error = %v, want an *ActiveSearchError", err)
	}
	if asErr.AccountLimit != 30 || !asErr.NeedUpgrade {
		t.Errorf("ActiveSearchError = %+v", asErr)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		code string
		want error
	}{
		{"401", inoreader.ErrUnauthorized},
		{"403", inoreader.ErrForbidden},
		{"429", inoreader.ErrRateLimited},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			c := newClient(t, http.Header{"Prefer": {"code=" + tt.code}})

			_, err := c.UnreadCounts(t.Context())
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			var apiErr *inoreader.APIError
			if !errors.As(err, &apiErr) || apiErr.Message == "" {
				t.Errorf("error = %#v, want an *APIError with a message", err)
			}
		})
	}
}

func TestRateLimitedReportsUsage(t *testing.T) {
	c := newClient(t, http.Header{"Prefer": {"code=429"}})

	err := c.MarkAsRead(t.Context(), 1)
	var apiErr *inoreader.APIError
	if !errors.As(err, &apiErr) || !errors.Is(err, inoreader.ErrRateLimited) {
		t.Fatalf("error = %v, want a rate limit *APIError", err)
	}
	if apiErr.RateLimit == nil || apiErr.RateLimit.ResetAfter != time.Hour {
		t.Errorf("RateLimit = %+v", apiErr.RateLimit)
	}
}

func TestUnauthenticated(t *testing.T) {
	base := startMock(t)
	c, err := inoreader.NewClient(appID, appKey,
		inoreader.WithBaseURL(base),
		inoreader.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.UserInfo(t.Context()); !errors.Is(err, inoreader.ErrUnauthorized) {
		t.Errorf("error = %v, want ErrUnauthorized", err)
	}
}
