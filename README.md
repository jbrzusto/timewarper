# Timewarper
A package that provides a clock and other mechanisms for warping and manipulating time.

```Go
package main

import (
	"fmt"
	"github.com/SnugglyCoder/timewarper"
	"time"
)

func main() {
	startTime := time.Now()
	timeDilationFactor := float64(time.Hour / time.Second)
	timewarperClock := timewarper.NewClock(timeDilationFactor, startTime)
	time.Sleep(time.Second)
	dilatedTimeSinceStart := timewarperClock.Now().Sub(startTime)
	fmt.Printf("Dilated time since start: %v\n", dilatedTimeSinceStart)
	fmt.Printf("Actual time since startg: %v\n", time.Since(startTime))
}
```

It also provides for the creation of timers that provide the same time dilation properties.

```Go
package main

import (
	"fmt"
	"github.com/SnugglyCoder/timewarper"
	"time"
)

func main() {
	startTime := time.Now()
	timeDilationFactor := float64(time.Hour / time.Second)
	timewarperClock := timewarper.NewClock(timeDilationFactor, startTime)
	dilatedExpirationTime := <-timewarperClock.After(time.Hour)
	fmt.Printf("Dilated time since start: %v\n", dilatedExpirationTime.Sub(startTime))
	fmt.Printf("Actual time since startg: %v\n", time.Since(startTime))
}
```

Tickers are also provided

```Go
package main

import (
	"fmt"
	"github.com/SnugglyCoder/timewarper"
	"time"
)

func main() {
	startTime := time.Now()
	timeDilationFactor := float64(time.Hour / time.Second)
	timewarperClock := timewarper.NewClock(timeDilationFactor, startTime)
	dilatedTicker := timewarperClock.NewTicker(time.Second)
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
	fmt.Printf("Time since start: %.2f\n", time.Since(startTime).Seconds())
}
```

# Notes
The dilation is not perfect and suffers from rounding errors and also begins to depend on the speed of the machine running the program.
Consider the examples above, where the dilation factor is 7600.
An hour is being compressed into a second, so a second according to that clock is really 131,578 nanoseconds.

Something to note to give perspective, if you were to be travelling 99.99% for one minute at the speed of light, about 1 hour and 10 minutes would have elapsed on Earth.
That is a dilation ratio of 1:70, and these examples above are 1:7600.
The more fine grained accuracy you need, the lower a dilation factor you can use with your timewarper clock.