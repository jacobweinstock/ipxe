// Package ipxe implements the iPXE tftp and http server.
package ipxe

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/backend"
	"github.com/jacobweinstock/ipxe/http"
	"github.com/jacobweinstock/ipxe/tftp"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

// Serve will listen and server iPXE binaries over TFTP and HTTP.
// See binary/binary.go for the iPXE files that are served.
func Serve(ctx context.Context, l logr.Logger, b backend.Reader, tftpAddr, httpAddr netaddr.IPPort) error {
	if l.GetSink() == nil {
		l = logr.Discard()
	}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return tftp.Serve(ctx, l, b, tftpAddr)
	})

	g.Go(func() error {
		return http.ListenAndServe(ctx, l, b, httpAddr)
	})

	return g.Wait()
}
