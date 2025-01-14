package lattice

import (
	"context"
	"time"

	"go.wasmcloud.dev/x/wasmbus"
	"go.wasmcloud.dev/x/wasmbus/events"
)

type Component struct {
	Id string
}

type Host struct {
	Id        string
	FirstSeen time.Time
	LastSeen  time.Time
}

type Cache struct {
	Lattice string
	Bus     wasmbus.Bus
}

func (c *Cache) NeedLeaderElection() bool {
	return false
}

func (c *Cache) Start(ctx context.Context) error {
	evSubscription, err := events.Subscribe(c.Bus, c.Lattice, wasmbus.PatternAll, wasmbus.NoBackLog, c)
	if err != nil {
		return err
	}

	<-ctx.Done()

	if err := evSubscription.Drain(); err != nil {
		return err
	}

	return nil
}

func (c *Cache) HandleEvent(context.Context, events.Event)            {}
func (c *Cache) HandleError(context.Context, *wasmbus.Message, error) {}
