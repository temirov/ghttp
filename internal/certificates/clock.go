package certificates

import "time"

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

// SystemClock returns the wall clock time.
type SystemClock struct{}

// NewSystemClock constructs a SystemClock.
func NewSystemClock() SystemClock {
	return SystemClock{}
}

// Now reports the current wall clock time.
func (systemClock SystemClock) Now() time.Time {
	return time.Now()
}
