# harald

(like a herald, but better)

Harald has one goal and one goal only: forward traffic if you want it.

## Config

The config can be written in:

- YAML (`*.yml` or `*.yaml`)
- JSON (`*.json`)
- TOML (`*.toml`)

Version 1 has been deprecated and is no longer accepted.

See the `examples/` directory for full config examples.

Config version 2 is structured like this:

```yaml
# Version of this config file.
version: 2
# See https://pkg.go.dev/log/slog#Level.UnmarshalJSON for details.
log_level: "debug"
# Default dial_timeout, can be overwritten in a rule, must be in a format that
# can be parsed by https://pkg.go.dev/time#ParseDuration.
dial_timeout: "10ms"
# Whether to start all listeners right away.
enable_listeners: false
# The rules for forwarding traffic, each rule has a name which will be used for
# logging.
rules:
  http: { }
  ssh: { }
```

### Rules

A rule looks like this:

```yaml
# the two arguments passed to https://pkg.go.dev/net#Listen
listen:
  network: tcp
  address: :60001
# the two arguments passed to https://pkg.go.dev/net#Dial
connect:
  network: tcp
  address: localhost:8080
# configuration for server-side TLS
tls:
  # protocols offered via the ALPN TLS extension
  application_protocols: [ "http/1.1", "h2" ]
  # the level as described at https://pkg.go.dev/crypto/tls#ClientAuthType
  client_auth: 5
  # server certificate as PEM encoded
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  # key for the server certificate
  key: |
    -----BEGIN EC PRIVATE KEY-----
    ...
    -----END EC PRIVATE KEY-----
  # client CAs, will be used according to the client_auth level
  client_cas: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```
