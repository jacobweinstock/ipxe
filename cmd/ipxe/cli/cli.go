package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/imdario/mergo"
	"github.com/jacobweinstock/ipxe"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"inet.af/netaddr"
)

const rootCLI = "ipxe"

// Config is the configuration for the ipxe CLI.
type Config struct {
	TFTPAddr string
	HTTPAddr string
	LogLevel string
	Log      logr.Logger
}

// IpxeBin returns the CLI command for the ipxe CLI app.
func IpxeBin() *ffcli.Command {
	cfg := &Config{}
	fs := flag.NewFlagSet(rootCLI, flag.ExitOnError)
	RegisterFlags(cfg, fs)
	return &ffcli.Command{
		Name:       rootCLI,
		ShortUsage: rootCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			return cfg.Exec(ctx, nil)
		},
	}
}

// RegisterFlags registers the flags for the ipxe CLI app.
func RegisterFlags(cfg *Config, fs *flag.FlagSet) {
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")
	fs.StringVar(&cfg.LogLevel, "loglevel", "info", "log level (optional)")
}

// Exec is the main entry point for the ipxe CLI app.
func (f *Config) Exec(ctx context.Context, _ []string) error {
	defaults := Config{
		TFTPAddr: "0.0.0.0:69",
		HTTPAddr: "0.0.0.0:8080",
		LogLevel: "info",
		Log:      defaultLogger("info"),
	}
	err := mergo.Merge(f, defaults)
	if err != nil {
		return err
	}
	if f.Log.GetSink() == nil {
		f.Log = defaultLogger(f.LogLevel)
	}
	f.Log = f.Log.WithName("ipxe")

	tAddr, err := netaddr.ParseIPPort(f.TFTPAddr)
	if err != nil {
		return errors.Wrapf(err, "could not parse tftp-addr %q", f.TFTPAddr)
	}
	hAddr, err := netaddr.ParseIPPort(f.HTTPAddr)
	if err != nil {
		return errors.Wrapf(err, "could not parse http-addr %q", f.HTTPAddr)
	}
	f.Log.Info("starting ipxe", "tftp-addr", f.TFTPAddr, "http-addr", f.HTTPAddr)
	c := ipxe.Config{
		TFTP: ipxe.TFTP{Addr: tAddr},
		HTTP: ipxe.HTTP{Addr: hAddr},
		Log:  f.Log,
	}
	return c.Serve(ctx)
}

// defaultLogger is zap logr implementation.
func defaultLogger(level string) logr.Logger {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	zapLogger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}

	return zapr.NewLogger(zapLogger)
}
