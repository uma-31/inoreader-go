package inoreader

import (
	"net/http"
	"strconv"
	"time"
)

// RateLimit is the daily request quota of the app, reported with each
// response.
//
// Methods that read data count against zone 1 and methods that change data
// against zone 2.
type RateLimit struct {
	Zone1 Quota
	Zone2 Quota

	// ResetAfter is how long after the response the usage resets.
	ResetAfter time.Duration

	// ResetAt is when the usage resets, estimated from when the response
	// was received.
	ResetAt time.Time
}

// Quota is the daily limit of a zone and how much of it is used.
type Quota struct {
	Limit int
	Usage int
}

// Remaining returns how many requests are left today.
func (q Quota) Remaining() int {
	return max(q.Limit-q.Usage, 0)
}

// parseRateLimit returns nil unless h carries every rate limit header.
func parseRateLimit(h http.Header, now time.Time) *RateLimit {
	var values [5]int
	for i, name := range [...]string{
		"X-Reader-Zone1-Limit",
		"X-Reader-Zone1-Usage",
		"X-Reader-Zone2-Limit",
		"X-Reader-Zone2-Usage",
		"X-Reader-Limits-Reset-After",
	} {
		v, err := strconv.Atoi(h.Get(name))
		if err != nil {
			return nil
		}
		values[i] = v
	}

	resetAfter := time.Duration(values[4]) * time.Second
	return &RateLimit{
		Zone1:      Quota{Limit: values[0], Usage: values[1]},
		Zone2:      Quota{Limit: values[2], Usage: values[3]},
		ResetAfter: resetAfter,
		ResetAt:    now.Add(resetAfter),
	}
}
