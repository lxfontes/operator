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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NOTE(lxf): Disabled as we are not using this type.
// ClusterSpec & Status reconciliation updates configmaps.
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters/status,verbs=get;update;patch
// kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=clusters/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=secrets;configmaps;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers;configmaps/finalizers;services/finalizers,verbs=update
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cluster k8sv1alpha1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cluster.DeletionTimestamp.IsZero() {
		// The object is being deleted
		return ctrl.Result{}, nil
	}

	// if err := r.reconcileCertificateAuthority(ctx, &cluster); err != nil {
	// 	logger.Error(err, "Failed to reconcile certificates")
	// 	return ctrl.Result{}, err
	// }

	if err := r.reconcileNats(ctx, &cluster); err != nil {
		logger.Error(err, "Failed to reconcile nats")
		return ctrl.Result{}, err
	}

	if err := r.reconcileWadm(ctx, &cluster); err != nil {
		logger.Error(err, "Failed to reconcile wadm")
		return ctrl.Result{}, err
	}

	// if err := r.reconcileHostGroups(ctx, &cluster); err != nil {
	// 	logger.Error(err, "Failed to reconcile hostgroups")
	// 	return ctrl.Result{}, err
	// }

	if err := r.reconcileAddons(ctx, &cluster); err != nil {
		logger.Error(err, "Failed to reconcile addons")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.Cluster{}).
		Named("k8s-cluster").
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
