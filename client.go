package inoreader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the address of the Inoreader API.
const DefaultBaseURL = "https://www.inoreader.com"

const apiPath = "/reader/api/0"

// defaultTimeout bounds each request of the default HTTP client, which
// [http.DefaultClient] leaves unbounded.
const defaultTimeout = 30 * time.Second

// Client calls the Inoreader API on behalf of one user.
//
// The client authenticates the app with its App ID and App key. The user is
// authenticated by the [http.Client] given with [WithHTTPClient], typically
// one built from an [golang.org/x/oauth2.Config], or by a ClientLogin token
// given with [WithClientLoginToken].
//
// A Client is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	appID      string
	appKey     string
	userAgent  string
	authToken  string

	mu        sync.Mutex
	rateLimit *RateLimit
}

// Option configures a [Client].
type Option func(*Client) error

// WithHTTPClient sets the HTTP client used for requests. Pass the client of
// an [golang.org/x/oauth2.Config] to authenticate the user with OAuth 2.0.
//
// The default client times out after 30 seconds; set a timeout on the given
// client too. Redirects are refused whatever the given client allows; see
// [ErrRedirect].
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return errors.New("inoreader: nil HTTP client")
		}
		c.httpClient = httpClient
		return nil
	}
}

// WithBaseURL replaces [DefaultBaseURL], to call a mock server for instance.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("inoreader: invalid base URL: %w", err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("inoreader: base URL %q must be absolute", baseURL)
		}
		c.baseURL = strings.TrimSuffix(u.String(), "/")
		return nil
	}
}

// WithUserAgent sets the User-Agent header of requests.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		c.userAgent = userAgent
		return nil
	}
}

// WithClientLoginToken authenticates the user with a token from
// [Client.ClientLogin] instead of OAuth 2.0.
func WithClientLoginToken(token string) Option {
	return func(c *Client) error {
		c.authToken = token
		return nil
	}
}

// NewClient returns a client for the app registered with appID and appKey.
func NewClient(appID, appKey string, opts ...Option) (*Client, error) {
	if appID == "" || appKey == "" {
		return nil, errors.New("inoreader: app ID and app key are required")
	}

	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		appID:      appID,
		appKey:     appKey,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Copy the client rather than change the caller's.
	hc := *c.httpClient
	hc.CheckRedirect = refuseRedirect
	c.httpClient = &hc
	return c, nil
}

// refuseRedirect stops redirects before they are followed. The API does not
// redirect, and following one would send the app credentials, which are
// custom headers, and an OAuth 2.0 token to wherever it points.
func refuseRedirect(req *http.Request, _ []*http.Request) error {
	return fmt.Errorf("%w to %s", ErrRedirect, req.URL.Redacted())
}

// RateLimit returns the rate limit reported by the latest response that
// carried one, or nil before any such response.
func (c *Client) RateLimit() *RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rateLimit == nil {
		return nil
	}
	rl := *c.rateLimit
	return &rl
}

// newRequest builds a request to path, which must already be escaped.
func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("inoreader: invalid request URL: %w", err)
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("inoreader: %w", err)
	}
	req.Header.Set("AppId", c.appID)
	req.Header.Set("AppKey", c.appKey)
	if c.authToken != "" {
		req.Header.Set("Authorization", "GoogleLogin auth="+c.authToken)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

// do sends req and returns the status code and body of the response. A
// status other than 2xx is returned as an [*APIError].
func (c *Client) do(req *http.Request) (int, []byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("inoreader: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("inoreader: reading response: %w", err)
	}

	rl := parseRateLimit(resp.Header, time.Now())
	if rl != nil {
		c.mu.Lock()
		c.rateLimit = rl
		c.mu.Unlock()
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, nil, newAPIError(resp.StatusCode, body, rl)
	}
	return resp.StatusCode, body, nil
}

// getJSON sends a GET request and decodes the JSON response into v.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, v any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	_, body, err := c.do(req)
	if err != nil {
		return err
	}
	return decodeJSON(body, v)
}

// postForm sends a form and decodes the JSON response into v, or expects
// the plain `OK` that most methods changing data respond with when v is nil.
func (c *Client) postForm(ctx context.Context, path string, form url.Values, v any) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, nil, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	status, body, err := c.do(req)
	if err != nil {
		return err
	}
	if v == nil {
		return expectOK(status, body)
	}
	return decodeJSON(body, v)
}

func decodeJSON(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("inoreader: decoding response: %w", err)
	}
	return nil
}

func expectOK(status int, body []byte) error {
	if !strings.EqualFold(string(bytes.TrimSpace(body)), "OK") {
		return newAPIError(status, body, nil)
	}
	return nil
}
