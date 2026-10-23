package inoreader

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// recorded is a request received by a test server.
type recorded struct {
	method  string
	path    string // Escaped, as sent.
	query   url.Values
	header  http.Header
	form    url.Values
	rawBody string
}

// newTestClient returns a client of a server that records each request and
// answers with status and body.
func newTestClient(t *testing.T, status int, body string, header http.Header) (*Client, *[]recorded) {
	t.Helper()

	var reqs []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		form, _ := url.ParseQuery(string(raw))
		reqs = append(reqs, recorded{
			method:  r.Method,
			path:    r.URL.EscapedPath(),
			query:   r.URL.Query(),
			header:  r.Header.Clone(),
			form:    form,
			rawBody: string(raw),
		})

		for k, vs := range header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient("app-id", "app-key", WithBaseURL(srv.URL), WithUserAgent("inoreader-go-test"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, &reqs
}

func onlyRequest(t *testing.T, reqs *[]recorded) recorded {
	t.Helper()
	if len(*reqs) != 1 {
		t.Fatalf("got %d requests, want 1", len(*reqs))
	}
	return (*reqs)[0]
}

func rateLimitHeader() http.Header {
	h := http.Header{}
	h.Set("X-Reader-Zone1-Limit", "100")
	h.Set("X-Reader-Zone1-Usage", "42")
	h.Set("X-Reader-Zone2-Limit", "50")
	h.Set("X-Reader-Zone2-Usage", "7")
	h.Set("X-Reader-Limits-Reset-After", "3600")
	return h
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		appID   string
		appKey  string
		opts    []Option
		wantErr bool
	}{
		{name: "defaults", appID: "id", appKey: "key"},
		{name: "missing app ID", appKey: "key", wantErr: true},
		{name: "missing app key", appID: "id", wantErr: true},
		{name: "relative base URL", appID: "id", appKey: "key", opts: []Option{WithBaseURL("/api")}, wantErr: true},
		{name: "nil HTTP client", appID: "id", appKey: "key", opts: []Option{WithHTTPClient(nil)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.appID, tt.appKey, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequestHeaders(t *testing.T) {
	c, reqs := newTestClient(t, http.StatusOK, `{"userId":"1"}`, nil)
	c.authToken = "login-token"

	if _, err := c.UserInfo(context.Background()); err != nil {
		t.Fatalf("UserInfo: %v", err)
	}

	h := onlyRequest(t, reqs).header
	for name, want := range map[string]string{
		"AppId":         "app-id",
		"AppKey":        "app-key",
		"Authorization": "GoogleLogin auth=login-token",
		"User-Agent":    "inoreader-go-test",
	} {
		if got := h.Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
}

func TestBaseURLWithPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	c, err := NewClient("id", "key", WithBaseURL(srv.URL+"/proxy/"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.UserInfo(context.Background()); err != nil {
		t.Fatalf("UserInfo: %v", err)
	}
	if want := "/proxy/reader/api/0/user-info"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestAPIError(t *testing.T) {
	tests := []struct {
		status  int
		body    string
		want    error
		wantMsg string
	}{
		{http.StatusBadRequest, "Error=Mandatory parameter missing", ErrBadRequest, "Mandatory parameter missing"},
		{http.StatusUnauthorized, "", ErrUnauthorized, ""},
		{http.StatusForbidden, "Error=Forbidden", ErrForbidden, "Forbidden"},
		{http.StatusTooManyRequests, "Error=Daily request limit reached\n", ErrRateLimited, "Daily request limit reached"},
		{http.StatusServiceUnavailable, "", ErrServiceUnavailable, ""},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			c, _ := newTestClient(t, tt.status, tt.body, rateLimitHeader())

			_, err := c.UserInfo(context.Background())
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error %T is not an *APIError", err)
			}
			if apiErr.StatusCode != tt.status || apiErr.Message != tt.wantMsg {
				t.Errorf("APIError = %d %q, want %d %q", apiErr.StatusCode, apiErr.Message, tt.status, tt.wantMsg)
			}
			if apiErr.RateLimit == nil {
				t.Error("APIError.RateLimit is nil")
			}
		})
	}
}

func TestUnexpectedPlainResponse(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, "Error=Something went wrong", nil)

	err := c.DeleteTag(context.Background(), LabelStream("Tech"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Something went wrong" {
		t.Errorf("error = %v, want an *APIError with the message", err)
	}
}

func TestRateLimit(t *testing.T) {
	c, _ := newTestClient(t, http.StatusOK, `{}`, rateLimitHeader())
	if c.RateLimit() != nil {
		t.Fatal("RateLimit() before any request is not nil")
	}

	before := time.Now()
	if _, err := c.UserInfo(context.Background()); err != nil {
		t.Fatalf("UserInfo: %v", err)
	}

	rl := c.RateLimit()
	if rl == nil {
		t.Fatal("RateLimit() is nil")
	}
	if rl.Zone1 != (Quota{Limit: 100, Usage: 42}) || rl.Zone2 != (Quota{Limit: 50, Usage: 7}) {
		t.Errorf("RateLimit() = %+v", rl)
	}
	if rl.Zone1.Remaining() != 58 {
		t.Errorf("Zone1.Remaining() = %d, want 58", rl.Zone1.Remaining())
	}
	if rl.ResetAfter != time.Hour || rl.ResetAt.Before(before.Add(time.Hour)) {
		t.Errorf("ResetAfter = %v, ResetAt = %v", rl.ResetAfter, rl.ResetAt)
	}
}

func TestParseRateLimitPartial(t *testing.T) {
	h := rateLimitHeader()
	h.Del("X-Reader-Limits-Reset-After")
	if rl := parseRateLimit(h, time.Now()); rl != nil {
		t.Errorf("parseRateLimit() = %+v, want nil", rl)
	}
}

func TestQuotaRemainingNeverNegative(t *testing.T) {
	if got := (Quota{Limit: 10, Usage: 12}).Remaining(); got != 0 {
		t.Errorf("Remaining() = %d, want 0", got)
	}
}
