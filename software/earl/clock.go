package main

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (c RealClock) Now() time.Time {
	return time.Now()
}

type MockClock struct {
	now time.Time
}

func (c MockClock) Now() time.Time {
	return c.now
}
