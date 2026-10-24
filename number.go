package inoreader

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// flexInt decodes an integer that the API writes as a JSON number in some
// responses and as a decimal string in others, even within one response.
type flexInt int

func (n *flexInt) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		var i int
		if err := json.Unmarshal(data, &i); err != nil {
			return fmt.Errorf("inoreader: invalid integer %s", data)
		}
		*n = flexInt(i)
		return nil
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("inoreader: invalid integer %q", s)
	}
	*n = flexInt(i)
	return nil
}
