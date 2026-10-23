package inoreader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// ActiveSearchFilterType is where an active search looks.
type ActiveSearchFilterType string

// Active search filter types.
const (
	ActiveSearchInFolder       ActiveSearchFilterType = "folder"
	ActiveSearchInSubscription ActiveSearchFilterType = "subscription"
	ActiveSearchInPublic       ActiveSearchFilterType = "public"
)

// ActiveSearchRequest is the request of [Client.CreateActiveSearch]. Empty
// optional fields take the API defaults.
type ActiveSearchRequest struct {
	// Term is what to search for. It becomes the title.
	Term string `json:"term"`

	FilterType ActiveSearchFilterType `json:"filter_type"`
	// FilterID is the folder or feed to search in, unless FilterType is
	// [ActiveSearchInPublic].
	FilterID StreamID `json:"filter_id,omitempty"`

	// ArticleIDs are articles to include.
	ArticleIDs []ItemID `json:"-"`

	// SearchFilters is `all`, `titles` or `contents`.
	SearchFilters string `json:"search_filters,omitempty"`
	// SearchMatch is `all_words`, `any_word`, `full_phrase` or `advanced`.
	SearchMatch string `json:"search_match,omitempty"`
	// SearchFeedPopularity is `all_sites`, `popular_sites` or
	// `most_popular_sites`, for public searches.
	SearchFeedPopularity string `json:"search_feed_popularity,omitempty"`
}

// ActiveSearch is a created active search.
type ActiveSearch struct {
	ID    StreamID `json:"id"`
	Title string   `json:"title"`
}

// ActiveSearchError reports that the API refused to create an active
// search.
type ActiveSearchError struct {
	Message string

	// AccountLimit is set when the account has the most active searches it
	// can have.
	AccountLimit int
	// NeedUpgrade reports that the user's plan does not allow it.
	NeedUpgrade bool
	// Localized reports that Message is in the user's language.
	Localized bool
}

func (e *ActiveSearchError) Error() string {
	return "inoreader: active search not created: " + e.Message
}

// CreateActiveSearch creates an active search. A refusal from the API is
// returned as an [*ActiveSearchError].
func (c *Client) CreateActiveSearch(ctx context.Context, req ActiveSearchRequest) (*ActiveSearch, error) {
	if req.Term == "" || req.FilterType == "" {
		return nil, errors.New("inoreader: term and filter type are required")
	}
	if req.FilterType != ActiveSearchInPublic && req.FilterID == "" {
		return nil, errors.New("inoreader: filter ID is required unless the search is public")
	}

	type payload ActiveSearchRequest
	body, err := json.Marshal(struct {
		payload
		ArticleIDs []int64 `json:"article_ids,omitempty"`
	}{payload(req), itemIDsToInts(req.ArticleIDs)})
	if err != nil {
		return nil, fmt.Errorf("inoreader: encoding request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, apiPath+"/active_search/create", nil, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	status, respBody, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}

	if status == http.StatusAccepted {
		var wire struct {
			Error        string `json:"error"`
			AccountLimit int    `json:"accountLimit"`
			UserOutput   string `json:"userOutput"`
			NeedUpgrade  string `json:"needUpgrade"`
		}
		if err := decodeJSON(respBody, &wire); err != nil {
			return nil, err
		}
		return nil, &ActiveSearchError{
			Message:      wire.Error,
			AccountLimit: wire.AccountLimit,
			NeedUpgrade:  wire.NeedUpgrade == "1",
			Localized:    wire.UserOutput == "1",
		}
	}

	var search ActiveSearch
	if err := decodeJSON(respBody, &search); err != nil {
		return nil, err
	}
	return &search, nil
}

// DeleteActiveSearch deletes an active search.
func (c *Client) DeleteActiveSearch(ctx context.Context, id StreamID) error {
	if id == "" {
		return errors.New("inoreader: active search ID is required")
	}

	req, err := c.newRequest(ctx, http.MethodGet, apiPath+"/active_search/delete", url.Values{"id": {string(id)}}, nil)
	if err != nil {
		return err
	}
	status, body, err := c.do(req)
	if err != nil {
		return err
	}
	return expectOK(status, body)
}

func itemIDsToInts(ids []ItemID) []int64 {
	if len(ids) == 0 {
		return nil
	}
	ints := make([]int64, len(ids))
	for i, id := range ids {
		ints[i] = int64(id)
	}
	return ints
}
