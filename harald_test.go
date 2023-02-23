package main

import (
	"io"
	"net"
	"testing"
)

func echoChamber(listen string, t *testing.T) {
	t.Helper()
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		t.Fatal(err.Error())
	}
	defer func() { _ = listener.Close() }()

	conn, err := listener.Accept()
	if err != nil {
		t.Fatalf("unable to accept connection: %s", err.Error())
	}

	defer func() { _ = conn.Close() }()

	_, _ = io.Copy(conn, conn)
}

func TestClosingListenerDoesntCloseConnection(t *testing.T) {
	forwarder := Forwarder{
		ForwardRule: ForwardRule{
			Listen:  ":60001",
			Connect: "localhost:60002",
		},
		l:        nil,
		interval: 0,
		tls:      nil,
	}

	go echoChamber(":60002", t)

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
