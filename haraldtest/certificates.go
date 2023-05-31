package haraldtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"
)

type CA struct {
	cert             *x509.Certificate
	key              *rsa.PrivateKey
	certPem          []byte
	nextSerialNumber int64
}

func NewCertificateAuthority(t *testing.T) *CA {
	t.Helper()

	template := x509.Certificate{
		IsCA:               true,
		KeyUsage:           x509.KeyUsageCertSign,
		SignatureAlgorithm: x509.SHA256WithRSA,
		NotAfter:           time.Now().Add(time.Hour),
		NotBefore:          time.Now(),
		Subject: pkix.Name{
			Organization:       []string{"harald"},
			OrganizationalUnit: []string{"test"},
		},
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("unable to generate key: %s", err.Error())
		return nil
	}

	certDer, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("unable to generate certificate authority: %s", err.Error())
		return nil
	}

	ca, err := x509.ParseCertificate(certDer)
	if err != nil {
		t.Fatalf("unable to parse generated certificate: %s", err.Error())
		return nil
	}

	return &CA{
		cert: ca,
		key:  priv,
		certPem: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDer,
		}),
		nextSerialNumber: 2,
	}
}

func (ca *CA) Certificate() *x509.Certificate {
	return ca.cert
}

func (ca *CA) PEM() []byte {
	return ca.certPem
}

// NewServerCertificate returns a new certificate and private key encoded as
// PEM. The certificate is valid and signed by the CA it is called on. The
// result can directly be passed to tls.X509KeyPair.
func (ca *CA) NewServerCertificate(t *testing.T) (cert []byte, key []byte) {
	cert, key, err := ca.newCertificate([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	if err != nil {
		t.Fatalf("new server cert: %s", err.Error())
	}

	return cert, key
}

// NewClientCertificate returns a new certificate and private key encoded as
// PEM. The certificate is valid and signed by the CA it is called on. The
// result can directly be passed to tls.X509KeyPair.
func (ca *CA) NewClientCertificate(t *testing.T) (cert []byte, key []byte) {
	cert, key, err := ca.newCertificate([]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	if err != nil {
		t.Fatalf("new client cert: %s", err.Error())
	}

	return cert, key
}

func (ca *CA) newCertificate(usages []x509.ExtKeyUsage) (cert []byte, key []byte, err error) {
	template := &x509.Certificate{
		IsCA:               true,
		KeyUsage:           x509.KeyUsageCertSign,
		SignatureAlgorithm: x509.SHA256WithRSA,
		NotAfter:           time.Now().Add(time.Hour),
		NotBefore:          time.Now(),
		Subject: pkix.Name{
			Country:            []string{"DE"},
			Organization:       []string{"harald"},
			OrganizationalUnit: []string{"server"},
		},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		SerialNumber: big.NewInt(ca.nextSerialNumber),
		ExtKeyUsage:  usages,
	}
	ca.nextSerialNumber++

	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}

	certDer, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &priv.PublicKey, ca.key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDer,
		}), pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		}), nil
}
