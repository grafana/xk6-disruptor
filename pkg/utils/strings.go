package utils

import (
	"fmt"
	"strings"
	"time"
)

// DurationSeconds returns the duration is seconds with a precession of 2 decimals,
// removing trailing zeros (e.g. "1.5s")
func DurationSeconds(d time.Duration) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", d.Seconds()), "0"), ".") + "s"
}
