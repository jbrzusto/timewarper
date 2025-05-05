package timewarper

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
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
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			waitGroup, ctx := errgroup.WithContext(test.Context())
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
							test.Errorf("dilation factor from test run of %v is out of tolerance of %v - %v", dilationFactor, minimumAcceptableDilationFactor, maximumAcceptableDilationFactor)
							test.Logf("\ntrueTime:    %v\ndilatedTime: %v", trueTime, dilatedTime)
							test.Logf("\nTrueTimePassage:    %v\ndilatedTimePassage: %v", trueTimePassage, dilatedTimePassage)
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
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			startTime := time.Now()
			trueTimeChannel := make(chan time.Time)
			dilatedTimeChannel := make(chan time.Time)
			waitGroup, ctx := errgroup.WithContext(test.Context())
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
							test.Errorf("dilation factor from test run of %v is out of tolerance of %v - %v", dilationFactor, minimumAcceptableDilationFactor, maximumAcceptableDilationFactor)
							test.Logf("\ntrueTime:    %v\ndilatedTime: %v", trueTime.Format(time.RFC3339), dilatedTime.Format(time.RFC3339))
							test.Logf("\nTrueTimePassage:    %v\ndilatedTimePassage: %v", trueTimePassage, dilatedTimePassage)
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

func TestClockAfter(test *testing.T) {
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
			minimumAllowedTimerTime := normalTimerFinishTime.Add(-3 * time.Millisecond)
			maximumAllowedTimerTime := normalTimerFinishTime.Add(3 * time.Millisecond)
			warpedTimerEndedOutOfExpectedBounds := warpedTimerFinishTime.Before(minimumAllowedTimerTime) || warpedTimerFinishTime.After(maximumAllowedTimerTime)
			if warpedTimerEndedOutOfExpectedBounds {
				test.Errorf("The warped timer did not return the correct time.\nExpected %v\nActual   %v\nDifference(Normal Time - Warped Time): %v",
					normalTimerFinishTime.Format(time.RFC3339), warpedTimerFinishTime.Format(time.RFC3339), normalTimerFinishTime.Sub(warpedTimerFinishTime))
			}
			maximumAllowedRealDuration := time.Duration(float64(testCase.timerDuration)/testCase.dilationFactor) + 3*time.Millisecond
			minimumAllowedRealDuration := time.Duration(float64(testCase.timerDuration)/testCase.dilationFactor) - 3*time.Millisecond
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

func TestTimersWithChangesInDilationFactor(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		timerDuration  time.Duration
	}{
		{
			name:           "The-Test",
			dilationFactor: 1,
			timerDuration:  10 * time.Second,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			startTime := time.Now()
			clock := NewClock(testCase.dilationFactor, startTime)
			timer := clock.After(testCase.timerDuration)
			time.Sleep(testCase.timerDuration / 2)
			clock.ChangeDilationFactor(testCase.dilationFactor * 2)
			timersExpirationTime := <-timer
			timersRealDuration := time.Since(startTime)
			timersDilatedDuration := timersExpirationTime.Sub(startTime)
			minimumAllowedDuration := testCase.timerDuration - 2*time.Millisecond
			maximumAllowedDuration := testCase.timerDuration + 2*time.Millisecond
			timersDurationIsOutOfTolerance := timersDilatedDuration < minimumAllowedDuration || timersDilatedDuration > maximumAllowedDuration
			if timersDurationIsOutOfTolerance {
				test.Errorf("Timer's dilated duration was out of tolerance.\nExpected: %v +/- 2ms\nActual: %v", testCase.timerDuration, timersDilatedDuration)
			}
			firstPortionExpectedRealDuration := time.Duration((float64(testCase.timerDuration) / 2) / testCase.dilationFactor)
			secondPortionExpectedRealDuration := time.Duration((float64(testCase.timerDuration) / 2) / (testCase.dilationFactor * 2))
			expectedRealDuration := firstPortionExpectedRealDuration + secondPortionExpectedRealDuration
			minimumAllowedDuration = expectedRealDuration - 2*time.Millisecond
			maximumAllowedDuration = expectedRealDuration + 2*time.Millisecond
			timersDurationIsOutOfTolerance = timersRealDuration < minimumAllowedDuration || timersRealDuration > maximumAllowedDuration
			if timersDurationIsOutOfTolerance {
				test.Errorf("Timer's real duration was out of tolerance.\nExpected: %v +/- 2ms\nActual: %v", expectedRealDuration, timersRealDuration)
			}
		})
	}
}

func TestTicker(test *testing.T) {
	testCases := []struct {
		name           string
		dilationFactor float64
		tickPeriod     time.Duration
		testDuration   time.Duration
		resetAfter     time.Duration
	}{
		{
			name:           "LowerSpeed",
			dilationFactor: 2,
			tickPeriod:     time.Second,
			testDuration:   10*time.Second + 500*time.Millisecond,
			resetAfter:     5 * time.Second,
		},
		{
			name: "Higher-Speed",
			//			dilationFactor: 7150,
			dilationFactor: 900,
			tickPeriod:     time.Second,
			testDuration:   1*time.Second + time.Millisecond,
			resetAfter:     time.Second,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			startTime := time.Now()
			clock := NewClock(testCase.dilationFactor, startTime)
			dilatedTicker := clock.NewTicker(testCase.tickPeriod)
			dilatedTickerToBeStopped := clock.NewTicker(testCase.tickPeriod)
			normalTicker := time.NewTicker(testCase.tickPeriod)
			timer := time.NewTimer(testCase.testDuration)
			dilatedTickCounter := 0
			stoppedTickCounter := 0
			normalTickCounter := 0
			testStillRunning := true
			expectedTicksForTickerGoingToBeStopped := int(float64(testCase.testDuration/testCase.resetAfter) * testCase.dilationFactor)
			for testStillRunning {
				select {
				case <-timer.C:
					testStillRunning = false
				case <-normalTicker.C:
					normalTickCounter++
				case <-dilatedTicker.C:
					dilatedTickCounter++
				case <-dilatedTickerToBeStopped.C:
					stoppedTickCounter++
					if stoppedTickCounter == expectedTicksForTickerGoingToBeStopped {
						dilatedTickerToBeStopped.Stop()
					}
				}
			}
			actualTickRatio := float64(dilatedTickCounter / normalTickCounter)
			expectedTickRatio := testCase.dilationFactor
			tickRatioOutOfTolerance := expectedTickRatio != actualTickRatio
			if tickRatioOutOfTolerance {
				test.Errorf("Expected tick ratio of %.4f but actually got %.4f", expectedTickRatio, actualTickRatio)
			}
			if stoppedTickCounter != expectedTicksForTickerGoingToBeStopped {
				test.Errorf("Expected %v ticks from stopped ticker but actually got %v", expectedTicksForTickerGoingToBeStopped, stoppedTickCounter)
			}
			fmt.Printf("%v", time.Second/7600)
		})
	}
}

func TestTickerWithDilationChange(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		tickPeriod     time.Duration
		testDuration   time.Duration
	}{
		{
			name:           "Test",
			dilationFactor: 2,
			tickPeriod:     time.Second,
			testDuration:   10*time.Second + 50*time.Millisecond,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			clock := NewClock(testCase.dilationFactor, time.Now())
			ticker := clock.NewTicker(testCase.tickPeriod)
			timer := time.NewTimer(testCase.testDuration)
			changeDilationTimer := time.NewTimer(testCase.testDuration / 2)
			expectedNumberOfTicks := float64(testCase.testDuration/time.Second) / 2 * testCase.dilationFactor
			expectedNumberOfTicks += float64(testCase.testDuration/time.Second) / 2 * testCase.dilationFactor * 2
			numberOfTicks := 0
			testStillRunning := true
			for testStillRunning {
				select {
				case <-timer.C:
					testStillRunning = false
				case <-changeDilationTimer.C:
					clock.ChangeDilationFactor(testCase.dilationFactor * 2)
				case <-ticker.C:
					numberOfTicks++
				}
			}
			if int(expectedNumberOfTicks) != numberOfTicks {
				test.Errorf("Expected %v ticks but actually got %v", expectedNumberOfTicks, numberOfTicks)
			}
		})
	}
}

func TestTickerReset(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		tickPeriod     time.Duration
		testDuration   time.Duration
	}{
		{
			name:           "The-Test",
			dilationFactor: 2,
			tickPeriod:     time.Second,
			testDuration:   10*time.Second + 50*time.Millisecond,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			clock := NewClock(testCase.dilationFactor, time.Now())
			ticker := clock.NewTicker(testCase.tickPeriod)
			timer := time.NewTimer(testCase.testDuration)
			resetTimer := time.NewTimer(testCase.testDuration / 2)
			numberOfTicks := 0
			testStillRunning := true
			for testStillRunning {
				select {
				case <-timer.C:
					testStillRunning = false
				case <-resetTimer.C:
					ticker.Reset(testCase.tickPeriod / 2)
				case <-ticker.C:
					numberOfTicks++
				}
			}
			expectedNumberOfTicks := float64(testCase.testDuration/time.Second) / 2 * testCase.dilationFactor
			expectedNumberOfTicks += float64(testCase.testDuration/time.Second) / 2 * testCase.dilationFactor * 2
			if int(expectedNumberOfTicks) != numberOfTicks {
				test.Errorf("Expected %v ticks but actually got %v", expectedNumberOfTicks, numberOfTicks)
			}
		})
	}
}

func TestSleep(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		sleepDuration  time.Duration
	}{
		{
			name:           "The-Test",
			dilationFactor: 2,
			sleepDuration:  10 * time.Second,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(test *testing.T) {
			test.Parallel()
			startTime := time.Now()
			clock := NewClock(testCase.dilationFactor, time.Now())
			clock.Sleep(testCase.sleepDuration)
			elapsedTime := time.Since(startTime)
			expectedElapsedTime := time.Duration(float64(testCase.sleepDuration) / testCase.dilationFactor)
			minimumAllowedElapsedTime := expectedElapsedTime - time.Millisecond
			maximumAllowedElapsedTime := expectedElapsedTime + time.Millisecond
			elapsedTimeIsOutsideOfTolerances := elapsedTime < minimumAllowedElapsedTime || elapsedTime > maximumAllowedElapsedTime
			if elapsedTimeIsOutsideOfTolerances {
				test.Errorf("Expected elapsed time to be %v +/- 1ms but actually got %v", expectedElapsedTime, elapsedTime)
			}
		})
	}
}

func TestTimers(test *testing.T) {
	test.Parallel()
	testCases := []struct {
		name           string
		dilationFactor float64
		timerDuration  time.Duration
		numberOfTimers int
	}{
		{
			name:           "The-Test",
			dilationFactor: 2,
			timerDuration:  time.Second,
		},
	}
	for _, testCase := range testCases {
		startTime := time.Now()
		clock := NewClock(testCase.dilationFactor, time.Now())
		timer := clock.NewTimer(testCase.timerDuration)
		dilatedTimerExpirationTime := <-timer.C
		realElapsedTime := time.Since(startTime)
		dilatedElapsedTime := dilatedTimerExpirationTime.Sub(startTime)
		expectedElapsedTime := testCase.timerDuration
		minimumAllowedElapsedTime := expectedElapsedTime - 2*time.Millisecond
		maximumAllowedElapsedTime := expectedElapsedTime + 2*time.Millisecond
		elapsedTimeNotInTolerance := dilatedElapsedTime < minimumAllowedElapsedTime || dilatedElapsedTime > maximumAllowedElapsedTime
		if elapsedTimeNotInTolerance {
			test.Errorf("Expected dilated elapsed time to be %v +/- 1ms but got %v", expectedElapsedTime, dilatedElapsedTime)
		}
		expectedElapsedTime = time.Duration(float64(testCase.timerDuration) / testCase.dilationFactor)
		minimumAllowedElapsedTime = expectedElapsedTime - time.Millisecond
		maximumAllowedElapsedTime = expectedElapsedTime + time.Millisecond
		elapsedTimeNotInTolerance = realElapsedTime < minimumAllowedElapsedTime || realElapsedTime > maximumAllowedElapsedTime
		if elapsedTimeNotInTolerance {
			test.Errorf("Expected real elapsed time to be %v +/- 1ms but got %v", expectedElapsedTime, realElapsedTime)
		}
	}
}

func ExampleNewClock() {
	startTime := time.Date(2025, 2, 27, 7, 0, 0, 0, time.Local)
	realStartTime := time.Now()
	clock := NewClock(7200, startTime)
	timeForWork := startTime.Add(2 * time.Hour)
	timeForLunch := timeForWork.Add(4 * time.Hour)
	timeToGoBackToWork := timeForLunch.Add(time.Hour)
	timeToGoHome := timeToGoBackToWork.Add(4 * time.Hour)
	timeToGoToSleep := timeToGoHome.Add(5 * time.Hour)
	timeToWakeUp := timeToGoToSleep.Add(8 * time.Hour)
	goToWorkTimer := clock.NewTimer(timeForWork.Sub(startTime))
	goToLunchTimer := clock.NewTimer(timeForLunch.Sub(startTime))
	goBackToWorkTimer := clock.NewTimer(timeToGoBackToWork.Sub(startTime))
	goHomeTimer := clock.NewTimer(timeToGoHome.Sub(startTime))
	goToSleepTimer := clock.NewTimer(timeToGoToSleep.Sub(startTime))
	wakeupTimer := clock.NewTimer(timeToWakeUp.Sub(startTime))
	fmt.Printf("The time Bob work up: %v\n", startTime.Format(time.DateTime))
	timeBobGotToWork := <-goToWorkTimer.C
	fmt.Printf("The time Bob got to work: %v\n", timeBobGotToWork.Format(time.Kitchen))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	timeBobWentToLunch := <-goToLunchTimer.C
	fmt.Printf("The time Bob went to lunch: %v\n", timeBobWentToLunch.Format(time.Kitchen))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	timeBobGotDoneWithLunch := <-goBackToWorkTimer.C
	fmt.Printf("The time Bob was done with lunch: %v\n", timeBobGotDoneWithLunch.Format(time.Kitchen))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	timeBobWentHome := <-goHomeTimer.C
	fmt.Printf("The time Bob went home: %v\n", timeBobWentHome.Format(time.Kitchen))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	timeBobWentToSleep := <-goToSleepTimer.C
	fmt.Printf("The time Bob went to sleep: %v\n", timeBobWentToSleep.Format(time.Kitchen))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	timeBobWokeUpTheNextDay := <-wakeupTimer.C
	fmt.Printf("The time Bob woke up the next day: %v\n", timeBobWokeUpTheNextDay.Format(time.DateTime))
	fmt.Printf("Real elapsed time: %.2fs\n", time.Since(realStartTime).Seconds())
	// Output:
	// The time Bob work up: 2025-02-27 07:00:00
	// The time Bob got to work: 9:00AM
	// Real elapsed time: 1.00s
	// The time Bob went to lunch: 1:00PM
	// Real elapsed time: 3.00s
	// The time Bob was done with lunch: 2:00PM
	// Real elapsed time: 3.50s
	// The time Bob went home: 6:00PM
	// Real elapsed time: 5.50s
	// The time Bob went to sleep: 11:00PM
	// Real elapsed time: 8.00s
	// The time Bob woke up the next day: 2025-02-28 07:00:00
	// Real elapsed time: 12.00s
}

func ExampleClock_NewTicker() {
	startTime := time.Now()
	timeDilationFactor := float64(time.Hour / time.Second)
	clock := NewClock(timeDilationFactor, startTime)
	dilatedTicker := clock.NewTicker(time.Second)
	normalTicker := time.NewTicker(time.Second)
	normalTimer := time.NewTimer(2*time.Second + time.Millisecond)
	numberOfNormalTicks := 0
	numberOfDilatedTicks := 0
	timerIsRunning := true
	for timerIsRunning {
		select {
		case <-normalTicker.C:
			numberOfNormalTicks++
		case <-dilatedTicker.C:
			numberOfDilatedTicks++
		case <-normalTimer.C:
			timerIsRunning = false
		}
	}
	fmt.Printf("Number of ticks from dilated ticker: %d\n", numberOfDilatedTicks)
	fmt.Printf("Number of ticks from normal ticker: %d\n", numberOfNormalTicks)
	// Output:
	// Number of ticks from dilated ticker: 3600
	// Number of ticks from normal ticker: 2
}
