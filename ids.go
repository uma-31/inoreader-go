package inoreader

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// StreamID identifies a stream of articles: a feed, a folder or tag, or a
// system stream such as the starred articles.
//
// IDs in responses carry the user's numeric ID, as in
// `user/1005921515/label/Tech`. Requests may use `-` in its place, which the
// constants and constructors of this package do.
type StreamID string

// System streams.
const (
	// StreamReadingList holds every article of the followed feeds.
	StreamReadingList StreamID = "user/-/state/com.google/reading-list"
	// StreamRead holds the read articles.
	StreamRead StreamID = "user/-/state/com.google/read"
	// StreamStarred holds the starred articles.
	StreamStarred StreamID = "user/-/state/com.google/starred"
	// StreamBroadcast holds the broadcast articles.
	StreamBroadcast StreamID = "user/-/state/com.google/broadcast"
	// StreamLike holds the liked articles.
	StreamLike StreamID = "user/-/state/com.google/like"
	// StreamAnnotated holds the annotated articles.
	StreamAnnotated StreamID = "user/-/state/com.google/annotated"
	// StreamSavedWebPages holds the saved web pages.
	StreamSavedWebPages StreamID = "user/-/state/com.google/saved-web-pages"
	// StreamRoot is the root of the feed and folder tree, for ordering.
	StreamRoot StreamID = "user/-/state/com.google/root"
)

const (
	feedPrefix  = "feed/"
	labelPrefix = "user/-/label/"
)

// FeedStream returns the stream of the feed at feedURL.
func FeedStream(feedURL string) StreamID {
	return StreamID(feedPrefix + feedURL)
}

// LabelStream returns the stream of the folder or tag named name.
func LabelStream(name string) StreamID {
	return StreamID(labelPrefix + name)
}

var userIDPattern = regexp.MustCompile(`^user/\d+/`)

// Normalize replaces the user's numeric ID with `-`, so that IDs from
// responses compare equal to those built by this package.
func (s StreamID) Normalize() StreamID {
	return StreamID(userIDPattern.ReplaceAllLiteralString(string(s), "user/-/"))
}

// FeedURL returns the URL of the feed, if s is a feed.
func (s StreamID) FeedURL() (string, bool) {
	return strings.CutPrefix(string(s), feedPrefix)
}

// Label returns the name of the folder or tag, if s is one.
func (s StreamID) Label() (string, bool) {
	return strings.CutPrefix(string(s.Normalize()), labelPrefix)
}

const longItemIDPrefix = "tag:google.com,2005:reader/item/"

// ItemID identifies an article.
//
// The API writes it in two forms: the long form, a prefix followed by the ID
// as 16 hexadecimal digits, and the short form, the same 64 bits as a signed
// decimal number.
type ItemID int64

// ParseItemID parses either form of an article ID.
func ParseItemID(s string) (ItemID, error) {
	if hex, ok := strings.CutPrefix(s, longItemIDPrefix); ok {
		if len(hex) != 16 {
			return 0, fmt.Errorf("inoreader: invalid item ID %q", s)
		}
		u, err := strconv.ParseUint(hex, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("inoreader: invalid item ID %q", s)
		}
		return ItemID(u), nil
	}

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ItemID(i), nil
	}
	// Tolerate the unsigned decimal form of IDs beyond the int64 range.
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("inoreader: invalid item ID %q", s)
	}
	return ItemID(u), nil
}

// String returns the short form.
func (id ItemID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// Long returns the long form.
func (id ItemID) Long() string {
	return fmt.Sprintf("%s%016x", longItemIDPrefix, uint64(id))
}

// UnmarshalJSON accepts either form, as a string or a number.
func (id *ItemID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("inoreader: invalid item ID %s", data)
		}
		s = n.String()
	}

	parsed, err := ParseItemID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

var sortIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{8}$`)

func validSortID(s string) bool {
	return sortIDPattern.MatchString(s)
}
