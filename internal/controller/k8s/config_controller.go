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

	"go.wasmcloud.dev/x/wasmbus"
	"go.wasmcloud.dev/x/wasmbus/config"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
)

type ConfigReconciler struct {
	client.Client

	Lattice string
	Bus     wasmbus.Bus
	Scheme  *runtime.Scheme

	configServer *configServer
}

func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var err error
	r.configServer = newConfigServer(r.Bus, "default")

	if err = mgr.Add(r.configServer); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Named("wasmcloud-config").
		Complete(r)
}

type configServer struct {
	server *config.Server
}

var _ config.API = (*configServer)(nil)

func (s *configServer) Host(
	ctx context.Context,
	req *config.HostRequest) (*config.HostResponse, error) {
	logger := log.FromContext(ctx)
	logger.Info("Host request received", "req", req)
	return &config.HostResponse{}, nil
}

func newConfigServer(bus wasmbus.Bus, lattice string) *configServer {
	s := &configServer{}

	server := config.NewServer(bus, lattice, s)
	s.server = server

	return s
}

func (s *configServer) NeedLeaderElection() bool {
	return false
}

func (s *configServer) Start(ctx context.Context) error {
	if err := s.server.Serve(); err != nil {
		return err
	}
	<-ctx.Done()
	return s.server.Drain()
}
