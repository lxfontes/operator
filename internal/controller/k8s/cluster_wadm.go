package k8s

import (
	"context"
	"fmt"

	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ClusterReconciler) reconcileWadm(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	if err := r.reconcileWadmStatefulset(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileWadmStatefulset(ctx, cluster); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcileWadmStatefulset(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
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

		// wadm specific vars
		{
			Name:  "WADM_NATS_SERVER",
			Value: fmt.Sprintf("nats-%s:4222", cluster.GetName()),
		},
		{
			Name:  "WADM_NATS_CREDS_FILE",
			Value: "/creds/user.jwt",
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "creds",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: cluster.NatsClientSecret(),
				},
			},
		},
	}
	defaultMounts := []corev1.VolumeMount{
		{
			Name:      "creds",
			MountPath: "/creds",
		},
	}
	image := "ghcr.io/wasmcloud/wadm:canary"
	hostContainer := corev1.Container{
		Name:         "wadm",
		Image:        image,
		EnvFrom:      mergeEnvFromSource(cluster.Spec.Wadm.Managed.EnvFrom),
		Env:          mergeEnvVar(cluster.Spec.Wadm.Managed.Env, defaultEnv),
		VolumeMounts: mergeMounts(defaultMounts, cluster.Spec.Wadm.Managed.VolumeMounts),
	}

	volumes = append(volumes, cluster.Spec.Wadm.Managed.Volumes...)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: mergeLabels(wantLabels),
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks:            boolPtr(false),
			AutomountServiceAccountToken:  cluster.Spec.Wadm.Managed.AutomountServiceAccountToken,
			TerminationGracePeriodSeconds: int64Ptr(0),
			ServiceAccountName:            cluster.Spec.Wadm.Managed.ServiceAccountName,
			Containers:                    []corev1.Container{hostContainer},
			Volumes:                       volumes,
		},
	}

	spec := appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: wantLabels,
		},
		Replicas: &cluster.Spec.Wadm.Managed.Replicas,
		Template: podTemplate,
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "wadm-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			Labels:          mergeLabels(cluster.Spec.Wadm.Managed.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulset, func() error {
		statefulset.Spec = spec
		// labels might have been modified elsewhere, so merge them
		statefulset.SetLabels(mergeLabels(statefulset.GetLabels(), cluster.Spec.Wadm.Managed.Labels, defaultLabels))
		return nil
	})

	return err
}
