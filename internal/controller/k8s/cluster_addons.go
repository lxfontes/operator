package k8s

import (
	"context"

	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var prometheusTemplate = `
global:
  evaluation_interval: 15s
storage:
  tsdb:
    out_of_order_time_window: 30m
otlp:
  keep_identifying_resource_attributes: true
  promote_resource_attributes:
    - service.instance.id
    - service.name
    - service.namespace
    - cloud.availability_zone
    - cloud.region
    - container.name
    - deployment.environment.name
    - k8s.cluster.name
    - k8s.container.name
    - k8s.cronjob.name
    - k8s.daemonset.name
    - k8s.deployment.name
    - k8s.job.name
    - k8s.namespace.name
    - k8s.pod.name
    - k8s.replicaset.name
    - k8s.statefulset.name
`

func (r *ClusterReconciler) reconcileAddons(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	if cluster.Spec.Addons == nil {
		return nil
	}

	if cluster.Spec.Addons.Prometheus != nil {
		if err := r.reconcilePrometheus(ctx, cluster); err != nil {
			return err
		}
	}
	// Policy Service
	// Secrets Service
	// Observability
	return nil
}

func (r *ClusterReconciler) reconcilePrometheus(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	if err := r.reconcilePrometheusConfig(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcilePrometheusStatefulset(ctx, cluster); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcilePrometheusConfig(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	var cm corev1.ConfigMap

	if err := r.Get(
		ctx,
		client.ObjectKey{Namespace: cluster.GetNamespace(), Name: "prometheus-" + cluster.GetName()},
		&cm); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	} else {
		// ConfigMap already exists
		return nil
	}

	cm.ObjectMeta = metav1.ObjectMeta{
		Name:            "prometheus-" + cluster.GetName(),
		Namespace:       cluster.GetNamespace(),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
	}

	data := map[string]string{
		"prometheus.yaml": prometheusTemplate,
	}
	cm.Data = data

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &cm, func() error {
		cm.Data = data
		return nil
	})

	return err
}

func (r *ClusterReconciler) reconcilePrometheusStatefulset(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	wantLabels := map[string]string{
		"cluster": cluster.GetName(),
	}

	defaultLabels := map[string]string{
		"cluster": cluster.GetName(),
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
	}

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "prometheus-" + cluster.GetName(),
					},
				},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewScaledQuantity(1, 9), // 1GB
				},
			},
		},
	}
	defaultMounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/config",
		},
		{
			Name:      "data",
			MountPath: "/data",
		},
	}
	image := "prom/prometheus:v3.1.0"
	hostContainer := corev1.Container{
		Name:  "prometheus",
		Image: image,
		Args: []string{
			"--config.file=/config/prometheus.yaml",
			"--web.enable-otlp-receiver",
			"--enable-feature=native-histograms,auto-gomemlimit",
		},
		EnvFrom:      mergeEnvFromSource(cluster.Spec.Addons.Prometheus.EnvFrom),
		Env:          mergeEnvVar(cluster.Spec.Addons.Prometheus.Env, defaultEnv),
		VolumeMounts: mergeMounts(defaultMounts, cluster.Spec.Addons.Prometheus.VolumeMounts),
		Ports: []corev1.ContainerPort{
			{
				Name:          "prometheus",
				ContainerPort: 9090,
			},
		},
	}

	volumes = append(volumes, cluster.Spec.Addons.Prometheus.Volumes...)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: mergeLabels(wantLabels),
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks:            boolPtr(false),
			AutomountServiceAccountToken:  cluster.Spec.Addons.Prometheus.AutomountServiceAccountToken,
			TerminationGracePeriodSeconds: int64Ptr(0),
			ServiceAccountName:            cluster.Spec.Addons.Prometheus.ServiceAccountName,
			Containers:                    []corev1.Container{hostContainer},
			Volumes:                       volumes,
		},
	}

	spec := appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: wantLabels,
		},
		Replicas:            &cluster.Spec.Addons.Prometheus.Replicas,
		PodManagementPolicy: appsv1.ParallelPodManagement,
		ServiceName:         "prometheus-" + cluster.GetName(),
		Template:            podTemplate,
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "prometheus-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			Labels:          mergeLabels(cluster.Spec.Addons.Prometheus.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulset, func() error {
		statefulset.Spec = spec
		// labels might have been modified elsewhere, so merge them
		statefulset.SetLabels(mergeLabels(statefulset.GetLabels(), cluster.Spec.Addons.Prometheus.Labels, defaultLabels))
		return nil
	})

	return err
}
