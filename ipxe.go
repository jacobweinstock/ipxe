// Package ipxe implements the iPXE tftp and http server.
package ipxe

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/pin/tftp"
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

type logger logr.Logger

// Serve will listen and serve iPXE binaries over TFTP and HTTP.
// See binary/binary.go for the iPXE files that are served.
func (c Config) Serve(ctx context.Context) error {
	defaults := Config{
		TFTP:      TFTP{Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 69), Timeout: 5 * time.Second},
		HTTP:      HTTP{Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 8080), Timeout: 5 * time.Second},
		MACPrefix: true,
		Log:       logr.Discard(),
	}
	err := mergo.Merge(&c, defaults, mergo.WithTransformers(ipport{}), mergo.WithTransformers(logger{}))
	if err != nil {
		return err
	}

	t := &HandleTFTP{log: c.Log}
	st := tftp.NewServer(t.ReadHandler, t.WriteHandler)
	st.SetTimeout(c.TFTP.Timeout)
	g, ctx := errgroup.WithContext(ctx)
	var tftpServerErr error
	g.Go(func() error {
		c.Log.Info("serving TFTP", "addr", c.TFTP.Addr, "timeout", c.TFTP.Timeout)
		if err := ListenAndServeTFTP(ctx, c.TFTP.Addr, st); err != nil {
			tftpServerErr = fmt.Errorf("tftp serve error: %w", err)
			return tftpServerErr
		}
		return nil
	})

	router := http.NewServeMux()
	s := HandleHTTP{log: c.Log}
	router.HandleFunc("/", s.Handler)

	srv := &http.Server{
		Handler: router,
	}

	var httpServerErr error
	g.Go(func() error {
		c.Log.Info("serving HTTP", "addr", c.HTTP.Addr, "timeout", c.HTTP.Timeout)
		if err := ListenAndServeHTTP(ctx, c.HTTP.Addr, srv); err != nil {
			httpServerErr = fmt.Errorf("http serve error: %w", err)
			return httpServerErr
		}
		return nil
	})

	// errgroup.WithContext: The derived Context is canceled the first time a function
	// passed to Go returns a non-nil error or the first time Wait returns, whichever occurs first.
	<-ctx.Done()

	// Don't shutdown if the TFTP server failed, this will cause an immediate program exit without a stacktrace.
	if tftpServerErr == nil {
		st.Shutdown()
	}
	// Don't shutdown if the HTTP server failed, this will cause an immediate program exit without a stacktrace.
	if httpServerErr == nil {
		_ = srv.Shutdown(ctx)
	}
	c.Log.Info("shutting down", "msg", ctx.Err(), "tftp", tftpServerErr, "http", httpServerErr)

	return g.Wait()
}

func (l logger) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(logr.Logger{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("GetSink")
				result := isZero.Call([]reflect.Value{})
				if result[0].IsNil() {
					dst.Set(src)
				}
			}
			return nil
		}
	}
	return nil
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
