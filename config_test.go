package harald

import (
	"encoding/json"
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
