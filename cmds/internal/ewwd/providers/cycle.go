package providers

import (
	"context"
	"time"
)

type Cycle struct {
	state     StateSetter
	name      string
	interval  time.Duration
	done      chan struct{}
	active    bool
	last      int
	published bool
}

func NewCycle(state StateSetter, name string, interval time.Duration) Provider {
	return &Cycle{
		state:    state,
		name:     name,
		interval: interval,
		done:     make(chan struct{}),
	}
}

func (c *Cycle) Name() string {
	return c.name
}

func (c *Cycle) Start(ctx context.Context, notify func(data any)) error {
	c.active = true
	c.publish(notify, time.Now())

	for waitForNextBoundary(ctx, c.done, c.interval) {
		c.publish(notify, time.Now())
	}
	return nil
}

func (c *Cycle) Stop() error {
	if c.active {
		close(c.done)
		c.active = false
	}
	return nil
}

func (c *Cycle) publish(notify func(data any), now time.Time) {
	value := int(now.UnixNano()/c.interval.Nanoseconds()) % 2
	if c.published && c.last == value {
		return
	}
	c.last = value
	c.published = true
	c.state.Set(c.name, value)
	notify(value)
}
