# inoreader-go

[![CI](https://github.com/uma-31/inoreader-go/actions/workflows/ci.yaml/badge.svg)](https://github.com/uma-31/inoreader-go/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/uma-31/inoreader-go.svg)](https://pkg.go.dev/github.com/uma-31/inoreader-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/uma-31/inoreader-go/badge)](https://scorecard.dev/viewer/?uri=github.com/uma-31/inoreader-go)

A Go client library for the [Inoreader API](https://www.inoreader.com/developers/).

```sh
go get github.com/uma-31/inoreader-go
```

It needs Go 1.26 or later.

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

Everything is written in Go. Any [Go](https://go.dev/dl/) from 1.21 on will do, since the `toolchain` directive in `go.mod` makes it download and use the version this module is developed with; Go 1.21 is the first release that does so. The linters, [golangci-lint](https://golangci-lint.run/) and [vacuum](https://quobix.com/vacuum/), and the tasks are managed with [mise](https://mise.jdx.dev/): run `mise install` once, then `mise tasks` to list the tasks.

The `go` directive in `go.mod` is the oldest Go version dependents need, which the Go 1.26 above refers to. CI runs the unit tests on it and on the latest release, without the toolchain download, which together cover the releases Go supports.

| Task | What it does |
| :-- | :-- |
| `mise run test:unit` | Run the unit tests |
| `mise run test:e2e` | Run the E2E tests against the mock server |
| `mise run test:live` | Run the live tests against the real API, with the credentials in `.env` |
| `mise run live:authorize` | Get a refresh token for the live tests into `.env` |
| `mise run go:check` | Check formatting, lint and `go.mod` of both modules |
| `mise run go:fix` | Format and apply lint fixes |
| `mise run api:lint` | Lint the API description |
| `mise run mock` | Start the mock server on its own |

### API description

[`api/openapi.yaml`](api/openapi.yaml) describes the Inoreader API in OpenAPI 3.1. Inoreader publishes no machine-readable description, so it is written by hand from the developer portal. It is the contract the client is written against, and the source of the mock server. `mise run api:lint` checks it with the rules in [`api/ruleset.yaml`](api/ruleset.yaml).

### Tests

Unit tests run the client against `httptest` servers and check the requests it builds and how it decodes responses.

E2E tests, under [`e2e/`](e2e), run the client against a mock server that serves the API from its description. The mock server, in [`e2e/mock`](e2e/mock), needs no code per endpoint: it validates each request against the description with [libopenapi-validator](https://github.com/pb33f/libopenapi-validator), and answers with the examples of the documented response. Each test starts its own, so `mise run test:e2e`, or `go test ./...` in `e2e/`, is all it takes; arguments after `--` go to `go test`, as in `mise run test:e2e -- -run TestStreams -v`.

`e2e/` is a Go module of its own, so that the dependencies of the mock server stay out of the client's `go.mod`.

The mock server:

- returns the examples in the description, with their headers such as the rate limit
- answers requests without credentials with `401`
- fails the test on a request that doesn't match the description, and answers it with `400`
- fails the test on a response example that doesn't match its schema
- returns any documented response on request, through the `Prefer` header, e.g. `Prefer: code=429` for a reached rate limit

It keeps no state: marking an article as read does not change what the next request returns.

### Live tests

Live tests, under [`live/`](live) behind the `live` build tag, run the client against the real Inoreader API to catch differences from the API description that the mock server cannot show. They only read data, so the account stays as it is, and make about ten requests, a tenth of the default daily limit.

The [Live workflow](.github/workflows/live.yaml) runs them weekly and on demand from the Actions tab, never on pull requests. Its logs are public, so failure messages leave out the account's data, and the access token and the user's identity are masked. To set it up:

1. Register an app in the [Inoreader preferences](https://www.inoreader.com/preferences/other) (Developer API), with the redirect URI `http://localhost:8765/callback` and the OAuth scope Read only.
2. Put its App ID and App key in `.env` at the repository root, which git ignores and mise loads:

   ```sh
   INOREADER_APP_ID=...
   INOREADER_APP_KEY=...
   ```

3. Run `mise run live:authorize`, open the URL it prints and authorize the app. It adds `INOREADER_REFRESH_TOKEN` to `.env` without printing it. The whole flow stays between your machine and Inoreader.
4. Upload the three variables as repository secrets for Actions with the [GitHub CLI](https://cli.github.com/):

   ```sh
   gh secret set -f .env
   ```

The tests exchange the refresh token for an access token on each run. Whether Inoreader rotates refresh tokens is not documented; if it does, the run logs that the token was rotated, and the secret needs a new one once the old one stops working.

To run them locally, run `mise run test:live` with the same `.env`.

### Mock server

To start the mock server on its own, for manual checks, which logs the violations instead:

```sh
mise run mock                         # http://127.0.0.1:4010
mise run mock --addr 127.0.0.1:8080
```

### Releasing

Releases follow [semantic versioning](https://go.dev/doc/modules/version-numbers) and stay at v0 until the API settles. The API compatibility job in CI runs [gorelease](https://pkg.go.dev/golang.org/x/exp/cmd/gorelease) on every pull request: it compares the exported API with the latest release and suggests the next version.

To release, tag the commit on `main` with an annotated tag and push it, then write the release notes on GitHub:

```sh
git tag -a v0.1.0 -m v0.1.0
git push origin v0.1.0
```

A published version cannot be changed or deleted, as the Go module proxy keeps it forever. To withdraw a bad one, release a new version whose `go.mod` retracts it.

### Dependencies

[Renovate](https://docs.renovatebot.com/) keeps the Go modules, the GitHub Actions and the mise tools up to date, a week after each release. The `go` directive is left alone, as it is the oldest Go version dependents need.

### Security

See [SECURITY.md](SECURITY.md) to report a vulnerability.
