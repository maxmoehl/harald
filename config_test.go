package harald

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func Test_duration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		b       []byte
		want    time.Duration
		wantErr bool
	}{
		{
			[]byte(`"1m"`),
			time.Minute,
			false,
		},
		{
			[]byte(`"1mm"`),
			0,
			true,
		},
		{
			[]byte(``),
			0,
			true,
		},
		{
			[]byte(`""`),
			0,
			true,
		},
		{
			[]byte(`1mm`),
			0,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.b), func(t *testing.T) {
			var d Duration

			err := json.Unmarshal(tt.b, &d)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if time.Duration(d) != tt.want {
				t.Errorf("UnmarshalJSON() d = %v, want %v", d, tt.want)
			}
		})
	}
}

//go:embed examples/config.v2.json
var exampleConfigJson []byte

//go:embed examples/config.v2.toml
var exampleConfigToml []byte

//go:embed examples/config.v2.yml
var exampleConfigYaml []byte

func TestLoadConfig(t *testing.T) {
	exampleConfig := Config{
		Version:         2,
		LogLevel:        slog.LevelDebug,
		DialTimeout:     Duration(10 * time.Millisecond),
		EnableListeners: true,
		Rules: map[string]ForwardRule{
			"http": {
				DialTimeout: Duration(5 * time.Millisecond),
				Listen: NetConf{
					Network: "tcp",
					Address: ":60001",
				},
				Connect: NetConf{
					Network: "tcp",
					Address: "localhost:8080",
				},
				TLS: &TLS{
					Certificate:          "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
					Key:                  "-----BEGIN EC PRIVATE KEY-----\n...\n-----END EC PRIVATE KEY-----",
					ClientCAs:            "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
					ApplicationProtocols: []string{"h2", "http/1.1"},
				},
			},
		},
	}
	tests := map[string][]byte{
		"json": exampleConfigJson,
		"toml": exampleConfigToml,
		"yaml": exampleConfigYaml,
		"yml":  exampleConfigYaml,
	}
	for ext, data := range tests {
		t.Run(ext, func(t *testing.T) {
			w, err := os.CreateTemp("", fmt.Sprintf("harald-test-*-.%s", ext))
			if err != nil {
				t.Fatalf("create temporary file: %s", err.Error())
			}
			defer w.Close()
			defer os.Remove(w.Name())

			_, err = w.Write(data)
			if err != nil {
				t.Fatalf("write test data: %s", err.Error())
			}
			_ = w.Close()

			got, err := LoadConfig(w.Name())
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if exampleConfig.Version != got.Version {
				t.Fatalf("Config.Version: want = %v; got = %v", exampleConfig.Version, got.Version)
			}
			if exampleConfig.DialTimeout != got.DialTimeout {
				t.Fatalf("Config.DialTimeout: want = %v; got = %v", exampleConfig.DialTimeout, got.DialTimeout)
			}
		})
	}
}
