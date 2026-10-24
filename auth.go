package inoreader

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// OAuth 2.0 scopes.
const (
	// ScopeRead allows the methods that read data.
	ScopeRead = "read"
	// ScopeWrite allows the methods that change data, together with
	// [ScopeRead].
	ScopeWrite = "write"
)

// Endpoint is the OAuth 2.0 endpoint of Inoreader, for an
// [oauth2.Config] whose ClientID and ClientSecret are the App ID and App
// key.
var Endpoint = NewEndpoint(DefaultBaseURL)

// NewEndpoint returns the OAuth 2.0 endpoint of the server at baseURL, such
// as a mock server.
func NewEndpoint(baseURL string) oauth2.Endpoint {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return oauth2.Endpoint{
		AuthURL:   baseURL + "/oauth2/auth",
		TokenURL:  baseURL + "/oauth2/token",
		AuthStyle: oauth2.AuthStyleInParams,
	}
}

// ClientLogin signs the user in with their email or username and password,
// and returns a token for [WithClientLoginToken].
//
// It is the legacy method kept for Google Reader clients; prefer OAuth 2.0,
// which does not hand the user's password to the app.
func (c *Client) ClientLogin(ctx context.Context, email, password string) (string, error) {
	form := url.Values{"Email": {email}, "Passwd": {password}}
	req, err := c.newRequest(ctx, http.MethodPost, "/accounts/ClientLogin", nil, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Del("Authorization")

	_, body, err := c.do(req)
	if err != nil {
		return "", err
	}

	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		if token, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "Auth="); ok && token != "" {
			return token, nil
		}
	}
	return "", errors.New("inoreader: no Auth token in ClientLogin response")
}
