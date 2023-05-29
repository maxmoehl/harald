package harald

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http/httptrace"
	"testing"
	"time"
)

// echoChamber starts a new listener on a random port on the loopback address
// and returns the address it is listening on. The listener is only active for
// the first connection that is established to it. Any data received on the
// connection is sent back through it.
func echoChamber(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err.Error())
	}

	go func() {
		defer func() { _ = listener.Close() }()

		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("unable to accept connection: %s", err.Error())
			return
		}
		defer func() { _ = conn.Close() }()

		_, _ = io.Copy(conn, conn)
	}()

	return listener.Addr().String()
}

// TestClosingListenerDoesntCloseConnection ensures that active connections
// are not affected by the listener being closed.
func TestClosingListenerDoesntCloseConnection(t *testing.T) {
	forwarder := Forwarder{
		ForwardRule: ForwardRule{
			Listen: NetConf{
				Network: "tcp",
				Address: "127.0.0.1:60001",
			},
			Connect: NetConf{
				Network: "tcp",
				Address: echoChamber(t),
			},
		},
		listener: nil,
		tlsConf:  nil,
	}

	err := forwarder.Start()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer forwarder.Stop()

	conn, err := net.Dial("tcp", "localhost:60001")
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
	forwarder := Forwarder{
		ForwardRule: ForwardRule{
			Listen: NetConf{
				Network: "tcp",
				Address: "127.0.0.1:60001",
			},
			Connect: NetConf{
				Network: "tcp",
				Address: "127.0.0.1:0",
			},
		},
		listener: nil,
		// Technically we need a TLS config, but since there is no upstream
		// connection we expect that the connection never gets to the TLS
		// handshake.
		tlsConf: &tls.Config{},
	}

	err := forwarder.Start()
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

	conn, err := dialer.DialContext(traceCtx, "tcp", "localhost:60001")
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected TLS dial to fail")
	}
	if trace.TlsHandshakeStart == true {
		t.Fatal("did not expect to be able to start the TLS handshake")
	}
}
