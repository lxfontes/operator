package k8s

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var natsConfigTemplate = `
{
  "port": 4222,
  "http_port": 8222,
  "lame_duck_duration": "30s",
  "lame_duck_grace_period": "10s",
  "operator": "{{.OperatorJWT}}",
  "system_account": "{{.SystemPub}}",
  "resolver": {
    "type": "memory",
  },
  "resolver_preload": {
    "{{.SystemPub}}": "{{.SystemJWT}}",
    "{{.AccountPub}}": "{{.AccountJWT}}",
  },
  "accounts":{
    "AUTH": {
  		"users": [ { "nkey": "{{.AuthPub}}" } ]
	}
  },
  "authorization": {
	"auth_callout": {
		"issuer": "{{.AccountPub}}",
		"auth_users": [ "{{.AuthPub}}" ],
		"account": "AUTH",
	},
  },
  "jetstream": {
    "domain": "default",
    "store_dir": "/data"
  },
  "leafnodes": {
    "no_advertise": true,
    "port": 7422
  },
  "cluster": {
    "name": "{{ .Name }}",
	"port": 6222,
	"no_advertise": true,
	"routes": [
	 {{- range $idx, $route := .Routes }} {{ if ne $idx 0 }},{{ end }}"{{ $route }}" {{- end }}
	]
}
`

func (r *ClusterReconciler) reconcileNats(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {

	// if err := r.reconcileCertificate(ctx, cluster, "nats-client"); err != nil {
	// 	return err
	// }

	if err := r.reconcileNatsCredentials(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileNatsServerConfig(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileNatsClientConfig(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileNatsStatefulset(ctx, cluster); err != nil {
		return err
	}

	if err := r.reconcileNatsServices(ctx, cluster); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcileNatsServices(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	wantLabels := map[string]string{
		"cluster": cluster.GetName(),
	}

	defaultLabels := map[string]string{
		"cluster": cluster.GetName(),
	}

	headlessSpec := corev1.ServiceSpec{
		ClusterIP:                "None",
		Selector:                 wantLabels,
		PublishNotReadyAddresses: true,
		Ports: []corev1.ServicePort{
			{
				Name:       "nats",
				Protocol:   corev1.ProtocolTCP,
				Port:       4222,
				TargetPort: intstr.FromInt(4222),
			},
			{
				Name:       "leafnodes",
				Protocol:   corev1.ProtocolTCP,
				Port:       7422,
				TargetPort: intstr.FromInt(7422),
			},
			{
				Name:       "cluster",
				Protocol:   corev1.ProtocolTCP,
				Port:       6222,
				TargetPort: intstr.FromInt(6222),
			},
			{
				Name:       "monitor",
				Protocol:   corev1.ProtocolTCP,
				Port:       8222,
				TargetPort: intstr.FromInt(8222),
			},
		},
	}

	// service for discovery
	headlessService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "natsd-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			Labels:          mergeLabels(cluster.Spec.Nats.Managed.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Spec: headlessSpec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, headlessService, func() error {
		headlessService.Spec = headlessSpec
		// labels might have been modified elsewhere, so merge them
		headlessService.SetLabels(mergeLabels(headlessService.GetLabels(), cluster.Spec.Nats.Managed.Labels, defaultLabels))
		return nil
	})

	if err != nil {
		return err
	}

	userSpec := corev1.ServiceSpec{
		Selector: wantLabels,
		Ports: []corev1.ServicePort{
			{
				Name:       "nats",
				Protocol:   corev1.ProtocolTCP,
				Port:       4222,
				TargetPort: intstr.FromInt(4222),
			},
			{
				Name:       "leafnodes",
				Protocol:   corev1.ProtocolTCP,
				Port:       7422,
				TargetPort: intstr.FromInt(7422),
			},
		},
	}

	// service for hosts / clients
	userService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nats-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			Labels:          mergeLabels(cluster.Spec.Nats.Managed.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Spec: userSpec,
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, userService, func() error {
		userService.Spec = userSpec
		// labels might have been modified elsewhere, so merge them
		userService.SetLabels(mergeLabels(userService.GetLabels(), cluster.Spec.Nats.Managed.Labels, defaultLabels))
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcileNatsCredentials(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	var creds corev1.Secret
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: cluster.GetNamespace(), Name: cluster.NatsSeedSecret()},
		&creds); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return err
		}
	} else {
		// secret already exists
		return nil
	}

	operator, err := nkeys.CreateOperator()
	if err != nil {
		return err
	}
	operatorSeed, err := operator.Seed()
	if err != nil {
		return err
	}

	system, err := nkeys.CreateAccount()
	if err != nil {
		return err
	}
	systemSeed, err := system.Seed()
	if err != nil {
		return err
	}

	account, err := nkeys.CreateAccount()
	if err != nil {
		return err
	}
	accountSeed, err := account.Seed()
	if err != nil {
		return err
	}

	user, err := nkeys.CreateUser()
	if err != nil {
		return err
	}
	userSeed, err := user.Seed()
	if err != nil {
		return err
	}

	auth, err := nkeys.CreateUser()
	if err != nil {
		return err
	}
	authSeed, err := auth.Seed()
	if err != nil {
		return err
	}

	secretData := map[string][]byte{
		"operator": operatorSeed,
		"system":   systemSeed,
		"account":  accountSeed,
		"user":     userSeed,
		"auth":     authSeed,
	}

	creds.ObjectMeta = metav1.ObjectMeta{
		Name:            cluster.NatsSeedSecret(),
		Namespace:       cluster.GetNamespace(),
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
	}
	creds.Data = secretData

	return r.Create(ctx, &creds)
}

func (r *ClusterReconciler) reconcileNatsClientConfig(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	var creds corev1.Secret
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: cluster.GetNamespace(), Name: cluster.NatsSeedSecret()},
		&creds); err != nil {
		return err
	}
	rawAccount, ok := creds.Data["account"]
	if !ok {
		return fmt.Errorf("missing account seed")
	}
	accountKp, err := nkeys.FromSeed(rawAccount)
	if err != nil {
		return err
	}

	rawUser, ok := creds.Data["user"]
	if !ok {
		return fmt.Errorf("missing user seed")
	}
	userKp, err := nkeys.FromSeed(rawUser)
	if err != nil {
		return err
	}

	user, err := newUser("client", userKp, accountKp)
	if err != nil {
		return err
	}

	userJWT, err := user.Encode(accountKp)
	if err != nil {
		return err
	}

	userSeed, err := userKp.Seed()
	if err != nil {
		return err
	}

	userCreds, err := jwt.FormatUserConfig(userJWT, userSeed)
	if err != nil {
		return err
	}

	userSecretData := map[string][]byte{
		"user.jwt": userCreds,
	}

	userSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cluster.NatsClientSecret(),
			Namespace:       cluster.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, userSecret, func() error {
		userSecret.Data = userSecretData
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (r *ClusterReconciler) reconcileNatsServerConfig(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	var creds corev1.Secret
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: cluster.GetNamespace(), Name: cluster.NatsSeedSecret()},
		&creds); err != nil {
		return err
	}
	rawOperator, ok := creds.Data["operator"]
	if !ok {
		return fmt.Errorf("missing operator seed")
	}
	operatorKp, err := nkeys.FromSeed(rawOperator)
	if err != nil {
		return err
	}

	rawSystem, ok := creds.Data["system"]
	if !ok {
		return fmt.Errorf("missing system seed")
	}
	sysKp, err := nkeys.FromSeed(rawSystem)
	if err != nil {
		return err
	}

	rawAccount, ok := creds.Data["account"]
	if !ok {
		return fmt.Errorf("missing account seed")
	}
	accountKp, err := nkeys.FromSeed(rawAccount)
	if err != nil {
		return err
	}

	rawAuth, ok := creds.Data["auth"]
	if !ok {
		return fmt.Errorf("missing auth seed")
	}
	authKp, err := nkeys.FromSeed(rawAuth)
	if err != nil {
		return err
	}

	sysAccount, err := newSystemAccount(sysKp)
	if err != nil {
		return err
	}

	operator, err := newOperator(operatorKp, sysKp)
	if err != nil {
		return err
	}

	account, err := newAccount("wasmcloud", accountKp)
	if err != nil {
		return err
	}
	// enable jetstream
	account.Limits.JetStreamLimits.MemoryStorage = -1
	account.Limits.JetStreamLimits.DiskStorage = -1

	operatorJWT, err := operator.Encode(operatorKp)
	if err != nil {
		return err
	}

	sysJWT, err := sysAccount.Encode(operatorKp)
	if err != nil {
		return err
	}

	accountJWT, err := account.Encode(operatorKp)
	if err != nil {
		return err
	}

	tmpl, err := template.New("nats-server.conf").Parse(natsConfigTemplate)
	if err != nil {
		return err
	}

	routes := []string{}
	for i := 0; i < int(cluster.Spec.Nats.Managed.Replicas); i++ {
		routes = append(routes, fmt.Sprintf("nats://nats-%s-%d.natsd-%s:6222", cluster.GetName(), i, cluster.GetName()))
	}

	sysPub, err := sysKp.PublicKey()
	if err != nil {
		return err
	}

	accountPub, err := accountKp.PublicKey()
	if err != nil {
		return err
	}

	authPub, err := authKp.PublicKey()
	if err != nil {
		return err
	}

	data := struct {
		Name   string
		Routes []string

		OperatorJWT string

		SystemJWT string
		SystemPub string

		AccountJWT string
		AccountPub string

		AuthPub string
	}{
		Name:   cluster.GetName(),
		Routes: routes,

		OperatorJWT: operatorJWT,

		SystemJWT: sysJWT,
		SystemPub: sysPub,

		AccountJWT: accountJWT,
		AccountPub: accountPub,

		AuthPub: authPub,
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return err
	}

	cmData := map[string]string{
		"nats.conf": b.String(),
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nats-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Data: cmData,
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = cmData
		return nil
	})

	return err
}

func (r *ClusterReconciler) reconcileNatsStatefulset(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	wantLabels := map[string]string{
		"cluster": cluster.GetName(),
	}

	defaultLabels := map[string]string{
		"cluster": cluster.GetName(),
	}

	defaultEnv := []corev1.EnvVar{
		{
			Name: "SERVER_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
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
						Name: "nats-" + cluster.GetName(),
					},
				},
			},
		},
	}
	defaultMounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/config",
		},
	}
	image := "nats:2.10.22-alpine"
	hostContainer := corev1.Container{
		Name:         "nats",
		Image:        image,
		Args:         []string{"--config", "/config/nats.conf", "--pid", "/tmp/nats.pid", "--name", "$(SERVER_NAME)"},
		EnvFrom:      mergeEnvFromSource(cluster.Spec.Nats.Managed.EnvFrom),
		Env:          mergeEnvVar(cluster.Spec.Nats.Managed.Env, defaultEnv),
		VolumeMounts: mergeMounts(defaultMounts, cluster.Spec.Nats.Managed.VolumeMounts),
		Ports: []corev1.ContainerPort{
			{
				Name:          "nats",
				ContainerPort: 4222,
			},
			{
				Name:          "leafnodes",
				ContainerPort: 6222,
			},
			{
				Name:          "cluster",
				ContainerPort: 7222,
			},
			{
				Name:          "monitor",
				ContainerPort: 8222,
			},
		},
	}

	volumes = append(volumes, cluster.Spec.Nats.Managed.Volumes...)

	podTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: mergeLabels(wantLabels),
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks:            boolPtr(false),
			AutomountServiceAccountToken:  cluster.Spec.Nats.Managed.AutomountServiceAccountToken,
			TerminationGracePeriodSeconds: int64Ptr(0),
			ServiceAccountName:            cluster.Spec.Nats.Managed.ServiceAccountName,
			Containers:                    []corev1.Container{hostContainer},
			Volumes:                       volumes,
		},
	}

	spec := appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: wantLabels,
		},
		Replicas:    &cluster.Spec.Nats.Managed.Replicas,
		Template:    podTemplate,
		ServiceName: "natsd-" + cluster.GetName(),
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nats-" + cluster.GetName(),
			Namespace:       cluster.GetNamespace(),
			Labels:          mergeLabels(cluster.Spec.Nats.Managed.Labels, defaultLabels),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cluster, cluster.GroupVersionKind())},
		},
		Spec: spec,
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, statefulset, func() error {
		statefulset.Spec = spec
		// labels might have been modified elsewhere, so merge them
		statefulset.SetLabels(mergeLabels(statefulset.GetLabels(), cluster.Spec.Nats.Managed.Labels, defaultLabels))
		return nil
	})

	return err
}

func newOperator(kp nkeys.KeyPair, sysKp nkeys.KeyPair) (*jwt.OperatorClaims, error) {
	kpPub, err := kp.PublicKey()
	if err != nil {
		return nil, err
	}

	systemAccountPub, err := sysKp.PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewOperatorClaims(kpPub)
	claims.SystemAccount = systemAccountPub
	// NOTE(lxf): If we want to go with signing keys. altho feels overkill given it is all automated.
	//	claims.StrictSigningKeyUsage = true
	claims.Name = "operator"
	claims.AccountServerURL = "nats://0.0.0.0:4222"

	return claims, nil
}

func newSystemAccount(kp nkeys.KeyPair) (*jwt.AccountClaims, error) {
	claims, err := newAccount("SYS", kp)
	if err != nil {
		return nil, err
	}

	claims.Exports = append(claims.Exports,
		&jwt.Export{
			Name:                 "account-monitoring-streams",
			Subject:              "$SYS.ACCOUNT.*.>",
			Type:                 jwt.Stream,
			AccountTokenPosition: 3,
			Info: jwt.Info{
				Description: "Account specific monitoring stream",
				InfoURL:     "https://docs.nats.io/nats-server/configuration/sys_accounts",
			},
		},

		&jwt.Export{
			Name:                 "account-monitoring-services",
			Subject:              "$SYS.REQ.ACCOUNT.*.*",
			Type:                 jwt.Service,
			ResponseType:         jwt.ResponseTypeStream,
			AccountTokenPosition: 4,
			Info: jwt.Info{
				Description: "Request account specific monitoring services for: SUBSZ, CONNZ, LEAFZ, JSZ and INFO",
				InfoURL:     "https://docs.nats.io/nats-server/configuration/sys_accounts",
			},
		},
	)

	return claims, nil
}

func newAccount(name string, kp nkeys.KeyPair) (*jwt.AccountClaims, error) {
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewAccountClaims(pub)
	claims.Name = name
	claims.Limits.Subs = -1
	claims.Limits.Data = -1
	claims.Limits.Payload = -1
	claims.Limits.Imports = -1
	claims.Limits.Exports = -1
	claims.Limits.Conn = -1
	claims.Limits.LeafNodeConn = -1
	claims.Limits.WildcardExports = true

	return claims, nil
}

func newUser(name string, kp nkeys.KeyPair, account nkeys.KeyPair) (*jwt.UserClaims, error) {
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, err
	}

	accountPub, err := account.PublicKey()
	if err != nil {
		return nil, err
	}

	claims := jwt.NewUserClaims(pub)
	claims.Name = name
	claims.IssuerAccount = accountPub

	return claims, nil
}
