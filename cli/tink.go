package cli

import (
	"context"
	"flag"

	"github.com/jacobweinstock/ipxe-bin/backend"
	"github.com/jacobweinstock/ipxe-bin/http"
	"github.com/jacobweinstock/ipxe-bin/tftp"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/tinkerbell/tink/protos/hardware"
	"golang.org/x/sync/errgroup"
)

const tinkCLI = "tink"

type tinkCfg struct {
	config
	// TLS can be one of the following
	// 1. location on disk of a cert
	// example: /location/on/disk/of/cert
	// 2. URL from which to GET a cert
	// example: http://weburl:8080/cert
	// 3. boolean; true if the tink server (specified by the Tink key/value) has a cert from a known CA
	// false if the tink server does not have TLS enabled
	// example: true
	TLS string
	// Tink is the URL:Port for the tink server
	Tink string `validate:"required"`
}

func tink() *ffcli.Command {
	cfg := tinkCfg{}
	fs := flag.NewFlagSet(tinkCLI, flag.ExitOnError)
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")
	fs.StringVar(&cfg.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&cfg.Tink, "tink", "", "tink server URL (required)")
	description := "(file:///path/to/cert/tink.cert, http://tink-server:42114/cert, boolean (false - no TLS, true - tink has a cert from known CA) (optional)"
	fs.StringVar(&cfg.TLS, "tls", "false", "tink server TLS "+description)

	return &ffcli.Command{
		Name:       tinkCLI,
		ShortUsage: tinkCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			cfg.Log = defaultLogger(cfg.LogLevel)
			cfg.Log.V(0).Info("starting ipxe-bin", "tftp-addr", cfg.TFTPAddr, "http-addr", cfg.HTTPAddr)
			g, ctx := errgroup.WithContext(ctx)
			gc, err := backend.SetupClient(ctx, cfg.Log, cfg.TLS, cfg.Tink)
			if err != nil {
				return err
			}
			c := hardware.NewHardwareServiceClient(gc)
			/*if _, err := c.ByIP(ctx, &hardware.GetRequest{}); err != nil {
				return fmt.Errorf("unable to communicate with tink server: %v", cfg.Tink)
			}*/
			g.Go(func() error {
				
				return tftp.ServeTFTP(ctx, cfg.Log, &backend.Tinkerbell{Client: c, Log: cfg.Log}, cfg.TFTPAddr)
			})

			g.Go(func() error {
				return http.ListenAndServe(ctx, cfg.Log, &backend.Tinkerbell{Client: c, Log: cfg.Log}, cfg.HTTPAddr)
			})

			return g.Wait()
		},
	}
}
