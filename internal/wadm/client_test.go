package wadm

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func checkWadm() error {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		return err
	}
	defer nc.Close()

	c := NewClient(nc, "default")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = c.ModelList(ctx)
	return err
}

func withWash(t *testing.T, fn func(c *nats.Conn)) {
	t.Helper()

	if err := exec.Command("wash", "up", "-d").Run(); err != nil {
		t.Fatalf("failed to start wash: %v", err)
	}

	defer func() {
		_ = exec.Command("wash", "down").Run()
	}()

	maxTimeout := time.After(10 * time.Second)
	connected := false
	for !connected {
		select {
		case <-maxTimeout:
			t.Fatalf("timeout waiting for wash to start")
			return
		case <-time.After(1 * time.Second):
			if err := checkWadm(); err == nil {
				connected = true
				continue
			} else {
				t.Logf("waiting for wash to start: %v", err)
			}
		}
	}

	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer nc.Close()
	fn(nc)
}

func TestModelPut(t *testing.T) {
	withWash(t, func(nc *nats.Conn) {
		c := NewClient(nc, "default")
		manifest, err := LoadManifest("./fixtures/valid_manifest.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resp, err := c.ModelPut(context.TODO(), manifest)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if want, got := false, resp.IsError(); got != want {
			t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
		}
	})
}
