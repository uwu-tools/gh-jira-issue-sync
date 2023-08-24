package clock

import "time"

type realClock struct {
}

func NewRealClock() Clock {
	return &realClock{}
}

func (r *realClock) Now() time.Time {
	return time.Now()
}
