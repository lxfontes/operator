package wasmbus

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"go.wasmcloud.dev/operator/internal/wasmbus/events"
)

func TestEventSubscription(t *testing.T) {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Fatal(err)
	}

	c := NewClient(nc, "default")

	s, err := c.EventSubscribe(PatternAll, NoBackLog)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	s.Handle(ctx, func(ctx context.Context, data []byte, ev events.Event, err error) {
		if err != nil {
			t.Log(err)
			return
		}
		switch bv := ev.BusEvent.(type) {
		// case *events.HostHeartbeat:
		// 	t.Logf("Host Heartbeat %+v", bv)
		// case *events.HealthCheckStatus:
		// 	t.Logf("Health Check Status %+v", bv)
		case *events.LinkDefSet:
			t.Logf("Set %+v", bv)
		case *events.LinkDefDeleted:
			t.Logf("Delete %+v", bv)
			// default:
			// 	t.Log("Unknown event type", ev.CloudEvent.Type())
		}
	})
	t.Log("exiting")
}
