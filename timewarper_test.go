package timewarper

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"math/rand/v2"
	"sort"
	"testing"
	"time"
)

func TestStaticDilation(test *testing.T) {
	test.Parallel()
	timeDilationTolerance := 0.001
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
	timeDilationTolerance := 0.005
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
							subTest.Logf("\ntrueTime:    %v\ndilatedTime: %v", trueTime.Format(time.RFC3339), dilatedTime.Format(time.RFC3339))
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
		name                     string
		timerDuration            time.Duration
		dilationFactor           float64
		normalTimerShouldBeFirst bool
	}{
		{
			name:                     "Timer-With-Faster-Dilation",
			timerDuration:            1 * time.Second,
			dilationFactor:           2,
			normalTimerShouldBeFirst: false,
		},
		{
			name:                     "Time-With-Slower-Dilation",
			timerDuration:            1 * time.Second,
			dilationFactor:           0.5,
			normalTimerShouldBeFirst: true,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			clock := NewClock(testCase.dilationFactor, time.Now())
			startTime := time.Now()
			timeoutDuration := testCase.timerDuration
			if testCase.dilationFactor < 1 {
				timeoutDuration = time.Duration(float64(timeoutDuration) / testCase.dilationFactor)
			}
			timeoutDuration += time.Second
			ctx, cancel := context.WithTimeout(test.Context(), timeoutDuration)
			defer cancel()
			var warpedTimerFinishTime time.Time
			var normalTimerFinishTime time.Time
			var normalTimerDuration time.Duration
			var warpedTimerDuration time.Duration
			waitGroup, ctx := errgroup.WithContext(ctx)
			waitGroup.Go(func() error {
				select {
				case normalTimerFinishTime = <-time.After(testCase.timerDuration):
					normalTimerDuration = time.Since(startTime)
					return nil
				case <-ctx.Done():
					return fmt.Errorf("could not get normal timer finish time: %v", ctx.Err())
				}
			})
			waitGroup.Go(func() error {
				select {
				case warpedTimerFinishTime = <-clock.After(testCase.timerDuration):
					warpedTimerDuration = time.Since(startTime)
					return nil
				case <-ctx.Done():
					return fmt.Errorf("could not get warped timer finished time: %v", ctx.Err())
				}
			})
			err := waitGroup.Wait()
			if err != nil {
				test.Errorf("encountered error waiting on timers: %v", err)
			}
			normalTimerWasFirst := normalTimerDuration < warpedTimerDuration
			testFailed := normalTimerWasFirst != testCase.normalTimerShouldBeFirst
			if testFailed {
				test.Errorf("timers were not triggered in the correct order. Expected normal timer first: %v but normal timer was first: %v", testCase.normalTimerShouldBeFirst, normalTimerWasFirst)
			}
			minimumAllowedTimerTime := normalTimerFinishTime.Add(-2 * time.Millisecond)
			maximumAllowedTimerTime := normalTimerFinishTime.Add(2 * time.Millisecond)
			warpedTimerEndedOutOfExpectedBounds := warpedTimerFinishTime.Before(minimumAllowedTimerTime) || warpedTimerFinishTime.After(maximumAllowedTimerTime)
			if warpedTimerEndedOutOfExpectedBounds {
				test.Errorf("The warped timer did not return the correct time.\nExpected %v\nActual   %v\nDifference(Normal Time - Warped Time): %v",
					normalTimerFinishTime.Format(time.RFC3339), warpedTimerFinishTime.Format(time.RFC3339), normalTimerFinishTime.Sub(warpedTimerFinishTime))
			}
			maximumAllowedRealDuration := time.Duration(float64(testCase.timerDuration)/testCase.dilationFactor) + time.Millisecond
			minimumAllowedRealDuration := time.Duration(float64(testCase.timerDuration)/testCase.dilationFactor) - time.Millisecond
			timeIsNotWithinTolerance := warpedTimerDuration < minimumAllowedRealDuration || warpedTimerDuration > maximumAllowedRealDuration
			if timeIsNotWithinTolerance {
				test.Errorf("warped timer duration %v was not within acceptable range of %v - %v", warpedTimerDuration, minimumAllowedRealDuration, maximumAllowedRealDuration)
			}
		})
	}
}

func TestTimeJumping(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		jumpDistance   time.Duration
		timerDurations []time.Duration
	}{
		{
			name:           "Just-A-Jump",
			dilationFactor: 1,
			jumpDistance:   time.Hour,
		},
		{
			name:           "Jump-An-Hour-Forward-And-Trigger-A-Timer",
			dilationFactor: 1,
			jumpDistance:   time.Hour,
			timerDurations: []time.Duration{
				30 * time.Minute,
				time.Hour + time.Second,
			},
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			startTime := time.Now()
			clock := NewClock(testCase.dilationFactor, startTime)
			timerInfos := make([]struct {
				expirationTime time.Time
				channel        <-chan time.Time
			}, len(testCase.timerDurations))
			for i, timerDuration := range testCase.timerDurations {
				timerInfos[i].expirationTime = startTime.Add(timerDuration)
				timerInfos[i].channel = clock.After(timerDuration)
			}
			sort.Slice(timerInfos, func(i, j int) bool {
				return timerInfos[i].expirationTime.Before(timerInfos[j].expirationTime)
			})
			for timeRemainingToJump := testCase.jumpDistance; timeRemainingToJump > 0; timeRemainingToJump -= clock.JumpToTheFuture(timeRemainingToJump) {
				if timeRemainingToJump > 0 {
					for i := 0; i < len(timerInfos); i++ {
						select {
						case expirationTime := <-timerInfos[i].channel:
							differenceBetweenActualAndExpectedExpirationTimesIs := expirationTime.Sub(timerInfos[i].expirationTime)
							if differenceBetweenActualAndExpectedExpirationTimesIs < 0 {
								differenceBetweenActualAndExpectedExpirationTimesIs *= -1
							}
							if differenceBetweenActualAndExpectedExpirationTimesIs > time.Millisecond {
								test.Errorf("difference between actual and expected expiration times is outside of tolerance: actual time: %v expected time: %v", expirationTime, timerInfos[i].expirationTime)
							}
							timerInfos = append(timerInfos[:i], timerInfos[i+1:]...)
							i--
						default:
						}
					}
				}
			}
			nowIs := clock.Now()
			for _, timerInfo := range timerInfos {
				if nowIs.After(timerInfo.expirationTime) {
					test.Errorf("a timer that was suppose to be triggered was not triggered\nnowIs:      %v\nexpiration: %v\ndistanceFromNowToStart: %v", nowIs.Format(time.RFC3339), timerInfo.expirationTime.Format(time.RFC3339), nowIs.Sub(startTime))
				}
			}
		})
	}
}
