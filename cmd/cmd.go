package cmd

import (
	"context"
	"os"

	"github.com/jacobweinstock/ipxe-bin/cli"
)

func Execute(ctx context.Context) error {
	root := cli.IpxeBin()
	return root.ParseAndRun(ctx, os.Args[1:])
}
