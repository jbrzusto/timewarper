package timewarper

import (
	"log"
	"time"
)

// aclock.go: support for time from alternative clocks

// ATimer provides a timer running on an alternative clock
type ATimer interface {
	Stop()
	Reset(time.Duration)
	Chan() <-chan time.Time
}

// ATicker provides a ticker running on an alternative clock
type ATicker interface {
	Stop()
	Reset(time.Duration)
	Chan() <-chan time.Time
}

// AClock provides functions for using alternative clocks.
type AClock interface {
	// Now returns the AClock's current time
	Now() time.Time
	// After returns a channel to which the AClock's current time will be
	// written after waiting for the given Duration.
	After(time.Duration) <-chan time.Time
	// Sleep waits for the given Duration.
	Sleep(time.Duration)
	// NewATimer returns an ATimer of the given duration (measured by the AClock)
	NewATimer(time.Duration) ATimer
	// NewStoppedATimer returns a stopped ATimer; this ATimer will do nothing until
	// its .Reset() method is called
	NewStoppedATimer() ATimer
	// NewATicker returns an ATicker of the given repeat period (measured by the AClock)
	NewATicker(time.Duration) ATicker
	// ChangeDilationFactor changes the dilation of the AClock, where permitted
	ChangeDilationFactor(float64)
	// JumpToTheFuture advances the AClock by a Duration, triggering any timers
	// whose trigger time is passed along the way.
	JumpToTheFuture(time.Duration) int
	// JumpToFutureTime is like JumpToTheFuture but accepts a target time.Time,
	// rather than a duration.  This can be useful if multiple threads are trying
	// to jump the AClock into the future simultaneously.  It also returns the
	// number of timers triggered by the jump.
	JumpToFutureTime(time.Time) int
	// DeleteTimer deletes an ATimer, making it available for GC.  If a thread is reading
	// from the ATimer channel, it will receive an unspecified value.
	DeleteTimer(ATimer)
	// DeleteTicker deletes an ATicker, making it available for GC.  If a thread is reading
	// from the ATicker channel, it will receive an unspecified value.
	DeleteTicker(ATicker)
	// RealTime returns the real time.Time corresponding to the given dilated time
	RealTime(time.Time) time.Time
	// RealDuration returns the real time.Duration corresponding to the given dilated duration
	RealDuration(time.Duration) time.Duration
}

// StandardTimer provides an ATimer based on the system clock.
// i.e. it wraps the standard time functions into an AClock
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

// NewStoppedATimer returns an already-stopped timer based on the system clock
func (sc *StandardClock) NewStoppedATimer() ATimer {
	// create the timer with a true 1 microsecond wait time
	rv := time.NewTimer(time.Microsecond)
	<-rv.C
	return StandardTimer{rv}
}

// NewATicker returns a StandardTicker, which is based on the system clock
func (sc *StandardClock) NewATicker(d time.Duration) ATicker {
	return StandardTicker{time.NewTicker(d)}
}

// JumpToTheFuture returns 0, because we can't do it with the standard
// clock (or rather, we choose not to).
func (sc *StandardClock) JumpToTheFuture(d time.Duration) int {
	log.Printf("warning: using JumpToTheFuture with a StandardClock does nothing")
	return 0
}

// JumpToFutureTime returns 0, because we can't do it with the standard
// clock (or rather, we choose not to).
func (sc *StandardClock) JumpToFutureTime(t time.Time) int {
	log.Printf("warning: using JumpToFutureTime with a StandardClock does nothing")
	return 0
}

// ChangeDilationFactor does nothing, because we can't do it with the standard
// clock.
func (sc *StandardClock) ChangeDilationFactor(d float64) {
	log.Printf("warning: using ChangeDilationFactor with a StandardClock does nothing")
}

// DeleteTimer tries to allow GC of the Timer by resetting it to a zero duration
func (sc *StandardClock) DeleteTimer(at ATimer) {
	t := at.(StandardTimer)
	t.Timer.Reset(time.Duration(0))
}

// DeleteTicker tries to allow GC of the Timer by resetting it to a zero duration
func (sc *StandardClock) DeleteTicker(at ATicker) {
	t := at.(StandardTicker)
	t.Ticker.Reset(time.Duration(0))
}

func (sc *StandardClock) RealTime(t time.Time) time.Time {
	return t
}

// RealDuration returns the real time.Duration corresponding to the given dilated duration
func (sc *StandardClock) RealDuration(d time.Duration) time.Duration {
	return d
}

// GetStandardClock returns an AClock that uses the system clock.
func GetStandardClock() AClock {
	return &StandardClock{}
}

// WarpedClock provides an AClock based on a timewarper.Clock
type WarpedClock = Clock

// NewATimer returns a WarpedTimer, which is based on the timewarper clock
func (wc *WarpedClock) NewATimer(d time.Duration) ATimer {
	return wc.NewTimer(d)
}

// NewStoppedATimer returns an already-stopped WarpedTimer, which is based on the timewarper clock
func (wc *WarpedClock) NewStoppedATimer() ATimer {
	// create the timer with a true 1 microsecond wait time
	rv := wc.NewTimer(time.Duration(float64(time.Microsecond) * wc.getDilationFactor()))
	<-rv.Chan()
	return rv
}

// NewATicker returns a WarpedTicker, which is based on the timewarper clock
func (wc *WarpedClock) NewATicker(d time.Duration) ATicker {
	return wc.NewTicker(d)
}

// GetWarpedClock returns an AClock based on a timewarper Clock
func GetWarpedClock(dilationFactor float64, initialEpoch time.Time) AClock {
	return NewClock(dilationFactor, initialEpoch)
}

// DeleteTimer tries to allow GC of the Timer
func (wc *WarpedClock) DeleteTimer(at ATimer) {
	wc.DelTimer(at.(*Timer))
}

// DeleteTicker tries to allow GC of the Ticker
func (wc *WarpedClock) DeleteTicker(at ATicker) {
	wc.DelTicker(at.(*Ticker))
}

func (wc *WarpedClock) RealTime(t time.Time) time.Time {
	return wc.trueEpoch.Add(time.Duration(float64(t.Sub(wc.dilatedEpoch)) / wc.dilationFactor))
}

// RealDuration returns the real time.Duration corresponding to the given dilated duration
func (wc *WarpedClock) RealDuration(d time.Duration) time.Duration {
	return time.Duration(float64(d) / wc.dilationFactor)
}
