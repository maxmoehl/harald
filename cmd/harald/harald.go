package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/maxmoehl/harald"

	"github.com/BurntSushi/toml"
	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

var logLevel = &slog.LevelVar{}

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})).With("component", "harald"))
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	err := Main(os.Args, signals)
	if err != nil {
		slog.Error("fatal error - exiting", "error", err.Error())
		os.Exit(1)
	}
}

func Main(args []string, signals <-chan os.Signal) error {
	slog.Info("Harald is getting started", "pid", os.Getpid())

	if len(args) != 2 {
		return fmt.Errorf("please provide the config file as first and only argument")
	}

	c, err := loadConfig(args[1])
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Until here we always only log INFO and higher, from now on we can use
	// all levels.
	logLevel.Set(c.LogLevel)

	return harald.Harald(c, signals)
}

func loadConfig(path string) (harald.Config, error) {
	r, err := os.Open(path)
	if err != nil {
		return harald.Config{}, fmt.Errorf("load config: %w", err)
	}

	parts := strings.Split(path, ".")

	var c harald.Config
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
		return harald.Config{}, fmt.Errorf("load config: %w", err)
	}

	if c.Version.Get() != 2 {
		return harald.Config{}, fmt.Errorf("load config: unknown version '%d'", c.Version)
	}

	return c, nil
}
