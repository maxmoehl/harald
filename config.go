package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	t, err := time.ParseDuration(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*d = duration(t)
	return nil
}

func (d *duration) Duration() time.Duration {
	if d == nil {
		return 0
	}
	return time.Duration(*d)
}

type Config struct {
	Version
	LogLevel        slog.Level    `json:"log_level" yaml:"log_level"`
	DialTimeout     duration      `json:"dial_timeout" yaml:"dial_timeout"`
	EnableListeners bool          `json:"enable_listeners" yaml:"enable_listeners"`
	TLS             *TLS          `json:"tls" yaml:"tls"`
	Rules           []ForwardRule `json:"rules" yaml:"rules"`
}

type Version struct {
	Version *int `json:"version" yaml:"version"`
}

func (v Version) Get() int {
	if v.Version == nil {
		return 1
	}

	return *v.Version
}

type ForwardRule struct {
	// Listen parameters to listen for new connections.
	Listen NetConf `json:"listen" yaml:"listen"`
	// Connect parameters
	Connect NetConf `json:"connect" yaml:"connect"`
}

// Forwarder creates the matching Forwarder to the rule and the given
// additional parameters. If tlsConf is nil no TLS will be used to listen for
// new connections.
func (r ForwardRule) Forwarder(tlsConf *tls.Config, dialTimeout time.Duration) *Forwarder {
	return &Forwarder{
		ForwardRule: r,
		tlsConf:     tlsConf,
		timeout:     dialTimeout,
	}
}

type NetConf struct {
	Network string `json:"network" yaml:"network"`
	Address string `json:"address" yaml:"address"`
}

// TLS configuration for the server side.
type TLS struct {
	Certificate string `json:"certificate" yaml:"certificate"`
	Key         string `json:"key" yaml:"key"`
	ClientCAs   string `json:"client_cas" yaml:"client_cas"`
	KeyLogFile  string `json:"key_log_file" yaml:"key_log_file"`
}

func (t *TLS) Config() (conf *tls.Config, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("tls config: %w", err)
		}
	}()

	if t == nil {
		return nil, nil
	}
	conf = &tls.Config{}

	// set up TLS keylogger
	if t.KeyLogFile != "" {
		slog.Warn("enabling logging of tls session keys")

		conf.KeyLogWriter, err = os.OpenFile(t.KeyLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("create key log writer: %w", err)
		}
	}

	// parse certificate and key
	cert, err := tls.X509KeyPair([]byte(t.Certificate), []byte(t.Key))
	if err != nil {
		return nil, fmt.Errorf("parse certificate and private key: %w", err)
	}
	conf.Certificates = []tls.Certificate{cert}

	// parse client certificate authorities
	var block *pem.Block
	var certs int

	conf.ClientCAs = x509.NewCertPool()
	d := []byte(t.ClientCAs)

	for len(d) > 0 {
		block, d = pem.Decode(d)
		if block == nil {
			return nil, fmt.Errorf("found unparsable section in client CAs '%s'", string(d))
		}

		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("unexpected block '%s' in client CAs", block.Type)
		}

		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		certs++
		conf.ClientCAs.AddCert(c)
	}

	if t.ClientCAs != "" && certs == 0 {
		// there was something configured, but we didn't pick up any certs
		return nil, fmt.Errorf("unable to parse provided client CAs")
	}

	if certs > 0 {
		conf.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return conf, nil
}

func loadConfig(path string) (Config, error) {
	r, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	parts := strings.Split(path, ".")

	var c Config
	switch parts[len(parts)-1] {
	case "yaml", "yml":
		err = yaml.NewDecoder(r).Decode(&c)
	case "json":
		err = json.NewDecoder(r).Decode(&c)
	default:
		err = fmt.Errorf("unknown file extension '%s'", parts[len(parts)-1])
	}
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	if c.Version.Get() != 1 {
		return Config{}, fmt.Errorf("load config: unknown version '%d'", c.Version)
	}

	return c, nil
}
