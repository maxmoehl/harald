//go:build unix

// Package harald contains the core logic of harald.
//
// Harald is a great guy. He takes care of forwarding connections and listens
// to your needs. Get him started with SIGUSR1, stop him with SIGUSR2 and shut
// him down for good with SIGTERM. Currently only unix-like systems (as
// determined by the go build constraint `unix`) are supported due to the
// dependency to the process signals.
//
// Any logging is done through the default logger of log/slog. Consult the
// documentation for how to configure it.
package harald

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// Helpers to format common logging fields consistently.
var (
	attrBytesWritten = func(n int64) slog.Attr { return slog.Int64("bytes-written", n) }
	attrConnId       = func(id uuid.UUID) slog.Attr { return slog.Any("conn-id", fmt.Stringer(id)) }
	attrError        = func(err error) slog.Attr { return slog.String("error", err.Error()) }
	attrForwarder    = func(f *Forwarder) slog.Attr { return slog.Any("forwarder", fmt.Stringer(f)) }
	attrSignal       = func(s os.Signal) slog.Attr { return slog.String("signal", s.String()) }
)

// Harald is the main entrypoint. The config controls the behaviour and the
// signals channel is used to bring up / shut down the listeners and stop the
// execution. The channel should be subscribed to SIGTERM, SIGUSR1 and SIGUSR2.
func Harald(c Config, signals <-chan os.Signal) (err error) {
	var forwarders Forwarders

	var f *Forwarder
	for name, r := range c.Rules {
		f, err = r.NewForwarder(name, c.DialTimeout.Duration())
		if err != nil {
			return fmt.Errorf("harald: %w", err)
		}
		forwarders = append(forwarders, f)
	}

	if len(forwarders) == 0 {
		return fmt.Errorf("harald: no forwarders configured")
	}

	slog.Info("harald is ready")

	if c.EnableListeners {
		forwarders.Start()
		slog.Info("started listeners")
	}

	for sig := range signals {
		slog.Info("received signal", attrSignal(sig))

		switch sig {
		case syscall.SIGTERM:
			slog.Info("shutting down")
			forwarders.Stop()
			slog.Info("stopped listeners")
			return nil // cannot break because of the switch
		case syscall.SIGUSR1:
			forwarders.Start()
			slog.Info("started listeners")
		case syscall.SIGUSR2:
			forwarders.Stop()
			slog.Info("stopped listeners")
		default:
			slog.Debug("ignoring unknown signal", attrSignal(sig))
		}
	}

	return nil
}

type Forwarder struct {
	ForwardRule
	name     string
	listener net.Listener
	tlsConf  *tls.Config
	timeout  time.Duration
	log      *slog.Logger
}

// Start opens a new listener.
func (f *Forwarder) Start() (err error) {
	if f.listener != nil {
		f.log.Debug("listener already open, not starting again")
		return nil
	}
	f.log.Debug("starting listener")

	f.listener, err = net.Listen(f.Listen.Network, f.Listen.Address)
	if err != nil {
		return err
	}

	go func() {
		for {
			c, err := f.listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					// net.ErrClosed is expected in cases where we shut down the listener so
					// this is not considered a real error but the clean exit case.
					return
				} else {
					// otherwise we log the error and continue
					// TODO: could this result in a short-circuit where we constantly log the same error?
					f.log.Error("unable to accept connection", attrError(err))
					continue
				}
			}

			go f.handle(c)
		}
	}()

	return nil
}

func (f *Forwarder) handle(source net.Conn) {
	log := f.log.With(attrConnId(uuid.Must(uuid.NewRandom())))
	log.Debug("handle start")

	defer func() { _ = source.Close() }()

	target, err := net.DialTimeout(f.Connect.Network, f.Connect.Address, f.timeout)
	if err != nil {
		log.Error("connecting upstream failed", attrError(err))
		return
	}
	defer func() { _ = target.Close() }()

	log.Debug("established upstream connection")

	// only after the tcp connection could be established upstream we add TLS
	// to the connection.
	if f.tlsConf != nil {
		source = tls.Server(source, f.tlsConf)
	}

	// we only wait until one end closes the connection. We return after that
	// which runs the defers and closes both connections. This causes the
	// second copy operation to return as well.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer cancel()
		log.Debug("copy source->target started")
		n, err := io.Copy(target, source)
		if err != nil {
			log.Error("copy source->target stopped", attrBytesWritten(n), attrError(err))
		} else {
			log.Debug("copy source->target stopped", attrBytesWritten(n))
		}

	}()

	go func() {
		defer cancel()
		log.Debug("copy target->source started")
		n, err := io.Copy(source, target)
		if err != nil {
			log.Error("copy target->source stopped", attrBytesWritten(n), attrError(err))
		} else {
			log.Debug("copy target->source stopped", attrBytesWritten(n))
		}
	}()

	<-ctx.Done()
	log.Debug("handle done")
}

// Stop will close the listener if it is open. The reference to the listener is
// also set to nil to prevent further usage.
// TODO: does this need explicit synchronization?
func (f *Forwarder) Stop() {
	if f.listener == nil {
		f.log.Debug("listener already cosed")
		return
	}
	f.log.Debug("closing listener")

	// First, we copy the pointer, then we set the listener to nil. The case
	// in which this happens twice and one of the routines gets nil is handled
	// below.
	l := f.listener
	f.listener = nil

	if l == nil {
		// Since we do not properly synchronize we have the risk that two calls
		// to stop run in parallel. In such cases the if at the beginning of
		// the function is not enough to prevent us from still getting a nil
		// listener which would cause a panic when we try to call Close on it.
		f.log.Warn("detected double stop, ignoring second stop")
		return
	}

	err := l.Close()
	if err != nil {
		// Only a warning because the listener is closed in any case.
		f.log.Warn("error while closing listener", attrError(err))
	}
}

// String representation of the Forwarder. The format of the addresses is
// inspired by the '-i' argument of lsof.
func (f *Forwarder) String() string {
	return fmt.Sprintf("Forwarder(%s; %s@%s->%s@%s)",
		f.name, f.Listen.Network, f.Listen.Address, f.Connect.Network, f.Connect.Address)
}

// Forwarders maintains a list of pointers to Forwarder. It holds pointers
// because each struct may maintain data that can not be copied.
type Forwarders []*Forwarder

// Start all forwarders in the list. Logs errors encountered while starting a
// forwarder but continues starting the forwarders.
func (forwarders Forwarders) Start() {
	var err error
	for _, f := range forwarders {
		err = f.Start()
		if err != nil {
			f.log.Error("failed to start forwarder", attrError(err))
		}
	}
}

// Stop all forwarders in the list.
func (forwarders Forwarders) Stop() {
	for _, f := range forwarders {
		f.Stop()
	}
}
