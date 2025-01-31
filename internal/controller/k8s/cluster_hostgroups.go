package k8s

import (
	"context"

	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ClusterReconciler) reconcileHostGroups(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	for _, hostGroup := range cluster.Spec.Hosts {
		if err := r.reconcileHostGroup(ctx, cluster, &hostGroup); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClusterReconciler) reconcileHostGroup(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostSpec) error {
	if err := r.reconcileHostGroupDeployment(ctx, cluster, hostGroup); err != nil {
		return err
	}

	if err := r.reconcileHostGroupService(ctx, cluster, hostGroup); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcileHostGroupDeployment(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostSpec) error {
	wantLabels := map[string]string{
		"cluster":    cluster.GetName(),
		"host-group": hostGroup.Name,
	}

	defaultLabels := map[string]string{
		"cluster":    cluster.GetName(),
		"host-group": hostGroup.Name,
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
			Name:  "WASMCLOUD_HTTP_ADMIN",
			Value: "0.0.0.0:9090",
		},
		{
			Name:  "WASMCLOUD_LOG_LEVEL",
			Value: "INFO",
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
			Value: hostGroup.Name,
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
			Value: "nats-" + cluster.GetName(),
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
					SecretName: cluster.NatsClientSecret(),
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

	image := hostGroup.Image
	if image == "" {
		image = "ghcr.io/wasmcloud/wasmcloud:canary"
	}

	hostContainer := corev1.Container{
		Name:         "host",
		Image:        image,
		Command:      hostGroup.Command,
		Args:         hostGroup.Args,
		WorkingDir:   hostGroup.WorkingDir,
		EnvFrom:      mergeEnvFromSource(hostGroup.EnvFrom),
		Env:          mergeEnvVar(hostGroup.Env, defaultEnv),
		VolumeMounts: mergeMounts(defaultMounts, hostGroup.VolumeMounts),
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
				Name:          "admin",
				ContainerPort: 9090,
			},
		},
	}

	if hostGroup.Resources != nil {
		hostContainer.Resources = *hostGroup.Resources
	}

	volumes = append(volumes, hostGroup.Volumes...)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: mergeLabels(hostGroup.Labels, wantLabels),
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks:            boolPtr(false),
			AutomountServiceAccountToken:  hostGroup.AutomountServiceAccountToken,
			TerminationGracePeriodSeconds: int64Ptr(0),
			ServiceAccountName:            hostGroup.ServiceAccountName,
			Containers:                    []corev1.Container{hostContainer},
			Volumes:                       volumes,
		},
	}

	spec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: wantLabels,
		},
		Template: podTemplate,
		Replicas: int32Ptr(hostGroup.Replicas),
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostGroup.Name + "-" + cluster.GetName(),
			Namespace: cluster.GetNamespace(),
			Labels:    mergeLabels(hostGroup.Labels, defaultLabels),
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = spec
		// labels might have been modified elsewhere, so merge them
		deployment.SetLabels(mergeLabels(deployment.GetLabels(), hostGroup.Labels, defaultLabels))
		return nil
	})

	return err
}

func (r *ClusterReconciler) reconcileHostGroupService(ctx context.Context, cluster *k8sv1alpha1.Cluster, hostGroup *k8sv1alpha1.HostSpec) error {
	wantLabels := map[string]string{
		"cluster":    cluster.GetName(),
		"host-group": hostGroup.Name,
	}

	defaultLabels := map[string]string{
		"cluster":    cluster.GetName(),
		"host-group": hostGroup.Name,
	}

	spec := corev1.ServiceSpec{
		ClusterIP: "None",
		Selector:  wantLabels,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostGroup.Name + "-" + cluster.GetName(),
			Namespace: cluster.GetNamespace(),
			Labels:    mergeLabels(hostGroup.Labels, defaultLabels),
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		service.Spec = spec
		// labels might have been modified elsewhere, so merge them
		service.SetLabels(mergeLabels(service.GetLabels(), hostGroup.Labels, defaultLabels))
		return nil
	})

	return err
}
