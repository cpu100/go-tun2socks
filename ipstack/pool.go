package ipstack

import (
    "sync"
    "time"
)

var timerPool sync.Pool

func poolGetTimer(d time.Duration) *time.Timer {
    if value := timerPool.Get(); nil == value {
        return time.NewTimer(d)
    } else {
        timer := value.(*time.Timer)
        timer.Reset(d)
        return timer
    }
}

func poolPutTimer(timer *time.Timer, stop bool) {
    if stop && !timer.Stop() {
        <-timer.C
    }
    timerPool.Put(timer)
}


