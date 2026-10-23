package inoreader

import (
	"context"
	"time"
)

// User is the signed-in user.
type User struct {
	ID                  string
	Name                string
	ProfileID           string
	Email               string
	IsBloggerUser       bool
	SignedUpAt          time.Time
	IsMultiLoginEnabled bool
}

// UserInfo returns the signed-in user.
func (c *Client) UserInfo(ctx context.Context) (*User, error) {
	var wire struct {
		UserID              string  `json:"userId"`
		UserName            string  `json:"userName"`
		UserProfileID       string  `json:"userProfileId"`
		UserEmail           string  `json:"userEmail"`
		IsBloggerUser       bool    `json:"isBloggerUser"`
		SignupTimeSec       unixSec `json:"signupTimeSec"`
		IsMultiLoginEnabled bool    `json:"isMultiLoginEnabled"`
	}
	if err := c.getJSON(ctx, apiPath+"/user-info", nil, &wire); err != nil {
		return nil, err
	}

	return &User{
		ID:                  wire.UserID,
		Name:                wire.UserName,
		ProfileID:           wire.UserProfileID,
		Email:               wire.UserEmail,
		IsBloggerUser:       wire.IsBloggerUser,
		SignedUpAt:          time.Time(wire.SignupTimeSec),
		IsMultiLoginEnabled: wire.IsMultiLoginEnabled,
	}, nil
}
