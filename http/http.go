package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/jacobweinstock/ipxe/backend"
	"github.com/jacobweinstock/ipxe/binary"
	"inet.af/netaddr"
)

type server struct {
	backend backend.Reader
	log     logr.Logger
}

func ListenAndServe(ctx context.Context, l logr.Logger, b backend.Reader, addr netaddr.IPPort, _ int) error {
	router := http.NewServeMux()
	s := server{backend: b, log: l}
	l.V(0).Info("serving http", "addr", addr)
	for name := range binary.Files {
		router.HandleFunc(fmt.Sprintf("/%s", name), s.serveFile)
	}
	srv := http.Server{
		Addr:    addr.String(), // TODO(jacobweinstock): addr needs to be in host:port format
		Handler: router,
	}
	errChan := make(chan error)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			errChan <- err
		}
		errChan <- nil
	}()

	var err error
	select {
	case <-ctx.Done():
		err = srv.Shutdown(ctx)
	case e := <-errChan:
		err = e
	}
	return err
}

func (s server) serveFile(w http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		s.log.Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "could not get your IP address")
	}
	mac, err := net.ParseMAC(path.Base(host))
	if err != nil {
		s.log.Info("could not parse mac from request URI", "err", err.Error())
	}
	allowed, err := s.backend.Allowed(context.TODO(), net.ParseIP(host), mac)
	if err != nil {
		// TODO(jacobweinstock): connections errors should probably be 500 but not found errors should be 403
		http.Error(w, "error talking with backend", http.StatusInternalServerError)
		s.log.Error(err, "error talking with backend")
		return
	}
	if !allowed {
		http.Error(w, "not allowed", http.StatusForbidden)
		s.log.Error(fmt.Errorf("%s: not allowed", req.RemoteAddr), "reported as not allowed")
		return
	}
	got := filepath.Base(req.URL.Path)
	file := binary.Files[got]
	if _, err := w.Write(file); err != nil {
		s.log.Error(err, "error serving file")
	}
}
