package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/backend"
	"github.com/jacobweinstock/ipxe/http"
	"github.com/jacobweinstock/ipxe/tftp"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"golang.org/x/sync/errgroup"
)

const fileCLI = "file"

type FileCfg struct {
	Config
	Filename string
}

func File() (*ffcli.Command, *FileCfg) {
	cfg := &FileCfg{}
	fs := flag.NewFlagSet(fileCLI, flag.ExitOnError)
	RegisterFlagsFile(cfg, fs)

	return &ffcli.Command{
		Name:       fileCLI,
		ShortUsage: fileCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			return cfg.Exec(ctx, nil)
		},
	}, cfg
}

func RegisterFlagsFile(cfg *FileCfg, fs *flag.FlagSet) {
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")
	fs.StringVar(&cfg.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&cfg.Filename, "filename", "", "filename to read data (required)")
}

type Opt func(*FileCfg)

func WithLogger(log logr.Logger) Opt {
	return func(cfg *FileCfg) {
		cfg.Log = log
	}
}

func WithFilename(filename string) Opt {
	return func(cfg *FileCfg) {
		cfg.Filename = filename
	}
}

func WithTFTPAddr(tftpAddr string) Opt {
	return func(cfg *FileCfg) {
		cfg.TFTPAddr = tftpAddr
	}
}

func WithHTTP(addr string) Opt {
	return func(cfg *FileCfg) {
		cfg.HTTPAddr = addr
	}
}

func WithLogLevel(level string) Opt {
	return func(cfg *FileCfg) {
		cfg.LogLevel = level
	}
}

func NewFile(opts ...Opt) *FileCfg {
	c := &FileCfg{
		Config: Config{
			Log:      logr.Discard(),
			TFTPAddr: "0.0.0.0:69",
			HTTPAddr: "0.0.0.0:8080",
			LogLevel: "info",
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (f *FileCfg) Exec(ctx context.Context, _ []string) error {
	if f.Log.GetSink() == nil {
		f.Log = logr.Discard()
	}

	f.Log.Info("starting ipxe", "tftp-addr", f.TFTPAddr, "http-addr", f.HTTPAddr)

	saData, err := ioutil.ReadFile(f.Filename)
	if err != nil {
		return errors.Wrapf(err, "could not read file %q", f.Filename)
	}
	dsDB := []*hardware.Hardware{}
	if err := json.Unmarshal(saData, &dsDB); err != nil {
		return errors.Wrapf(err, "unable to parse configuration file %q", f.Filename)
	}

	fb := &backend.File{DB: dsDB}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return tftp.ServeTFTP(ctx, f.Log, fb, f.TFTPAddr)
	})

	g.Go(func() error {
		return http.ListenAndServe(ctx, f.Log, fb, f.HTTPAddr)
	})

	return g.Wait()
}
