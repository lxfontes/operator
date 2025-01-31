/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8s

import (
	"context"
	"os"
	"sync"

	"github.com/open-policy-agent/opa/v1/rego"
	"go.wasmcloud.dev/x/wasmbus"
	"go.wasmcloud.dev/x/wasmbus/policy"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ConfigMapNamespace string
	ConfigMapName      string

	policyServer *policyServer
}

// NOTE(lxf): Disabled as we are not using this type.
// PolicySpec & Status reconciliation updates configmaps.
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters/status,verbs=get;update;patch
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=secrets;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets;configmaps/finalizers,verbs=update
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var configMap corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &configMap); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !configMap.DeletionTimestamp.IsZero() {
		// The object is being deleted
		return ctrl.Result{}, nil
	}

	policies := []func(r *rego.Rego){}

	for name, policy := range configMap.Data {
		policies = append(policies, rego.Module(name, policy))
	}

	r.policyServer.setPolicies(policies...)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	// TODO(lxf): hook this back up
	if true {
		return nil
	}
	r.policyServer = newPolicyServer(nil, "wasmcloud.policy")

	if err = mgr.Add(r.policyServer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(rawObj client.Object) bool {
			obj := rawObj.(*corev1.ConfigMap)
			return obj.Name == r.ConfigMapName && obj.Namespace == r.ConfigMapNamespace
		})).
		Named("wasmcloud-policy").
		Complete(r)
}

type policyServer struct {
	server   *policy.Server
	policies []func(r *rego.Rego)
	lock     sync.Mutex
}

var _ policy.API = (*policyServer)(nil)

func (s *policyServer) PerformInvocation(
	ctx context.Context,
	req *policy.PerformInvocationRequest) (*policy.Response, error) {
	return servePolicy(ctx, req, s.policies...)
}

func (s *policyServer) StartComponent(
	ctx context.Context,
	req *policy.StartComponentRequest) (*policy.Response, error) {
	return servePolicy(ctx, req, s.policies...)
}

func (s *policyServer) StartProvider(
	ctx context.Context,
	req *policy.StartProviderRequest) (*policy.Response, error) {
	return servePolicy(ctx, req, s.policies...)
}

func newPolicyServer(bus wasmbus.Bus, subject string) *policyServer {
	s := &policyServer{}

	server := policy.NewServer(bus, subject, s)
	s.server = server

	return s
}

func (s *policyServer) NeedLeaderElection() bool {
	return false
}

func (s *policyServer) setPolicies(policy ...func(r *rego.Rego)) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.policies = policy
}

func (s *policyServer) Start(ctx context.Context) error {
	if err := s.server.Serve(); err != nil {
		return err
	}
	<-ctx.Done()
	return s.server.Drain()
}

type policyRequest interface {
	Decision(allowed bool, msg string) *policy.Response
}

func servePolicy[T policyRequest](ctx context.Context, req T, policies ...func(*rego.Rego)) (*policy.Response, error) {
	logger := log.FromContext(ctx)

	rawQuery := []func(r *rego.Rego){
		rego.Dump(os.Stdout),
		rego.EnablePrintStatements(true),
		rego.Query("x = data.wasmcloud.access.allow"),
	}

	rawQuery = append(rawQuery, policies...)

	query, err := rego.New(
		rawQuery...,
	).PrepareForEval(ctx)
	if err != nil {
		logger.Info("failed to prepare query", "error", err)
		return req.Decision(false, err.Error()), nil
	}

	results, err := query.Eval(ctx, rego.EvalInput(req))
	if err != nil {
		logger.Info("failed to evaluate query", "error", err)
		return req.Decision(false, err.Error()), nil
	}

	if len(results) > 0 {
		if x, ok := results[0].Bindings["x"].(bool); ok && x {
			logger.Info("policy checks passed", "request", req)
			return req.Decision(true, "policy checks passed"), nil
		}
	}

	logger.Info("policy checks failed")
	return req.Decision(false, "policy checks failed"), nil
}
