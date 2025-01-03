package wadm

import (
	"context"
	"errors"
)

type API interface {
	// ModelList returns a list of models
	// wadm.api.{lattice-id}.model.get
	ModelList(ctx context.Context, req *ModelListRequest) (*ModelListResponse, error)
	// ModelGet returns a model by name and version
	// wadm.api.{lattice-id}.model.get.{name}
	ModelGet(ctx context.Context, req *ModelGetRequest) (*ModelGetResponse, error)
	// ModelStatus returns the status of a model
	// wadm.api.{lattice-id}.model.status.{name}
	ModelStatus(ctx context.Context, req *ModelStatusRequest) (*ModelStatusResponse, error)
	// ModelPut creates or updates a model
	// wadm.api.{lattice-id}.model.put
	ModelPut(ctx context.Context, req *ModelPutRequest) (*ModelPutResponse, error)
	// ModelDelete deletes a model
	// wadm.api.{lattice-id}.model.delete.{name}
	ModelDelete(ctx context.Context, req *ModelDeleteRequest) (*ModelDeleteResponse, error)

	// ModelDeploy deploys a model
	// wadm.api.{lattice-id}.model.deploy.{name}
	ModelDeploy(ctx context.Context, req *ModelDeployRequest) (*ModelDeployResponse, error)
	// ModelUndeploy undeploys a model
	// wadm.api.{lattice-id}.model.undeploy.{name}
	ModelUndeploy(ctx context.Context, req *ModelUndeployRequest) (*ModelUndeployResponse, error)
}

var ErrEncode = errors.New("encode error")
var ErrInternal = errors.New("internal error")
var ErrDecode = errors.New("decode error")
var ErrTransport = errors.New("transport error")
var ErrOperation = errors.New("operation error")
var ErrValidation = errors.New("validation error")

const (
	VersionAnnotation = "version"
)
