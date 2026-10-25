package inoreader

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

// The fuzz tests run their seeds with the unit tests. Run one for longer
// with `go test -run '^$' -fuzz '^FuzzResponses$' -fuzztime 1m`.

func FuzzParseItemID(f *testing.F) {
	for _, s := range []string{
		"tag:google.com,2005:reader/item/00000000148b9369",
		"tag:google.com,2005:reader/item/ffffffffffffffff",
		"344691561",
		"-9223372036854775808",
		"18446744073709551615",
		"tag:google.com,2005:reader/item/",
		"",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		id, err := ParseItemID(s)
		if err != nil {
			return
		}
		// Both forms of an accepted ID name the same article.
		for _, form := range []string{id.String(), id.Long()} {
			got, err := ParseItemID(form)
			if err != nil || got != id {
				t.Errorf("ParseItemID(%q) = %d, %v; want %d, parsed from %q", form, got, err, id, s)
			}
		}
	})
}

func FuzzStreamID(f *testing.F) {
	for _, s := range []string{
		"user/1005921515/label/Tech",
		"user/-/label/Tech",
		"feed/http://example.com/feed",
		string(StreamReadingList),
		"user/1/user/2/",
		"",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		id := StreamID(s)
		if n := id.Normalize(); n.Normalize() != n {
			t.Errorf("Normalize is not idempotent on %q: %q, then %q", s, n, n.Normalize())
		}
		if got, ok := FeedStream(s).FeedURL(); !ok || got != s {
			t.Errorf("FeedStream(%q).FeedURL() = %q, %t", s, got, ok)
		}
		if got, ok := LabelStream(s).Label(); !ok || got != s {
			t.Errorf("LabelStream(%q).Label() = %q, %t", s, got, ok)
		}
	})
}

// fuzzTransport answers every request with the same response.
type fuzzTransport struct {
	status int
	header http.Header
	body   []byte
}

func (t fuzzTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Header:     t.header,
		Body:       io.NopCloser(bytes.NewReader(t.body)),
		Request:    req,
	}, nil
}

// FuzzResponses feeds arbitrary responses to the methods, which must report
// an error rather than panic, and must not accept an error status.
func FuzzResponses(f *testing.F) {
	for _, seed := range []struct {
		status    int
		rateLimit string
		body      string
	}{
		{200, "100", `{"userId":"1005921515","userName":"user","signupTimeSec":1163850013}`},
		{200, "100", `{"max":1000,"unreadcounts":[{"id":"user/-/state/com.google/reading-list","count":"12","newestItemTimestampUsec":"1700000000000000"}]}`},
		{200, "", `{"subscriptions":[{"id":"feed/http://example.com/feed","sortid":"0000000A","categories":[{"id":"user/1/label/Tech"}]}]}`},
		{200, "", `{"items":[{"id":"tag:google.com,2005:reader/item/00000000148b9369","timestampUsec":"1700000000000000","origin":{"streamId":"feed/x"}}],"continuation":"abc"}`},
		{200, "", `{"itemRefs":[{"id":"344691561","timestampUsec":"1700000000000000"}]}`},
		{200, "", `{"streamprefs":{"user/1/state/com.google/root":[{"id":"subscription-ordering","value":"0000000A0000000B12"}]}}`},
		{200, "", "OK"},
		{429, "-1", "Error=Daily request limit reached"},
		{401, "", ""},
	} {
		f.Add(seed.status, seed.rateLimit, []byte(seed.body))
	}

	f.Fuzz(func(t *testing.T, status int, rateLimit string, body []byte) {
		if status < 100 || status > 999 {
			return
		}
		header := http.Header{}
		for _, name := range []string{
			"X-Reader-Zone1-Limit", "X-Reader-Zone1-Usage",
			"X-Reader-Zone2-Limit", "X-Reader-Zone2-Usage",
			"X-Reader-Limits-Reset-After",
		} {
			header.Set(name, rateLimit)
		}
		c, err := NewClient("id", "key", WithHTTPClient(&http.Client{
			Transport: fuzzTransport{status: status, header: header, body: body},
		}))
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		failed := status < 200 || status > 299

		check := func(method string, err error) {
			if failed && err == nil {
				t.Errorf("%s accepted status %d", method, status)
			}
		}

		_, err = c.UserInfo(ctx)
		check("UserInfo", err)
		_, err = c.UnreadCounts(ctx)
		check("UnreadCounts", err)
		_, err = c.ListSubscriptions(ctx, nil)
		check("ListSubscriptions", err)
		_, err = c.ListTags(ctx, &ListTagsOptions{Counts: true})
		check("ListTags", err)
		_, err = c.StreamContents(ctx, StreamReadingList, nil)
		check("StreamContents", err)
		_, err = c.ItemIDs(ctx, StreamReadingList, nil)
		check("ItemIDs", err)
		prefs, err := c.StreamPreferences(ctx)
		check("StreamPreferences", err)
		for _, id := range prefs.SubscriptionOrdering(StreamRoot) {
			if len(id) != 8 {
				t.Errorf("SubscriptionOrdering returned sort ID %q", id)
			}
		}
		err = c.SetSubscriptionOrdering(ctx, StreamRoot, []string{"0000000A"})
		check("SetSubscriptionOrdering", err)
		_ = c.RateLimit()
	})
}
