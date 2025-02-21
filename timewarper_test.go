package timewarper

import (
	"context"
	"golang.org/x/sync/errgroup"
	"math/rand/v2"
	"testing"
	"time"
)

func TestStaticDilation(test *testing.T) {
	test.Parallel()
	timeDilationTolerance := 0.0001
	tickDuration := time.Second
	totalTicks := 10
	testCases := []struct {
		name           string
		dilationFactor float64
	}{
		{
			name:           "Dilate-Forward-Faster",
			dilationFactor: 2,
		},
		{
			name:           "Dilate-Forward-Slow",
			dilationFactor: 0.5,
		},
		{
			name:           "Dilate-Backwards-At-Same-Speed",
			dilationFactor: -1,
		},
		{
			name:           "Dilate-Backward-Faster",
			dilationFactor: -2,
		},
		{
			name:           "Dilate-Backward-Slower",
			dilationFactor: -0.5,
		},
		{
			name:           "Random-Dilation",
			dilationFactor: rand.Float64()*10 - 5,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()
			waitGroup, ctx := errgroup.WithContext(subTest.Context())
			startTime := time.Now()
			trueTimeChannel := make(chan time.Time)
			dilatedTimeChannel := make(chan time.Time)
			waitGroup.Go(func() error {
				clock := NewClock(testCase.dilationFactor, startTime)
				for i := 0; i < totalTicks; i++ {
					select {
					case <-ctx.Done():
						close(dilatedTimeChannel)
						return ctx.Err()
					case <-time.After(tickDuration):
						dilatedTimeChannel <- clock.Now()
					}
				}
				close(dilatedTimeChannel)
				return nil
			})
			waitGroup.Go(func() error {
				for i := 0; i < totalTicks; i++ {
					select {
					case <-ctx.Done():
						close(trueTimeChannel)
						return ctx.Err()
					case <-time.After(tickDuration):
						trueTimeChannel <- time.Now()
					}
				}
				close(trueTimeChannel)
				return nil
			})
			waitGroup.Go(func() error {
				maximumAcceptableDilationFactor := testCase.dilationFactor + timeDilationTolerance
				minimumAcceptableDilationFactor := testCase.dilationFactor - timeDilationTolerance
				for i := 0; i < totalTicks; i++ {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						trueTime := <-trueTimeChannel
						dilatedTime := <-dilatedTimeChannel
						trueTimePassage := trueTime.Sub(startTime)
						dilatedTimePassage := dilatedTime.Sub(startTime)
						dilationFactor := float64(dilatedTimePassage) / float64(trueTimePassage)
						dilationFactorIsTooLow := dilationFactor < minimumAcceptableDilationFactor
						dilationFactorIsTooHigh := dilationFactor > maximumAcceptableDilationFactor
						if dilationFactorIsTooLow || dilationFactorIsTooHigh {
							subTest.Errorf("dilation factor from test run of %v is out of tolerance of %v - %v", dilationFactor, minimumAcceptableDilationFactor, maximumAcceptableDilationFactor)
							subTest.Logf("\ntrueTime:    %v\ndilatedTime: %v", trueTime, dilatedTime)
							subTest.Logf("\nTrueTimePassage:    %v\ndilatedTimePassage: %v", trueTimePassage, dilatedTimePassage)
						}
					}
				}
				return nil
			})
			err := waitGroup.Wait()
			if err != nil {
				test.Errorf("unexpected error occured: %v", err)
			}
		})
	}
}

func TestDynamicDilation(test *testing.T) {
	test.Parallel()
	timeDilationTolerance := 0.001
	tickDuration := time.Second
	testCases := []struct {
		name            string
		dilationFactors []float64
	}{
		{
			name:            "Forward",
			dilationFactors: []float64{1, 2, 3, 4, 5},
		},
		{
			name:            "Backward",
			dilationFactors: []float64{-1, -2, -3, -4, -5},
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()
			startTime := time.Now()
			trueTimeChannel := make(chan time.Time)
			dilatedTimeChannel := make(chan time.Time)
			waitGroup, ctx := errgroup.WithContext(subTest.Context())
			waitGroup.Go(func() error {
				clock := NewClock(0, startTime)
				for i := 0; i < len(testCase.dilationFactors); i++ {
					clock.ChangeDilationFactor(testCase.dilationFactors[i])
					select {
					case <-ctx.Done():
						close(dilatedTimeChannel)
						return ctx.Err()
					case <-time.After(tickDuration):
						dilatedTimeChannel <- clock.Now()
					}
				}
				close(dilatedTimeChannel)
				return nil
			})
			waitGroup.Go(func() error {
				for i := 0; i < len(testCase.dilationFactors); i++ {
					select {
					case <-ctx.Done():
						close(trueTimeChannel)
						return ctx.Err()
					case <-time.After(tickDuration):
						trueTimeChannel <- time.Now()
					}
				}
				close(trueTimeChannel)
				return nil
			})
			waitGroup.Go(func() error {
				dilationSum := float64(0)
				for i := 0; i < len(testCase.dilationFactors); i++ {
					dilationSum += testCase.dilationFactors[i]
					dilationAverage := dilationSum / float64(i+1)
					maximumAcceptableDilationFactor := dilationAverage + timeDilationTolerance
					minimumAcceptableDilationFactor := dilationAverage - timeDilationTolerance
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						trueTime := <-trueTimeChannel
						dilatedTime := <-dilatedTimeChannel
						trueTimePassage := trueTime.Sub(startTime)
						dilatedTimePassage := dilatedTime.Sub(startTime)
						dilationFactor := float64(dilatedTimePassage) / float64(trueTimePassage)
						dilationFactorIsTooLow := dilationFactor < minimumAcceptableDilationFactor
						dilationFactorIsTooHigh := dilationFactor > maximumAcceptableDilationFactor
						if dilationFactorIsTooLow || dilationFactorIsTooHigh {
							subTest.Errorf("dilation factor from test run of %v is out of tolerance of %v - %v", dilationFactor, minimumAcceptableDilationFactor, maximumAcceptableDilationFactor)
							subTest.Logf("\ntrueTime:    %v\ndilatedTime: %v", trueTime, dilatedTime)
							subTest.Logf("\nTrueTimePassage:    %v\ndilatedTimePassage: %v", trueTimePassage, dilatedTimePassage)
						}
					}
				}
				return nil
			})
			err := waitGroup.Wait()
			if err != nil {
				test.Errorf("unexpected error occured: %v", err)
			}
		})
	}
}

func TestTimers(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name                    string
		timerDuration           time.Duration
		dilationFactor          float64
		normalTimeShouldBeFirst bool
	}{
		{
			name:                    "Timer-With-Faster-Dilation",
			timerDuration:           1 * time.Second,
			dilationFactor:          2,
			normalTimeShouldBeFirst: false,
		},
		{
			name:                    "Time-With-Slower-Dilation",
			timerDuration:           1 * time.Second,
			dilationFactor:          0.5,
			normalTimeShouldBeFirst: true,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			clock := NewClock(testCase.dilationFactor, time.Now())
			var normalTimerActualFinishTime time.Time
			var warpedTimerActualFinishTime time.Time
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(test.Context(), testCase.timerDuration+time.Second)
			defer cancel()
			waitGroup, ctx := errgroup.WithContext(ctx)
			waitGroup.Go(func() error {
				select {
				case normalTimerActualFinishTime = <-time.After(testCase.timerDuration):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
			waitGroup.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case warpedTimerActualFinishTime = <-clock.After(testCase.timerDuration):
					return nil
				}
			})
			err := waitGroup.Wait()
			if err != nil {
				test.Errorf("encountered error waiting on timers: %v", err)
			}
			normalTimerWasFirst := normalTimerActualFinishTime.Before(warpedTimerActualFinishTime)
			testFailed := normalTimerWasFirst != testCase.normalTimeShouldBeFirst
			if testFailed {
				test.Errorf("timers were not triggered in the correct order. Expected normal timer first: %v but normal timer was first: %v", testCase.normalTimeShouldBeFirst, normalTimerWasFirst)
			}
			warpedTimerRealDuration := float64(warpedTimerActualFinishTime.Sub(startTime))
			maximumAllowedRealDuration := 0.01 + float64(testCase.timerDuration)/testCase.dilationFactor
			minimumAllowedRealDuration := 0.01 - float64(testCase.timerDuration)/testCase.dilationFactor
			timeIsNotWithinTolerance := warpedTimerRealDuration < minimumAllowedRealDuration || warpedTimerRealDuration >= maximumAllowedRealDuration
			if timeIsNotWithinTolerance {
				test.Errorf("warped timer duration %v was not within tolerance", warpedTimerRealDuration)
			}
		})
	}
}

func TestTimeJumping(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name                    string
		dilationFactor          float64
		jumpDistance            time.Duration
		timerBeforeJumpDuration time.Duration
		timerAfterJumpDuration  time.Duration
	}{
		{
			name:         "Just-A-Jump",
			jumpDistance: time.Hour,
		},
		{
			name:                    "Jump-An-Hour-Forward-And-Trigger-A-Timer",
			jumpDistance:            time.Hour,
			timerBeforeJumpDuration: 30 * time.Minute,
			timerAfterJumpDuration:  time.Hour + time.Second,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			aTimerThatWillBeTriggeredByJumpExists := testCase.timerBeforeJumpDuration > 0
			aTimerThatWontBeTriggeredExists := testCase.timerAfterJumpDuration > testCase.jumpDistance
			startTime := time.Now()
			clock := NewClock(testCase.dilationFactor, startTime)
			var beforeJumpTimer <-chan time.Time
			var afterJumpTimer <-chan time.Time
			if aTimerThatWillBeTriggeredByJumpExists {
				beforeJumpTimer = clock.After(testCase.timerBeforeJumpDuration)
			}
			if aTimerThatWontBeTriggeredExists {
				afterJumpTimer = clock.After(testCase.timerAfterJumpDuration)
			}
			clock.JumpToTheFuture(testCase.jumpDistance)
			timeClockActuallyJumped := clock.Now().Sub(startTime)
			minimumAllowedJumpedTime := testCase.jumpDistance
			maximumAllowedJumpedTime := minimumAllowedJumpedTime + time.Millisecond
			timeClockActuallyJumpedNotTolerable := timeClockActuallyJumped < minimumAllowedJumpedTime || timeClockActuallyJumped > maximumAllowedJumpedTime
			if timeClockActuallyJumpedNotTolerable {
				test.Errorf("time that the clock actually jumped %v is not acceptabled", timeClockActuallyJumped)
			}
			if aTimerThatWillBeTriggeredByJumpExists {
				select {
				// Why is something coming on the channel?
				case triggeredTime := <-beforeJumpTimer:
					timeBetweenStartAndEndOfTimer := triggeredTime.Sub(startTime)
					minimumToleratedTime := testCase.timerBeforeJumpDuration
					maximumToleratedTime := minimumToleratedTime + time.Millisecond
					timeBetweenStartAndEndOfTimerOutsideTolerances := timeBetweenStartAndEndOfTimer < minimumToleratedTime || timeBetweenStartAndEndOfTimer > maximumToleratedTime
					if timeBetweenStartAndEndOfTimerOutsideTolerances {
						test.Errorf("timer that should have been triggered by time jump was triggered outside of tolerances. Time between start and end was %v", timeBetweenStartAndEndOfTimer)
					}
				default:
					// We should be getting here in this failure case
					test.Errorf("timer that was suppose to be triggered by jump was not triggered")
				}
			}
			if aTimerThatWontBeTriggeredExists {
				select {
				// Why is something coming on the channel?
				case triggeredTime := <-afterJumpTimer:
					test.Errorf("timer that was suppose to trigger after the time jump was triggered at time %v", triggeredTime)
				default:
					// We should be getting here
					// This is the desired outcome
				}
			}
		})
	}
}
