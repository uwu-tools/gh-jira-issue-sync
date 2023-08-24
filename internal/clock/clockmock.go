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
	return time.Unix(838850400, 0)
}
