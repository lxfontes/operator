package wadm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

var testDataPath = path.Join(".", "testdata")

const helloComponent = "ghcr.io/wasmcloud/components/http-hello-world-rust:0.1.0"

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

func testExec(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	// NOTE(lxf): Uncomment the following lines to see the output of wash commands
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	return cmd.Run()
}

func washDeploy(t *testing.T, path string) {
	t.Helper()
	if err := testExec("wash", "app", "deploy", path); err != nil {
		t.Fatalf("failed to deploy manifest: %v", err)
	}
}

func withWash(t *testing.T) (*nats.Conn, func(*testing.T)) {
	t.Helper()

	if err := testExec("wash", "up", "-d"); err != nil {
		t.Fatalf("failed to start wash: %v", err)
		return nil, nil
	}

	maxTimeout := time.After(10 * time.Second)
	connected := false
	for !connected {
		select {
		case <-maxTimeout:
			t.Fatalf("timeout waiting for wash to start")
			return nil, nil
		case <-time.After(250 * time.Millisecond):
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
		t.Fatalf("failed to connect to nats: %v", err)
		return nil, nil
	}

	return nc, func(*testing.T) {
		nc.Close()
		_ = testExec("wash", "down", "--purge-jetstream", "all")
	}
}

func createApp(c *Client, name string) error {
	manifest := newAppManifest(name)
	resp, err := c.ModelPut(context.TODO(), manifest)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("error creating app: %v", resp.Message)
	}

	return nil
}

func newAppManifest(name string) *Manifest {
	metadata := ManifestMetadata{
		Name: name,
		Annotations: map[string]string{
			"description": "test app",
		},
	}
	spec := ManifestSpec{
		Components: []Component{
			{
				Name: "hello",
				Type: ComponentTypeComponent,
				Properties: ComponentProperties{
					Image: helloComponent,
				},
			},
		},
	}
	return &Manifest{
		ApiVersion: DefaultManifestApiVersion,
		Kind:       DefaultManifestKind,
		Metadata:   metadata,
		Spec:       spec,
	}
}

func loadFixture(filePath string) ([]byte, error) {
	fullPath := path.Join(testDataPath, filePath)
	return os.ReadFile(fullPath)
}

func listFixtures(filePath string, pattern string) []string {
	files, err := filepath.Glob(path.Join(testDataPath, filePath, pattern))
	if err != nil {
		panic(err)
	}
	for i, file := range files {
		files[i] = strings.TrimPrefix(file, testDataPath+"/")
	}

	return files
}
