package wadm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
)

const (
	LatestVersion = ""
)

type API interface {
	// wadm.api.{lattice-id}.model.get
	ModelList(ctx context.Context) (*ModelListResponse, error)
	// wadm.api.{lattice-id}.model.get.{name}
	ModelGet(ctx context.Context, name string, version string) (*ModelGetResponse, error)
	// wadm.api.{lattice-id}.model.status.{name}
	ModelStatus(ctx context.Context, name string) (*ModelStatusResponse, error)
	// wadm.api.{lattice-id}.model.put
	ModelPut(ctx context.Context, model *Manifest) (*ModelPutResponse, error)
	// wadm.api.{lattice-id}.model.delete.{name}
	ModelDelete(ctx context.Context, name string, version string) (*ModelDeleteResponse, error)

	// wadm.api.{lattice-id}.model.deploy.{name}
	ModelDeploy(ctx context.Context, name string, version string) (*ModelDeployResponse, error)
	// wadm.api.{lattice-id}.model.undeploy.{name}
	ModelUndeploy(ctx context.Context, name string) (*ModelUndeployResponse, error)
}

var _ API = (*Client)(nil)

type Client struct {
	nc      *nats.Conn
	lattice string
}

func NewClient(nc *nats.Conn, lattice string) *Client {
	return &Client{
		nc:      nc,
		lattice: lattice,
	}
}

func ParseManifest(data []byte) (*Manifest, error) {
	manifest := &Manifest{}
	// try to unmarshal as json first, as it errors out faster
	if err := json.Unmarshal(data, manifest); err == nil {
		return manifest, nil
	}

	return manifest, yaml.Unmarshal(data, manifest)
}

func LoadManifest(path string) (*Manifest, error) {
	rawManifest, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseManifest(rawManifest)
}

func (c *Client) ModelStatus(ctx context.Context, name string) (*ModelStatusResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "status", name),
		&ModelStatusRequest{})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelStatusResponse{})
}

func (c *Client) ModelPut(ctx context.Context, model *Manifest) (*ModelPutResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "put"),
		&ModelPutRequest{
			Manifest: model,
		})
	if err != nil {
		return nil, err
	}
	msg.Header.Set("Content-Type", "application/json")
	return runRequest(ctx, c.nc, msg, &ModelPutResponse{})
}

func (c *Client) ModelGet(ctx context.Context, name string, version string) (*ModelGetResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "get", name),
		&ModelDeployRequest{
			Version: version,
		})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelGetResponse{})
}

func (c *Client) ModelDelete(ctx context.Context, name string, version string) (*ModelDeleteResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "delete", name),
		&ModelDeployRequest{
			Version: version,
		})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelDeleteResponse{})
}

func (c *Client) ModelDeploy(ctx context.Context, name string, version string) (*ModelDeployResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "deploy", name),
		&ModelDeployRequest{
			Version: version,
		})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelDeployResponse{})
}

func (c *Client) ModelUndeploy(ctx context.Context, name string) (*ModelUndeployResponse, error) {
	msg, err := c.newRequest(c.subject("model", "undeploy"), &ModelUndeployRequest{})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelUndeployResponse{})
}

func (c *Client) ModelList(ctx context.Context) (*ModelListResponse, error) {
	msg, err := c.newRequest(c.subject("model", "get"), &ModelListRequest{})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelListResponse{})
}

func (c *Client) subject(ids ...string) string {
	parts := append([]string{"wadm", "api", c.lattice}, ids...)
	return strings.Join(parts, ".")
}

func (c *Client) newRequest(subject string, payload any) (*nats.Msg, error) {
	natsMsg := nats.NewMsg(subject)

	if payload != nil {
		rawReq, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInternal, err)
		}
		natsMsg.Data = rawReq
	}

	return natsMsg, nil
}

func runRequest[T any](ctx context.Context, nc *nats.Conn, msg *nats.Msg, resp *T) (*T, error) {
	rawResp, err := nc.RequestMsgWithContext(ctx, msg)
	if err != nil {
		return resp, fmt.Errorf("%w: %s", ErrTransport, err)
	}

	if err := json.Unmarshal(rawResp.Data, resp); err != nil {
		return resp, fmt.Errorf("%w: %s", ErrDecode, err)
	}

	return resp, nil
}
