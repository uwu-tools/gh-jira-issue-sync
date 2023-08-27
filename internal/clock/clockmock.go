package clock

import (
	"time"
)

type clockMock struct {
}

func NewClockMock() Clock {
	return &clockMock{}
}

func (c *clockMock) Now() time.Time {
	return time.Date(1996, 8, 1, 0, 0, 0, 0, time.FixedZone("CEST", 7200))
}
