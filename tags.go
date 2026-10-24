package inoreader

import (
	"context"
	"errors"
	"net/url"
)

// TagType is the type of a [Tag].
type TagType string

// Tag types.
const (
	TagTypeTag          TagType = "tag"
	TagTypeFolder       TagType = "folder"
	TagTypeActiveSearch TagType = "active_search"
)

// Tag is a folder, a tag, an active search or a system stream.
type Tag struct {
	ID StreamID `json:"id"`

	// SortID orders the tag; see [StreamPreferences.SubscriptionOrdering].
	SortID string `json:"sortid"`

	// Type is set with [ListTagsOptions.Types], and empty for system
	// streams.
	Type TagType `json:"type"`

	// UnreadCount and UnseenCount are set with [ListTagsOptions.Counts].
	// Unseen articles are those new since the user's last visit.
	UnreadCount int `json:"unread_count"`
	UnseenCount int `json:"unseen_count"`

	// OrganizationID is set when the tag is a team asset.
	OrganizationID string `json:"organization_id"`
}

// ListTagsOptions are the options of [Client.ListTags].
type ListTagsOptions struct {
	// Types sets [Tag.Type].
	Types bool
	// Counts sets the unread and unseen counts, and implies Types.
	Counts bool
	// TeamAssets includes the team folders and tags.
	TeamAssets bool
}

// ListTags returns the folders and tags, in no particular order.
func (c *Client) ListTags(ctx context.Context, opts *ListTagsOptions) ([]Tag, error) {
	query := url.Values{}
	if opts != nil {
		if opts.Types || opts.Counts {
			query.Set("types", "1")
		}
		if opts.Counts {
			query.Set("counts", "1")
		}
		if opts.TeamAssets {
			query.Set("team_assets", "1")
		}
	}

	var wire struct {
		Tags []Tag `json:"tags"`
	}
	if err := c.getJSON(ctx, apiPath+"/tag/list", query, &wire); err != nil {
		return nil, err
	}
	return wire.Tags, nil
}

// RenameTag renames a folder or tag.
func (c *Client) RenameTag(ctx context.Context, tag StreamID, newName string) error {
	if tag == "" || newName == "" {
		return errors.New("inoreader: tag and new name are required")
	}
	form := url.Values{"s": {string(tag)}, "dest": {newName}}
	return c.postForm(ctx, apiPath+"/rename-tag", form, nil)
}

// DeleteTag deletes a folder or tag.
func (c *Client) DeleteTag(ctx context.Context, tag StreamID) error {
	if tag == "" {
		return errors.New("inoreader: tag is required")
	}
	form := url.Values{"s": {string(tag)}}
	return c.postForm(ctx, apiPath+"/disable-tag", form, nil)
}

// EditTagRequest is the request of [Client.EditTag].
type EditTagRequest struct {
	// Add is the tag to add, such as [StreamRead] or a [LabelStream].
	Add StreamID
	// Remove is the tag to remove.
	Remove StreamID
	// Items are the articles to edit.
	Items []ItemID
}

// EditTag adds tags to articles and removes tags from them, which also marks
// them as read or starred. Marking an article older than its feed's
// [Subscription.FirstItemAt] as unread is silently ignored.
func (c *Client) EditTag(ctx context.Context, req EditTagRequest) error {
	if len(req.Items) == 0 {
		return errors.New("inoreader: no items to edit")
	}
	if req.Add == "" && req.Remove == "" {
		return errors.New("inoreader: no tag to add or remove")
	}

	form := url.Values{}
	setIfNotEmpty(form, "a", string(req.Add))
	setIfNotEmpty(form, "r", string(req.Remove))
	for _, id := range req.Items {
		form.Add("i", id.String())
	}
	return c.postForm(ctx, apiPath+"/edit-tag", form, nil)
}

// MarkAsRead marks articles as read.
func (c *Client) MarkAsRead(ctx context.Context, items ...ItemID) error {
	return c.EditTag(ctx, EditTagRequest{Add: StreamRead, Items: items})
}

// MarkAsUnread marks articles as unread.
func (c *Client) MarkAsUnread(ctx context.Context, items ...ItemID) error {
	return c.EditTag(ctx, EditTagRequest{Remove: StreamRead, Items: items})
}

// Star stars articles.
func (c *Client) Star(ctx context.Context, items ...ItemID) error {
	return c.EditTag(ctx, EditTagRequest{Add: StreamStarred, Items: items})
}

// Unstar removes the star from articles.
func (c *Client) Unstar(ctx context.Context, items ...ItemID) error {
	return c.EditTag(ctx, EditTagRequest{Remove: StreamStarred, Items: items})
}
