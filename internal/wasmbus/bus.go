package wasmbus

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"go.wasmcloud.dev/operator/internal/wasmbus/events"
)

type Client struct {
	nc      *nats.Conn
	lattice string
}

type EventCallback func(context.Context, []byte, events.Event, error)

const PatternAll = "*"
const NoBackLog = 0

func NewClient(nc *nats.Conn, lattice string) *Client {
	return &Client{
		nc:      nc,
		lattice: lattice,
	}
}

type Subscription struct {
	ch chan *nats.Msg
	ns *nats.Subscription
}

func (s *Subscription) Handle(ctx context.Context, callback EventCallback) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-s.ch:
			if !ok {
				return
			}
			ce, err := events.ParseEvent(msg.Data)
			callback(ctx, msg.Data, ce, err)
		}
	}
}

func (s *Subscription) Drain() error {
	err := s.ns.Drain()
	close(s.ch)
	return err
}

func (c *Client) EventSubscribe(pattern string, backlog int) (*Subscription, error) {
	subject := fmt.Sprintf("wasmbus.evt.%s.%s", c.lattice, pattern)
	ch := make(chan *nats.Msg, backlog)
	sub, err := c.nc.ChanSubscribe(subject, ch)
	if err != nil {
		return nil, err
	}

	return &Subscription{
			ch: ch,
			ns: sub,
		},
		nil
}
