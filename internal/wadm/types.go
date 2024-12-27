package wadm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// API structure
// wadm.api.{lattice-id}.{category}.{operation}.{object}

type (
	ComponentType string
	TraitType     string
	StatusType    string

	StatusResult string
	DeployResult string
	DeleteResult string
	GetResult    string
	PutResult    string
)

func (r GetResult) IsError() bool {
	return r != GetResultSuccess
}

func (r StatusResult) IsError() bool {
	return r != StatusResultOk
}

func (r DeleteResult) IsError() bool {
	return r == DeleteResultError
}

func (r DeployResult) IsError() bool {
	return r != DeployResultAcknowledged
}

func APISubject(ids ...string) string {
	parts := append([]string{"wadm", "api"}, ids...)
	return strings.Join(parts, ".")
}

const (
	ComponentTypeComponent  ComponentType = "component"
	ComponentTypeCapability ComponentType = "capability"

	TraitTypeLink         TraitType = "link"
	TraitTypeSpreadScaler TraitType = "spreadscaler"
	TraitTypeDaemonScaler TraitType = "daemonscaler"

	StatusTypeWaiting     StatusType = "waiting"
	StatusTypeUndeployed  StatusType = "undeployed"
	StatusTypeReconciling StatusType = "reconciling"
	StatusTypeDeployed    StatusType = "deployed"
	StatusTypeFailed      StatusType = "failed"

	StatusResultError StatusResult = "error"
	// NOTE(lxf): inconsistency (should be succcess) ?
	StatusResultOk       StatusResult = "ok"
	StatusResultNotFound StatusResult = "notfound"

	DeployResultError        DeployResult = "error"
	DeployResultAcknowledged DeployResult = "acknowledged"
	DeployResultNotFound     DeployResult = "notfound"

	DeleteResultError   DeleteResult = "error"
	DeleteResultNoop    DeleteResult = "noop"
	DeleteResultDeleted DeleteResult = "deleted"

	GetResultError    GetResult = "error"
	GetResultSuccess  GetResult = "success"
	GetResultNotFound GetResult = "not_found"

	PutResultError      PutResult = "error"
	PutResultCreated    PutResult = "created"
	PutResultNewVersion PutResult = "newversion"

	DefaultManifestApiVersion string = "core.oam.dev/v1beta1"
	DefaultManifestKind       string = "Manifest"

	// LatestVersion is a constant that represents the latest version of a model
	LatestVersion = ""
)

// RawMessage knows how to stash json & yaml
type RawMessage []byte

func (m RawMessage) MarshalJSON() ([]byte, error) { return m.marshal() }
func (m RawMessage) MarshalYAML() ([]byte, error) { return m.marshal() }

func (m RawMessage) marshal() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

func (m *RawMessage) UnmarshalJSON(data []byte) error { return m.unmarshal(data) }
func (m *RawMessage) UnmarshalYAML(data []byte) error { return m.unmarshal(data) }

func (m *RawMessage) unmarshal(data []byte) error {
	if m == nil {
		return errors.New("RawMessage: unmarshal on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}

type Error struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

type Status struct {
	Status  StatusInfo     `json:"status"`
	Scalers []ScalerStatus `json:"scalers,omitempty"`
}

type ModelStatusRequest struct {
	Name string `json:"name"`
}

func (m *ModelStatusRequest) setName(name string) {
	m.Name = name
}

type ModelStatusResponse struct {
	Result  StatusResult `json:"result"`
	Message string       `json:"message"`
	Status  *Status      `json:"status,omitempty"`
}

type ModelPutRequest struct {
	Manifest `json:",inline"`
}

type ModelPutResponse struct {
	Name           string    `json:"name,omitempty"`
	TotalVersions  int       `json:"total_versions,omitempty"`
	CurrentVersion string    `json:"current_version,omitempty"`
	Message        string    `json:"message,omitempty"`
	Result         PutResult `json:"result"`
}

func (m *ModelPutResponse) IsError() bool {
	return m.Result == PutResultError
}

type StatusInfo struct {
	Type    StatusType `json:"type"`
	Message string     `json:"message,omitempty"`
}

type ScalerStatus struct {
	Id     string     `json:"id"`
	Kind   string     `json:"kind"`
	Name   string     `json:"name"`
	Status StatusInfo `json:"status"`
}

type DetailedStatus struct {
	Info    StatusInfo     `json:"status"`
	Scalers []ScalerStatus `json:"scalers,omitempty"`
}

type ModelSummary struct {
	Name            string          `json:"name"`
	Version         string          `json:"version"`
	Description     string          `json:"description,omitempty"`
	DeployedVersion string          `json:"deployed_version,omitempty"`
	DetailedStatus  *DetailedStatus `json:"detailed_status,omitempty"`

	// Deprecated
	Status StatusType `json:"status,omitempty"`
}

type ManifestMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type Policy struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

type ConfigProperty struct {
	Name       string            `json:"name"`
	Properties map[string]string `json:"properties,omitempty"`
}

type SecretSourceProperty struct {
	Policy  string `json:"policy"`
	Key     string `json:"key"`
	Field   string `json:"field,omitempty"`
	Version string `json:"version,omitempty"`
}

type SecretProperty struct {
	Name       string               `json:"name"`
	Properties SecretSourceProperty `json:"properties"`
}

type SharedApplicationComponentProperties struct {
	Name      string `json:"name"`
	Component string `json:"component"`
}

type ComponentProperties struct {
	Image       string                                `json:"image"`
	Application *SharedApplicationComponentProperties `json:"application,omitempty"`
	Id          string                                `json:"id,omitempty"`
	Config      []ConfigProperty                      `json:"config,omitempty"`
	Secrets     []SecretProperty                      `json:"secrets,omitempty"`
}

type ConfigDefinition struct {
	Config  []ConfigProperty `json:"config,omitempty"`
	Secrets []SecretProperty `json:"secrets,omitempty"`
}

type TargetConfigDefinition struct {
	Name    string           `json:"name"`
	Config  []ConfigProperty `json:"config,omitempty"`
	Secrets []SecretProperty `json:"secrets,omitempty"`
}

type rawTargetConfigDefinition TargetConfigDefinition

func (t *TargetConfigDefinition) UnmarshalYAML(data []byte) error {
	*t = TargetConfigDefinition{}
	if err := yaml.Unmarshal(data, &t.Name); err == nil {
		return nil
	}

	rt := &rawTargetConfigDefinition{}
	if err := yaml.Unmarshal(data, rt); err != nil {
		return err
	}
	*t = TargetConfigDefinition(*rt)

	return nil
}

type LinkProperty struct {
	Name string `json:"name,omitempty"`

	Namespace  string                  `json:"namespace"`
	Package    string                  `json:"package"`
	Interfaces []string                `json:"interfaces"`
	Source     *ConfigDefinition       `json:"source,omitempty"`
	Target     *TargetConfigDefinition `json:"target,omitempty"`
}

type Spread struct {
	Name         string            `json:"name"`
	Requirements map[string]string `json:"requirements,omitempty"`
	Weight       *int              `json:"weight,omitempty"`
}

type SpreadScalerProperty struct {
	Instances int      `json:"instances"`
	Spread    []Spread `json:"spread,omitempty"`
}

type Trait struct {
	Type         TraitType             `json:"type"`
	Link         *LinkProperty         `json:"-"`
	SpreadScaler *SpreadScalerProperty `json:"-"`
}

type rawTrait struct {
	Type       TraitType  `json:"type"`
	Properties RawMessage `json:"properties,omitempty"`
}

func (t Trait) MarshalYAML() ([]byte, error) {
	return t.marshal(yaml.Marshal)
}

func (t Trait) MarshalJSON() ([]byte, error) {
	return t.marshal(json.Marshal)
}

func (t Trait) marshal(fn func(interface{}) ([]byte, error)) ([]byte, error) {
	r := rawTrait{Type: t.Type}

	var err error
	switch t.Type {
	case TraitTypeLink:
		r.Properties, err = fn(t.Link)
	case TraitTypeSpreadScaler, TraitTypeDaemonScaler:
		r.Properties, err = fn(t.SpreadScaler)
	default:
		err = ErrEncode
	}
	if err != nil {
		return nil, err
	}

	return fn(r)
}

func (t *Trait) unmarshal(data []byte, fn func([]byte, interface{}) error) error {
	var r rawTrait
	if err := fn(data, &r); err != nil {
		return err
	}

	*t = Trait{Type: r.Type}

	var err error
	switch r.Type {
	case TraitTypeLink:
		t.Link = &LinkProperty{}
		err = fn(r.Properties, t.Link)
	case TraitTypeSpreadScaler, TraitTypeDaemonScaler:
		t.SpreadScaler = &SpreadScalerProperty{}
		err = fn(r.Properties, t.SpreadScaler)
	default:
		err = ErrDecode
	}
	if err != nil {
		return err
	}

	return nil
}

func (t *Trait) UnmarshalJSON(data []byte) error {
	return t.unmarshal(data, json.Unmarshal)
}

func (t *Trait) UnmarshalYAML(data []byte) error {
	return t.unmarshal(data, yaml.Unmarshal)
}

type Component struct {
	Name       string              `json:"name"`
	Type       ComponentType       `json:"type"`
	Properties ComponentProperties `json:"properties"`
	Traits     []Trait             `json:"traits,omitempty"`
}

type ManifestSpec struct {
	Components []Component `json:"components,omitempty"`
	Policies   []Policy    `json:"policies,omitempty"`
}

type Manifest struct {
	ApiVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ManifestMetadata `json:"metadata"`
	Spec       ManifestSpec     `json:"spec"`
}

func (m *Manifest) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Manifest) ToYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

func (m *Manifest) Validate() []error {
	var errs []error

	// no duplicate component names, check 'id' field too
	componentNames := make(map[string]bool)
	for _, c := range m.Spec.Components {
		id := c.Properties.Id
		if id == "" {
			id = c.Name
		}

		if _, ok := componentNames[id]; ok {
			errs = append(errs, fmt.Errorf("%w: duplicate component name %s", ErrValidation, id))
			continue
		}

		componentNames[id] = true
	}

	// no version latest
	if m.Metadata.Annotations != nil {
		if version, ok := m.Metadata.Annotations[VersionAnnotation]; ok {
			if version == "latest" {
				errs = append(errs, fmt.Errorf("%w: '%s' version is reserved", ErrValidation, version))
			}
		}
	}

	return errs
}

func (m *Manifest) IsValid() bool {
	return len(m.Validate()) == 0
}

type ModelListRequest struct{}

type ModelListResponse struct {
	Result  GetResult      `json:"result"`
	Message string         `json:"message"`
	Models  []ModelSummary `json:"models,omitempty"`
}

func (m *ModelListResponse) IsError() bool {
	return m.Result == GetResultError
}

type ModelGetRequest struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func (m *ModelGetRequest) setName(name string) {
	m.Name = name
}

type ModelGetResponse struct {
	Result   GetResult `json:"result"`
	Message  string    `json:"message"`
	Manifest *Manifest `json:"manifest,omitempty"`
}

type ModelDeleteRequest struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func (m *ModelDeleteRequest) setName(name string) {
	m.Name = name
}

type ModelDeleteResponse struct {
	Result   DeleteResult `json:"result"`
	Message  string       `json:"message"`
	Undeploy bool         `json:"undeploy"`
}

type ModelDeployRequest struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

func (m *ModelDeployRequest) setName(name string) {
	m.Name = name
}

type ModelDeployResponse struct {
	Result  DeployResult `json:"result"`
	Message string       `json:"message"`
	Name    string       `json:"name"`
	Version string       `json:"version,omitempty"`
}

type ModelUndeployRequest struct {
	Name string `json:"name"`
}

func (m *ModelUndeployRequest) setName(name string) {
	m.Name = name
}

type ModelUndeployResponse struct {
	Result  DeployResult `json:"result"`
	Message string       `json:"message"`
	Name    string       `json:"name"`
	Version string       `json:"version,omitempty"`
}
