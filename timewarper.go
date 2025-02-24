package timewarper

import (
	"sort"
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
	timers         []Timer
	idCounter      int
}

// NewClock creates a clock initialized with the given initialDilationFactor.
// A factor of 1 runs in lock step with real time, while a factor of 0.5 would run at half the pace of real time and a factor of 2 would run at twice the speed of real time.
func NewClock(initialDilationFactor float64, initialEpoch time.Time) Clock {
	return Clock{
		trueEpoch:      initialEpoch,
		dilatedEpoch:   initialEpoch,
		dilationFactor: initialDilationFactor,
		timers:         make([]Timer, 0),
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
	for i := range clock.timers {
		trueTimeRemaining := clock.timers[i].expectedTriggerTime.Sub(clock.trueEpoch)
		newDilatedDuration := time.Duration(float64(trueTimeRemaining) / clock.dilationFactor)
		clock.timers[i].trueTimer.Reset(newDilatedDuration)
	}
}

// JumpToTheFuture will use the given jumpDistance to move the clock forward that far.
//
// A check will be made for any timers that will expire before the new time.
// If one is found, it will be triggered as normal, and the clock will jump to that point in time.
// The duration returned will be the distance that was jumped.
// It is up to you to handle these timers being triggered, it's your program after all.
// The duration returned by JumpToTheFuture can be compared to the duration passed in and if they are not the same then you will know a timer was triggered.
//
// Example: It is 12:00 according to clock.Now(), and you want to jump an hour into the future, but there is a timer set to expire in 30 minutes.
// That timer be triggered and the time that will come out of its channel will be 12:30.
// JumpToTheFuture will return a duration of 30 minutes, and clock.Now() will return a time of 12:30
// You must handle, or not, it's your program, that timer and then call JumpToTheFuture again with your original jump distance minus the distance returned by
// JumpToTheFuture if you wish to complete the jump to your originally desired future time.
func (clock *Clock) JumpToTheFuture(jumpDistance time.Duration) time.Duration {
	clock.access.Lock()
	defer clock.access.Unlock()
	theNewDilatedEpoch := clock.dilatedEpoch.Add(jumpDistance)
	sort.Slice(clock.timers, func(i, j int) bool {
		return clock.timers[i].expectedTriggerTime.Before(clock.timers[j].expectedTriggerTime)
	})
	for i := 0; i < len(clock.timers); i++ {
		if clock.timers[i].expectedTriggerTime.After(theNewDilatedEpoch) {
			break
		}
		if clock.timers[i].expectedTriggerTime.Before(theNewDilatedEpoch.Add(1)) {
			theNewDilatedEpoch = clock.timers[i].expectedTriggerTime
			index := i
			go func() {
				timer := clock.timers[index]
				timer.outputChannel <- timer.expectedTriggerTime
				clock.deleteTimer(timer.id)
			}()
		}
	}
	actualDistanceJumped := theNewDilatedEpoch.Sub(clock.dilatedEpoch)
	clock.dilatedEpoch = theNewDilatedEpoch
	return actualDistanceJumped
}

func (clock *Clock) deleteTimer(timerId int) {
	clock.access.Lock()
	defer clock.access.Unlock()
	for i := 0; i < len(clock.timers); i++ {
		if clock.timers[i].id == timerId {
			clock.timers = append(clock.timers[:i], clock.timers[i+1:]...)
			return
		}
	}
}

func (clock *Clock) After(desiredDuration time.Duration) <-chan time.Time {
	clock.access.Lock()
	defer clock.access.Unlock()
	dilatedDuration := time.Duration(float64(desiredDuration) / clock.dilationFactor)
	newTrueTimer := time.NewTimer(dilatedDuration)
	newWarpedTimer := Timer{
		id:                  clock.idCounter,
		trueTimer:           newTrueTimer,
		outputChannel:       make(chan time.Time),
		expectedTriggerTime: now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor).Add(desiredDuration),
		hasNotBeenTriggered: true,
	}
	go func() {
		<-newWarpedTimer.trueTimer.C
		dilatedTimeNow := clock.Now()
		newWarpedTimer.outputChannel <- dilatedTimeNow
	}()
	clock.timers = append(clock.timers, newWarpedTimer)
	clock.idCounter++
	return newWarpedTimer.outputChannel
}

type Timer struct {
	id                  int
	trueTimer           *time.Timer
	expectedTriggerTime time.Time
	outputChannel       chan time.Time
	hasNotBeenTriggered bool
}
