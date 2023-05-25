package main

import "time"

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
	DialTimeout duration      `json:"dial_timeout"`
	TLS         *TLS          `json:"tlsConf"`
	Rules       []ForwardRule `json:"rules"`
}

type TLS struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
	ClientCAs   string `json:"client_cas"`
}

type NetConf struct {
	Network string `json:"network"`
	Address string `json:"address"`
}
