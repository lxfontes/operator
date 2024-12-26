package wadm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
)

var _ API = (*Client)(nil)

type Client struct {
	nc      *nats.Conn
	lattice string
}

// NewClient creates a new wadm client, using the provided nats connection and lattice id (nats prefix)
func NewClient(nc *nats.Conn, lattice string) *Client {
	return &Client{
		nc:      nc,
		lattice: lattice,
	}
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
		c.subject("model", "del", name),
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
	msg, err := c.newRequest(c.subject("model", "undeploy", name), &ModelUndeployRequest{})
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
