package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe-bin/backend"
	"github.com/jacobweinstock/ipxe-bin/http"
	"github.com/jacobweinstock/ipxe-bin/tftp"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"
)

type config struct {
	TFTPAddr string
	HTTPAddr string
	Log      logr.Logger
}

func IpxeBin() *ffcli.Command {
	appName := "ipxe-bin"
	cfg := config{}
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")

	return &ffcli.Command{
		Name:       "ipxe-bin",
		ShortUsage: "ipxe-bin",
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			fmt.Printf("%+v\n", cfg)
			fmt.Println("done")
			g, ctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return tftp.ServeTFTP(ctx, cfg.Log, &backend.Tinkerbell{}, cfg.TFTPAddr)
			})

			g.Go(func() error {
				return http.ListenAndServe(ctx, cfg.Log, &backend.Tinkerbell{}, cfg.HTTPAddr)
			})

			return g.Wait()
		},
	}
}
