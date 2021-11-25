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

type TFTP struct {
	Addr netaddr.IPPort
	// Timeout (in seconds) is the timeout for serving TFTP files.
	Timeout int
}

type HTTP struct {
	Addr netaddr.IPPort
	// Timeout (in seconds) is the timeout for serving HTTP files.
	Timeout int
}

// Serve will listen and serve iPXE binaries over TFTP and HTTP.
// See binary/binary.go for the iPXE files that are served.
func Serve(ctx context.Context, l logr.Logger, b backend.Reader, t TFTP, h HTTP) error {
	if l.GetSink() == nil {
		l = logr.Discard()
	}
	if t.Timeout == 0 {
		t.Timeout = 5
	}
	if h.Timeout == 0 {
		h.Timeout = 5
	}
	// TODO(jacobweinstock): Add validation of t.Addr and h.Addr.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return tftp.Serve(ctx, l, b, t.Addr, t.Timeout)
	})

	g.Go(func() error {
		return http.ListenAndServe(ctx, l, b, h.Addr, h.Timeout)
	})

	return g.Wait()
}
