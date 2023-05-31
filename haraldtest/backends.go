package haraldtest

import (
	"io"
	"net"
	"testing"
)

// EchoChamber starts a new listener on a random port on the loopback address
// and returns the address it is listening on. The listener is only active for
// the first connection that is established to it. Any data received on the
// connection is sent back through it.
func EchoChamber(t *testing.T) string {
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
