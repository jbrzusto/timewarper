package timewarper

import (
	"log"
	"sort"
	"sync"
	"time"
)

// Clock is a time mechanism that also allows for dilating time to make it run faster or slower relative to real time.
//
// Clocks are safe for multithreaded access, unless unsafe==true, in which case the user must ensure
// only one thread at a time can change clock characteristics, allocate Timers and Tickers, or jumpToTheFuture.
type Clock struct {
	trueEpoch      time.Time
	dilatedEpoch   time.Time
	dilationFactor float64
	access         sync.Mutex
	timers         []*Timer
	tickers        []*Ticker
	idCounter      int
	unsafe         bool
}

// SystemTimerAccuracy is the expected accuracy of underlying system timers.
// This is used when jumping the clock forward, to check whether doing so will
// trigger a timer.
var SystemTimerAccuracy = float64(10 * time.Millisecond)

// NewClock creates a new timewarper clock initialized with the given initialDilationFactor and given initialEpoch values.
//
// A dilation factor of 1 will have the produced timewarper clock running in lock step with real time, while a factor of 0.5 would run at half the pace of real time,
// and a factor of 2 would run at twice the speed of real time.
func NewClock(dilationFactor float64, initialEpoch time.Time) *Clock {
	return &Clock{
		trueEpoch:      time.Now(),
		dilatedEpoch:   initialEpoch,
		dilationFactor: dilationFactor,
		timers:         make([]*Timer, 0),
	}
}

// Now returns the time right now according to the timewarper clock.
//
// It will take into account all the time warping that has happened in its past.
func (clock *Clock) Now() time.Time {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	return now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor)
}

// SetUnsafe makes the clock unsafe, if the parameter is true; otherwise,
// the clock is made safe again.  When the clock is unsafe, timers are more
// accurate, but the clock and its timers and tickers are no longer thread-safe.
// SetUnsafe is intended for applications where a single thread uses the clock, and
// where greater precision in Timers and Tickers is desired.
func (clock *Clock) SetUnsafe(x bool) {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	clock.unsafe = x
}

func now(trueEpoch, dilatedEpoch time.Time, dilationFactor float64) time.Time {
	timeSinceTrueEpoch := time.Since(trueEpoch)
	dilatedTime := time.Duration(float64(timeSinceTrueEpoch) * dilationFactor)
	dilatedNow := dilatedEpoch.Add(dilatedTime)
	return dilatedNow
}

func (clock *Clock) dilatedTime(trueTime time.Time) time.Time {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	trueElapsed := trueTime.Sub(clock.trueEpoch)
	dilatedElapsed := time.Duration(float64(trueElapsed) * clock.dilationFactor)
	return clock.dilatedEpoch.Add(dilatedElapsed)
}

// ChangeDilationFactor will set the timewarper clock's dilation factor to the factor give.
// The time given after changing the dilation factor will take into account all the previous time dilation changes and warps that have occurred in the past
func (clock *Clock) ChangeDilationFactor(newDilationFactor float64) {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	clock.dilatedEpoch = now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor)
	clock.trueEpoch = time.Now()
	clock.dilationFactor = newDilationFactor
	for i := range clock.timers {
		dilatedTimeRemaining := clock.timers[i].dilatedTriggerTime.Sub(clock.dilatedEpoch)
		go clock.timers[i].Reset(dilatedTimeRemaining)
	}
	for i := range clock.tickers {
		newDilatedDuration := time.Duration(float64(clock.tickers[i].dilatedPeriod) / newDilationFactor)
		clock.tickers[i].trueTicker.Reset(newDilatedDuration)
	}
}

// JumpToTheFuture will use the given jumpDistance to move the clock forward that far.
// Nothing happens for jumpDistance <= SystemTimerAccuracy * clock.dilationFactor.
//
// For any timers that will expire before the new time, their dilatedTriggerTime is sent
// on their channel.
//
// The remaining timers will have their trueTimer reset to a new duration, so that they
// will trigger at the appropriate warped time.
//
// The function returns the number of timers which triggered due to the jump.
func (clock *Clock) JumpToTheFuture(jumpDistance time.Duration) (rv int) {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	newDilatedEpoch := clock.dilatedEpoch.Add(jumpDistance)
	return clock.jumpToFutureTime(newDilatedEpoch)
}

// JumpToFutureTime jumps to a specific time.Time.  This avoids race conditions
// that arise from JumpToTheFuture if multiple threads are calling it.
func (clock *Clock) JumpToFutureTime(jumpTarget time.Time) (rv int) {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	return clock.jumpToFutureTime(jumpTarget)
}

// jumpToFutureTime requires the caller to have locked clock.access
func (clock *Clock) jumpToFutureTime(dilatedTarget time.Time) (rv int) {
	if !dilatedTarget.Add(-time.Duration(SystemTimerAccuracy * clock.dilationFactor)).After(clock.dilatedEpoch) {
		log.Printf("skipping insufficiently large future jump\n")
		return 0
	}
	sort.Slice(clock.timers, func(i, j int) bool {
		return clock.timers[i].dilatedTriggerTime.Before(clock.timers[j].dilatedTriggerTime)
	})
	clock.dilatedEpoch = dilatedTarget
	for i := 0; i < len(clock.timers); i++ {
		dur := clock.timers[i].dilatedTriggerTime.Sub(dilatedTarget)
		if float64(dur)/clock.dilationFactor > SystemTimerAccuracy {
			// fmt.Printf("   changing duration for timer %d (trig was %s)\n", i, clock.timers[i].dilatedTriggerTime.Format(time.StampMilli))
			go clock.timers[i].Jump(dur)
		} else {
			// fmt.Printf("   triggering        for timer %d (trig was %s)\n", i, clock.timers[i].dilatedTriggerTime.Format(time.StampMilli))
			timer := clock.timers[i]
			if !clock.unsafe {
				timer.access.Lock()
			}
			stopped := timer.stopped
			if !clock.unsafe {
				timer.access.Unlock()
			}
			if stopped {
				continue
			}
			rv++
			go func(t *Timer) {
				t.Stop()
				t.C <- timer.dilatedTriggerTime
			}(timer)
		}
	}
	return
}

// After returns a channel on which the dilated time will be written after
// dilatedDuration.
func (clock *Clock) After(dilatedDuration time.Duration) <-chan time.Time {
	return clock.NewTimer(dilatedDuration).C
}

// Sleep pauses the thread for a dilated duration.
func (clock *Clock) Sleep(dilatedSleepDuration time.Duration) {
	if !clock.unsafe {
		clock.access.Lock()
	}
	trueSleepDuration := time.Duration(float64(dilatedSleepDuration) / clock.dilationFactor)
	if !clock.unsafe {
		clock.access.Unlock()
	}
	time.Sleep(trueSleepDuration)
}

// timerEventType is an enumerated type for
// different timer events
type TimerEventType int

const (
	Quit TimerEventType = iota
	Stop
	Reset
	ResetTo
	TimeJump
	Confirm
)

type TimerEvent struct {
	TimerEventType
	time.Duration
	time.Time
}

// Timer is an analog to time.Timer for a warped clock
type Timer struct {
	id                 int
	trueTimer          *time.Timer
	trueDuration       time.Duration
	dilatedTriggerTime time.Time
	C                  chan time.Time
	cancelChan         chan bool
	clock              *Clock
	// stopped is used by JumpToTheFuture; when true, the Timer is not
	// triggered by a JumpToTheFuture that passes its dilatedTriggerTime.
	stopped bool
	access  sync.Mutex
}

// Ticker is an analog to time.Ticker for a warped clock
type Ticker struct {
	id            int
	dilatedPeriod time.Duration
	trueTicker    *time.Ticker
	C             <-chan time.Time
	clock         *Clock
}

func (clock *Clock) getDilationFactor() float64 {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	return clock.dilationFactor
}

// NewTicker returns a Ticker with dilatedPeriod
func (clock *Clock) NewTicker(dilatedPeriod time.Duration) *Ticker {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	truePeriod := time.Duration(float64(dilatedPeriod) / clock.dilationFactor)
	trueTicker := time.NewTicker(truePeriod)
	dilatedTicker := Ticker{
		id:            clock.idCounter,
		dilatedPeriod: dilatedPeriod,
		trueTicker:    trueTicker,
		C:             trueTicker.C,
		clock:         clock,
	}
	clock.tickers = append(clock.tickers, &dilatedTicker)
	clock.idCounter++
	return &dilatedTicker
}

// Stop stops the Ticker
func (ticker *Ticker) Stop() {
	ticker.trueTicker.Stop()
}

// Reset restarts a Ticker with a new dilatedPeriod
func (ticker *Ticker) Reset(dilatedPeriod time.Duration) {
	ticker.dilatedPeriod = dilatedPeriod
	dilationFactor := ticker.clock.getDilationFactor()
	truePeriod := time.Duration(float64(dilatedPeriod) / dilationFactor)
	ticker.trueTicker.Reset(truePeriod)
}

// Chan returns the channel for a Ticker
func (ticker *Ticker) Chan() <-chan time.Time {
	return ticker.C
}

// NewTimer returns a Timer with the dilatedDuration
func (clock *Clock) NewTimer(dilatedDuration time.Duration) *Timer {
	if !clock.unsafe {
		clock.access.Lock()
		defer clock.access.Unlock()
	}
	trueDuration := time.Duration(float64(dilatedDuration) / clock.dilationFactor)
	newTrueTimer := time.NewTimer(trueDuration)
	newWarpedTimer := &Timer{
		id:                 clock.idCounter,
		trueTimer:          newTrueTimer,
		trueDuration:       trueDuration,
		C:                  make(chan time.Time, 0),
		cancelChan:         make(chan bool, 0),
		dilatedTriggerTime: now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor).Add(dilatedDuration),
		clock:              clock,
		stopped:            false,
	}
	go newWarpedTimer.waitForTrueTimer()
	clock.timers = append(clock.timers, newWarpedTimer)
	clock.idCounter++
	return newWarpedTimer
}

// NewATimerTo returns a Timer with the given dilatedTriggerTime
func (clock *Clock) NewATimerTo(dilatedTriggerTime time.Time) ATimer {
	return clock.NewTimer(dilatedTriggerTime.Sub(clock.Now()))
}

// waitForTrueTimer is run as a goroutine and waits on a time.Timer, writing
// the dilated time to the Timer's channel.
func (timer *Timer) waitForTrueTimer() {
	for {
		select {
		case t, ok := <-timer.trueTimer.C:
			if ok {
				timer.C <- timer.clock.dilatedTime(t)
			}
		case <-timer.cancelChan:
			select {
			case timer.C <- time.Time{}:
			default:
			}
			return
		}
	}
}

// DelTimer stops the goroutine for a Timer and deletes it from the
// clock's set of all Timer, allowing it to be GC'd.  Returns true if
// the Timer was found.
func (clock *Clock) DelTimer(t *Timer) bool {
	for i, v := range clock.timers {
		if v == t {
			t.trueTimer.Stop()
			t.cancelChan <- false
			clock.timers = append(clock.timers[0:i], clock.timers[i+1:]...)
			return true
		}
	}
	return false
}

// DelTicker deletes the Ticker from the clock's set of all Ticker,
// allowing it to be GC'd.  Returns true if the Ticker was found.
func (clock *Clock) DelTicker(t *Ticker) bool {
	for i, v := range clock.tickers {
		if v == t {
			clock.tickers[i].trueTicker.Stop()
			clock.tickers = append(clock.tickers[0:i], clock.tickers[i+1:]...)
			return true
		}
	}
	return false
}

// Stop stops the Timer by sending a value on its cancel channel,
// and waiting for confirmation.
func (timer *Timer) Stop() {
	if !timer.clock.unsafe {
		timer.access.Lock()
		defer timer.access.Unlock()
	}
	timer.stopped = true
	timer.trueTimer.Stop()
}

// Reset sets a new dilated duration for the Timer.
func (timer *Timer) Reset(dilatedDuration time.Duration) {
	if !timer.clock.unsafe {
		timer.access.Lock()
		timer.clock.access.Lock()
		defer timer.clock.access.Unlock()
		defer timer.access.Unlock()
	}
	trueDuration := time.Duration(float64(dilatedDuration) / timer.clock.dilationFactor)
	timer.dilatedTriggerTime = now(timer.clock.trueEpoch, timer.clock.dilatedEpoch, timer.clock.dilationFactor).Add(dilatedDuration)
	timer.stopped = false
	timer.trueTimer.Reset(trueDuration)
}

// ResetTo sets a new dilated trigger time for the Timer
func (timer *Timer) ResetTo(dilatedTriggerTime time.Time) {
	if !timer.clock.unsafe {
		timer.access.Lock()
		timer.clock.access.Lock()
		defer timer.clock.access.Unlock()
		defer timer.access.Unlock()
	}
	dilatedDuration := dilatedTriggerTime.Sub(timer.clock.Now())
	trueDuration := time.Duration(float64(dilatedDuration) / timer.clock.dilationFactor)
	timer.stopped = false
	timer.dilatedTriggerTime = dilatedTriggerTime
	timer.trueTimer.Reset(trueDuration)
}

// Target returns the dilatedTriggerTime
func (timer *Timer) Target() time.Time {
	return timer.dilatedTriggerTime
}

// Jump tells a timer the warped clock is advancing by the specified dilatedDuration
func (timer *Timer) Jump(dilatedDuration time.Duration) {
	if !timer.clock.unsafe {
		timer.access.Lock()
		timer.clock.access.Lock()
		defer timer.clock.access.Unlock()
		defer timer.access.Unlock()
	}
	trueDuration := time.Duration(float64(dilatedDuration) / timer.clock.dilationFactor)
	// Note: we don't modify the dilatedTriggerTime
	timer.trueTimer.Reset(trueDuration)
}

// Chan returns the channel for a Timer
func (timer *Timer) Chan() <-chan time.Time {
	return timer.C
}
