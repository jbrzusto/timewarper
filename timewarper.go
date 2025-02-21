package timewarper

import (
	"sync"
	"time"
)

// Clock is a time mechanism that also allows for dilating time to make it run faster or slower relative to real time
// Clocks are safe for multi thread access
type Clock struct {
	trueEpoch      time.Time
	dilatedEpoch   time.Time
	dilationFactor float64
	access         sync.Mutex
	timers         map[int]Timer
	idCounter      int
}

// NewClock creates a clock initialized with the given initialDilationFactor.
// A factor of 1 runs in lock step with real time, while a factor of 0.5 would run at half the pace of real time and a factor of 2 would run at twice the speed of real time.
func NewClock(initialDilationFactor float64, initialEpoch time.Time) Clock {
	return Clock{
		trueEpoch:      initialEpoch,
		dilatedEpoch:   initialEpoch,
		dilationFactor: initialDilationFactor,
		timers:         make(map[int]Timer),
	}
}

// Now returns the time right now according to the timewarper clock.
// It will take into account all the time warping that has happened in its past.
func (clock *Clock) Now() time.Time {
	clock.access.Lock()
	defer clock.access.Unlock()
	return now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor)
}

func now(trueEpoch, dilatedEpoch time.Time, dilationFactor float64) time.Time {
	timeSinceTrueEpoch := time.Since(trueEpoch)
	dilatedTime := time.Duration(float64(timeSinceTrueEpoch) * dilationFactor)
	dilatedNow := dilatedEpoch.Add(dilatedTime)
	return dilatedNow
}

// ChangeDilationFactor will set the timewarper clock's dilation factor to the factor give.
// The time given after changing the dilation factor will take into account all the previous time dilation changes and warps that have occurred in the past
func (clock *Clock) ChangeDilationFactor(newDilationFactor float64) {
	clock.access.Lock()
	defer clock.access.Unlock()
	clock.dilatedEpoch = now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor)
	clock.trueEpoch = time.Now()
	clock.dilationFactor = newDilationFactor
}

func (clock *Clock) JumpToTheFuture(jumpDistance time.Duration) {
	clock.access.Lock()
	defer clock.access.Unlock()
	clock.dilatedEpoch = clock.dilatedEpoch.Add(jumpDistance)
}

func (clock *Clock) After(realDuration time.Duration) <-chan time.Time {
	clock.access.Lock()
	defer clock.access.Unlock()
	dilatedDuration := time.Duration(float64(realDuration) / clock.dilationFactor)
	newTrueTimer := time.NewTimer(dilatedDuration)
	newWarpedTimer := Timer{
		id:        clock.idCounter,
		trueTimer: newTrueTimer,
	}
	clock.timers[newWarpedTimer.id] = newWarpedTimer
	clock.idCounter++
	return newTrueTimer.C
}

type Timer struct {
	id        int
	trueTimer *time.Timer
}

func (timer *Timer) Stop() {
	timer.trueTimer.Stop()
}

func (timer *Timer) Reset(newDuration time.Duration) bool {
	return timer.trueTimer.Reset(newDuration)
}
