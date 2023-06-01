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
version: 2 # the version of this config file
log_level: "debug" # see https://pkg.go.dev/log/slog#Level.UnmarshalJSON for details
dial_timeout: "10ms" # default dial_timeout, can be overwritten in a rule, must be in a format that can be parsed by https://pkg.go.dev/time#ParseDuration
enable_listeners: false # whether to start all listeners right away
rules: # the rules for forwarding traffic
  http: { }, # rules have names to be able to identify them in logs etc.
  ssh: { },
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
  # client CAs, when set all clients have to provide a valid client certificate
  client_cas: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```
