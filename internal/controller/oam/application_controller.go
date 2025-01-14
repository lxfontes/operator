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

package oam

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	coreoamv1beta1 "go.wasmcloud.dev/operator/api/oam/core/v1beta1"
	"go.wasmcloud.dev/x/wasmbus"
	"go.wasmcloud.dev/x/wasmbus/wadm"
)

const refreshInterval = 5 * time.Second
const finalizer = "k8s.wasmcloud.dev/application-finalizer"

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Bus    wasmbus.Bus
	// Which lattice to place Application objects.
	// Will use the object namespace if blank.
	// wasmcloud clusters usually operate on a single 'default' lattice.
	Lattice string
}

// +kubebuilder:rbac:groups=core.oam.dev,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.oam.dev,resources=applications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.oam.dev,resources=applications/finalizers,verbs=update
func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var application coreoamv1beta1.Application
	if err := r.Get(ctx, req.NamespacedName, &application); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !application.DeletionTimestamp.IsZero() {
		// deletion timestamp is set.
		// cleanup resources if we have a finalizer.
		if controllerutil.ContainsFinalizer(&application, finalizer) {
			if err := r.finalize(ctx, &application); err != nil {
				logger.Error(err, "unable to finalize")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&application, finalizer)
			if err := r.Update(ctx, &application); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if err := r.reconcileSpec(ctx, &application); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, &application); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: refreshInterval}, nil
}

func (r *ApplicationReconciler) reconcileSpec(ctx context.Context, application *coreoamv1beta1.Application) error {
	if application.Status.ObservedGeneration == application.Generation {
		return nil
	}

	// ensure finalizer
	if !controllerutil.ContainsFinalizer(application, finalizer) {
		controllerutil.AddFinalizer(application, finalizer)
		if err := r.Update(ctx, application); err != nil {
			return err
		}
	}

	rawSpec, err := json.Marshal(application)
	if err != nil {
		return err
	}

	wadmManifest, err := wadm.ParseJSONManifest(rawSpec)
	if err != nil {
		return err
	}

	c := r.wadmClient(application)
	putResp, err := c.ModelPut(ctx, &wadm.ModelPutRequest{
		Manifest: *wadmManifest,
	})
	if err != nil {
		return err
	}
	if putResp.IsError() {
		return fmt.Errorf("model put error: %s", putResp.Message)
	}

	deployResp, err := c.ModelDeploy(ctx, &wadm.ModelDeployRequest{
		Name: application.Name,
	})
	if err != nil {
		return err
	}
	if deployResp.IsError() {
		return fmt.Errorf("model deploy error: %s", deployResp.Message)
	}

	application.Status.ObservedGeneration = application.Generation
	application.Status.ObservedVersion = deployResp.Version

	return r.Status().Update(ctx, application)
}

func (r *ApplicationReconciler) reconcileStatus(ctx context.Context, application *coreoamv1beta1.Application) error {
	c := r.wadmClient(application)
	req := &wadm.ModelStatusRequest{
		Name: application.Name,
	}
	resp, err := c.ModelStatus(ctx, req)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf("model status error: %s", resp.Message)
	}

	scalers := []coreoamv1beta1.ScalerStatus{}
	for _, scaler := range resp.Status.Scalers {
		scalers = append(scalers, coreoamv1beta1.ScalerStatus{
			Id:      scaler.Id,
			Kind:    scaler.Kind,
			Name:    scaler.Name,
			Status:  string(scaler.Status.Type),
			Message: scaler.Status.Message,
		})
	}

	application.Status.ScalerStatus = scalers
	application.Status.Phase = wadmStatusToPhase(resp.Status.Status.Type)

	return r.Status().Update(ctx, application)
}

func (r *ApplicationReconciler) finalize(ctx context.Context, application *coreoamv1beta1.Application) error {
	if application.Status.ObservedVersion == "" {
		// never deployed, nothing to do
		return nil
	}

	c := r.wadmClient(application)

	_, err := c.ModelDelete(ctx, &wadm.ModelDeleteRequest{
		Name: application.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *ApplicationReconciler) lattice(application *coreoamv1beta1.Application) string {
	lattice := r.Lattice
	if lattice == "" {
		lattice = application.GetNamespace()
	}
	return lattice
}

func (r *ApplicationReconciler) wadmClient(application *coreoamv1beta1.Application) *wadm.Client {
	return wadm.NewClient(r.Bus, r.lattice(application))
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&coreoamv1beta1.Application{}).
		Named("oam-application").
		Complete(r)
}

func wadmStatusToPhase(status wadm.StatusType) coreoamv1beta1.ApplicationPhase {
	// TODO(lxf): keeping this so we can translate the status from wadm to oam
	return coreoamv1beta1.ApplicationPhase(status)
}
