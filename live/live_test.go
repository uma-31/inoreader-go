//go:build live

// Package live_test runs the client against the real Inoreader API, to
// catch differences between the API and its description that the E2E tests
// against the mock server cannot. Run it with `mise run test:live`, or from
// the Live workflow on GitHub Actions.
//
// It needs an app registered in the Inoreader preferences, passed in
// INOREADER_APP_ID and INOREADER_APP_KEY, and an OAuth 2.0 token for it:
// either INOREADER_REFRESH_TOKEN, which it exchanges for an access token
// once per run, or INOREADER_ACCESS_TOKEN, used as is. It only reads data,
// so the account stays as it is, and makes about ten requests, a tenth of
// the default daily limit.
//
// Its logs are public when the repository is, so failure messages name the
// check that failed and never the user's data: no email, feed URLs, tags or
// article content. On GitHub Actions it also masks the access token and the
// user's identity, in case they reach the log anyway.
package live_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/uma-31/inoreader-go"
)

var appID, appKey, accessToken string

func TestMain(m *testing.M) {
	appID = os.Getenv("INOREADER_APP_ID")
	appKey = os.Getenv("INOREADER_APP_KEY")
	if appID == "" || appKey == "" {
		fail("set INOREADER_APP_ID and INOREADER_APP_KEY to run the live tests")
	}

	switch refreshToken := os.Getenv("INOREADER_REFRESH_TOKEN"); {
	case refreshToken != "":
		token, err := refresh(refreshToken)
		if err != nil {
			fail("refreshing the access token: %v", err)
		}
		accessToken = token.AccessToken
		// Whether Inoreader rotates refresh tokens is undocumented. If it
		// does, the stored one may stop working; never print the token.
		if token.RefreshToken != "" && token.RefreshToken != refreshToken {
			fmt.Fprintln(os.Stderr, "note: the refresh token was rotated; the stored one may expire")
		}
	case os.Getenv("INOREADER_ACCESS_TOKEN") != "":
		accessToken = os.Getenv("INOREADER_ACCESS_TOKEN")
	default:
		fail("set INOREADER_REFRESH_TOKEN or INOREADER_ACCESS_TOKEN to run the live tests")
	}
	mask(accessToken)

	os.Exit(m.Run())
}

// mask hides values in the rest of the log when running on GitHub Actions,
// which replaces them with ***. Secrets are masked already, but values that
// are derived at run time are not.
func mask(values ...string) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return
	}
	for _, v := range values {
		if v != "" {
			fmt.Printf("::add-mask::%s\n", v)
		}
	}
}

// refresh exchanges a refresh token for an access token. The App ID and App
// key are the OAuth 2.0 client ID and secret.
func refresh(refreshToken string) (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conf := &oauth2.Config{ClientID: appID, ClientSecret: appKey, Endpoint: inoreader.Endpoint}
	return conf.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}).Token()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func newClient(t *testing.T, appKey, token string) *inoreader.Client {
	t.Helper()

	ctx := t.Context()
	httpClient := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	httpClient.Timeout = 30 * time.Second

	c, err := inoreader.NewClient(appID, appKey,
		inoreader.WithHTTPClient(httpClient),
		inoreader.WithUserAgent("inoreader-go-live-test"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestUserInfo(t *testing.T) {
	c := newClient(t, appKey, accessToken)

	u, err := c.UserInfo(t.Context())
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	mask(u.ID, u.ProfileID, u.Name, u.Email)
	if u.ID == "" || u.SignedUpAt.IsZero() {
		t.Errorf("UserInfo() lacks an ID (%t) or a sign-up time (%t)", u.ID == "", u.SignedUpAt.IsZero())
	}

	rl := c.RateLimit()
	if rl == nil {
		t.Fatal("RateLimit() is nil: the response had no rate limit headers")
	}
	if rl.Zone1.Limit <= 0 || rl.Zone2.Limit <= 0 || rl.ResetAfter <= 0 {
		t.Errorf("RateLimit() = %+v", rl)
	}
	t.Logf("zone 1: %d of %d used; zone 2: %d of %d used; resets in %v",
		rl.Zone1.Usage, rl.Zone1.Limit, rl.Zone2.Usage, rl.Zone2.Limit, rl.ResetAfter)
}

func TestFeedsAndFolders(t *testing.T) {
	c := newClient(t, appKey, accessToken)
	ctx := t.Context()

	subs, err := c.ListSubscriptions(ctx, nil)
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	for i, s := range subs {
		if _, ok := s.ID.FeedURL(); !ok && s.FeedType != "team_channel" {
			t.Errorf("subscription %d of %d, of type %q, has no feed URL", i+1, len(subs), s.FeedType)
		}
	}

	tags, err := c.ListTags(ctx, &inoreader.ListTagsOptions{Counts: true})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	prefs, err := c.StreamPreferences(ctx)
	if err != nil {
		t.Fatalf("StreamPreferences: %v", err)
	}

	// The root ordering should only name sort IDs of known feeds and folders.
	known := map[string]bool{}
	for _, s := range subs {
		known[s.SortID] = true
	}
	for _, tag := range tags {
		known[tag.SortID] = true
	}
	for _, id := range prefs.SubscriptionOrdering(inoreader.StreamRoot) {
		if !known[id] {
			t.Errorf("root ordering names unknown sort ID %q", id)
		}
	}
	t.Logf("%d feeds, %d tags", len(subs), len(tags))
}

func TestUnreadCounts(t *testing.T) {
	c := newClient(t, appKey, accessToken)

	counts, err := c.UnreadCounts(t.Context())
	if err != nil {
		t.Fatalf("UnreadCounts: %v", err)
	}
	if counts.Max <= 0 {
		t.Errorf("Max = %d", counts.Max)
	}
	for _, uc := range counts.Counts {
		if uc.ID.Normalize() == inoreader.StreamReadingList {
			return
		}
	}
	t.Errorf("no count for %s", inoreader.StreamReadingList)
}

func TestArticles(t *testing.T) {
	c := newClient(t, appKey, accessToken)
	ctx := t.Context()
	const n = 5

	contents, err := c.StreamContents(ctx, inoreader.StreamReadingList, &inoreader.StreamContentsOptions{
		StreamOptions: inoreader.StreamOptions{Count: n},
	})
	if err != nil {
		t.Fatalf("StreamContents: %v", err)
	}
	if len(contents.Items) > n {
		t.Errorf("StreamContents returned %d items, asked for %d", len(contents.Items), n)
	}
	for _, it := range contents.Items {
		if it.Timestamp.IsZero() || it.Origin.StreamID == "" {
			t.Errorf("item %s lacks a timestamp (%t) or an origin (%t)",
				it.ID.Long(), it.Timestamp.IsZero(), it.Origin.StreamID == "")
		}
		if !it.HasCategory(inoreader.StreamReadingList) {
			t.Errorf("item %s is not in the reading list, among its %d categories", it.ID.Long(), len(it.Categories))
		}
	}

	refs, err := c.ItemIDs(ctx, inoreader.StreamReadingList, &inoreader.StreamOptions{Count: n})
	if err != nil {
		t.Fatalf("ItemIDs: %v", err)
	}

	// The long IDs of stream contents and the short IDs of item IDs should
	// decode to the same articles. A new article arriving between the two
	// requests can shift the page, so expect an overlap, not equality.
	if len(contents.Items) == 0 || len(refs.Refs) == 0 {
		t.Skip("the reading list is empty")
	}
	ids := map[inoreader.ItemID]bool{}
	for _, r := range refs.Refs {
		ids[r.ID] = true
	}
	for _, it := range contents.Items {
		if ids[it.ID] {
			return
		}
	}
	t.Errorf("no article ID in common between StreamContents and ItemIDs")
}

func TestErrors(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		c := newClient(t, appKey, "invalid-token")
		if _, err := c.UserInfo(t.Context()); !errors.Is(err, inoreader.ErrUnauthorized) {
			t.Errorf("error = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("invalid app key", func(t *testing.T) {
		c := newClient(t, "invalid-app-key", accessToken)
		_, err := c.UserInfo(t.Context())
		var apiErr *inoreader.APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("error = %v, want an *APIError", err)
		}
		// The developer portal documents 403 for wrong app credentials.
		if apiErr.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d (message %q)", apiErr.StatusCode, http.StatusForbidden, apiErr.Message)
		}
	})
}
