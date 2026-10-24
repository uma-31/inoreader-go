package inoreader

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func assertRequest(t *testing.T, got recorded, method, path string) {
	t.Helper()
	if got.method != method || got.path != path {
		t.Errorf("request = %s %s, want %s %s", got.method, got.path, method, path)
	}
}

func assertValues(t *testing.T, name string, got, want url.Values) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", name, diff)
	}
}

func TestUserInfo(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{
		"userId": "1005921515", "userName": "Bender", "userProfileId": "1005921515",
		"userEmail": "bender@example.com", "isBloggerUser": true,
		"signupTimeSec": 1163850013, "isMultiLoginEnabled": false
	}`, nil)

	u, err := c.UserInfo(t.Context())
	if err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	assertRequest(t, onlyRequest(t, reqs), http.MethodGet, "/reader/api/0/user-info")

	want := &User{
		ID: "1005921515", Name: "Bender", ProfileID: "1005921515", Email: "bender@example.com",
		IsBloggerUser: true, SignedUpAt: time.Unix(1163850013, 0),
	}
	if diff := cmp.Diff(want, u); diff != "" {
		t.Errorf("UserInfo() mismatch (-want +got):\n%s", diff)
	}
}

func TestListSubscriptions(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"subscriptions": [{
		"id": "feed/http://example.com/feed", "title": "Example",
		"categories": [{"id": "user/1005921515/label/Tech", "label": "Tech"}],
		"sortid": "00DA6134", "firstitemmsec": 1424501776942006,
		"url": "http://example.com/feed", "htmlUrl": "http://example.com/", "iconUrl": "", "feedType": "rss"
	}]}`, nil)

	subs, err := c.ListSubscriptions(t.Context(), &ListSubscriptionsOptions{TeamAssets: true})
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodGet, "/reader/api/0/subscription/list")
	assertValues(t, "query", req.query, url.Values{"team_assets": {"1"}})

	want := []Subscription{{
		ID: "feed/http://example.com/feed", Title: "Example",
		Categories: []Category{{ID: "user/1005921515/label/Tech", Label: "Tech"}},
		SortID:     "00DA6134", FirstItemAt: time.UnixMicro(1424501776942006),
		URL: "http://example.com/feed", HTMLURL: "http://example.com/", FeedType: "rss",
	}}
	if diff := cmp.Diff(want, subs); diff != "" {
		t.Errorf("ListSubscriptions() mismatch (-want +got):\n%s", diff)
	}
}

func TestSubscribe(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK,
		`{"query": "feed/http://example.com/feed", "numResults": 1, "streamId": "feed/http://example.com/feed", "streamName": "Example"}`, nil)

	got, err := c.Subscribe(t.Context(), FeedStream("http://example.com/feed"))
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/reader/api/0/subscription/quickadd")
	if ct := req.header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ct)
	}
	assertValues(t, "form", req.form, url.Values{"quickadd": {"feed/http://example.com/feed"}})
	want := &Subscribed{StreamID: "feed/http://example.com/feed", Title: "Example"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Subscribe() mismatch (-want +got):\n%s", diff)
	}
}

func TestSubscribeNotAdded(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `{"query": "feed/x", "numResults": 0}`, nil)
	if _, err := c.Subscribe(t.Context(), "feed/x"); !errors.Is(err, ErrNotSubscribed) {
		t.Errorf("error = %v, want ErrNotSubscribed", err)
	}
}

func TestEditSubscription(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)

	err := c.EditSubscription(t.Context(), EditSubscriptionRequest{
		Action: SubscriptionEdit, Feed: "feed/x", Title: "X", AddTo: LabelStream("A"), RemoveFrom: LabelStream("B"),
	})
	if err != nil {
		t.Fatalf("EditSubscription: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/reader/api/0/subscription/edit")
	assertValues(t, "form", req.form, url.Values{
		"ac": {"edit"}, "s": {"feed/x"}, "t": {"X"}, "a": {"user/-/label/A"}, "r": {"user/-/label/B"},
	})
}

func TestUnsubscribe(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)
	if err := c.Unsubscribe(t.Context(), "feed/x"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	assertValues(t, "form", onlyRequest(t, reqs).form, url.Values{"ac": {"unsubscribe"}, "s": {"feed/x"}})
}

func TestUnreadCounts(t *testing.T) {
	// The API writes some counts as strings.
	c, reqs := newTestClient(t, http.StatusOK, `{"max": 1000, "unreadcounts": [
		{"id": "user/1/state/com.google/reading-list", "count": 4, "newestItemTimestampUsec": "1415620910006331"},
		{"id": "user/1/label/Tech", "count": "3", "newestItemTimestampUsec": "1415620910006331"}
	]}`, nil)

	got, err := c.UnreadCounts(t.Context())
	if err != nil {
		t.Fatalf("UnreadCounts: %v", err)
	}
	assertRequest(t, onlyRequest(t, reqs), http.MethodGet, "/reader/api/0/unread-count")

	want := &UnreadCounts{Max: 1000, Counts: []UnreadCount{
		{ID: "user/1/state/com.google/reading-list", Count: 4, NewestItemAt: time.UnixMicro(1415620910006331)},
		{ID: "user/1/label/Tech", Count: 3, NewestItemAt: time.UnixMicro(1415620910006331)},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("UnreadCounts() mismatch (-want +got):\n%s", diff)
	}
}

func TestListTags(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"tags": [
		{"id": "user/1/state/com.google/starred", "sortid": "FFFFFFFF"},
		{"id": "user/1/label/Tech", "sortid": "BECC0BE1", "type": "folder", "unread_count": 3, "unseen_count": 1}
	]}`, nil)

	got, err := c.ListTags(t.Context(), &ListTagsOptions{Counts: true})
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodGet, "/reader/api/0/tag/list")
	assertValues(t, "query", req.query, url.Values{"types": {"1"}, "counts": {"1"}})

	want := []Tag{
		{ID: "user/1/state/com.google/starred", SortID: "FFFFFFFF"},
		{ID: "user/1/label/Tech", SortID: "BECC0BE1", Type: TagTypeFolder, UnreadCount: 3, UnseenCount: 1},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListTags() mismatch (-want +got):\n%s", diff)
	}
}

func TestRenameAndDeleteTag(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)
	ctx := t.Context()

	if err := c.RenameTag(ctx, LabelStream("Old"), "New"); err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if err := c.DeleteTag(ctx, LabelStream("New")); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	assertRequest(t, (*reqs)[0], http.MethodPost, "/reader/api/0/rename-tag")
	assertValues(t, "form", (*reqs)[0].form, url.Values{"s": {"user/-/label/Old"}, "dest": {"New"}})
	assertRequest(t, (*reqs)[1], http.MethodPost, "/reader/api/0/disable-tag")
	assertValues(t, "form", (*reqs)[1].form, url.Values{"s": {"user/-/label/New"}})
}

func TestEditTagHelpers(t *testing.T) {
	ctx := t.Context()
	tests := []struct {
		name string
		call func(*Client) error
		want url.Values
	}{
		{
			"MarkAsRead", func(c *Client) error { return c.MarkAsRead(ctx, 1, 2) },
			url.Values{"a": {"user/-/state/com.google/read"}, "i": {"1", "2"}},
		},
		{
			"MarkAsUnread", func(c *Client) error { return c.MarkAsUnread(ctx, 1) },
			url.Values{"r": {"user/-/state/com.google/read"}, "i": {"1"}},
		},
		{
			"Star", func(c *Client) error { return c.Star(ctx, -1) },
			url.Values{"a": {"user/-/state/com.google/starred"}, "i": {"-1"}},
		},
		{
			"Unstar", func(c *Client) error { return c.Unstar(ctx, 1) },
			url.Values{"r": {"user/-/state/com.google/starred"}, "i": {"1"}},
		},
		{"EditTag", func(c *Client) error {
			return c.EditTag(ctx, EditTagRequest{Add: LabelStream("x"), Remove: StreamRead, Items: []ItemID{3}})
		}, url.Values{"a": {"user/-/label/x"}, "r": {"user/-/state/com.google/read"}, "i": {"3"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, reqs := newTestClient(t, http.StatusOK, "OK", nil)
			if err := tt.call(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			req := onlyRequest(t, reqs)
			assertRequest(t, req, http.MethodPost, "/reader/api/0/edit-tag")
			assertValues(t, "form", req.form, tt.want)
		})
	}
}

func TestEditTagValidation(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)
	ctx := t.Context()

	if err := c.MarkAsRead(ctx); err == nil {
		t.Error("MarkAsRead without items succeeded")
	}
	if err := c.EditTag(ctx, EditTagRequest{Items: []ItemID{1}}); err == nil {
		t.Error("EditTag without tags succeeded")
	}
	if len(*reqs) != 0 {
		t.Errorf("sent %d requests", len(*reqs))
	}
}

const streamContentsBody = `{
	"direction": "ltr", "id": "user/-/label/Tech", "title": "Tech", "description": "",
	"updated": 1618212570, "updatedUsec": "1618212570146918",
	"items": [{
		"crawlTimeMsec": "1618211779000", "timestampUsec": "1618211779000000",
		"id": "tag:google.com,2005:reader/item/0000000693c3bc0c",
		"categories": ["user/1005921515/state/com.google/reading-list", "user/1005921515/state/com.google/read"],
		"title": "Title", "published": 1617969599, "updated": 1617990787,
		"canonical": [{"href": "https://example.com/1"}],
		"alternate": [{"href": "https://example.com/1", "type": "text/html"}],
		"summary": {"direction": "ltr", "content": "<p>Hi</p>"},
		"author": "Author", "likingUsers": [], "comments": [], "commentsNum": -1,
		"annotations": [{"id": 1, "start": 2, "end": 5, "added_on": 1618211779, "text": "abc", "note": "n",
			"user_id": 1005921515, "user_name": "Bender", "user_profile_picture": ""}],
		"summaries": [{"id": 9, "custom_prompt": "", "prompt_id": 1, "prompt_name": "One sentence summary",
			"prompt_icon": "", "summary": "<p>Short</p>", "is_mobilized": 0}],
		"origin": {"streamId": "feed/http://example.com/feed", "title": "Example", "htmlUrl": "http://example.com/"}
	}],
	"continuation": "next"
}`

func TestStreamContents(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, streamContentsBody, nil)
	since := time.UnixMicro(1600000000123456)

	got, err := c.StreamContents(t.Context(), FeedStream("http://example.com/a b?c"), &StreamContentsOptions{
		StreamOptions: StreamOptions{
			Count: 50, OldestFirst: true, Since: since, Exclude: StreamRead, Include: StreamStarred,
			Continuation: "prev", ExcludeFolderTags: true,
		},
		Annotations: true,
		Summaries:   true,
	})
	if err != nil {
		t.Fatalf("StreamContents: %v", err)
	}

	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodGet, "/reader/api/0/stream/contents/feed%2Fhttp:%2F%2Fexample.com%2Fa%20b%3Fc")
	assertValues(t, "query", req.query, url.Values{
		"n": {"50"}, "r": {"o"}, "ot": {"1600000000123456"},
		"xt": {"user/-/state/com.google/read"}, "it": {"user/-/state/com.google/starred"},
		"c": {"prev"}, "includeAllDirectStreamIds": {"false"}, "annotations": {"1"}, "summaries": {"1"},
	})

	if got.Continuation != "next" || got.UpdatedAt != time.UnixMicro(1618212570146918) || len(got.Items) != 1 {
		t.Fatalf("StreamContents() = %+v", got)
	}
	want := Item{
		ID:         0x693c3bc0c,
		Timestamp:  time.UnixMicro(1618211779000000),
		Published:  time.Unix(1617969599, 0),
		Updated:    time.Unix(1617990787, 0),
		Categories: []StreamID{"user/1005921515/state/com.google/reading-list", "user/1005921515/state/com.google/read"},
		Title:      "Title",
		Author:     "Author",
		Canonical:  []Link{{Href: "https://example.com/1"}},
		Alternate:  []Link{{Href: "https://example.com/1", Type: "text/html"}},
		Content:    Content{Direction: "ltr", HTML: "<p>Hi</p>"},
		Origin:     Origin{StreamID: "feed/http://example.com/feed", Title: "Example", HTMLURL: "http://example.com/"},
		Annotations: []Annotation{{
			ID: 1, Start: 2, End: 5, Text: "abc", Note: "n", UserID: 1005921515, UserName: "Bender",
		}},
		Summaries: []Summary{{ID: 9, PromptID: 1, PromptName: "One sentence summary", HTML: "<p>Short</p>"}},
	}
	if diff := cmp.Diff(want, got.Items[0]); diff != "" {
		t.Errorf("item mismatch (-want +got):\n%s", diff)
	}
	if !got.Items[0].HasCategory(StreamRead) || got.Items[0].HasCategory(StreamStarred) {
		t.Error("HasCategory() is wrong")
	}
	if u := got.Items[0].URL(); u != "https://example.com/1" {
		t.Errorf("URL() = %q", u)
	}
}

func TestItemIDs(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"items": [], "itemRefs": [
		{"id": "3614359203", "directStreamIds": ["user/1/label/MTB"], "timestampUsec": "1416313130505268"}
	], "continuation": "next"}`, nil)

	got, err := c.ItemIDs(t.Context(), StreamReadingList, &StreamOptions{Count: 1000})
	if err != nil {
		t.Fatalf("ItemIDs: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodGet, "/reader/api/0/stream/items/ids")
	assertValues(t, "query", req.query, url.Values{"n": {"1000"}, "s": {"user/-/state/com.google/reading-list"}})

	want := &ItemRefs{Continuation: "next", Refs: []ItemRef{{
		ID: 3614359203, DirectStreamIDs: []StreamID{"user/1/label/MTB"}, Timestamp: time.UnixMicro(1416313130505268),
	}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ItemIDs() mismatch (-want +got):\n%s", diff)
	}
}

// pagedServer serves a stream of three articles in pages of one, chained by
// continuations, and records the continuation of each request.
func pagedServer(t *testing.T) (*Client, *[]string) {
	t.Helper()

	type page struct{ id, next string }
	pages := map[string]page{"": {"1", "c1"}, "c1": {"2", "c2"}, "c2": {"3", ""}}

	var continuations []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cont := r.URL.Query().Get("c")
		continuations = append(continuations, cont)

		p, ok := pages[cont]
		if !ok {
			t.Errorf("unknown continuation %q", cont)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		item := map[string]string{"id": p.id, "timestampUsec": "1"}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":        []any{item},
			"itemRefs":     []any{item},
			"continuation": p.next,
		})
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient("id", "key", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, &continuations
}

func TestAllItems(t *testing.T) {
	c, continuations := pagedServer(t)

	var ids []ItemID
	for it, err := range c.AllItems(t.Context(), LabelStream("Tech"), nil) {
		if err != nil {
			t.Fatalf("AllItems: %v", err)
		}
		ids = append(ids, it.ID)
	}
	if diff := cmp.Diff([]ItemID{1, 2, 3}, ids); diff != "" {
		t.Errorf("ids mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"", "c1", "c2"}, *continuations); diff != "" {
		t.Errorf("continuations mismatch (-want +got):\n%s", diff)
	}
}

func TestAllItemRefsStopsEarly(t *testing.T) {
	c, continuations := pagedServer(t)

	for ref, err := range c.AllItemRefs(t.Context(), LabelStream("Tech"), nil) {
		if err != nil {
			t.Fatalf("AllItemRefs: %v", err)
		}
		if ref.ID == 2 {
			break
		}
	}
	if len(*continuations) != 2 {
		t.Errorf("fetched %d pages, want 2", len(*continuations))
	}
}

func TestAllItemsStuckContinuation(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, streamContentsBody, nil)

	var err error
	n := 0
	for _, err = range c.AllItems(t.Context(), LabelStream("Tech"), &StreamContentsOptions{
		StreamOptions: StreamOptions{Continuation: "next"},
	}) {
		if err != nil {
			break
		}
		n++
	}
	if !errors.Is(err, errStuckContinuation) || n != 1 {
		t.Errorf("err = %v after %d items, want errStuckContinuation after 1", err, n)
	}
}

func TestAllItemsError(t *testing.T) {
	c, _ := newTestClient(t, http.StatusTooManyRequests, "Error=limit", nil)

	for _, err := range c.AllItems(t.Context(), LabelStream("Tech"), nil) {
		if !errors.Is(err, ErrRateLimited) {
			t.Errorf("err = %v, want ErrRateLimited", err)
		}
	}
}

func TestMarkAllAsRead(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)

	if err := c.MarkAllAsRead(t.Context(), LabelStream("Tech"), time.UnixMicro(1618212570146918)); err != nil {
		t.Fatalf("MarkAllAsRead: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/reader/api/0/mark-all-as-read")
	assertValues(t, "form", req.form, url.Values{"s": {"user/-/label/Tech"}, "ts": {"1618212570146918"}})

	if err := c.MarkAllAsRead(t.Context(), LabelStream("Tech"), time.Time{}); err == nil {
		t.Error("MarkAllAsRead without seenAt succeeded")
	}
}

func TestStreamPreferences(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"streamprefs": {
		"user/1005921515/state/com.google/root": [{"id": "subscription-ordering", "value": "BEC24025BEBD5A25"}],
		"user/1005921515/label/MIT": [{"id": "is-expanded", "value": "true"}]
	}}`, nil)

	prefs, err := c.StreamPreferences(t.Context())
	if err != nil {
		t.Fatalf("StreamPreferences: %v", err)
	}
	assertRequest(t, onlyRequest(t, reqs), http.MethodGet, "/reader/api/0/preference/stream/list")

	got := prefs.SubscriptionOrdering(StreamRoot)
	if diff := cmp.Diff([]string{"BEC24025", "BEBD5A25"}, got); diff != "" {
		t.Errorf("SubscriptionOrdering() mismatch (-want +got):\n%s", diff)
	}
	if v, ok := prefs.Get(LabelStream("MIT"), "is-expanded"); !ok || v != "true" {
		t.Errorf("Get() = %q, %v", v, ok)
	}
	if got := prefs.SubscriptionOrdering(LabelStream("MIT")); got != nil {
		t.Errorf("SubscriptionOrdering() of a folder without one = %v", got)
	}
}

func TestSetSubscriptionOrdering(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "OK", nil)

	if err := c.SetSubscriptionOrdering(t.Context(), StreamRoot, []string{"BEC24025", "00da6134"}); err != nil {
		t.Fatalf("SetSubscriptionOrdering: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/reader/api/0/preference/stream/set")
	assertValues(t, "form", req.form, url.Values{
		"s": {"user/-/state/com.google/root"}, "k": {"subscription-ordering"}, "v": {"BEC2402500da6134"},
	})

	if err := c.SetSubscriptionOrdering(t.Context(), StreamRoot, []string{"XYZ"}); err == nil {
		t.Error("SetSubscriptionOrdering with an invalid sort ID succeeded")
	}
}

func TestCreateActiveSearch(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"id": "feed\\keyword-monitoring-AbCdEf", "title": "amazing"}`, nil)

	got, err := c.CreateActiveSearch(t.Context(), ActiveSearchRequest{
		Term: "amazing", FilterType: ActiveSearchInSubscription, FilterID: "feed/x",
		ArticleIDs: []ItemID{12363668564}, SearchMatch: "any_word",
	})
	if err != nil {
		t.Fatalf("CreateActiveSearch: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/reader/api/0/active_search/create")
	if ct := req.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(req.rawBody), &body); err != nil {
		t.Fatalf("request body: %v", err)
	}
	wantBody := map[string]any{
		"term": "amazing", "filter_type": "subscription", "filter_id": "feed/x",
		"article_ids": []any{12363668564.0}, "search_match": "any_word",
	}
	if diff := cmp.Diff(wantBody, body); diff != "" {
		t.Errorf("request body mismatch (-want +got):\n%s", diff)
	}
	want := &ActiveSearch{ID: `feed\keyword-monitoring-AbCdEf`, Title: "amazing"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("CreateActiveSearch() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateActiveSearchRefused(t *testing.T) {
	c, _ := newTestClient(t, http.StatusAccepted, `{"error": "Limit reached", "accountLimit": 30, "needUpgrade": "1"}`, nil)

	_, err := c.CreateActiveSearch(t.Context(), ActiveSearchRequest{Term: "x", FilterType: ActiveSearchInPublic})
	var asErr *ActiveSearchError
	if !errors.As(err, &asErr) {
		t.Fatalf("error = %v, want an *ActiveSearchError", err)
	}
	if want := (ActiveSearchError{Message: "Limit reached", AccountLimit: 30, NeedUpgrade: true}); *asErr != want {
		t.Errorf("ActiveSearchError = %+v, want %+v", *asErr, want)
	}
}

func TestCreateActiveSearchValidation(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "{}", nil)
	ctx := t.Context()

	if _, err := c.CreateActiveSearch(ctx, ActiveSearchRequest{FilterType: ActiveSearchInPublic}); err == nil {
		t.Error("CreateActiveSearch without a term succeeded")
	}
	if _, err := c.CreateActiveSearch(ctx, ActiveSearchRequest{Term: "x", FilterType: ActiveSearchInFolder}); err == nil {
		t.Error("CreateActiveSearch in a folder without its ID succeeded")
	}
	if len(*reqs) != 0 {
		t.Errorf("sent %d requests", len(*reqs))
	}
}

func TestDeleteActiveSearch(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "ok", nil)

	if err := c.DeleteActiveSearch(t.Context(), "user/-/label/search"); err != nil {
		t.Fatalf("DeleteActiveSearch: %v", err)
	}
	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodGet, "/reader/api/0/active_search/delete")
	assertValues(t, "query", req.query, url.Values{"id": {"user/-/label/search"}})
}

func TestClientLogin(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, "SID=null\nLSID=null\nAuth=G2UlCaFx\n", nil)
	c.authToken = "stale"

	token, err := c.ClientLogin(t.Context(), "user@example.com", "secret")
	if err != nil {
		t.Fatalf("ClientLogin: %v", err)
	}
	if token != "G2UlCaFx" {
		t.Errorf("token = %q", token)
	}

	req := onlyRequest(t, reqs)
	assertRequest(t, req, http.MethodPost, "/accounts/ClientLogin")
	assertValues(t, "form", req.form, url.Values{"Email": {"user@example.com"}, "Passwd": {"secret"}})
	if auth := req.header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization = %q, want none", auth)
	}
}

func TestClientLoginWithoutToken(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, "SID=null\n", nil)
	if _, err := c.ClientLogin(t.Context(), "u", "p"); err == nil {
		t.Error("ClientLogin succeeded without a token")
	}
}

func TestEndpoint(t *testing.T) {
	e := NewEndpoint("http://127.0.0.1:4010/")
	if e.AuthURL != "http://127.0.0.1:4010/oauth2/auth" || e.TokenURL != "http://127.0.0.1:4010/oauth2/token" {
		t.Errorf("NewEndpoint() = %+v", e)
	}
	if Endpoint.TokenURL != "https://www.inoreader.com/oauth2/token" {
		t.Errorf("Endpoint.TokenURL = %q", Endpoint.TokenURL)
	}
}

func TestUnreadCountsInvalidCount(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `{"max": 1000, "unreadcounts": [
		{"id": "user/1/label/Tech", "count": "many", "newestItemTimestampUsec": "1"}
	]}`, nil)

	_, err := c.UnreadCounts(t.Context())
	if err == nil || !strings.Contains(err.Error(), `"many"`) {
		t.Errorf("error = %v, want one naming the value", err)
	}
}
