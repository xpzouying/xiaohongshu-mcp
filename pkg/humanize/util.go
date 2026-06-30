package humanize

import (
	"math/rand"
	"time"
)

// randDuration returns a random duration in [min, max].
func randDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	return min + time.Duration(rand.Int63n(int64(max-min)))
}

// Sleep pauses for a random short duration, useful between operations.
func Sleep(min, max time.Duration) {
	time.Sleep(randDuration(min, max))
}
