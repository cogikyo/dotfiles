package providers

import (
	"context"
	"time"
)

type ClockState struct {
	Hour   string `json:"hour"`
	Minute string `json:"minute"`
	Second string `json:"second"`
}

type Clock struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	last      ClockState
	published bool
}

func NewClock(state StateSetter) Provider {
	return &Clock{state: state, done: make(chan struct{})}
}

func (c *Clock) Name() string {
	return "clock"
}

func (c *Clock) Start(ctx context.Context, notify func(data any)) error {
	c.active = true
	c.publish(notify, time.Now())

	for waitForNextBoundary(ctx, c.done, time.Second) {
		c.publish(notify, time.Now())
	}
	return nil
}

func (c *Clock) Stop() error {
	if c.active {
		close(c.done)
		c.active = false
	}
	return nil
}

func (c *Clock) publish(notify func(data any), now time.Time) {
	state := ClockState{
		Hour:   now.Format("15"),
		Minute: now.Format("04"),
		Second: now.Format("05"),
	}
	if c.published && c.last == state {
		return
	}
	c.last = state
	c.published = true
	c.state.Set("clock", &state)
	notify(&state)
}
