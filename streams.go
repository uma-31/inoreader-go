package inoreader

import (
	"context"
	"errors"
	"iter"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// UnreadCounts are the unread counts of the feeds, folders and tags.
type UnreadCounts struct {
	// Max is the highest count the API reports for this user. A count equal
	// to it means at least that many, conventionally shown as `1000+`.
	Max int

	Counts []UnreadCount
}

// UnreadCount is the unread count of a stream.
type UnreadCount struct {
	ID    StreamID
	Count int

	// NewestItemAt is the time of the newest article in the stream.
	NewestItemAt time.Time
}

// UnreadCounts returns the unread counts. [StreamReadingList] counts every
// unread article.
func (c *Client) UnreadCounts(ctx context.Context) (*UnreadCounts, error) {
	var wire struct {
		Max          int `json:"max"`
		UnreadCounts []struct {
			ID                      StreamID  `json:"id"`
			Count                   flexInt   `json:"count"` // A string for some streams.
			NewestItemTimestampUsec unixMicro `json:"newestItemTimestampUsec"`
		} `json:"unreadcounts"`
	}
	if err := c.getJSON(ctx, apiPath+"/unread-count", nil, &wire); err != nil {
		return nil, err
	}

	counts := &UnreadCounts{Max: wire.Max, Counts: make([]UnreadCount, len(wire.UnreadCounts))}
	for i, uc := range wire.UnreadCounts {
		counts.Counts[i] = UnreadCount{
			ID:           uc.ID,
			Count:        int(uc.Count),
			NewestItemAt: time.Time(uc.NewestItemTimestampUsec),
		}
	}
	return counts, nil
}

// StreamOptions select and page the articles of a stream.
type StreamOptions struct {
	// Count is the number of articles per page; zero for the API default.
	Count int

	// OldestFirst returns the oldest articles first.
	OldestFirst bool

	// Since returns only the articles newer than this time. With
	// OldestFirst, it is clamped to about one month ago.
	Since time.Time

	// Exclude leaves out the articles in this stream, such as [StreamRead]
	// for unread articles only.
	Exclude StreamID

	// Include returns only the articles in this stream, either
	// [StreamStarred] or [StreamLike].
	Include StreamID

	// Continuation fetches the page after the one that returned it.
	Continuation string

	// ExcludeFolderTags leaves out the tags inherited from folders.
	ExcludeFolderTags bool
}

func (o *StreamOptions) query() url.Values {
	query := url.Values{}
	if o == nil {
		return query
	}

	if o.Count > 0 {
		query.Set("n", strconv.Itoa(o.Count))
	}
	if o.OldestFirst {
		query.Set("r", "o")
	}
	if !o.Since.IsZero() {
		query.Set("ot", strconv.FormatInt(o.Since.UnixMicro(), 10))
	}
	setIfNotEmpty(query, "xt", string(o.Exclude))
	setIfNotEmpty(query, "it", string(o.Include))
	setIfNotEmpty(query, "c", o.Continuation)
	if o.ExcludeFolderTags {
		query.Set("includeAllDirectStreamIds", "false")
	}
	return query
}

// StreamContentsOptions are the options of [Client.StreamContents].
type StreamContentsOptions struct {
	StreamOptions

	// Annotations includes the user's annotations.
	Annotations bool
	// Summaries includes the Inoreader Intelligence summaries.
	Summaries bool
}

func (o *StreamContentsOptions) query() url.Values {
	if o == nil {
		return url.Values{}
	}

	query := o.StreamOptions.query()
	if o.Annotations {
		query.Set("annotations", "1")
	}
	if o.Summaries {
		query.Set("summaries", "1")
	}
	return query
}

// StreamContents is a page of articles.
type StreamContents struct {
	ID          StreamID
	Title       string
	Description string
	UpdatedAt   time.Time
	Items       []Item

	// Continuation fetches the next page; empty at the end of the stream.
	Continuation string
}

// Item is an article.
type Item struct {
	ID ItemID

	// Timestamp is when Inoreader received the article. Use it, rather than
	// Published, to compare and page articles.
	Timestamp time.Time

	// Published is the publication time set by the publisher, for display
	// only.
	Published time.Time
	// Updated is when the article changed after it was received, if it did.
	Updated time.Time

	// Categories are the states and tags of the article, such as
	// [StreamRead], with the user's numeric ID.
	Categories []StreamID

	Title  string
	Author string

	// Canonical and Alternate are the article's URLs.
	Canonical []Link
	Alternate []Link

	// Content is the article's content.
	Content Content

	// Origin is the feed of the article.
	Origin Origin

	Annotations []Annotation
	Summaries   []Summary
}

// HasCategory reports whether the article is in stream, such as
// [StreamRead].
func (it *Item) HasCategory(stream StreamID) bool {
	stream = stream.Normalize()
	return slices.ContainsFunc(it.Categories, func(c StreamID) bool {
		return c.Normalize() == stream
	})
}

// URL returns the article's URL, or empty if it has none.
func (it *Item) URL() string {
	for _, links := range [][]Link{it.Canonical, it.Alternate} {
		for _, l := range links {
			if l.Href != "" {
				return l.Href
			}
		}
	}
	return ""
}

// Link is a URL of an article.
type Link struct {
	Href string `json:"href"`
	Type string `json:"type"`
}

// Content is the HTML content of an article.
type Content struct {
	// Direction is `ltr` or `rtl`.
	Direction string `json:"direction"`
	HTML      string `json:"content"`
}

// Origin is the feed an article comes from.
type Origin struct {
	StreamID StreamID `json:"streamId"`
	Title    string   `json:"title"`
	HTMLURL  string   `json:"htmlUrl"`
}

// Annotation is a highlight the user made in an article.
type Annotation struct {
	ID int64 `json:"id"`

	// Start and End are the offsets of Text in the article.
	Start int `json:"start"`
	End   int `json:"end"`

	Text string `json:"text"`
	Note string `json:"note"`

	UserID   int64  `json:"user_id"`
	UserName string `json:"user_name"`
}

// Summary is an Inoreader Intelligence summary of an article.
type Summary struct {
	ID         int64  `json:"id"`
	PromptID   int64  `json:"prompt_id"`
	PromptName string `json:"prompt_name"`
	// HTML is the summary.
	HTML string `json:"summary"`
}

type wireItem struct {
	ID            ItemID       `json:"id"`
	TimestampUsec unixMicro    `json:"timestampUsec"`
	Published     unixSec      `json:"published"`
	Updated       unixSec      `json:"updated"`
	Categories    []StreamID   `json:"categories"`
	Title         string       `json:"title"`
	Author        string       `json:"author"`
	Canonical     []Link       `json:"canonical"`
	Alternate     []Link       `json:"alternate"`
	Summary       Content      `json:"summary"`
	Origin        Origin       `json:"origin"`
	Annotations   []Annotation `json:"annotations"`
	Summaries     []Summary    `json:"summaries"`
}

// StreamContents returns a page of the articles in stream.
func (c *Client) StreamContents(ctx context.Context, stream StreamID, opts *StreamContentsOptions) (*StreamContents, error) {
	if stream == "" {
		return nil, errors.New("inoreader: stream is required")
	}

	var wire struct {
		ID           StreamID   `json:"id"`
		Title        string     `json:"title"`
		Description  string     `json:"description"`
		UpdatedUsec  unixMicro  `json:"updatedUsec"`
		Items        []wireItem `json:"items"`
		Continuation string     `json:"continuation"`
	}
	path := apiPath + "/stream/contents/" + url.PathEscape(string(stream))
	if err := c.getJSON(ctx, path, opts.query(), &wire); err != nil {
		return nil, err
	}

	contents := &StreamContents{
		ID:           wire.ID,
		Title:        wire.Title,
		Description:  wire.Description,
		UpdatedAt:    time.Time(wire.UpdatedUsec),
		Items:        make([]Item, len(wire.Items)),
		Continuation: wire.Continuation,
	}
	for i, it := range wire.Items {
		contents.Items[i] = Item{
			ID:          it.ID,
			Timestamp:   time.Time(it.TimestampUsec),
			Published:   time.Time(it.Published),
			Updated:     time.Time(it.Updated),
			Categories:  it.Categories,
			Title:       it.Title,
			Author:      it.Author,
			Canonical:   it.Canonical,
			Alternate:   it.Alternate,
			Content:     it.Summary,
			Origin:      it.Origin,
			Annotations: it.Annotations,
			Summaries:   it.Summaries,
		}
	}
	return contents, nil
}

// ItemRefs is a page of article references.
type ItemRefs struct {
	Refs []ItemRef

	// Continuation fetches the next page; empty at the end of the stream.
	Continuation string
}

// ItemRef refers to an article without its content.
type ItemRef struct {
	ID ItemID

	// DirectStreamIDs are the tags of the article.
	DirectStreamIDs []StreamID

	Timestamp time.Time
}

// ItemIDs returns a page of references to the articles in stream. It is
// much lighter than [Client.StreamContents]; use it to sync states such as
// read.
func (c *Client) ItemIDs(ctx context.Context, stream StreamID, opts *StreamOptions) (*ItemRefs, error) {
	if stream == "" {
		return nil, errors.New("inoreader: stream is required")
	}

	var wire struct {
		ItemRefs []struct {
			ID              ItemID     `json:"id"`
			DirectStreamIDs []StreamID `json:"directStreamIds"`
			TimestampUsec   unixMicro  `json:"timestampUsec"`
		} `json:"itemRefs"`
		Continuation string `json:"continuation"`
	}
	query := opts.query()
	query.Set("s", string(stream))
	if err := c.getJSON(ctx, apiPath+"/stream/items/ids", query, &wire); err != nil {
		return nil, err
	}

	refs := &ItemRefs{Refs: make([]ItemRef, len(wire.ItemRefs)), Continuation: wire.Continuation}
	for i, r := range wire.ItemRefs {
		refs.Refs[i] = ItemRef{
			ID:              r.ID,
			DirectStreamIDs: r.DirectStreamIDs,
			Timestamp:       time.Time(r.TimestampUsec),
		}
	}
	return refs, nil
}

// errStuckContinuation guards against fetching the same page forever, which
// would burn through the rate limit.
var errStuckContinuation = errors.New("inoreader: continuation did not advance")

// AllItems iterates over the articles in stream from the page opts select,
// fetching the following pages as needed. It stops at the first error.
//
// Each page counts against the rate limit, so stop iterating once you have
// what you need.
func (c *Client) AllItems(ctx context.Context, stream StreamID, opts *StreamContentsOptions) iter.Seq2[Item, error] {
	return func(yield func(Item, error) bool) {
		page := StreamContentsOptions{}
		if opts != nil {
			page = *opts
		}

		for {
			contents, err := c.StreamContents(ctx, stream, &page)
			if err != nil {
				yield(Item{}, err)
				return
			}
			for _, it := range contents.Items {
				if !yield(it, nil) {
					return
				}
			}

			if contents.Continuation == "" {
				return
			}
			if contents.Continuation == page.Continuation {
				yield(Item{}, errStuckContinuation)
				return
			}
			page.Continuation = contents.Continuation
		}
	}
}

// AllItemRefs iterates over the article references in stream like
// [Client.AllItems].
func (c *Client) AllItemRefs(ctx context.Context, stream StreamID, opts *StreamOptions) iter.Seq2[ItemRef, error] {
	return func(yield func(ItemRef, error) bool) {
		page := StreamOptions{}
		if opts != nil {
			page = *opts
		}

		for {
			refs, err := c.ItemIDs(ctx, stream, &page)
			if err != nil {
				yield(ItemRef{}, err)
				return
			}
			for _, r := range refs.Refs {
				if !yield(r, nil) {
					return
				}
			}

			if refs.Continuation == "" {
				return
			}
			if refs.Continuation == page.Continuation {
				yield(ItemRef{}, errStuckContinuation)
				return
			}
			page.Continuation = refs.Continuation
		}
	}
}

// MarkAllAsRead marks the articles in stream as read, up to seenAt: when the
// stream was last fetched and shown, so that newer articles the user never
// saw stay unread.
func (c *Client) MarkAllAsRead(ctx context.Context, stream StreamID, seenAt time.Time) error {
	if stream == "" {
		return errors.New("inoreader: stream is required")
	}
	if seenAt.IsZero() {
		return errors.New("inoreader: seenAt is required")
	}

	form := url.Values{
		"s":  {string(stream)},
		"ts": {strconv.FormatInt(seenAt.UnixMicro(), 10)},
	}
	return c.postForm(ctx, apiPath+"/mark-all-as-read", form, nil)
}
