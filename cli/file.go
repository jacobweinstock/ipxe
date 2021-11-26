package cli

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/imdario/mergo"
	"github.com/jacobweinstock/ipxe"
	"github.com/jacobweinstock/ipxe/backend/file"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"github.com/tinkerbell/tink/protos/hardware"
	"inet.af/netaddr"
)

const fileCLI = "file"

// FileCfg is the configuration for the file backend.
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
	defaults := FileCfg{
		Config: Config{
			TFTPAddr: "0.0.0.0:69",
			HTTPAddr: "0.0.0.0:8080",
			LogLevel: "info",
			Log:      defaultLogger("info"),
		},
	}
	err := mergo.Merge(f, defaults)
	if err != nil {
		return err
	}
	if f.Log.GetSink() == nil {
		f.Log = defaultLogger(f.LogLevel)
	}
	f.Log = f.Log.WithName("ipxe")

	f.Log.Info("starting ipxe", "tftp-addr", f.TFTPAddr, "http-addr", f.HTTPAddr)

	saData, err := ioutil.ReadFile(f.Filename)
	if err != nil {
		return errors.Wrapf(err, "could not read file %q", f.Filename)
	}
	dsDB := []*hardware.Hardware{}
	if err := json.Unmarshal(saData, &dsDB); err != nil {
		return errors.Wrapf(err, "unable to parse configuration file %q", f.Filename)
	}

	fb := &file.File{DB: dsDB}
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
	return c.Serve(ctx, fb)
}
