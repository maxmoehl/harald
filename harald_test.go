package harald

import (
	"context"
	"crypto/tls"
	"net"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/maxmoehl/harald/haraldtest"
)

// TestClosingListenerDoesntCloseConnection ensures that active connections
// are not affected by the listener being closed.
func TestClosingListenerDoesntCloseConnection(t *testing.T) {
	r := ForwardRule{
		Listen: NetConf{
			Network: "tcp",
			Address: "127.0.0.1:0",
		},
		Connect: NetConf{
			Network: "tcp",
			Address: haraldtest.EchoChamber(t),
		},
	}

	forwarder, err := r.NewForwarder("test", 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	err = forwarder.Start()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer forwarder.Stop()

	conn, err := net.Dial("tcp", forwarder.listener.Addr().String())
	if err != nil {
		t.Fatal(err.Error())
	}

	writePayload := []byte("foobar")
	written, err := conn.Write(writePayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != len(writePayload) {
		t.Fatal("written bytes don't match")
	}

	readPayload := make([]byte, len(writePayload))
	read, err := conn.Read(readPayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != read {
		t.Fatal("read bytes don't match written bytes")
	}

	forwarder.Stop()

	writePayload = []byte("foobar")
	written, err = conn.Write(writePayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != len(writePayload) {
		t.Fatal("written bytes don't match")
	}

	readPayload = make([]byte, len(writePayload))
	read, err = conn.Read(readPayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != read {
		t.Fatal("read bytes don't match written bytes")
	}
}

// TestNoUpstreamConnection ensures that a client is unable to start a
// TLS handshake with harald if harald is unable to connect to its target.
func TestNoUpstreamConnection(t *testing.T) {
	r := ForwardRule{
		Listen: NetConf{
			Network: "tcp",
			Address: "127.0.0.1:0",
		},
		Connect: NetConf{
			Network: "tcp",
			Address: "127.0.0.1:0",
		},
	}

	forwarder, err := r.NewForwarder("test", 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	// Technically we need a TLS config, but since there is no upstream
	// connection we expect that the connection never gets to the TLS
	// handshake.
	forwarder.tlsConf = &tls.Config{}

	err = forwarder.Start()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer forwarder.Stop()

	var trace struct{ TlsHandshakeStart bool }
	traceCtx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
		TLSHandshakeStart: func() { trace.TlsHandshakeStart = true },
	})

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: time.Second},
		Config:    &tls.Config{InsecureSkipVerify: true},
	}

	conn, err := dialer.DialContext(traceCtx, "tcp", forwarder.listener.Addr().String())
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected TLS dial to fail")
	}
	if trace.TlsHandshakeStart == true {
		t.Fatal("did not expect to be able to start the TLS handshake")
	}
}

func TestTlsTermination(t *testing.T) {
	ca := haraldtest.NewCertificateAuthority(t)
	crt, key := ca.NewServerCertificate(t)

	r := ForwardRule{
		Listen: NetConf{
			Network: "tcp",
			Address: "127.0.0.1:0",
		},
		Connect: NetConf{
			Network: "tcp",
			Address: haraldtest.EchoChamber(t),
		},
		TLS: &TLS{
			Certificate: string(crt),
			Key:         string(key),
		},
	}

	forwarder, err := r.NewForwarder("test", 0)
	if err != nil {
		t.Fatal(err.Error())
	}

	err = forwarder.Start()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer forwarder.Stop()

	conn, err := tls.Dial("tcp", forwarder.listener.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	writePayload := []byte("foobar")
	written, err := conn.Write(writePayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != len(writePayload) {
		t.Fatal("written bytes don't match")
	}

	readPayload := make([]byte, len(writePayload))
	read, err := conn.Read(readPayload)
	if err != nil {
		t.Fatal(err.Error())
	}
	if written != read {
		t.Fatal("read bytes don't match written bytes")
	}

	forwarder.Stop()
}
