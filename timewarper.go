package timewarper

import (
	"sync"
	"time"
)

// Clock is a time mechanism that also allows for dilating time to make it run faster or slower relative to real time.
//
// Clocks are safe for multithreaded access.
type Clock struct {
	trueEpoch      time.Time
	dilatedEpoch   time.Time
	dilationFactor float64
	access         sync.Mutex
	timers         []*Timer
	tickers        []*Ticker
	idCounter      int
}

// NewClock creates a new timewarper clock initialized with the given initialDilationFactor and given initialEpoch values.
//
// A dilation factor of 1 will have the produced timewarper clock running in lock step with real time, while a factor of 0.5 would run at half the pace of real time,
// and a factor of 2 would run at twice the speed of real time.
func NewClock(dilationFactor float64, initialEpoch time.Time) Clock {
	return Clock{
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
		dilatedTimeRemaining := clock.timers[i].dilatedTriggerTime.Sub(clock.dilatedEpoch)
		go clock.timers[i].Reset(dilatedTimeRemaining)
	}
	for i := range clock.tickers {
		newDilatedDuration := time.Duration(float64(clock.tickers[i].dilatedPeriod) / newDilationFactor)
		clock.tickers[i].trueTicker.Reset(newDilatedDuration)
	}
}

// JumpToTheFuture will use the given jumpDistance to move the clock forward that far.
//
// For any timers that will expire before the new time, their dilatedTriggerTime is sent
// on their channel.
//
// The remaining timers will have their trueTimer reset to a new duration, so that they
// will trigger at the appropriate warped time.
//
// The function returns the number of timers which triggered due to the jump.
func (clock *Clock) JumpToTheFuture(jumpDistance time.Duration) (rv int) {
	clock.access.Lock()
	defer clock.access.Unlock()
	newDilatedEpoch := clock.dilatedEpoch.Add(jumpDistance)
	for i := 0; i < len(clock.timers); i++ {
		dur := clock.timers[i].dilatedTriggerTime.Sub(newDilatedEpoch)
		if dur > 0 {
			clock.timers[i].Reset(dur)
		} else {
			timer := clock.timers[i]
			timer.access.Lock()
			stopped := timer.stopped
			timer.access.Unlock()
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
	clock.dilatedEpoch = newDilatedEpoch
	return
}

// After returns a channel on which the dilated time will be written after
// dilatedDuration.
func (clock *Clock) After(dilatedDuration time.Duration) <-chan time.Time {
	return clock.NewTimer(dilatedDuration).C
}

// Sleep pauses the thread for a dilated duration.
func (clock *Clock) Sleep(dilatedSleepDuration time.Duration) {
	clock.access.Lock()
	trueSleepDuration := time.Duration(float64(dilatedSleepDuration) / clock.dilationFactor)
	clock.access.Unlock()
	time.Sleep(trueSleepDuration)
}

// Timer is an analog to time.Timer for a warped clock
type Timer struct {
	id                 int
	trueTimer          *time.Timer
	trueDuration       time.Duration
	dilatedTriggerTime time.Time
	C                  chan time.Time
	// reset is a channel on which new durations are sent to the
	// goroutine which waites for trueTimer.  A new duration of 0
	// stops the trueTimer, and a new duration < 0 exits the goroutine.
	reset chan time.Duration
	clock *Clock
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
	clock.access.Lock()
	defer clock.access.Unlock()
	return clock.dilationFactor
}

// NewTicker returns a Ticker with dilatedPeriod
func (clock *Clock) NewTicker(dilatedPeriod time.Duration) *Ticker {
	clock.access.Lock()
	defer clock.access.Unlock()
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

// NewTimer returns a Timer with the dilatedDuration
func (clock *Clock) NewTimer(dilatedDuration time.Duration) *Timer {
	clock.access.Lock()
	defer clock.access.Unlock()
	trueDuration := time.Duration(float64(dilatedDuration) / clock.dilationFactor)
	newTrueTimer := time.NewTimer(trueDuration)
	newWarpedTimer := &Timer{
		id:                 clock.idCounter,
		trueTimer:          newTrueTimer,
		trueDuration:       trueDuration,
		C:                  make(chan time.Time),
		dilatedTriggerTime: now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor).Add(dilatedDuration),
		reset:              make(chan time.Duration),
		clock:              clock,
		stopped:            false,
	}
	go newWarpedTimer.waitForTrueTimer()
	clock.timers = append(clock.timers, newWarpedTimer)
	clock.idCounter++
	return newWarpedTimer
}

// waitForTrueTimer is run as a goroutine and waits on a time.Timer, writing
// the dilated time to the Timer's channel, or for a value sent on its reset channel.
// A dilatedDuration value received from the reset channel is handled thus:
//   - negative: the goroutine exits.  This should only happen when the Timer is being garbage collected.
//     In this case, time.Time(0) is sent to the Timer's channel if a receiver is waiting.
//   - zero: the trueTimer is stopped, but the goroutine continues, allowing for a subsequent call to Timer.Reset
//   - positive: the trueTimer is reset to a new duration corresponding to the dilatedDuration received
func (timer *Timer) waitForTrueTimer() {
	timer.access.Lock()
	timer.stopped = false
	timer.access.Unlock()
lifespan:
	for {
		select {
		case <-timer.trueTimer.C:
			dilatedTimeNow := now(timer.clock.trueEpoch, timer.clock.dilatedEpoch, timer.clock.dilationFactor)
			timer.access.Lock()
			timer.stopped = true
			timer.access.Unlock()
			timer.C <- dilatedTimeNow
		case newDilatedDuration := <-timer.reset:
			switch {
			case newDilatedDuration < 0:
				// send 0 to C if a receiver is waiting, otherwise do nothing
				select {
				case timer.C <- time.Time{}:
				default:
				}
				break lifespan
			case newDilatedDuration == 0:
				timer.trueTimer.Stop()
				timer.access.Lock()
				timer.stopped = true
				timer.access.Unlock()
			case newDilatedDuration > 0:
				timer.access.Lock()
				trueDuration := time.Duration(float64(newDilatedDuration) / timer.clock.dilationFactor)
				timer.dilatedTriggerTime = now(timer.clock.trueEpoch, timer.clock.dilatedEpoch, timer.clock.dilationFactor).Add(newDilatedDuration)
				timer.stopped = false
				timer.trueTimer.Reset(trueDuration)
				timer.access.Unlock()
			}
		}
	}
}

// DelTimer stops the goroutine for a Timer and deletes it from the
// clock's set of all Timer, allowing it to be GC'd.  Returns true if
// the Timer was found.
func (clock *Clock) DelTimer(t *Timer) bool {
	for i, v := range clock.timers {
		if v == t {
			clock.timers[i].reset <- time.Duration(-1)
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

// Stop stops the Timer by sending a value on its cancel channel.
func (timer *Timer) Stop() {
	timer.reset <- time.Duration(0)
}

// Reset stops the existing wait for the Timer (if
func (timer *Timer) Reset(dilatedDuration time.Duration) {
	timer.reset <- dilatedDuration
}
