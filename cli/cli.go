package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/peterbourgon/ff/v3/ffcli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const rootCLI = "ipxe-bin"

type config struct {
	TFTPAddr string
	HTTPAddr string
	LogLevel string
	Log      logr.Logger
}

func IpxeBin() *ffcli.Command {
	return &ffcli.Command{
		Name:       rootCLI,
		ShortUsage: rootCLI,
		Subcommands: []*ffcli.Command{
			tink(),
			file(),
		},
		Exec: func(ctx context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}
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
