# inoreader-go

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

To start the mock server on its own, for manual checks:

```sh
mise run mock                # http://127.0.0.1:4010
mise run mock --port 8080
```
