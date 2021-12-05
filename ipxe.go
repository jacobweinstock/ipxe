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
	g.Go(func() error {
		c.Log.Info("serving TFTP", "addr", c.TFTP.Addr, "timeout", c.TFTP.Timeout)
		if err := ListenAndServeTFTP(ctx, c.TFTP.Addr, st); err != nil {
			return fmt.Errorf("tftp serve error: %w", err)
		}
		return nil
	})

	router := http.NewServeMux()
	s := HandleHTTP{log: c.Log}
	router.HandleFunc("/", s.Handler)

	srv := &http.Server{
		Handler: router,
	}
	g.Go(func() error {
		c.Log.Info("serving HTTP", "addr", c.HTTP.Addr, "timeout", c.HTTP.Timeout)
		if err := ListenAndServeHTTP(ctx, c.HTTP.Addr, srv); err != nil {
			return fmt.Errorf("http serve error: %w", err)
		}
		return nil
	})

	errChan := make(chan error)
	go func() {
		errChan <- g.Wait()
	}()

	select {
	case <-ctx.Done():
		c.Log.Info("shutting down")
		st.Shutdown()
		err = srv.Shutdown(ctx)
	case e := <-errChan:
		err = e
	}

	return err
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
