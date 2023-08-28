package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxmoehl/harald"
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

	c, err := harald.LoadConfig(args[1])
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Until here we always only log INFO and higher, from now on we can use
	// all levels.
	logLevel.Set(c.LogLevel)

	return harald.Harald(c, signals)
}
