// Package ipxe implements the iPXE tftp and http server.
package ipxe

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"golang.org/x/sync/errgroup"
	"inet.af/netaddr"
)

// Config holds the details for running the iPXE service.
type Config struct {
	// TFTP holds the details for the TFTP server.
	TFTP TFTP
	// HTTP holds the details for the HTTP server.
	HTTP HTTP
	// MACPrefix indicates whether to expect request URI's to be prefixed with MAC address or not
	MACPrefix bool
	// Log is the logger to use.
	Log logr.Logger
}

// TFTP is the configuration for the TFTP server.
type TFTP struct {
	// Addr is the address:port to listen on for TFTP requests.
	Addr netaddr.IPPort
	// Timeout is the timeout for serving TFTP files.
	Timeout time.Duration
}

// HTTP is the configuration for the HTTP server.
type HTTP struct {
	//  Addr is the address:port to listen on.
	Addr netaddr.IPPort
	// Timeout is the timeout for serving HTTP files.
	Timeout time.Duration
}

type ipport netaddr.IPPort

type Reader interface {
	Mac(context.Context, net.IP, net.HardwareAddr) (net.HardwareAddr, error) // seems to only be used for logging. might not need.
	Allowed(context.Context, net.IP, net.HardwareAddr) (bool, error)
}

// Transformer for merging netaddr.IPPort fields.
func (i ipport) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(netaddr.IPPort{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("IsZero")
				result := isZero.Call([]reflect.Value{})
				if result[0].Bool() {
					dst.Set(src)
				}
			}
			return nil
		}
	}
	return nil
}

// Serve will listen and serve iPXE binaries over TFTP and HTTP.
// See binary/binary.go for the iPXE files that are served.
func (c Config) Serve(ctx context.Context, b Reader) error {
	defaults := Config{
		TFTP:      TFTP{Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 69), Timeout: 5 * time.Second},
		HTTP:      HTTP{Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 8080), Timeout: 5 * time.Second},
		MACPrefix: true,
		Log:       logr.Discard(),
	}
	err := mergo.Merge(&c, defaults, mergo.WithTransformers(ipport{}))
	if err != nil {
		return err
	}
	if c.Log.GetSink() == nil {
		c.Log = logr.Discard()
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return fmt.Errorf("tftp error: %w", serveTFTP(ctx, c.Log, b, c.TFTP.Addr, c.TFTP.Timeout))
	})

	g.Go(func() error {
		return fmt.Errorf("http error: %w", ListenAndServe(ctx, c.Log, b, c.HTTP.Addr, c.HTTP.Timeout))
	})

	return g.Wait()
}
