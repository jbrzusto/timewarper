package timewarper

import (
	"golang.org/x/sync/errgroup"
	"math/rand/v2"
	"testing"
	"time"
)

func TestStaticDilation(test *testing.T) {
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
	// TODO: Add tests that vary how long they sit at each dilation factor
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

func TestTimeJump(test *testing.T) {
	testCases := []struct {
		name           string
		distanceToJump time.Duration
	}{
		{
			name:           "Jump Forward",
			distanceToJump: 1 * time.Hour,
		},
		{
			name:           "Jump Backward",
			distanceToJump: -1 * time.Hour,
		},
	}
	for _, testCase := range testCases {
		test.Run(testCase.name, func(subTest *testing.T) {
			subTest.Parallel()

		})
	}
}
