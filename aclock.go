package timewarper

import (
	"time"
)

// import github.com/jbrzusto/timewarper

// aclock.go: support for time from alternative clocks

// ATimer provides a timer backed by an alternative clock
type ATimer interface {
	Stop()
	Reset(time.Duration)
	Chan() <-chan time.Time
}

// ATimer provides a ticker backed by an alternative clock
type ATicker interface {
	Stop()
	Reset(time.Duration)
	Chan() <-chan time.Time
}

// AClock provides functions for using alternative clocks.
type AClock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
	Sleep(time.Duration)
	NewATimer(time.Duration) ATimer
	NewATicker(time.Duration) ATicker
	SetDilation(int)
	JumpToTheFuture(time.Duration) bool
}

// StandardTimer provides an ATimer based on the system clock.
type StandardTimer struct {
	*time.Timer
}

// Stop stops the timer
func (st StandardTimer) Stop() {
	st.Timer.Stop()
}

// Reset resets the timer
func (st StandardTimer) Reset(d time.Duration) {
	st.Timer.Reset(d)
}

// Chan returns the channel for the timer
func (st StandardTimer) Chan() <-chan time.Time {
	return st.Timer.C
}

// StandardTicker provides an ATicker based on the system clock.
type StandardTicker struct {
	*time.Ticker
}

// Stop stops the ticker
func (st StandardTicker) Stop() {
	st.Ticker.Stop()
}

// Reset resets the ticker
func (st StandardTicker) Reset(d time.Duration) {
	st.Ticker.Reset(d)
}

// Chan returns the channel for the ticker
func (st StandardTicker) Chan() <-chan time.Time {
	return st.Ticker.C
}

// StandardClock provides an AClock based on the system clock.
type StandardClock struct {
}

// Now returns the current time from the system clock
func (sc *StandardClock) Now() time.Time {
	return time.Now()
}

// After returns the channel for a new timer based on the system clock
func (sc *StandardClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// Sleep sleeps the thread for the given duration against the system clock
func (sc *StandardClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// NewATimer returns a StandardTimer, which is based on the system clock
func (sc *StandardClock) NewATimer(d time.Duration) ATimer {
	return StandardTimer{time.NewTimer(d)}
}

// NewATicker returns a StandardTicker, which is based on the system clock
func (sc *StandardClock) NewATicker(d time.Duration) ATicker {
	return StandardTicker{time.NewTicker(d)}
}

// JumpToTheFuture returns false, because we can't do it with the standard
// clock (or rather, we choose not to).
func (sc *StandardClock) JumpToTheFuture(d time.Duration) bool {
	return false
}

// SetDilation does nothing, because we can't do it with the standard
// clock.
func (sc *StandardClock) SetDilation(d int) {
}

func GetStandardClock() AClock {
	return &StandardClock{}
}
