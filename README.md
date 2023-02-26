# harald
(like a herald, but better)

Harald has one goal and one goal only: forward traffic if you want it.

## Config

Each config looks like this:
```json
{
  "dial_timeout": "1s",
  "tls": {},
  "rules": []
}
```
Fields:
- `dial_timeout`: sets the dial timeout for connections to the target. See [time.ParseDuration](https://pkg.go.dev/time#ParseDuration) for the format details.
- `tls`: TLS configuration for the listener, see [TLS](#TLS) for more details.
- `rules`: contains all forwarding rules, see [Rules](#Rules) for more details.

### TLS

The TLS config looks like this:
```json
{
  "certificate": "PEM",
  "key": "PEM",
  "client_cas": "PEM"
}
```

### Rules

A rule looks like this:
```json
{
  "listen": {
    "network": "tcp",
    "address": ":60001"
  },
  "connect": {
    "network": "tcp",
    "address": "localhost:8080"
  }
}
```

The `listen` key specifies how and where to listen and the `connect` settings will be used to connect to the target
address. For more details about the `listen` options see the [net.Listen](https://pkg.go.dev/net#Listen) documentation,
for  the `connect` details see [net.Dial](https://pkg.go.dev/net#Dial).
