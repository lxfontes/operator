package wadm

import (
	"context"
	"encoding/json"
	"fmt"

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

func (c *Client) ModelStatus(ctx context.Context, req *ModelStatusRequest) (*ModelStatusResponse, error) {
	msg, err := c.newRequest(
		c.subject("model", "status", req.Name),
		req)
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelStatusResponse{})
}

func (c *Client) ModelPut(ctx context.Context, req *ModelPutRequest) (*ModelPutResponse, error) {
	msg, err := c.newRequest(c.subject("model", "put"), req)
	if err != nil {
		return nil, err
	}
	msg.Header.Set("Content-Type", "application/json")
	return runRequest(ctx, c.nc, msg, &ModelPutResponse{})
}

func (c *Client) ModelGet(ctx context.Context, req *ModelGetRequest) (*ModelGetResponse, error) {
	msg, err := c.newRequest(c.subject("model", "get", req.Name), req)
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelGetResponse{})
}

func (c *Client) ModelDelete(ctx context.Context, req *ModelDeleteRequest) (*ModelDeleteResponse, error) {
	msg, err := c.newRequest(c.subject("model", "del", req.Name), req)
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelDeleteResponse{})
}

func (c *Client) ModelDeploy(ctx context.Context, req *ModelDeployRequest) (*ModelDeployResponse, error) {
	msg, err := c.newRequest(c.subject("model", "deploy", req.Name), req)
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelDeployResponse{})
}

func (c *Client) ModelUndeploy(ctx context.Context, req *ModelUndeployRequest) (*ModelUndeployResponse, error) {
	msg, err := c.newRequest(c.subject("model", "undeploy", req.Name), &ModelUndeployRequest{})
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelUndeployResponse{})
}

func (c *Client) ModelList(ctx context.Context, req *ModelListRequest) (*ModelListResponse, error) {
	msg, err := c.newRequest(c.subject("model", "get"), req)
	if err != nil {
		return nil, err
	}
	return runRequest(ctx, c.nc, msg, &ModelListResponse{})
}

func (c *Client) subject(ids ...string) string {
	parts := append([]string{c.lattice}, ids...)
	return APISubject(parts...)
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
