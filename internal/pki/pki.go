package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

const KeySize = 4096

func NewKeyPair() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return nil, err
	}
	pub := &key.PublicKey

	return &KeyPair{
		PrivateKey: key,
		PublicKey:  pub,
	}, nil
}

func LoadCertificateAuthority(rawCert []byte, rawKey []byte) (*CertificateAuthority, error) {
	bearer, err := loadBearer(rawCert, rawKey)
	if err != nil {
		return nil, err
	}

	return &CertificateAuthority{
		CertificateBearer: *bearer,
	}, nil
}

func loadBearer(rawCert []byte, rawKey []byte) (*CertificateBearer, error) {
	certBlock, _ := pem.Decode(rawCert)
	if certBlock == nil {
		return nil, fmt.Errorf("Failed to decode CA certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, err
	}

	keyBlock, _ := pem.Decode(rawKey)
	if keyBlock == nil {
		return nil, fmt.Errorf("Failed to decode CA private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return &CertificateBearer{
		Certificate: cert,
		KeyPair: &KeyPair{
			PrivateKey: key,
			PublicKey:  &key.PublicKey,
		},
	}, nil
}

func NewCertificateAuthority(name string) (*CertificateAuthority, error) {
	kp, err := NewKeyPair()
	if err != nil {
		return nil, err
	}

	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			CommonName:   name,
			Organization: []string{"wasmCloud"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(30, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	return &CertificateAuthority{
		CertificateBearer: CertificateBearer{
			Certificate: certificate,
			KeyPair:     kp,
		},
	}, nil
}

func (ca *CertificateAuthority) SelfSign() ([]byte, error) {
	return ca.Sign(ca.Certificate, ca.KeyPair.PublicKey)
}

func (ca *CertificateAuthority) Sign(template *x509.Certificate, pubKey *rsa.PublicKey) ([]byte, error) {
	newCert, err := x509.CreateCertificate(rand.Reader, template, ca.Certificate, pubKey, ca.KeyPair.PrivateKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: newCert}), nil
}

type CertificateBearer struct {
	KeyPair     *KeyPair
	Certificate *x509.Certificate
}

func (cb *CertificateBearer) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cb.Certificate.Raw})
}

func (cb *CertificateBearer) PrivateKeyPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(cb.KeyPair.PrivateKey)})
}

type CertificateAuthority struct {
	CertificateBearer
}

type Client struct {
	CertificateBearer
}

type KeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

func NewClient(name string) (*Client, error) {
	kp, err := NewKeyPair()
	if err != nil {
		return nil, err
	}

	certSpec := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName: name,
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	return &Client{
		CertificateBearer: CertificateBearer{
			KeyPair:     kp,
			Certificate: certSpec,
		},
	}, nil
}

func LoadClient(rawCert []byte, rawKey []byte) (*Client, error) {
	bearer, err := loadBearer(rawCert, rawKey)
	if err != nil {
		return nil, err
	}

	return &Client{
		CertificateBearer: *bearer,
	}, nil
}
