package timeutil

import (
	"time"
)

// NowUTC returns the current UTC time.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// EnsureUTC strips location and returns UTC time.
func EnsureUTC(t time.Time) time.Time {
	return t.UTC()
}

// ConvertToTimeZone converts a UTC time to a given location.
// The input t is assumed to be UTC; if it's not, it is converted to UTC first.
func ConvertToTimeZone(t time.Time, loc *time.Location) time.Time {
	return t.UTC().In(loc)
}

// MonotonicElapsed measures the elapsed time since start using the monotonic clock.
// It returns the duration as time.Duration.
func MonotonicElapsed(start time.Time) time.Duration {
	return time.Since(start) // time.Since uses monotonic clock if available
}

// Clock is an interface for testable time operations.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// NewRealClock returns a clock that uses the system time.
func NewRealClock() Clock {
	return realClock{}
}

// FixedClock returns a clock that always returns the given time – for testing.
func FixedClock(t time.Time) Clock {
	return fixedClock{t}
}

type fixedClock struct {
	t time.Time
}

func (c fixedClock) Now() time.Time                         { return c.t }
func (c fixedClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
