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

type fileCfg struct {
	config
	filename string
}

func file() *ffcli.Command {
	cfg := fileCfg{}
	fs := flag.NewFlagSet(fileCLI, flag.ExitOnError)
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")
	fs.StringVar(&cfg.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&cfg.filename, "filename", "", "filename to read data (required)")

	return &ffcli.Command{
		Name:       fileCLI,
		ShortUsage: fileCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			cfg.Log = defaultLogger(cfg.LogLevel)
			cfg.Log.V(0).Info("starting ipxe", "tftp-addr", cfg.TFTPAddr, "http-addr", cfg.HTTPAddr)

			saData, err := ioutil.ReadFile(cfg.filename)
			if err != nil {
				return errors.Wrapf(err, "could not read file %q", cfg.filename)
			}
			dsDB := []*hardware.Hardware{}
			if err := json.Unmarshal(saData, &dsDB); err != nil {
				return errors.Wrapf(err, "unable to parse configuration file %q", cfg.filename)
			}

			f := &backend.File{DB: dsDB}

			g, ctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return tftp.ServeTFTP(ctx, cfg.Log, f, cfg.TFTPAddr)
			})

			g.Go(func() error {
				return http.ListenAndServe(ctx, cfg.Log, f, cfg.HTTPAddr)
			})

			return g.Wait()
		},
	}
}
