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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.wasmcloud.dev/operator/api/condition"
	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	refreshInterval = 5 * time.Second
	finalizer       = "k8s.wasmcloud.dev/hostgroup-finalizer"
)

// HostGroupReconciler reconciles a HostGroup object
type HostGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=hostgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=hostgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.wasmcloud.dev,resources=hostgroups/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets/finalizers,verbs=update

func (r *HostGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var hostGroup k8sv1alpha1.HostGroup
	if err := r.Get(ctx, req.NamespacedName, &hostGroup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !hostGroup.DeletionTimestamp.IsZero() {
		// deletion timestamp is set.
		// cleanup resources if we have a finalizer.
		if controllerutil.ContainsFinalizer(&hostGroup, finalizer) {
			if err := r.finalize(ctx, &hostGroup); err != nil {
				logger.Error(err, "unable to finalize")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(&hostGroup, finalizer)
			if err := r.Update(ctx, &hostGroup); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	cluster, err := GetCluster(ctx, r.Client, hostGroup.Spec.Cluster.Namespace, hostGroup.Spec.Cluster.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileCredentials(ctx, cluster, &hostGroup); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileSpec(ctx, cluster, &hostGroup); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileStatus(ctx, cluster, &hostGroup); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: refreshInterval}, nil
}

func GetCluster(ctx context.Context, apiClient client.Client, namespace string, name string) (*k8sv1alpha1.Cluster, error) {
	var cluster k8sv1alpha1.Cluster
	if err := apiClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &cluster); err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (r *HostGroupReconciler) reconcileCredentials(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostGroup) error {
	// for now simply copy the secret from the cluster
	var sourceCreds corev1.Secret
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: cluster.GetNamespace(), Name: cluster.NatsClientSecret()},
		&sourceCreds); err != nil {
		return err
	}

	destCreds := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            hostGroup.NatsClientSecret(),
			Namespace:       hostGroup.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(hostGroup, hostGroup.GroupVersionKind())},
			Labels: map[string]string{
				"host-cluster": cluster.ResourceLabel(),
				"host-group":   hostGroup.GetName(),
			},
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, destCreds, func() error {
		destCreds.Data = map[string][]byte{
			"user.jwt": sourceCreds.Data["user.jwt"],
		}
		return nil
	})

	return err
}

func (r *HostGroupReconciler) reconcileSpec(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostGroup) error {
	// ensure finalizer
	if !controllerutil.ContainsFinalizer(hostGroup, finalizer) {
		controllerutil.AddFinalizer(hostGroup, finalizer)
		if err := r.Update(ctx, hostGroup); err != nil {
			return err
		}
	}

	if err := r.reconcileDeployment(ctx, cluster, hostGroup); err != nil {
		return err
	}

	if err := r.reconcileHeadlessService(ctx, cluster, hostGroup); err != nil {
		return err
	}

	hostGroup.Status.ObservedGeneration = hostGroup.Generation

	return r.Status().Update(ctx, hostGroup)
}

func serviceCondition(tpy string) condition.Condition {
	return condition.Condition{
		Type:               condition.ConditionType(tpy),
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}

func (r *HostGroupReconciler) reconcileHeadlessService(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostGroup) error {
	wantLabels := map[string]string{
		"host-cluster": cluster.ResourceLabel(),
		"host-group":   hostGroup.GetName(),
	}

	defaultLabels := map[string]string{
		"host-cluster": cluster.ResourceLabel(),
		"host-group":   hostGroup.GetName(),
	}

	spec := corev1.ServiceSpec{
		ClusterIP: "None",
		Selector:  wantLabels,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            hostGroup.ServiceName(),
			Namespace:       hostGroup.GetNamespace(),
			Labels:          mergeLabels(hostGroup.Spec.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(hostGroup, hostGroup.GroupVersionKind())},
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec = spec
		return nil
	})

	cond := serviceCondition("ServiceReady")
	cond.Status = corev1.ConditionTrue
	hostGroup.Status.SetConditions(cond)

	return err
}

func (r *HostGroupReconciler) reconcileDeployment(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostGroup) error {
	wantLabels := map[string]string{
		"host-cluster": cluster.ResourceLabel(),
		"host-group":   hostGroup.GetName(),
	}

	defaultLabels := map[string]string{
		"host-cluster": cluster.ResourceLabel(),
		"host-group":   hostGroup.GetName(),
	}

	defaultEnv := []corev1.EnvVar{
		// placement vars
		{
			Name: "WASMCLOUD_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "WASMCLOUD_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "WASMCLOUD_POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "WASMCLOUD_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},

		// wasmcloud specific vars
		{
			Name:  "WASMCLOUD_STRUCTURED_LOGGING_ENABLED",
			Value: "true",
		},
		{
			Name:  "WASMCLOUD_LOG_LEVEL",
			Value: "INFO",
		},
		{
			Name:  "WASMCLOUD_JS_DOMAIN",
			Value: "default",
		},
		{
			Name:  "WASMCLOUD_RPC_TIMEOUT_MS",
			Value: "4000",
		},
		{
			Name:  "WASMCLOUD_LABEL_kubernetes",
			Value: "true",
		},
		{
			Name:  "WASMCLOUD_LABEL_kubernetes.hostgroup",
			Value: hostGroup.GetName(),
		},
		{
			Name:  "WASMCLOUD_HTTP_ADMIN",
			Value: "0.0.0.0:9090",
		},
		{
			Name:  "WASMCLOUD_NATS_CREDS",
			Value: "/creds/user.jwt",
		},
		{
			Name:  "WASMCLOUD_LATTICE",
			Value: "default",
		},
		{
			Name:  "WASMCLOUD_NATS_HOST",
			Value: cluster.NatsHost(),
		},
		{
			Name:  "WASMCLOUD_NATS_PORT",
			Value: "4222",
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "wasmcloud-share",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: hostGroup.NatsClientSecret(),
				},
			},
		},
	}

	defaultMounts := []corev1.VolumeMount{
		{
			Name:      "wasmcloud-share",
			MountPath: "/share",
		},
		{
			Name:      "creds",
			MountPath: "/creds",
		},
	}

	image := hostGroup.Spec.Image
	if image == "" {
		image = "ghcr.io/wasmcloud/wasmcloud:canary"
	}

	hostContainer := corev1.Container{
		Name:         "host",
		Image:        image,
		Command:      hostGroup.Spec.Command,
		Args:         hostGroup.Spec.Args,
		WorkingDir:   hostGroup.Spec.WorkingDir,
		EnvFrom:      mergeEnvFromSource(hostGroup.Spec.EnvFrom),
		Env:          mergeEnvVar(hostGroup.Spec.Env, defaultEnv),
		VolumeMounts: mergeMounts(defaultMounts, hostGroup.Spec.VolumeMounts),
		LivenessProbe: &corev1.Probe{
			PeriodSeconds: 3,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/livez",
					Port: intstr.FromInt(9090),
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "metrics",
				ContainerPort: 9090,
			},
		},
	}

	if hostGroup.Spec.Resources != nil {
		hostContainer.Resources = *hostGroup.Spec.Resources
	}

	volumes = append(volumes, hostGroup.Spec.Volumes...)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: mergeLabels(hostGroup.Spec.Labels, wantLabels),
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks:            boolPtr(false),
			AutomountServiceAccountToken:  hostGroup.Spec.AutomountServiceAccountToken,
			TerminationGracePeriodSeconds: int64Ptr(0),
			ServiceAccountName:            hostGroup.Spec.ServiceAccountName,
			Containers:                    []corev1.Container{hostContainer},
			Volumes:                       volumes,
		},
	}

	spec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: wantLabels,
		},
		Template: podTemplate,
		Replicas: &hostGroup.Spec.Replicas,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            hostGroup.GetName(),
			Namespace:       hostGroup.GetNamespace(),
			Labels:          mergeLabels(hostGroup.Spec.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(hostGroup, hostGroup.GroupVersionKind())},
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = spec
		return nil
	})

	cond := serviceCondition("DeploymentReady")
	cond.Status = corev1.ConditionTrue
	hostGroup.Status.SetConditions(cond)

	return err
}

func (r *HostGroupReconciler) reconcileStatus(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostGroup) error {
	// get status from kube daemonset + lattice cache
	return nil
}

func (r *HostGroupReconciler) finalize(ctx context.Context, hostGroup *k8sv1alpha1.HostGroup) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HostGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sv1alpha1.HostGroup{}).
		Named("k8s-hostgroup").
		Owns(&appsv1.DaemonSet{}).
		Complete(r)
}
