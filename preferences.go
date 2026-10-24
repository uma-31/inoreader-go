package inoreader

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Preference is a stream preference, such as `subscription-ordering` or
// `is-expanded`.
type Preference struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

const subscriptionOrderingKey = "subscription-ordering"

// StreamPreferences are the preferences of the root and of each folder.
type StreamPreferences map[StreamID][]Preference

// Get returns the value of the preference id of stream.
func (p StreamPreferences) Get(stream StreamID, id string) (string, bool) {
	stream = stream.Normalize()
	for s, prefs := range p {
		if s.Normalize() != stream {
			continue
		}
		for _, pref := range prefs {
			if pref.ID == id {
				return pref.Value, true
			}
		}
	}
	return "", false
}

// SubscriptionOrdering returns the sort IDs of the feeds and folders in
// stream, [StreamRoot] or a folder, in display order. Match them with
// [Subscription.SortID] and [Tag.SortID].
func (p StreamPreferences) SubscriptionOrdering(stream StreamID) []string {
	value, ok := p.Get(stream, subscriptionOrderingKey)
	if !ok {
		return nil
	}

	ids := make([]string, 0, len(value)/8)
	for len(value) >= 8 {
		ids = append(ids, value[:8])
		value = value[8:]
	}
	return ids
}

// StreamPreferences returns the stream preferences.
func (c *Client) StreamPreferences(ctx context.Context) (StreamPreferences, error) {
	var wire struct {
		StreamPrefs StreamPreferences `json:"streamprefs"`
	}
	if err := c.getJSON(ctx, apiPath+"/preference/stream/list", nil, &wire); err != nil {
		return nil, err
	}
	return wire.StreamPrefs, nil
}

// SetSubscriptionOrdering orders the feeds and folders in stream,
// [StreamRoot] or a folder, by their sort IDs.
func (c *Client) SetSubscriptionOrdering(ctx context.Context, stream StreamID, sortIDs []string) error {
	for _, id := range sortIDs {
		if !validSortID(id) {
			return fmt.Errorf("inoreader: invalid sort ID %q", id)
		}
	}

	form := url.Values{
		"s": {string(stream)},
		"k": {subscriptionOrderingKey},
		"v": {strings.Join(sortIDs, "")},
	}
	return c.postForm(ctx, apiPath+"/preference/stream/set", form, nil)
}
