package inoreader

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// The API writes times as integers of various units, some as JSON numbers
// and others as strings. These types decode both into time.Time; zero
// becomes the zero time.

type unixSec time.Time

func (t *unixSec) UnmarshalJSON(data []byte) error {
	return unmarshalUnix(data, (*time.Time)(t), func(n int64) time.Time { return time.Unix(n, 0) })
}

type unixMicro time.Time

func (t *unixMicro) UnmarshalJSON(data []byte) error {
	return unmarshalUnix(data, (*time.Time)(t), time.UnixMicro)
}

func unmarshalUnix(data []byte, t *time.Time, conv func(int64) time.Time) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("inoreader: invalid timestamp %s", data)
		}
		s = n.String()
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("inoreader: invalid timestamp %q", s)
	}
	if n == 0 {
		*t = time.Time{}
		return nil
	}
	*t = conv(n)
	return nil
}
