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
		clock.timers[i].Reset(dilatedTimeRemaining)
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
	//	log.Printf("jumping to future by %v to %v", jumpDistance, newDilatedEpoch)
	for i := 0; i < len(clock.timers); i++ {
		dur := clock.timers[i].dilatedTriggerTime.Sub(newDilatedEpoch)
		if dur > 0 {
			//			log.Printf("adjusting timer %d to new duration %v", clock.timers[i].id, dur)
			clock.timers[i].Reset(dur)
		} else {
			timer := clock.timers[i]
			if timer.stopped {
				//				log.Printf("not stopping timer %d which is already stopped", timer.id)
				continue
			}
			rv++
			go func(t *Timer) {
				//				log.Printf("stopping timer %d and writing %v to its channel", timer.id, timer.dilatedTriggerTime)
				t.Stop()
				t.C <- timer.dilatedTriggerTime
			}(timer)
		}
	}
	clock.dilatedEpoch = newDilatedEpoch
	return
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

func (clock *Clock) After(dilatedDuration time.Duration) <-chan time.Time {
	return clock.NewTimer(dilatedDuration).C
}

func (clock *Clock) Sleep(dilatedSleepDuration time.Duration) {
	clock.access.Lock()
	trueSleepDuration := time.Duration(float64(dilatedSleepDuration) / clock.dilationFactor)
	clock.access.Unlock()
	time.Sleep(trueSleepDuration)
}

type Timer struct {
	id                  int
	trueTimer           *time.Timer
	trueDuration        time.Duration
	dilatedTriggerTime  time.Time
	C                   chan time.Time
	hasNotBeenTriggered bool
	cancel              chan bool
	clock               *Clock
	stopped             bool
	access              sync.Mutex
}

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

func (clock *Clock) NewTicker(dilatedPeriod time.Duration) *Ticker {
	clock.access.Lock()
	defer clock.access.Unlock()
	truePeriod := time.Duration(float64(dilatedPeriod) / clock.dilationFactor)
	//	log.Printf("starting true ticker %d with duration %v for dilated duration %v with dilationFactor = %g\n", clock.idCounter, truePeriod, dilatedPeriod, clock.dilationFactor)
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

func (ticker *Ticker) Stop() {
	ticker.trueTicker.Stop()
}

func (ticker *Ticker) Reset(dilatedPeriod time.Duration) {
	ticker.dilatedPeriod = dilatedPeriod
	dilationFactor := ticker.clock.getDilationFactor()
	truePeriod := time.Duration(float64(dilatedPeriod) / dilationFactor)
	ticker.trueTicker.Reset(truePeriod)
}

func (clock *Clock) NewTimer(dilatedDuration time.Duration) *Timer {
	clock.access.Lock()
	defer clock.access.Unlock()
	trueDuration := time.Duration(float64(dilatedDuration) / clock.dilationFactor)
	//	log.Printf("starting true timer %d with duration %v for dilated duration %v with dilationFactor = %g\n", clock.idCounter, trueDuration, dilatedDuration, clock.dilationFactor)
	newTrueTimer := time.NewTimer(trueDuration)
	newWarpedTimer := &Timer{
		id:                  clock.idCounter,
		trueTimer:           newTrueTimer,
		trueDuration:        trueDuration,
		C:                   make(chan time.Time),
		dilatedTriggerTime:  now(clock.trueEpoch, clock.dilatedEpoch, clock.dilationFactor).Add(dilatedDuration),
		hasNotBeenTriggered: true,
		cancel:              make(chan bool),
		clock:               clock,
		stopped:             false,
	}
	go newWarpedTimer.waitForTrueTimer()
	clock.timers = append(clock.timers, newWarpedTimer)
	clock.idCounter++
	return newWarpedTimer
}

func (timer *Timer) waitForTrueTimer() {
	//	log.Printf("called waitForTrueTimer on timer %d, to trigger at %v, true time %v", timer.id, timer.dilatedTriggerTime, time.Now().Add(timer.trueDuration))
	timer.access.Lock()
	defer timer.access.Unlock()
	if timer.stopped {
		return
	}
	select {
	case <-timer.trueTimer.C:
		//		log.Printf("true timer triggered for timer %d", timer.id)
		dilatedTimeNow := now(timer.clock.trueEpoch, timer.clock.dilatedEpoch, timer.clock.dilationFactor)
		timer.C <- dilatedTimeNow
	case <-timer.cancel:
		//		log.Printf("cancelled waitForTrueTimer on timer %d", timer.id)
	}
	timer.trueTimer.Stop()
	timer.stopped = true
}

func (timer *Timer) Stop() {
	timer.cancel <- true
}

func (timer *Timer) Reset(dilatedDuration time.Duration) {
	timer.cancel <- true
	timer.access.Lock()
	defer timer.access.Unlock()
	trueDuration := time.Duration(float64(dilatedDuration) / timer.clock.dilationFactor)
	//	log.Printf("resetting true timer %d with duration %v for dilated duration %v with dilationFactor = %g\n", timer.id, trueDuration, dilatedDuration, timer.clock.dilationFactor)
	timer.dilatedTriggerTime = now(timer.clock.trueEpoch, timer.clock.dilatedEpoch, timer.clock.dilationFactor).Add(dilatedDuration)
	timer.trueTimer.Reset(trueDuration)
	timer.stopped = false
	go timer.waitForTrueTimer()
}
