# inoreader-go

[![CI](https://github.com/uma-31/inoreader-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/uma-31/inoreader-go/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uma-31/inoreader-go.svg)](https://pkg.go.dev/github.com/uma-31/inoreader-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go client library for the [Inoreader API](https://www.inoreader.com/developers/).

```sh
go get github.com/uma-31/inoreader-go
```

## Usage

Register an app in the [Inoreader preferences](https://www.inoreader.com/preferences/other) to get its App ID and App key. API access needs an Inoreader Pro plan, except for public apps on platforms Inoreader is not available on.

Authenticate the user with OAuth 2.0 through [`golang.org/x/oauth2`](https://pkg.go.dev/golang.org/x/oauth2) and pass its HTTP client to the Inoreader client:

```go
conf := &oauth2.Config{
	ClientID:     appID,
	ClientSecret: appKey,
	Endpoint:     inoreader.Endpoint,
	RedirectURL:  "https://example.com/callback",
	Scopes:       []string{inoreader.ScopeRead, inoreader.ScopeWrite},
}
// Send the user to conf.AuthCodeURL(state), then exchange the code.
token, err := conf.Exchange(ctx, code)

client, err := inoreader.NewClient(appID, appKey,
	inoreader.WithHTTPClient(conf.Client(ctx, token)))

unread, err := client.UnreadCounts(ctx)
for item, err := range client.AllItems(ctx, inoreader.StreamReadingList, nil) {
	// ...
}
```

The API limits the requests of an app per day, and the default limit can be as low as 100. `client.RateLimit()` reports the usage, and a reached limit fails with `inoreader.ErrRateLimited`.

### Coverage

| API method | Client method |
| :-- | :-- |
| User information | `UserInfo` |
| Add feed | `Subscribe` |
| Edit feed | `EditSubscription`, `Unsubscribe` |
| Unread counts | `UnreadCounts` |
| Feeds list | `ListSubscriptions` |
| Folder/Tag list | `ListTags` |
| Stream contents | `StreamContents`, `AllItems` |
| Item IDs | `ItemIDs`, `AllItemRefs` |
| Stream preferences list | `StreamPreferences` |
| Stream preferences set | `SetSubscriptionOrdering` |
| Rename tag | `RenameTag` |
| Delete tag | `DeleteTag` |
| Edit tag | `EditTag`, `MarkAsRead`, `MarkAsUnread`, `Star`, `Unstar` |
| Mark all as read | `MarkAllAsRead` |
| Create active search | `CreateActiveSearch` |
| Delete active search | `DeleteActiveSearch` |
| ClientLogin (legacy) | `ClientLogin` |

## Development

Install [Go](https://go.dev/dl/) 1.21 or later; the `toolchain` directive in `go.mod` makes it download and use the version this module is developed with. The other tools and the tasks are managed with [mise](https://mise.jdx.dev/): run `mise install` once, then `mise tasks` to list the tasks.

The `go` directive in `go.mod` is the oldest Go version dependents need; CI runs the unit tests on it and on the latest release.

| Task | What it does |
| :-- | :-- |
| `mise run test:unit` | Run the unit tests |
| `mise run test:e2e` | Start the mock server and run the E2E tests against it |
| `mise run test:live` | Run the live tests against the real API, with the credentials in `.env` |
| `mise run go:check` | Check formatting, lint and `go.mod` |
| `mise run go:fix` | Format and apply lint fixes |
| `mise run api:lint` | Lint the API description |
| `mise run mock` | Start the mock server |

### API description

[`api/openapi.yaml`](api/openapi.yaml) describes the Inoreader API in OpenAPI 3.1. Inoreader publishes no machine-readable description, so it is written by hand from the developer portal. It is the contract the client is written against, and the source of the mock server.

### Tests

Unit tests run the client against `httptest` servers and check the requests it builds and how it decodes responses.

E2E tests, under [`e2e/`](e2e) behind the `e2e` build tag, run the client against a mock server that [Prism](https://stoplight.io/open-source/prism) serves from the API description. `mise run test:e2e` starts the mock server, runs the tests and stops it; arguments after `--` go to `go test`, as in `mise run test:e2e -- -run TestStreams -v`. To run the tests against a mock server that is already running, set `INOREADER_E2E_BASE_URL` to its address and run `go test -tags e2e ./e2e/...`.

The mock server:

- returns the examples in the description, with the rate limit headers
- rejects requests without credentials (`401`) and requests that don't match the description (`400`, with the violations in the `sl-violations` header, which fails the E2E tests)
- returns any documented response on request, through the `Prefer` header, e.g. `Prefer: code=429` for a reached rate limit

It keeps no state: marking an article as read does not change what the next request returns.

### Live tests

Live tests, under [`live/`](live) behind the `live` build tag, run the client against the real Inoreader API to catch differences from the API description that the mock server cannot show. They only read data, so the account stays as it is, and make about ten requests, a tenth of the default daily limit.

The [Live workflow](.github/workflows/live.yaml) runs them weekly and on demand from the Actions tab, never on pull requests. To set it up:

1. Register an app in the [Inoreader preferences](https://www.inoreader.com/preferences/other) (Developer panel). Read-only access is enough.
2. Get a refresh token from the [Google OAuth 2.0 Playground](https://developers.google.com/oauthplayground), set up as the [Inoreader documentation](https://www.inoreader.com/developers/oauth) describes, with the `read` scope.
3. In the repository settings, create the environment `inoreader-live` and add these secrets to it:

   | Secret | Value |
   | :-- | :-- |
   | `INOREADER_APP_ID` | App ID |
   | `INOREADER_APP_KEY` | App key |
   | `INOREADER_REFRESH_TOKEN` | Refresh token |

The tests exchange the refresh token for an access token on each run. Whether Inoreader rotates refresh tokens is not documented; if it does, the run logs that the token was rotated, and the secret needs a new one once the old one stops working.

To run them locally, put the same variables in `.env` at the repository root, which git ignores and mise loads, and run `mise run test:live`. `INOREADER_ACCESS_TOKEN` can stand in for the refresh token for a quick check.

### Mock server

To start the mock server on its own, for manual checks:

```sh
mise run mock                # http://127.0.0.1:4010
mise run mock --port 8080
```
