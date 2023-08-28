// Package integration holds the tests that run harald as a block box by
// calling harald.Harald without accessing any internal details.
//
// TODO: find a way to automatically select unused ports
package integration

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/maxmoehl/harald"
	"github.com/maxmoehl/harald/haraldtest"
)

func TestTlsWithClientAuth(t *testing.T) {
	signals := make(chan os.Signal, 1)

	ca := haraldtest.NewCertificateAuthority(t)
	serverCertPem, serverKeyPem := ca.NewServerCertificate(t)

	haraldAddr := "localhost:60001"
	backendAddr := haraldtest.EchoChamber(t)

	go harald.Harald(harald.Config{
		LogLevel:    slog.LevelDebug,
		DialTimeout: harald.Duration(10 * time.Millisecond),
		Rules: map[string]harald.ForwardRule{
			"http": {
				Listen: harald.NetConf{
					Network: "tcp",
					Address: haraldAddr,
				},
				Connect: harald.NetConf{
					Network: "tcp",
					Address: backendAddr,
				},
				TLS: &harald.TLS{
					Certificate: string(serverCertPem),
					Key:         string(serverKeyPem),
					ClientCAs:   string(ca.PEM()),
				},
			}},
	}, signals)

	signals <- syscall.SIGUSR1
	defer func() { signals <- syscall.SIGTERM }()

	// signal processing may take some time
	time.Sleep(100 * time.Millisecond)

	clientCert, err := tls.X509KeyPair(ca.NewClientCertificate(t))
	if err != nil {
		t.Fatalf("load x509 key pair: %s", err.Error())
	}

	clientConf := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      x509.NewCertPool(),
	}

	clientConf.RootCAs.AddCert(ca.Certificate())

	conn, err := tls.Dial("tcp", haraldAddr, clientConf)
	if err != nil {
		t.Fatalf("connect to harald: %s", err.Error())
	}
	defer conn.Close()

	message := []byte("Hello World!")

	written, err := conn.Write(message)
	if err != nil {
		t.Fatalf("write to harald: %s", err.Error())
	}
	if written != len(message) {
		t.Errorf("len(message) = %d != written = %d", len(message), written)
	}

	readBuf := make([]byte, len(message))
	read, err := conn.Read(readBuf)
	if err != nil {
		t.Errorf("read from harald: %s", err.Error())
	}
	if written != read {
		t.Errorf("written = %d != read = %d", len(message), len(readBuf))
	}
	if string(message) != string(readBuf) {
		t.Errorf("message = '%s' != read = '%s'", string(message), string(readBuf))
	}
}
