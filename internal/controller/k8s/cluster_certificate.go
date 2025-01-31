package k8s

import (
	"context"
	"fmt"

	k8sv1alpha1 "go.wasmcloud.dev/operator/api/k8s/v1alpha1"
	"go.wasmcloud.dev/operator/internal/pki"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ClusterReconciler) loadCA(ctx context.Context, cluster *k8sv1alpha1.Cluster) (*pki.CertificateAuthority, error) {
	var err error
	caSecret := &corev1.Secret{}
	if err = r.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name + "-ca",
	}, caSecret); err != nil {
		return nil, err
	}

	caBytes, ok := caSecret.Data["ca.crt"]
	if !ok {
		return nil, fmt.Errorf("CA certificate not found in secret %s", caSecret.Name)
	}

	caKey, ok := caSecret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("CA private key not found in secret %s", caSecret.Name)
	}

	return pki.LoadCertificateAuthority(caBytes, caKey)
}

func (r *ClusterReconciler) reconcileCertificate(ctx context.Context, cluster *k8sv1alpha1.Cluster, suffix string) error {
	ca, err := r.loadCA(ctx, cluster)
	if err != nil {
		return err
	}

	clientSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      cluster.Name + suffix,
		},
		Type: corev1.SecretTypeTLS,
	}

	if err = r.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name + suffix,
	}, clientSecret); err == nil {
		return nil
	}
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	client, err := pki.NewClient(suffix)
	if err != nil {
		return nil
	}

	cert, err := ca.Sign(client.Certificate, client.KeyPair.PublicKey)
	if err != nil {
		return err
	}

	clientSecret.Data = map[string][]byte{
		"ca.crt":  ca.CertificatePEM(),
		"tls.crt": cert,
		"tls.key": client.PrivateKeyPEM(),
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, clientSecret, func() error {
		return nil
	})
	return err
}

func (r *ClusterReconciler) reconcileCertificateAuthority(ctx context.Context, cluster *k8sv1alpha1.Cluster) error {
	var err error
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      cluster.Name + "-ca",
		},
		Type: corev1.SecretTypeTLS,
	}

	if err = r.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name + "-ca",
	}, caSecret); err == nil {
		return nil
	}
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	ca, err := pki.NewCertificateAuthority(cluster.Name)
	if err != nil {
		return nil
	}

	cert, err := ca.SelfSign()
	if err != nil {
		return err
	}

	caSecret.Data = map[string][]byte{
		"ca.crt":  cert,
		"tls.crt": cert,
		"tls.key": ca.PrivateKeyPEM(),
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, caSecret, func() error {
		return nil
	})
	return err
}
