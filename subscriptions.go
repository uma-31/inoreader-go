package inoreader

import (
	"context"
	"errors"
	"net/url"
	"time"
)

// Subscription is a followed feed.
type Subscription struct {
	ID          StreamID
	Title       string
	Description string

	// Categories are the folders the feed is in.
	Categories []Category

	// SortID orders the feed; see [StreamPreferences.SubscriptionOrdering].
	SortID string

	// FirstItemAt is the time of the oldest article that can be marked as
	// unread.
	FirstItemAt time.Time

	// URL is the address of the feed itself.
	URL string
	// HTMLURL is the address of the website.
	HTMLURL string
	// IconURL is the address of a 16x16 favicon.
	IconURL string

	// FeedType is `rss`, `webfeed`, `facebook`, `bluesky` or `team_channel`,
	// among others.
	FeedType string
}

// Category is a folder a feed is in.
type Category struct {
	ID    StreamID `json:"id"`
	Label string   `json:"label"`
}

// ListSubscriptionsOptions are the options of [Client.ListSubscriptions].
type ListSubscriptionsOptions struct {
	// TeamAssets includes the team channels.
	TeamAssets bool
}

// ListSubscriptions returns the followed feeds, in no particular order.
func (c *Client) ListSubscriptions(ctx context.Context, opts *ListSubscriptionsOptions) ([]Subscription, error) {
	query := url.Values{}
	if opts != nil && opts.TeamAssets {
		query.Set("team_assets", "1")
	}

	var wire struct {
		Subscriptions []struct {
			ID            StreamID   `json:"id"`
			Title         string     `json:"title"`
			Description   string     `json:"description"`
			Categories    []Category `json:"categories"`
			SortID        string     `json:"sortid"`
			FirstItemMsec unixMicro  `json:"firstitemmsec"` // Microseconds despite the name.
			URL           string     `json:"url"`
			HTMLURL       string     `json:"htmlUrl"`
			IconURL       string     `json:"iconUrl"`
			FeedType      string     `json:"feedType"`
		} `json:"subscriptions"`
	}
	if err := c.getJSON(ctx, apiPath+"/subscription/list", query, &wire); err != nil {
		return nil, err
	}

	subs := make([]Subscription, len(wire.Subscriptions))
	for i, s := range wire.Subscriptions {
		subs[i] = Subscription{
			ID:          s.ID,
			Title:       s.Title,
			Description: s.Description,
			Categories:  s.Categories,
			SortID:      s.SortID,
			FirstItemAt: time.Time(s.FirstItemMsec),
			URL:         s.URL,
			HTMLURL:     s.HTMLURL,
			IconURL:     s.IconURL,
			FeedType:    s.FeedType,
		}
	}
	return subs, nil
}

// ErrNotSubscribed reports that [Client.Subscribe] could not follow the
// feed.
var ErrNotSubscribed = errors.New("inoreader: feed not subscribed")

// Subscribed is a feed followed by [Client.Subscribe].
type Subscribed struct {
	StreamID StreamID
	Title    string
}

// Subscribe follows a feed. Following an already followed feed succeeds.
func (c *Client) Subscribe(ctx context.Context, feed StreamID) (*Subscribed, error) {
	var wire struct {
		NumResults int      `json:"numResults"`
		StreamID   StreamID `json:"streamId"`
		StreamName string   `json:"streamName"`
	}
	form := url.Values{"quickadd": {string(feed)}}
	if err := c.postForm(ctx, apiPath+"/subscription/quickadd", form, &wire); err != nil {
		return nil, err
	}
	if wire.NumResults == 0 {
		return nil, ErrNotSubscribed
	}
	return &Subscribed{StreamID: wire.StreamID, Title: wire.StreamName}, nil
}

// SubscriptionAction is the action of [Client.EditSubscription].
type SubscriptionAction string

// Subscription actions.
const (
	SubscriptionEdit        SubscriptionAction = "edit"
	SubscriptionSubscribe   SubscriptionAction = "subscribe"
	SubscriptionUnsubscribe SubscriptionAction = "unsubscribe"
)

// EditSubscriptionRequest is the request of [Client.EditSubscription].
type EditSubscriptionRequest struct {
	Action SubscriptionAction
	Feed   StreamID

	// Title renames the feed; empty keeps the title.
	Title string
	// AddTo adds the feed to a folder.
	AddTo StreamID
	// RemoveFrom removes the feed from a folder.
	RemoveFrom StreamID
}

// EditSubscription renames a feed, moves it between folders, follows or
// unfollows it.
func (c *Client) EditSubscription(ctx context.Context, req EditSubscriptionRequest) error {
	if req.Action == "" || req.Feed == "" {
		return errors.New("inoreader: action and feed are required")
	}

	form := url.Values{"ac": {string(req.Action)}, "s": {string(req.Feed)}}
	setIfNotEmpty(form, "t", req.Title)
	setIfNotEmpty(form, "a", string(req.AddTo))
	setIfNotEmpty(form, "r", string(req.RemoveFrom))
	return c.postForm(ctx, apiPath+"/subscription/edit", form, nil)
}

// Unsubscribe unfollows a feed.
func (c *Client) Unsubscribe(ctx context.Context, feed StreamID) error {
	return c.EditSubscription(ctx, EditSubscriptionRequest{Action: SubscriptionUnsubscribe, Feed: feed})
}

func setIfNotEmpty(v url.Values, key, value string) {
	if value != "" {
		v.Set(key, value)
	}
}
