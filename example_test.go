package inoreader_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/oauth2"

	"github.com/uma-31/inoreader-go"
)

func Example() {
	ctx := context.Background()

	conf := &oauth2.Config{
		ClientID:     "your-app-id",
		ClientSecret: "your-app-key",
		Endpoint:     inoreader.Endpoint,
		RedirectURL:  "https://example.com/callback",
		Scopes:       []string{inoreader.ScopeRead, inoreader.ScopeWrite},
	}
	// Send the user to conf.AuthCodeURL(state) and receive the code at the
	// redirect URL, then exchange it.
	token, err := conf.Exchange(ctx, "authorization-code")
	if err != nil {
		log.Fatal(err)
	}

	client, err := inoreader.NewClient(conf.ClientID, conf.ClientSecret,
		inoreader.WithHTTPClient(conf.Client(ctx, token)))
	if err != nil {
		log.Fatal(err)
	}

	// Print the unread articles of the last day, and mark them as read.
	seenAt := time.Now()
	var read []inoreader.ItemID
	for item, err := range client.AllItems(ctx, inoreader.StreamReadingList, &inoreader.StreamContentsOptions{
		StreamOptions: inoreader.StreamOptions{
			Exclude: inoreader.StreamRead,
			Since:   seenAt.Add(-24 * time.Hour),
		},
	}) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(item.Title, item.URL())
		read = append(read, item.ID)
	}
	if len(read) > 0 {
		if err := client.MarkAsRead(ctx, read...); err != nil {
			log.Fatal(err)
		}
	}
}
