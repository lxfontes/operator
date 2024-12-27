package wadm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type testServer struct{}

func (t *testServer) ModelList(ctx context.Context, req *ModelListRequest) (*ModelListResponse, error) {
	models := []ModelSummary{
		{
			Name:    "model1",
			Version: "v1",
			DetailedStatus: &DetailedStatus{
				Info: StatusInfo{
					Type: StatusTypeDeployed,
				},
			},
		},
	}

	return &ModelListResponse{
		Result:  GetResultSuccess,
		Message: "Model List follows",
		Models:  models,
	}, nil
}

func (t *testServer) ModelGet(ctx context.Context, req *ModelGetRequest) (*ModelGetResponse, error) {
	return &ModelGetResponse{
		Result:  GetResultSuccess,
		Message: "Model Get follows",
		Manifest: &Manifest{
			Metadata: ManifestMetadata{
				Name:        req.Name,
				Annotations: map[string]string{},
			},
			Spec: ManifestSpec{
				Components: []Component{
					{
						Name: "component1",
						Type: ComponentTypeComponent,
					},
				},
			},
		},
	}, nil
}

func (t *testServer) ModelStatus(ctx context.Context, req *ModelStatusRequest) (*ModelStatusResponse, error) {
	return &ModelStatusResponse{
		Result:  StatusResultOk,
		Message: "Model Status follows",
		Status: &Status{
			Status: StatusInfo{
				Type: StatusTypeDeployed,
			},
		},
	}, nil
}

func (t *testServer) ModelPut(ctx context.Context, req *ModelPutRequest) (*ModelPutResponse, error) {
	fmt.Printf("%+v\n", req)

	return &ModelPutResponse{
		Result:  PutResultCreated,
		Message: "Model Created",
		Name:    req.Metadata.Name,
	}, nil
}

func (t *testServer) ModelDelete(ctx context.Context, req *ModelDeleteRequest) (*ModelDeleteResponse, error) {
	return &ModelDeleteResponse{
		Result:  DeleteResultDeleted,
		Message: "Model Deleted",
	}, nil
}

func (t *testServer) ModelDeploy(ctx context.Context, req *ModelDeployRequest) (*ModelDeployResponse, error) {
	return &ModelDeployResponse{
		Result:  DeployResultAcknowledged,
		Name:    req.Name,
		Version: "xyz",
	}, nil
}

func (t *testServer) ModelUndeploy(ctx context.Context, req *ModelUndeployRequest) (*ModelUndeployResponse, error) {
	return &ModelUndeployResponse{
		Result: DeployResultAcknowledged,
	}, nil
}

func TestServer(t *testing.T) {
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Fatalf("failed to connect to nats: %v", err)
	}
	s := NewServer(nc, "default", &testServer{})
	if err := s.Handle(context.Background()); err != nil {
		t.Fatalf("failed to handle server: %v", err)
	}
	<-time.After(5 * time.Minute)
}
