package harald

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"golang.org/x/exp/slog"
)

type duration time.Duration

func (d *duration) UnmarshalText(b []byte) error {
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
	ConfigVersion
	LogLevel        slog.Level    `json:"log_level" yaml:"log_level"`
	DialTimeout     duration      `json:"dial_timeout" yaml:"dial_timeout"`
	EnableListeners bool          `json:"enable_listeners" yaml:"enable_listeners"`
	TLS             *TLS          `json:"tls" yaml:"tls"`
	Rules           []ForwardRule `json:"rules" yaml:"rules"`
}

type ConfigVersion struct {
	Version *int `json:"version" yaml:"version"`
}

func (v ConfigVersion) Get() int {
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
	// ApplicationProtocols offered via ALPN in order of preference. See the
	// IANA registry for a list of options:
	// https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml#alpn-protocol-ids
	ApplicationProtocols []string `json:"application_protocols" yaml:"application_protocols"`
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

	conf = &tls.Config{
		NextProtos: t.ApplicationProtocols,
		ClientCAs:  x509.NewCertPool(),
	}

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
