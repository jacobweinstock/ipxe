package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"

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

func (f *FileCfg) Exec(ctx context.Context, _ []string) error {
	f.Log = defaultLogger(f.LogLevel)
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
