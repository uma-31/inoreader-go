package inoreader

import (
	"encoding/json"
	"testing"
)

func TestItemIDForms(t *testing.T) {
	// Pairs from the developer portal, plus the sign boundary.
	tests := []struct {
		long  string
		short string
	}{
		{"tag:google.com,2005:reader/item/00000000148b9369", "344691561"},
		{"tag:google.com,2005:reader/item/00000000148b383e", "344668222"},
		{"tag:google.com,2005:reader/item/ffffffffffffffff", "-1"},
		{"tag:google.com,2005:reader/item/8000000000000000", "-9223372036854775808"},
	}
	for _, tt := range tests {
		fromLong, err := ParseItemID(tt.long)
		if err != nil {
			t.Fatalf("ParseItemID(%q): %v", tt.long, err)
		}
		fromShort, err := ParseItemID(tt.short)
		if err != nil {
			t.Fatalf("ParseItemID(%q): %v", tt.short, err)
		}
		if fromLong != fromShort {
			t.Errorf("long %q = %d, short %q = %d", tt.long, fromLong, tt.short, fromShort)
		}
		if got := fromLong.String(); got != tt.short {
			t.Errorf("String() = %q, want %q", got, tt.short)
		}
		if got := fromShort.Long(); got != tt.long {
			t.Errorf("Long() = %q, want %q", got, tt.long)
		}
	}
}

func TestParseItemIDUnsignedDecimal(t *testing.T) {
	id, err := ParseItemID("18446744073709551615")
	if err != nil || id != -1 {
		t.Errorf("ParseItemID() = %d, %v; want -1", id, err)
	}
}

func TestParseItemIDInvalid(t *testing.T) {
	for _, s := range []string{
		"",
		"abc",
		"tag:google.com,2005:reader/item/148b9369",
		"tag:google.com,2005:reader/item/zzzzzzzzzzzzzzzz",
	} {
		if _, err := ParseItemID(s); err == nil {
			t.Errorf("ParseItemID(%q) succeeded", s)
		}
	}
}

func TestItemIDUnmarshalJSON(t *testing.T) {
	var ids []ItemID
	data := `["tag:google.com,2005:reader/item/00000000148b9369", "344691561", 344691561]`
	if err := json.Unmarshal([]byte(data), &ids); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	for i, id := range ids {
		if id != 344691561 {
			t.Errorf("ids[%d] = %d", i, id)
		}
	}
}

func TestStreamID(t *testing.T) {
	if got := FeedStream("http://example.com/feed"); got != "feed/http://example.com/feed" {
		t.Errorf("FeedStream() = %q", got)
	}
	if got := LabelStream("Tech"); got != "user/-/label/Tech" {
		t.Errorf("LabelStream() = %q", got)
	}

	fromResponse := StreamID("user/1005921515/label/Tech")
	if got := fromResponse.Normalize(); got != LabelStream("Tech") {
		t.Errorf("Normalize() = %q", got)
	}
	if name, ok := fromResponse.Label(); !ok || name != "Tech" {
		t.Errorf("Label() = %q, %v", name, ok)
	}
	if _, ok := StreamStarred.Label(); ok {
		t.Error("Label() of a system stream reports ok")
	}
	if u, ok := FeedStream("http://example.com/feed").FeedURL(); !ok || u != "http://example.com/feed" {
		t.Errorf("FeedURL() = %q, %v", u, ok)
	}
	if got := StreamID("feed/http://example.com/user/1/x").Normalize(); got != "feed/http://example.com/user/1/x" {
		t.Errorf("Normalize() changed a feed: %q", got)
	}
}
