package wadm

import (
	"context"
	"testing"
)

func TestModelList(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)

	c := NewClient(nc, "default")

	if err := createApp(c, "test-list-1"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	if err := createApp(c, "test-list-2"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	if err := createApp(c, "test-list-3"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelList(context.TODO(), &ModelListRequest{})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}
func TestModelGet(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)
	c := NewClient(nc, "default")

	if err := createApp(c, "test-get"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelGet(context.TODO(), &ModelGetRequest{Name: "test-get", Version: LatestVersion})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.Result.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}

func TestModelStatus(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)
	c := NewClient(nc, "default")

	if err := createApp(c, "test-status"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelStatus(context.TODO(), &ModelStatusRequest{Name: "test-status"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.Result.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}

func TestModelPut(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)

	c := NewClient(nc, "default")
	manifest := newAppManifest("test-put")
	resp, err := c.ModelPut(context.TODO(), &ModelPutRequest{Manifest: *manifest})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}

func TestModelDelete(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)
	c := NewClient(nc, "default")

	if err := createApp(c, "test-delete"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelDelete(context.TODO(), &ModelDeleteRequest{Name: "test-delete", Version: LatestVersion})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.Result.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}

func TestModelDeploy(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)
	c := NewClient(nc, "default")

	if err := createApp(c, "test-deploy"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelDeploy(context.TODO(), &ModelDeployRequest{Name: "test-deploy", Version: LatestVersion})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.Result.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}

func TestModelUndeploy(t *testing.T) {
	nc, teardown := withWash(t)
	defer teardown(t)
	c := NewClient(nc, "default")

	if err := createApp(c, "test-undeploy"); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	resp, err := c.ModelUndeploy(context.TODO(), &ModelUndeployRequest{Name: "test-undeploy"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if want, got := false, resp.Result.IsError(); got != want {
		t.Fatalf("want %v, got %v: %v", want, got, resp.Message)
	}
}
