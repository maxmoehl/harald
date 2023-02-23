// Harald is a great guy. He takes care of forwarding connections and listens
// to your needs. Get him started with SIGUSR1, stop him with SIGUSR2 and shut
// him down for good with SIGTERM.
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

const (
	KeyForwarder = "forwarder"
	KeyError     = "error"
	KeySignal    = "signal"
	KeyPid       = "pid"
)

func init() {
	slog.SetDefault(slog.New(slog.HandlerOptions{
		AddSource:   false,
		Level:       level(slog.LevelDebug),
		ReplaceAttr: nil,
	}.NewJSONHandler(os.Stdout)))
}

func main() {
	slog.Info("Harald is getting started", KeyPid, os.Getpid())

	var err error
	defer func() {
		if err != nil {
			slog.Error("fatal error", err)
			os.Exit(1)
		}
	}()
	if len(os.Args) != 2 {
		err = fmt.Errorf("please provide the config file as first and only argument")
		return
	}

	configReader, err := os.Open(os.Args[1])
	if err != nil {
		return
	}

	var c Config
	err = json.NewDecoder(configReader).Decode(&c)
	if err != nil {
		return
	}

	var tlsConf *tls.Config
	if c.TLS != nil {
		err = fmt.Errorf("tls is not implemented yet")
		return
	}

	interval, err := time.ParseDuration(c.ListenInterval)
	if err != nil {
		return
	}

	var forwarders []*Forwarder
	for _, r := range c.Rules {
		forwarders = append(forwarders, NewForwarder(r, interval, tlsConf))
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	for sig := range signals {
		slog.Info("received signal", KeySignal, sig.String())
		switch sig {
		case syscall.SIGTERM:
			for _, f := range forwarders {
				f.Stop()
			}
			return
		case syscall.SIGUSR1:
			for _, f := range forwarders {
				err = f.Start()
				if err != nil {
					fmt.Printf("failed to start %s: %s\n", f, err.Error())
				}
			}
		case syscall.SIGUSR2:
			for _, f := range forwarders {
				f.Stop()
			}
		}
	}
}

type level slog.Level

func (l level) Level() slog.Level {
	return slog.Level(l)
}

type Config struct {
	Rules          []ForwardRule `json:"rules"`
	TLS            *TLS          `json:"tls"`
	Debug          bool          `json:"debug"`
	ListenInterval string        `json:"listen_interval"`
}

type TLS struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
	ClientCAs   string `json:"clientCAs"`
}

type ForwardRule struct {
	Listen  string `json:"listen"`
	Connect string `json:"connect"`
}

type Forwarder struct {
	ForwardRule
	l        net.Listener
	interval time.Duration
	tls      *tls.Config
}

func NewForwarder(rule ForwardRule, interval time.Duration, tlsConf *tls.Config) *Forwarder {
	return &Forwarder{
		ForwardRule: rule,
		interval:    interval,
		tls:         tlsConf,
	}
}

// Start opens a new listener.
func (f *Forwarder) Start() error {
	if f.l != nil {
		slog.Debug("listener already open, not starting again", KeyForwarder, f.String())
		return nil
	}
	slog.Debug("starting listener", KeyForwarder, f.String())

	var err error
	if f.tls == nil {
		f.l, err = net.Listen("tcp", f.Listen)
	} else {
		f.l, err = tls.Listen("tcp", f.Listen, f.tls)
	}
	if err != nil {
		return err
	}

	go func() {
		for {
			c, err := f.l.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					// net.ErrClosed is expected in cases where we shut down the listener so
					// this is not considered a real error but the clean exit case.
					return
				} else {
					// otherwise we log the error and continue
					// TODO: could this result in a short-circuit where we constantly log the same error?
					slog.Error("unable to accept connection", err, KeyForwarder, f.String(), KeyError, err.Error())
					continue
				}
			}

			go f.handle(c)
		}
	}()

	return nil
}

func (f *Forwarder) handle(source net.Conn) {
	defer func() { _ = source.Close() }()

	target, err := net.Dial("tcp", f.Connect)
	if err != nil {
		return
	}
	defer func() { _ = target.Close() }()

	// we only wait until one end closes the connection. We return after that which
	// runs the defers and closes both connections. This causes the second copy operation
	// to return as well.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer cancel()
		_, _ = io.Copy(source, target)
	}()
	go func() {
		defer cancel()
		_, _ = io.Copy(target, source)
	}()

	<-ctx.Done()
}

// Stop will close the listener if it is open. The reference to the listener is
// also set to nil to prevent further usage.
func (f *Forwarder) Stop() {
	if f.l == nil {
		slog.Debug("listener already cosed", KeyForwarder, f.String())
		return
	}
	slog.Debug("closing listener", KeyForwarder, f.String())

	l := f.l
	f.l = nil
	err := l.Close()
	if err != nil {
		slog.Warn("error while closing listener", KeyForwarder, f.String(), KeyError, err.Error())
	}
}

// String representation of the Forwarder.
func (f *Forwarder) String() string {
	return fmt.Sprintf("Forwarder(%s->%s)", f.Listen, f.Connect)
}
