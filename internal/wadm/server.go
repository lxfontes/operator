package wadm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
)

type Server struct {
	lattice       string
	nc            *nats.Conn
	handler       API
	subscriptions []*nats.Subscription
}

func NewServer(nc *nats.Conn, lattice string, handler API) *Server {
	return &Server{
		lattice: lattice,
		handler: handler,
		nc:      nc,
	}
}

func (s *Server) Drain() error {
	var errs []error //nolint:prealloc

	for _, sub := range s.subscriptions {
		errs = append(errs, sub.Drain())
	}
	s.subscriptions = nil

	return errors.Join(errs...)
}

func (s *Server) Handle(ctx context.Context) error {
	listLegacyCb := nakedHandler[ModelListRequest, ModelListResponse]{
		handler: s.handler.ModelList,
		ctx:     ctx,
		// NOTE(lxf): We need to transform the response to match the legacy API. Still used in 'wash' CLI.
		respTransformer: func(resp *ModelListResponse) any {
			for i := range resp.Models {
				resp.Models[i].Status = resp.Models[i].DetailedStatus.Info.Type
			}

			return resp.Models
		},
	}
	if err := s.registerHandler(
		s.subject("model.list"), listLegacyCb.handle); err != nil {
		return err
	}

	listCb := nakedHandler[ModelListRequest, ModelListResponse]{
		handler: s.handler.ModelList,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.get"), listCb.handle); err != nil {
		return err
	}

	putCb := nakedHandler[ModelPutRequest, ModelPutResponse]{
		handler: s.handler.ModelPut,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.put"), putCb.handle); err != nil {
		return err
	}

	getCb := namedHandler[ModelGetRequest, ModelGetResponse]{
		handler: s.handler.ModelGet,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.get.*"), getCb.handle); err != nil {
		return err
	}

	statusCb := namedHandler[ModelStatusRequest, ModelStatusResponse]{
		handler: s.handler.ModelStatus,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.status.*"), statusCb.handle); err != nil {
		return err
	}

	delCb := namedHandler[ModelDeleteRequest, ModelDeleteResponse]{
		handler: s.handler.ModelDelete,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.del.*"), delCb.handle); err != nil {
		return err
	}

	deployCb := namedHandler[ModelDeployRequest, ModelDeployResponse]{
		handler: s.handler.ModelDeploy,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.deploy.*"), deployCb.handle); err != nil {
		return err
	}

	undeployCb := namedHandler[ModelUndeployRequest, ModelUndeployResponse]{
		handler: s.handler.ModelUndeploy,
		ctx:     ctx,
	}
	if err := s.registerHandler(
		s.subject("model.undeploy.*"), undeployCb.handle); err != nil {
		return err
	}

	return nil
}

type namedRequest interface {
	ModelStatusRequest | ModelDeployRequest | ModelUndeployRequest | ModelDeleteRequest | ModelGetRequest
}

type nameSetter interface {
	setName(string)
}

type namedHandler[X namedRequest, Y any] struct {
	handler func(ctx context.Context, req *X) (*Y, error)
	ctx     context.Context
}

func (s *namedHandler[X, Y]) handle(reqMsg *nats.Msg) {
	var req X
	any(&req).(nameSetter).setName(lastSubjectPart(reqMsg.Subject))
	if len(reqMsg.Data) > 0 {
		if err := json.Unmarshal(reqMsg.Data, &req); err != nil {
			respondError(reqMsg, err)
			return
		}
	}

	resp, err := s.handler(s.ctx, &req)
	if err != nil {
		respondError(reqMsg, err)
		return
	}

	respMsg := nats.NewMsg(reqMsg.Reply)
	respMsg.Data, err = json.Marshal(resp)
	if err != nil {
		respondError(reqMsg, err)
		return
	}

	if err := reqMsg.RespondMsg(respMsg); err != nil {
		fmt.Println(err)
		return
	}
}

type nakedRequest interface {
	ModelListRequest | ModelPutRequest
}

type nakedHandler[X nakedRequest, Y any] struct {
	handler         func(ctx context.Context, req *X) (*Y, error)
	respTransformer func(*Y) any
	ctx             context.Context
}

func (s *nakedHandler[X, Y]) handle(reqMsg *nats.Msg) {
	var req X

	if len(reqMsg.Data) > 0 {
		contentType := reqMsg.Header.Get("Content-Type")
		switch contentType {
		case "application/json", "":
			if err := json.Unmarshal(reqMsg.Data, &req); err != nil {
				respondError(reqMsg, err)
				return
			}
		case "application/yaml":
			if err := yaml.Unmarshal(reqMsg.Data, &req); err != nil {
				respondError(reqMsg, err)
				return
			}
		default:
			respondError(reqMsg, errors.New("unsupported content type"))
		}
	}

	resp, err := s.handler(s.ctx, &req)
	if err != nil {
		respondError(reqMsg, err)
		return
	}

	var jsonRep any
	jsonRep = resp

	if s.respTransformer != nil {
		jsonRep = s.respTransformer(resp)
	}

	respMsg := nats.NewMsg(reqMsg.Reply)
	respMsg.Data, err = json.Marshal(jsonRep)
	if err != nil {
		respondError(reqMsg, err)
		return
	}

	if err := reqMsg.RespondMsg(respMsg); err != nil {
		fmt.Println(err)
		return
	}
}

func (s *Server) registerHandler(subject string, handler func(*nats.Msg)) error {
	sub, err := s.nc.Subscribe(subject, handler)
	if err != nil {
		return err
	}
	s.subscriptions = append(s.subscriptions, sub)

	return nil
}

func (s *Server) subject(id string) string {
	return fmt.Sprintf("wadm.api.%s.%s", s.lattice, id)
}

func lastSubjectPart(subject string) string {
	parts := strings.Split(subject, ".")
	return parts[len(parts)-1]
}

func respondError(reqMsg *nats.Msg, err error) {
	fmt.Println("ERROR", err)

	respMsg := nats.NewMsg(reqMsg.Reply)

	respErr := Error{
		Result:  "error",
		Message: err.Error(),
	}

	respMsg.Data, err = json.Marshal(&respErr)
	if err != nil {
		return
	}

	if err := reqMsg.RespondMsg(respMsg); err != nil {
		return
	}
}
