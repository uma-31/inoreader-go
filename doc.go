// Package inoreader is a client for the [Inoreader API].
//
// Register an app in the Inoreader preferences to get its App ID and App
// key, then authenticate the user with OAuth 2.0 through
// [golang.org/x/oauth2] and [Endpoint]:
//
//	conf := &oauth2.Config{
//		ClientID:     appID,
//		ClientSecret: appKey,
//		Endpoint:     inoreader.Endpoint,
//		RedirectURL:  "https://example.com/callback",
//		Scopes:       []string{inoreader.ScopeRead, inoreader.ScopeWrite},
//	}
//	// Send the user to conf.AuthCodeURL(state), then exchange the code.
//	token, err := conf.Exchange(ctx, code)
//	...
//	client, err := inoreader.NewClient(appID, appKey,
//		inoreader.WithHTTPClient(conf.Client(ctx, token)))
//
// # Rate limits
//
// The API limits the requests of an app per day, separately for the methods
// that read data (zone 1) and those that change data (zone 2). The default
// limit can be as low as 100 requests per zone, so prefer [Client.ItemIDs]
// and [Client.UnreadCounts] to detect changes before fetching articles.
// [Client.RateLimit] reports the usage, and a reached limit fails with
// [ErrRateLimited].
//
// [Inoreader API]: https://www.inoreader.com/developers/
package inoreader
