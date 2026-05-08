package pgd

import (
	"fmt"
	"strconv"
)

// parseInt32 parses a decimal string into int32. Returns an error if the
// value would overflow int32.
func parseInt32(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse int32 %q: %w", s, err)
	}
	return int32(n), nil
}
