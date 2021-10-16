package cmd

import (
	"context"
	"os"

	"github.com/jacobweinstock/ipxe/cli"
)

func Execute(ctx context.Context) error {
	root := cli.IpxeBin()
	return root.ParseAndRun(ctx, os.Args[1:])
}
