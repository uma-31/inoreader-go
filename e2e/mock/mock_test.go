package mock

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func readSpec(t *testing.T) []byte {
	t.Helper()
	spec, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

// served is what a server answered to a request, and the violations it
// reported.
type served struct {
	status     int
	header     http.Header
	body       string
	violations []string
}

// serve sends r to a server for spec.
func serve(t *testing.T, spec []byte, r *http.Request) served {
	t.Helper()

	var violations []string
	srv, err := New(spec, func(v string) { violations = append(violations, v) })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, r)
	return served{status: rec.Code, header: rec.Header(), body: rec.Body.String(), violations: violations}
}

func authorized(r *http.Request) *http.Request {
	r.Header.Set("AppId", "id")
	r.Header.Set("AppKey", "key")
	r.Header.Set("Authorization", "Bearer token")
	return r
}

func form(t *testing.T, path string, values url.Values) *http.Request {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return authorized(r)
}

func get(t *testing.T, target string) *http.Request {
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
}

func TestServe(t *testing.T) {
	spec := readSpec(t)

	tests := []struct {
		name          string
		req           *http.Request
		status        int
		body          string // Expected substring of the body.
		header        string // Expected response header.
		wantViolation bool
	}{
		{
			name:   "JSON example and headers",
			req:    authorized(get(t, "/reader/api/0/user-info")),
			status: http.StatusOK, body: `"userName":"mock-user"`, header: "X-Reader-Zone1-Limit",
		},
		{
			name:   "plain text example",
			req:    form(t, "/reader/api/0/edit-tag", url.Values{"a": {"user/-/state/com.google/read"}, "i": {"1"}}),
			status: http.StatusOK, body: "OK",
		},
		{
			name:   "escaped slashes in a path parameter",
			req:    authorized(get(t, "/reader/api/0/stream/contents/feed%2Fhttp:%2F%2Fexample.com%2Ffeed?n=5")),
			status: http.StatusOK, body: `"continuation"`,
		},
		{
			name: "preferred status",
			req: func() *http.Request {
				r := authorized(get(t, "/reader/api/0/user-info"))
				r.Header.Set("Prefer", "code=429")
				return r
			}(),
			status: http.StatusTooManyRequests, body: "Error=", header: "X-Reader-Limits-Reset-After",
		},
		{
			name: "no bearer token",
			req: func() *http.Request {
				r := authorized(get(t, "/reader/api/0/user-info"))
				r.Header.Del("Authorization")
				return r
			}(),
			status: http.StatusUnauthorized,
		},
		{
			name: "no app key",
			req: func() *http.Request {
				r := authorized(get(t, "/reader/api/0/user-info"))
				r.Header.Del("AppKey")
				return r
			}(),
			status: http.StatusUnauthorized,
		},
		{
			name:   "missing form field",
			req:    form(t, "/reader/api/0/edit-tag", url.Values{"a": {"user/-/state/com.google/read"}}),
			status: http.StatusBadRequest, wantViolation: true,
		},
		{
			name:   "misnamed form field",
			req:    form(t, "/reader/api/0/edit-tag", url.Values{"a": {"user/-/state/com.google/read"}, "item": {"1"}}),
			status: http.StatusBadRequest, wantViolation: true,
		},
		{
			name:   "query parameter out of range",
			req:    authorized(get(t, "/reader/api/0/stream/items/ids?s=user%2F-%2Flabel%2FTech&n=5000")),
			status: http.StatusBadRequest, wantViolation: true,
		},
		{
			name:   "unknown operation",
			req:    authorized(get(t, "/reader/api/0/nothing")),
			status: http.StatusNotFound, wantViolation: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serve(t, spec, tt.req)

			if got.status != tt.status {
				t.Errorf("status = %d, want %d (body %q)", got.status, tt.status, got.body)
			}
			if !strings.Contains(got.body, tt.body) {
				t.Errorf("body = %q, want it to contain %q", got.body, tt.body)
			}
			if tt.header != "" && got.header.Get(tt.header) == "" {
				t.Errorf("no %s header", tt.header)
			}
			if hasViolations := len(got.violations) > 0; hasViolations != tt.wantViolation {
				t.Errorf("violations = %q, want some: %v", got.violations, tt.wantViolation)
			}
		})
	}
}

func TestDriftedExample(t *testing.T) {
	// A string example where the schema says integer.
	spec := strings.Replace(string(readSpec(t)), "signupTimeSec: 1163850013", `signupTimeSec: "not a number"`, 1)
	if spec == string(readSpec(t)) {
		t.Fatal("the example to change is gone from the description")
	}

	got := serve(t, []byte(spec), authorized(get(t, "/reader/api/0/user-info")))
	if len(got.violations) == 0 {
		t.Error("no violation for an example that does not match its schema")
	}
}

func TestInvalidDescription(t *testing.T) {
	if _, err := New([]byte("openapi: 3.1.0\ninfo: {}\n"), func(string) {}); err == nil {
		t.Error("New accepted an invalid description")
	}
}
