package harald

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

var keyLogWriter io.Writer

func init() {
	path, ok := os.LookupEnv("SSLKEYLOGFILE")
	if !ok {
		return
	}
	slog.Warn("enabling logging of tls session keys")
	w, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		panic(fmt.Sprintf("error: unable to open SSLKEYLOGFILE: %s", err.Error()))
	}
	keyLogWriter = w
}

type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	t, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(t)
	return nil
}

func (d *Duration) Duration() time.Duration {
	if d == nil {
		return 0
	}
	return time.Duration(*d)
}

type Config struct {
	Version         int                    `json:"version" yaml:"version" toml:"version"`
	LogLevel        slog.Level             `json:"log_level" yaml:"log_level" toml:"log_level"`
	DialTimeout     Duration               `json:"dial_timeout" yaml:"dial_timeout" toml:"dial_timeout"`
	EnableListeners bool                   `json:"enable_listeners" yaml:"enable_listeners" toml:"enable_listeners"`
	Rules           map[string]ForwardRule `json:"rules" yaml:"rules" toml:"rules"`
}

type ForwardRule struct {
	DialTimeout Duration `json:"dial_timeout" yaml:"dial_timeout" toml:"dial_timeout"`
	Listen      NetConf  `json:"listen" yaml:"listen" toml:"listen"`
	Connect     NetConf  `json:"connect" yaml:"connect" toml:"connect"`
	TLS         *TLS     `json:"tls" yaml:"tls" toml:"tls"`
}

// NewForwarder initialize a new forwarder based on the rule it's called on and
// the additional parameters passed in.
func (r ForwardRule) NewForwarder(name string, defaultDialTimeout time.Duration) (*Forwarder, error) {
	var err error
	f := Forwarder{
		ForwardRule: r,
		name:        name,
		timeout:     defaultDialTimeout,
	}

	f.tlsConf, err = r.TLS.Config()
	if err != nil {
		return nil, fmt.Errorf("new forwarder: %s: %w", name, err)
	}

	if r.DialTimeout != 0 {
		f.DialTimeout = r.DialTimeout
	}

	f.log = slog.With(attrForwarder(&f))

	return &f, nil
}

type NetConf struct {
	Network string `json:"network" yaml:"network"`
	Address string `json:"address" yaml:"address"`
}

// TLS configuration for the server side.
type TLS struct {
	Certificate string `json:"certificate" yaml:"certificate" toml:"certificate"`
	Key         string `json:"key" yaml:"key" toml:"key"`
	ClientCAs   string `json:"client_cas" yaml:"client_cas" toml:"client_cas"`
	// ApplicationProtocols offered via ALPN in order of preference. See the
	// IANA registry for a list of options:
	// https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml#alpn-protocol-ids
	ApplicationProtocols []string `json:"application_protocols" yaml:"application_protocols" toml:"application_protocols"`
	// See the go documentation for details:
	// https://pkg.go.dev/crypto/tls#ClientAuthType
	ClientAuth tls.ClientAuthType `json:"client_auth" yaml:"client_auth" toml:"client_auth"`
}

func (t *TLS) Config() (conf *tls.Config, err error) {
	if t == nil {
		return nil, nil
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("tls config: %w", err)
		}
	}()

	if t.ClientCAs == "" && t.ClientAuth > 0 {
		return nil, fmt.Errorf("configuered client authentication but no client CAs provided")
	}

	conf = &tls.Config{
		NextProtos: t.ApplicationProtocols,
		ClientCAs:  x509.NewCertPool(),
		ClientAuth: t.ClientAuth,
	}

	conf.KeyLogWriter = keyLogWriter

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

	return conf, nil
}

func LoadConfig(path string) (Config, error) {
	r, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	parts := strings.Split(path, ".")

	if len(parts) == 0 {
		return Config{}, fmt.Errorf("load config: file hast no file extension")
	}

	var c Config
	switch parts[len(parts)-1] {
	case "yaml", "yml":
		err = yaml.NewDecoder(r).Decode(&c)
	case "json":
		err = json.NewDecoder(r).Decode(&c)
	case "toml":
		_, err = toml.NewDecoder(r).Decode(&c)
	default:
		err = fmt.Errorf("unknown file extension '%s'", parts[len(parts)-1])
	}
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	if c.Version != 2 {
		return Config{}, fmt.Errorf("load config: unknown version '%d'", c.Version)
	}

	return c, nil
}
