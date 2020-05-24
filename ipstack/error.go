package ipstack

type errTimeout struct {
}

func (e *errTimeout) Error() string {
    return "read timeout"
}

func (e *errTimeout) Timeout() bool {
    return true
}

func (e *errTimeout) Temporary() bool {
    return true
}

//
var ErrTimeout = new(errTimeout)
