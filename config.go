package main

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

func (d *duration) UnmarshalJSON(b []byte) error {
	t, err := time.ParseDuration(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*d = duration(t)
	return nil
}

type Config struct {
	DialTimeout duration      `json:"dial_timeout" yaml:"dial_timeout"`
	LogLevel    slog.Level    `json:"log_level" yaml:"log_level"`
	TLS         *TLS          `json:"tls" yaml:"tls"`
	Rules       []ForwardRule `json:"rules" yaml:"rules"`
}

// TLS configuration for the server side.
type TLS struct {
	Certificate string `json:"certificate" yaml:"certificate"`
	Key         string `json:"key" yaml:"key"`
	ClientCAs   string `json:"client_cas" yaml:"client_cas"`
	KeyLogFile  string `json:"key_log_file" yaml:"key_log_file"`
}

func (t *TLS) Config() (conf *tls.Config, err error) {
	if t == nil {
		return nil, nil
	}
	conf = &tls.Config{}

	conf.ClientCAs = x509.NewCertPool()

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
		return nil, fmt.Errorf("unable to parse certificate and private key: %w", err)
	}
	conf.Certificates = []tls.Certificate{cert}

	// parse client certificate authorities
	var block *pem.Block
	var certs int
	d := []byte(t.ClientCAs)
	for len(d) > 0 {
		block, d = pem.Decode(d)
		if block == nil {
			return nil, fmt.Errorf("found unparsable section in client ca list '%s'", string(d))
		}

		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("unexpected block '%s' in client ca list", block.Type)
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
		return nil, fmt.Errorf("unable to parse provided client certificate authorities")
	}

	if certs > 0 {
		conf.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return conf, nil
}

type NetConf struct {
	Network string `json:"network" yaml:"network"`
	Address string `json:"address" yaml:"address"`
}
