package cli

import (
	"context"
	"flag"

	"github.com/jacobweinstock/ipxe"
	"github.com/jacobweinstock/ipxe/backend/tink"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"inet.af/netaddr"
)

const tinkCLI = "tink"

type TinkCfg struct {
	Config
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

func Tink() (*ffcli.Command, *TinkCfg) {
	cfg := &TinkCfg{}
	fs := flag.NewFlagSet(tinkCLI, flag.ExitOnError)
	RegisterFlagsTink(cfg, fs)
	sf := fs.Lookup("tink")
	nfs := flag.NewFlagSet("testing", flag.ExitOnError)
	nfs.StringVar(&cfg.Tink, sf.Name, sf.DefValue, sf.Usage)

	return &ffcli.Command{
		Name:       tinkCLI,
		ShortUsage: tinkCLI,
		FlagSet:    fs,
		Exec: func(ctx context.Context, _ []string) error {
			return cfg.Exec(ctx, nil)
		},
	}, cfg
}

func RegisterFlagsTink(cfg *TinkCfg, fs *flag.FlagSet) {
	fs.StringVar(&cfg.TFTPAddr, "tftp-addr", "0.0.0.0:69", "IP and port to listen on for TFTP.")
	fs.StringVar(&cfg.HTTPAddr, "http-addr", "0.0.0.0:8080", "IP and port to listen on for HTTP.")
	fs.StringVar(&cfg.LogLevel, "loglevel", "info", "log level (optional)")
	fs.StringVar(&cfg.Tink, "tink", "", "tink server URL (required)")
	description := "(file:///path/to/cert/tink.cert, http://tink-server:42114/cert, boolean (false - no TLS, true - tink has a cert from known CA) (optional)"
	fs.StringVar(&cfg.TLS, "tls", "false", "tink server TLS "+description)
}

func (t *TinkCfg) Exec(ctx context.Context, _ []string) error {
	t.Log = defaultLogger(t.LogLevel)
	t.Log.Info("starting ipxe", "tftp-addr", t.TFTPAddr, "http-addr", t.HTTPAddr)
	gc, err := tink.SetupClient(ctx, t.Log, t.TLS, t.Tink)
	if err != nil {
		return err
	}
	c := hardware.NewHardwareServiceClient(gc)
	tb := &tink.Tinkerbell{Client: c, Log: t.Log}
	tAddr, err := netaddr.ParseIPPort(t.TFTPAddr)
	if err != nil {
		return errors.Wrapf(err, "could not parse tftp-addr %q", t.TFTPAddr)
	}
	hAddr, err := netaddr.ParseIPPort(t.HTTPAddr)
	if err != nil {
		return errors.Wrapf(err, "could not parse http-addr %q", t.HTTPAddr)
	}
	cfg := ipxe.Config{
		TFTP: ipxe.TFTP{Addr: tAddr},
		HTTP: ipxe.HTTP{Addr: hAddr},
		Log:  t.Log,
	}

	return cfg.Serve(ctx, tb)
}
